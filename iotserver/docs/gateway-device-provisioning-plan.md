# 网关设备自动注册设计（MQTT 清单上报）

> **内部设计文档。** 面向网关厂商的接口规范在 `docs/gateway-integration.md`；本文第 3 节定稿后需要合并进那份规范（新增一个上行主题、修订第 5 节的「未注册设备」行为）。
>
> **行号基准：** 本文所有 `file:line` 引用核对于 2026-10-16 的工作副本。注意网关接入相关文件（`gatewayapp`、`gatewayingest`、`models/gateway.go`、`docs/gateway-integration.md`）**当时尚未提交**，`origin/master` 上并不存在——落实现时请以实际代码为准。

## 0. 目标

用户在「网关管理」填完网关信息、拿到接入参数；网关上线后用**一条 MQTT 报文带上自己的全部子设备**，平台自动完成：产品映射 → 设备入库 → 绑定网关 → 配额校验 → 下行回执。之后这些设备的上行数据即可正常落库，不需要人在平台上一台台手工建设备。

现状是反过来的：平台要求设备**先存在**，网关上报未知 `deviceKey` 会被整条拒绝（`gatewayingest.service.go:374`）。

## 1. 现状盘点

### 1.1 已经具备（不需要重做）

| 能力 | 位置 |
| --- | --- |
| 网关 CRUD + 一次性接入参数组装 | `application/gatewayapp/gatewayapp.go:75` `mqttInfo` |
| 上行订阅与分发（data/batch/status/offline） | `application/gatewayingest/service.go:144` `Start` |
| 幂等：`(gateway_id, seq)` 去重 | `gatewayingest/service.go:241` |
| 复用既有上行管道落库 + 推 eKuiper | `gatewayingest/service.go:351` `ThingModelMsgReport` |
| 批量报文字段带各自采集时间落库 | `persistence/thinkmodel_lake.go:95` `DATA_BATCH_REPORT` |
| 设备「收到数据即置在线」+ 节流 | `gatewayingest/service.go:103` `markDeviceOnline` |
| 下行通道（config / cmd） | `tools/gatewaymqtt/publisher.go` |
| 设备/网关配额检查 | `application/tenantquota/quota.go:26` |
| 真实走廊的造数工具 | `tools/telemgen/main.go` |

关键事实：**落库不校验物模型**。`saveDeviceThingModelData` 对批量报文直接把 `code` 插进 DuckDB（`thinkmodel_lake.go:101-113`），物模型只在**读**路径用于取单位/类型（`thinkmodel_lake.go:327` 起）。所以「设备自动注册」不必等物模型齐备，物模型可以异步补齐。

另一条关键约束：`resolveTenant` 要求设备**必须已存在于元数据库**才能确定时序行的租户（`thinkmodel_lake.go:45`）。因此设备入库确实是数据落库的前置条件，这个需求不能绕。

### 1.2 阻塞项（已核实；不修则功能不成立）

1. ~~**网关密钥从未生成。**~~ **（更正）** 原述有误：密钥是生成的，但用的是**非加密随机**。`gatewayApp.GatewayAdd`（`gatewayapp.go:88`）自身不设 `Secret`，但 DB 层 `addGateway`（`infrastructure/sqlite/gateway_repo.go:189`）会补：`if g.Secret == "" { g.Secret = utils.GenerateDeviceSecret(16) }`，且 `GatewayAdd` 返回的正是这个补过的对象，所以一次性密钥确实会回给调用方，`docs/gateway-integration.md` 第 1 节的承诺是成立的。
   **真正的问题是**：`GenerateDeviceSecret`（`internal/pkg/utils/uuid.go:28`）用的是 `math/rand`（`rand2 "math/rand"`）而非 `crypto/rand`，密钥可预测；同一个文件里的 `RandomNum()`（被用作**主键**）更是 `math/rand` + 时间种子 + 8 位数字（1e8 空间）。这一条**不阻塞本需求**，但批量自动注册会放大主键碰撞风险（见 5.2 节：自动注册的设备改用 `uuid.NewString()`）。

2. **未注册设备被硬拒绝。** `resolveDevice`（`gatewayingest/service.go:365`）两步查找失败即返回错误，`ingestRecord` 随即放弃该条。这正是「携带设备信息入库」要改掉的行为。

3. **没有任何 API 能绑定设备到网关。** `DeviceAddRequest`（`dtos/device.go:171`）没有 `gateway_id` / `device_key` 字段；`gateway_id`、`device_key` 在全仓库只出现在 `gatewayingest`、`device_lookup.go`、以及 models 里。也就是说当前 **UI 上无法把设备绑到网关，也无处填 `deviceKey`**。`telemgen` 能跑通纯属侥幸——它把 `device_sn` 当成 `deviceKey` 用，命中的是 `resolveDevice` 里的 `DeviceBySn` 兜底。

4. **`GatewayDelete` 不解绑设备，且注释与实现不符。** `gatewayapp.go:133` 的注释写「其下设备保留但解除绑定」，但 `deleteGateway`（`infrastructure/sqlite/gateway_repo.go:115`）只有一条 `Delete`，没有任何 `UPDATE device SET gateway_id=''`。删掉网关后其设备带着**悬挂的 `gateway_id`**；若新网关复用了同一个 SN，`GatewayBySn` 会返回新 `id`，这些设备再也匹配不上。

5. **`(gateway_id, device_key)` 没有唯一约束。** `models/device.go:34,36` 上两者各自只有普通 `index`。自动注册由网关侧驱动、可能重连后整批重发，没有唯一键就会出现重复设备行。

6. **`gateway_ingest_log` 没有 `kind` 列**（`models/gateway.go:55`）。新增 `devices` 主题后，清单报文与数据报文的日志会混在一起，无法区分。

7. **Broker 侧鉴权未闭环。** 平台连的是外部 Broker（`docs/telemetry-test-env.md`：Docker `58090`），Go 服务只是客户端，`username=SN / password=Secret` 目前仅是一条**约定**。仓库里有一个通用凭据表 `mqtt_auth`（`ClientId` 唯一索引 + `ResourceId`/`ResourceType`，`models/mqttauth.go:22`）和设备侧的 `AddMqttAuth` 接口，但**网关的凭据没有写进这张表**，而是打算塞进 `gateway.secret` 单列；也未见供 Broker 回调的免登录鉴权端点。

8. **`DeviceBySn` 兜底会跨网关串数据。** `resolveDevice` 第二步按 `device_sn` 查找时**不带网关作用域**（`device_lookup.go:44`）。两个网关各自有 `M-001` 时，谁先查到就归谁。自动注册铺开后这个问题会被放大。

## 2. 端到端流程

```
用户                平台(iotserver-core)              网关
 |  填网关信息 ──────> POST /api/v1/gateway
 |  <──── 一次性接入参数(broker/clientId/username/password)
 |                                            配置 Broker 连接 + LWT
 |                                    连接 ──> iot/up/{sn}/status
 |                          <── 平台置网关在线
 |                                    上报全量清单 ──> iot/up/{sn}/devices
 |                          <── 平台按策略注册/绑设备、过配额、映射产品
 |  <── iot/down/{sn}/devices/ack（accepted/duplicated/rejected + 原因）
 |  在「设备管理」看到网关下自动出现的设备
 |                                    正常上报 ──> iot/up/{sn}/data|batch
 |                          <── 落库（设备已存在，租户可解析）
```

## 3. 协议设计

### 3.1 新上行主题 `iot/up/{sn}/devices`

```json
{
  "seq": 1,
  "ts": 1735689600000,
  "mode": "full",
  "devices": [
    {
      "deviceKey": "M-001",
      "productKey": "inj_01",
      "name": "1号注塑机",
      "deviceSn": "SN-0001",
      "model": "CX-100",
      "fw": "1.2.3",
      "properties": [
        { "code": "temp", "name": "温度", "type": "float", "unit": "℃" }
      ]
    }
  ]
}
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `seq` | 建议 | 与 data 同语义，参与幂等去重 |
| `mode` | 必填 | `full` 全量声明 / `delta` 增量 |
| `devices[].deviceKey` | 必填 | 网关内唯一标识，落 `device.device_key` |
| `devices[].productKey` | 必填 | 映射 `product.key`，决定设备归属哪个产品 |
| `devices[].name` | 可选 | 平台展示名；缺省用 `<网关名>-<deviceKey>` |
| `devices[].deviceSn` | 可选 | 落 `device.device_sn`；缺省同 `deviceKey` |
| `devices[].properties[]` | 可选 | 测点自描述，用于自动补物模型（非落库前置条件） |

**`mode` 语义**

- `delta`：只做 upsert，不动清单之外的设备。
- `full`：清单是权威全集。**清单里没有的设备标记为「已摘除」**（新增 `device.provision_state`，或 `status` 之外单独字段），**不删除设备行、不删历史数据**——删了会连带影响时序数据和告警规则引用。
- 建议网关侧只在上线首次、以及子设备增减时发，不要周期性发全量。

### 3.2 下行回执 `iot/down/{sn}/devices/ack`

```json
{
  "seq": 1,
  "mode": "full",
  "accepted": ["M-001"],
  "duplicated": ["M-002"],
  "rejected": [{ "deviceKey": "M-003", "reason": "quota_exceeded" }],
  "pending": ["M-004"]
}
```

这一步不做则网关只能盲发——它无从知道设备有没有注册成功。`reason` 建议用固定枚举：`quota_exceeded` / `product_unknown` / `invalid_key` / `policy_pending`。

### 3.3 幂等

沿用现有机制，但**必须能与 data 的 seq 区分**：给 `gateway_ingest_log` 加 `kind` 列（`data` / `batch` / `devices`），去重键从 `(gateway_id, seq)` 变成 `(gateway_id, kind, seq)`。否则网关侧 data 和 devices 各自从 1 开始编号时，第二个报文会被误判为重复而丢弃——这是很容易踩的坑。

## 4. 注册策略（核心决策点，需要你拍板）

每个网关一个 `provision_policy`：

| 策略 | 行为 | 适用 |
| --- | --- | --- |
| `auto` | 直接建设备并绑定；`productKey` 未命中时自动建产品 | 自有/可信网关，追求零配置接入 |
| `pending` | 写入「待确认设备」表，前端提示，管理员确认后转正 | **建议默认** |
| `reject` | 只记日志，等价于现状 | 严格管控场景 |

**建议默认 `pending`**，理由是：SN + 密钥能连上 Broker，只证明「这是我们的网关」，不等于「管理员授权这些设备占用租户配额」。`tenantquota.Devices` 是真实存在的配额（`tenantquota/quota.go:26`），`auto` 等于把配额的决策权交给了网关固件。`pending` 顺带给了「给设备改名、归到哪个组织节点」的人工确认机会——自动注册出来的设备名通常是厂商内部的点位号，直接入库对使用者并不友好。

无论哪种策略：

- `auto` **也必须**走 `tenantquota.Ensure(ctx, db, lc, tenantquota.Devices)`（参照 `gatewayapp.go:91` 的用法）；超配额时该设备落到 `pending` 并回执 `quota_exceeded`，**不要**整体失败——一个网关可能报 200 台，不该因为第 201 台而全部拒收。
- 尊重网关的 `DeptId`，作为新建设备的默认组织归属。

## 5. 落库设计

### 5.1 upsert 键与唯一索引

在 `device` 表加**部分唯一索引** `(tenant_id, gateway_id, device_key)`（`gateway_id != ''`）。这是整套设计里唯一一处必须动 schema 的硬要求：没有它，网关断线重连整批重发就会产生重复设备。

upsert 语义：按 `(gateway_id, device_key)` 命中 → 更新可变的描述字段（name/deviceSn/model）；未命中 → 插入。全程在**一个元数据库事务**内完成，避免并发重发时两次都查空、两次都插入。

### 5.2 字段映射

| `device` 列 | 来源 |
| --- | --- |
| `id` | 复用 `AddDevice` 的主键生成逻辑（`deviceapp.go:129`） |
| `gateway_id` | 当前网关 `id` |
| `device_key` | 清单 `deviceKey` |
| `name` | 清单 `name` > `<网关名>-<deviceKey>` |
| `device_sn` | 清单 `deviceSn` > `deviceKey` |
| `dept_id` | 清单 `deptId` > 网关 `dept_id` |
| `product_id` | `product.key == productKey` 查得 |
| `platform` | `LocalIot`（与 `telemgen` 一致，`tools/telemgen/main.go:204`） |
| `status` | 初始离线；首帧数据到达时由既有 `markDeviceOnline` 置在线 |

### 5.3 产品与物模型

- `productKey` → `product.key` 映射（`models/product.go:27` 的 `Key` 就是「产品标识」）。
- 未命中且策略允许自动建产品时：建 `Product{Key: productKey, Name: productKey, Platform: LocalIot, Protocol: MQTT, NodeType: direct}`。
- 物模型：从清单 `properties[]` 与**首帧 data 的测点码**自发现补齐。因为落库不校验物模型（见 1.1），这一步是**异步补齐**，不阻塞入库。建议按「发现新 code 就补一条 property 定义（type 由首个值的 JSON 类型推断，unit 留空待人工补）」实现，并把补齐动作记一条日志，别静默写元数据。

### 5.4 乱序兜底：惰性注册（关键）

MQTT 只保证 QoS1 的至少一次投递，**不保证 `devices` 一定先于 `data` 到达**（网关重连、清单包丢失、网关固件实现顺序不同都会发生）。若只靠清单注册，就会出现「清单没到 → 数据整条被丢」的静默数据丢失。

因此 `handleData` 的 `resolveDevice` 失败分支要改成**惰性注册**：策略允许时，用报文里已有的 `deviceKey` / `productKey`（可选 `deviceName`）现场注册并绑定，然后立刻落库；策略为 `pending` 时，把数据缓存到一个短期待处理队列（或直接丢弃并计入 `pending`），等管理员确认后不补历史（明确取舍：宁可丢待确认设备的数据，也不要静默把未授权设备入库）。

这条改动直接对应你要的「携带设备信息入库」：即使清单报文丢了，**每条数据报文本身也带够了注册所需的最小信息**。

### 5.5 失败语义

- 元数据库事务失败 → 该设备计入回执 `rejected`，其余设备继续处理（逐台隔离，不要整包失败）。
- 设备注册成功但时序落库失败 → 设备已存在、无数据。可接受：网关按 QoS1 重传同一 `seq` 时，幂等日志里那条是失败状态，重传会重新尝试。

## 6. 代码落点

| 改动 | 文件 |
| --- | --- |
| 订阅 `iot/up/{sn}/devices`、`HandleMessage` 加 `case "devices"` | `application/gatewayingest/service.go:144,187` |
| 新增 `GatewayDeviceInventoryMessage` / `InventoryDevice` 结构 | 同上（与 `GatewayUplinkMessage` 并列） |
| `resolveDevice` → `resolveOrProvision`（含惰性注册） | `gatewayingest/service.go:365` |
| 注册编排（策略 / 配额 / 产品映射 / upsert / 回执） | 新包 `application/gatewayprovision`（独立于 `deviceapp`：这里是网关驱动的批量内部流程，不走 HTTP 请求语义） |
| `GatewayItf` 加清单处理与待确认设备相关方法 | `core/interface/gateway.go:10` |
| `UpsertGatewayDevice` / `MarkGatewayDevicesAbsent` / 待确认设备 CRUD | `infrastructure/sqlite/gateway_repo.go`（+ mysql 侧同步） |
| `Gateway` 加 `ProvisionPolicy`；`Device` 加 `ProvisionState` + 唯一索引；`GatewayIngestLog` 加 `Kind` | `models/gateway.go`、`models/device.go` |
| 清单报文、回执、策略、待确认设备 DTO | `dtos/gateway.go` |
| 策略设置 / 待确认列表 / 确认接口 | `controller/http/gateway/gateway.go` + `core/route/gateway.go`（`v1Auth` 组，参照现有 `ctl.Perm(...)` 权限写法） |
| 设备新增/编辑支持 `gateway_id` + `device_key`（**当前完全没有**） | `dtos/device.go:171`、`application/deviceapp/deviceapp.go`、前端 `pages/gateway/DeviceManage` |
| 新增主题常量、ack 主题 | `tools/gatewaymqtt/publisher.go:53` |
| 建表/索引迁移 | `core/bootstrap/database`（注意 commit `67f9177` 修过「全新安装缺业务表」，新表要一并补） |
| 厂商规范加 3.5 节、改第 5 节 | `docs/gateway-integration.md` |
| 前端：策略下拉、待确认设备页签、绑定时显示 deviceKey | `iotserver-ui/src/pages/gateway/GatewayManage`、`api/gateway.ts` |

## 7. 需先修的既有缺陷

这两条不属于新功能，但不修则新功能是残缺的：

1. **密钥与主键的随机源**（更正后的问题，见 1.2 节第 1 条）。`GenerateDeviceSecret` 与 `RandomNum` 都基于 `math/rand`：前者生成 MQTT 接入密钥，后者被当作主键。建议改为 `crypto/rand` / `uuid`。设计取舍：`mqtt_auth` 表（`models/mqttauth.go`）已经是通用 MQTT 凭据表且有唯一索引，**建议网关凭据也落到这张表**（`ResourceType=gateway`, `ResourceId=gateway.id`），与设备侧统一，而不是继续用 `gateway.secret` 单列。存哈希还是明文取决于 `mqtt_auth` 现状——它目前是明文列，先保持一致、后续统一加哈希，别只给网关做加密造成两套语义。

2. **`GatewayDelete` 真正处理设备绑定。** 建议改为**软删除网关**（保留行 + `deleted_at`），并显式决定其下设备是解绑还是级联摘除。硬删 + SN 复用会让历史数据的归属彻底错乱。

3. **收紧 `DeviceBySn` 兜底**（`device_lookup.go:44`）：加租户作用域；仅当匹配**唯一**时才接受，否则报「设备标识有歧义」而不是猜一个。自动注册铺开后这条兜底会变成跨网关串数据的入口。

## 8. 测试与验收

单测（扩充 `gatewayingest/service_test.go`，现有 414 行已覆盖上行基础路径）：

- 清单全量注册 / 增量 upsert / 重复 `seq` 幂等
- `mode=full` 时缺失设备被标记摘除、数据未删
- `productKey` 未命中（策略 auto 建产品 / 策略 reject 回执 `product_unknown`）
- 超配额：第 201 台落 `pending` 且其余正常入库
- **乱序**：先 data 后 devices，数据不丢（惰性注册生效）
- 并发重发同一清单不产生重复设备（唯一索引生效）

端到端：给 `tools/telemgen` 加 `-send-inventory` 开关，走真实走廊发清单，再验证 `/api/v1/devices` 里出现设备、历史查询有数据。

## 9. 分期

| 阶段 | 内容 | 产出 |
| --- | --- | --- |
| **P0** | Secret 生成 + `devices` 主题 + `auto` 策略（含配额）+ upsert 唯一索引 + ack 回执 | 端到端可用 |
| **P1** | `pending` 策略 + 前端确认页 + 惰性注册 + `GatewayDelete`/`DeviceBySn` 两个缺陷 | 可交付生产 |
| **P2** | 物模型自发现、`full` 模式的摘除语义、Broker 鉴权闭环（`mqtt_auth` 落地到 EMQX） | 运维完善 |

## 10. 待决策

1. **默认策略取 `auto` 还是 `pending`？** 本文推荐 `pending`；若这是自有网关、且希望"填完就能用"的体验优先，则 `auto`，但必须配配额兜底。
2. **`full` 模式下清单外的设备怎么处理？** 本文推荐标记摘除而非删除；若产品语义里"网关清单即权威、删了就是没了"，可以做成配置项。
3. **网关凭据是否统一进 `mqtt_auth`？** 本文推荐统一，但那会让本次改动多碰一块共享表。
4. **网关是否需要软删除？** 取决于你们对 SN 复用和历史数据归属的要求。
5. **对齐参考系统还是保留本文方案？** 见第 11 节的对照表，尤其「匹配键」和「逐点时间」两行——它们决定上报协议要不要改、网关固件要不要跟着动。

## 11. 与参考系统（iotddc / things）的对照（含实测）

参考项目 `iotddc`（Java）的 `/edge/...` 协议是一套跑在生产上的同类实现，本次抓到了它的一台在线网关的真实报文，因此这部分不是推断。

> 为避免把生产接入信息写进仓库，本节样例与地址一律用占位符（实测原始报文只在对话里）。

### 11.1 主题地图

| 方向 | 主题 | 说明 |
| --- | --- | --- |
| 上行 | `/edge/connect/{pk}/{sn}` | **网关接入**：`ip`/`model`/`gwType` + **`tags[]`（设备清单）** + 各协议版本 |
| 上行 | `/edge/property/{pk}/{sn}/post` | 实时数据 |
| 上行 | `/edge/property/history/{pk}/{sn}/post` | 历史补传（topic 含 `history` 即历史） |
| 上行 | `/edge/property/{pk}/{sn}/set` \| `/get`（+ `/reply`） | 读写属性 |
| 上行 | `/edge/lastwill/{pk}/{sn}` | 遗嘱 → 离线 |
| 上行 | `/edge/4g/{pk}/{sn}/post` | 4G 通道数据 |
| 下行 | `/edge/db/{key}/{sn}/update`（+ `/reply`） | 工程/组态下发（`url` + `crc`） |
| 下行 | `/edge/upgrade/{pk}/{sn}/notify`（+ `/reply`） | OTA |
| 下行 | `/edge/realInquire` \| `scadaHistory` \| `scadaGetTagsTree` \| `alarm` `/{sn}/{sn}/get`（+ `/reply`） | 组态/告警查询 |

平台侧一律用 EMQX 共享订阅 `$share/group/...`（`GatewayConnectProcessor`、`PropertyPostProcessor` 等都是）。

### 11.2 实测数据报文格式

`/edge/property/260112xxx/260112xxx/post`，QoS 0，**上报周期整 60 秒**：

```json
[{"dn":"THmeter",
  "properties":{"AirRoomTemp1":"30.0000","CoilHumidity":"18.0000","Energy":"139164.0000",
                "DRoom Temp":"0.0000","Spare3":"-19108","Switch1":"0"},
  "time":1792122130},
 {"dn":"system","properties":{"TIME_MINUTE":"42","TIME_SECOND":"10"},"time":1792122130}]
```

| 事实 | 影响 |
| --- | --- |
| payload 是 **JSON 数组** | 一个报文可带多台设备；数组每项一台 |
| `dn` = 设备标识，平台按 **`Device.name`** 匹配 | 与本文的 `deviceKey` 不同（见 11.4） |
| `properties` 是**扁平字符串 map** | 值全是字符串（`"30.0000"`/`"0"`/`"-19108"`），落库时 `new BigDecimal()` 转换，转不动则按字符串存 |
| `time` 是 **Unix 秒**，整包一个 | 无逐测点时间 |
| 测点名**可含空格**（实测 `DRoom Temp`） | 解析与白名单不能假设无空格 |
| `Energy` 是累计量 | 60 秒涨 46，不是瞬时值 |

### 11.3 鉴权（EMQX exhook，`exhook/service/HookService.java`）

- username 形如 `{pk}|{sn}`；`THINGSIOTWEBMQTT|` 前缀是 Web 端。
- gateway 侧 password：`HMAC-SHA256(secret, "clientId{sub}devicename{sn}productkey{pk}timestamp{ts}")`，小写 hex（`MD5Utils.hmacsha256`）；clientId 必须匹配 `^(.+)\|securemode=[2-3],signmethod=(\w+),timestamp=(\d+)\|$`。
- 另一路 `userAuth`：`HMAC-MD5(key="12345", msg=password)` 与 Redis `mosquitto` 哈希比对（`hmacMD5` 的 key 是常量盐，不是空）。
- ACL（`onClientAuthorize`）：`{pk}|{sn}` 只能读写 `/things/{username}` 前缀；非 SN 用户名则把 **username 当**主题正则**匹配。**
- 这与本文第 7 节「网关凭据统一进 `mqtt_auth`」的判断一致：参考系统也是**独立凭据源 + Broker 侧回调校验**，不是把密码塞在网关表里。

### 11.4 与本文方案的差异（需要拍板）

| 维度 | 参考系统 | 本文方案 | 取舍要点 |
| --- | --- | --- | --- |
| 数据报文 | 扁平数组、整包一个 `time`、值全字符串 | `properties[code]={value,time}` 逐点带时间 | 逐点时间对**断点续传补传**更准确，但要求网关固件支持 |
| 设备匹配键 | `dn` ↔ `Device.name` | `deviceKey` ↔ `device.key` | 名字会被人改，键更稳；参考系统用名字，因为它的设备表没有独立键 |
| 清单外设备 | **物理删除**（`delete from build_device`） | 标记摘除、保留历史 | 参考系统是硬删；元数据删了时序历史仍在，但**告警规则引用会断** |
| 测点白名单 | 有：只有登记过的 `DeviceProperty` 才落库（`{gwSn}.{dn}.{tag}`） | 无：报什么存什么 | 白名单是参考系统数据质量好的原因，代价是**新增测点必须先同步，否则静默丢数** |
| 清单粒度 | 设备 + 测点（`tags[]` 同时含两者） | 只有设备 | 参考系统是**设备级测点表**，本平台物模型是**产品级**，粒度不同，不能直接照搬 |
| 事务 | 增/改/删比对在**一个 `Db.tx`** 内批量提交 | 同（本文 5.1 要求） | 一致 |

`/edge/connect` 报文里 `tags[]` 的字段全集（`DeviceServiceImpl.compareDevice`/`compareTags`）：`deviceCode`、`deviceDescription`、`tagCode`、`description`、`objectType`。`objectType` 按结尾字母推读写权限：`I`→`r`（AI/DI）、`O`→`w`（AO/DO）、`V`→`rw`、其他→`r`，缺省 `AI`。

**未能实测的部分**：`/edge/connect` 与 `/edge/lastwill` 只在网关启动/掉线时各发一次，本次连续观测 7 分钟（5 条数据报文，60 秒周期稳定）期间该网关一直在线，两者均未出现。上述 connect 报文结构来自参考代码，**未经线上报文验证**。要拿到真实样本，需在平台上对网关下发一次 `sync` 或重启。