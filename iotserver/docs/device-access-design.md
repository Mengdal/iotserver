# 设备接入设计（网关子设备 / 驱动镜像 两条路径）

> 内部设计文档。上游背景见 `docs/gateway-device-provisioning-plan.md`（网关自动注册方案）与 `docs/gateway-integration.md`（面向厂商的自有协议规范）。
> 行号基准：2026-10-16 工作副本。`/edge` 协议格式来自对参考系统在线网关的**实测**（见 `gateway-device-provisioning-plan.md` 第 11 节）。

## 0. 结论先行

两条接入路径，**产出同一种设备元数据**，上层（时序存储、看板、组态、告警）无差别消费：

| | 路径 A：网关子设备接入 | 路径 B：驱动镜像接入 |
| --- | --- | --- |
| 用到它的场景 | 第三方工业网关 / 边缘网关，**存量设备已按 iotddc `/edge` 协议上报** | 用户自带的设备、自带的协议栈 |
| 谁代表一批设备 | 一个 MQTT 连接（网关）代表多台子设备 | 一个驱动镜像实例代表一个外部平台/协议族 |
| 设备怎么来 | **随接入报文自带清单**（`/edge/connect` 的 `tags[]`），平台自动注册 | 驱动上报 + 平台显式创建 |
| 物模型 | **允许没有，直接接入**（本设计的核心诉求） | 有产品与物模型 |
| 平台既有支撑 | 需新建（本设计） | 已有：`CloudService` / `CloudInstance` / `dmi/docker` / `driverserviceapp` |

关键判断：**路径 B 不需要新建**——平台已有 `CloudService`（`constants/cloudservice.go`：参考系统/阿里/华为/腾讯/涂鸦/OneNET）、`CloudInstance`（docker 驱动实例，带 `authorization_config`/`extend_template`，`constants.CloudInstance = "cloudInstanceService"`）、`application/dmi/docker`、`driverserviceapp`、`controller/rpcserver/driverserver`，设备与产品上已有 `Platform`/`CloudProductId`/`CloudInstanceId`。本次工作集中在**路径 A + 两条路径的统一层**。

> 注：`IotPlatform_LuomiIot`（`internal/pkg/constants/product.go:27`）的取值即 iotddc/things 平台。因此接入它**不必新增枚举值**，直接复用。

## 1. 路径 A：网关子设备接入（兼容 `/edge` 协议）

### 1.1 主题与订阅

```
上行  /edge/connect/{pk}/{sn}                网关接入 + tags[] 设备清单 → 自动注册
      /edge/property/{pk}/{sn}/post          实时数据
      /edge/property/history/{pk}/{sn}/post  历史补传
      /edge/lastwill/{pk}/{sn}               遗嘱 → 离线
下行  /edge/db/{key}/{sn}/update (+/reply)   工程/组态下发   ← 可选，见待决策 3
      /edge/upgrade/{pk}/{sn}/notify (+/reply) OTA            ← 可选
```

**解析陷阱**：`/edge/property/history/{pk}/{sn}/post` 比实时主题**多一段**。段数不同不能靠固定下标，必须先判 `history` 前缀再取 `pk`/`sn`。参考系统是靠 `message.getTopic().contains(Constants.HISTORY)` 区分的（`PropertyPostProcessor.java:111`）。

**连哪个 broker**：参考系统的上行数据在**另一个 broker** 上，与平台内部的消息总线（`MessageQueue`，`PublishTopicPrefix = eventbus/in`）不是同一个。因此有独立配置段 `[EdgeIngest]`（`Broker` / `Username` / `Password`），`Broker` 留空则回落到 `MessageQueue`。**不能**直接把 `MessageQueue.Host` 改过去——那会把平台的内部总线一起搬到对方去，规则引擎/事件链路会断。

**订阅粒度：按网关逐个拼主题，绝不用通配。** 平台只订阅「网关管理里登记过的」网关：

```
/edge/connect/{pk}/{sn}
/edge/property/{pk}/{sn}/post
/edge/property/history/{pk}/{sn}/post
/edge/lastwill/{pk}/{sn}
```

`{pk}` 取 `gateway.pk`，为空则用 `sn` 顶替（实测两者取值相同；**若对方的 pk 与 sn 不一致，必须在网关里把 pk 填上**，否则主题拼不对、订阅不到数据）。两条理由：

1. **数据边界**：参考系统的 broker 上挂着**别的客户**的网关，而该 broker 实测**允许匿名订阅**——用 `/edge/property/+/+/post` 这种通配，等于把别人家的生产数据一起拉进本平台。
2. **开销**：网关一多，通配订阅的流量与解析成本不可控，而这些数据平台根本用不上。

订阅集合由 `syncSubscriptions` 按网关表**增量对齐**（默认每 60 秒，配置 `[EdgeIngest].ResyncIntervalSec`）：新网关自动订阅、已删网关自动退订，**界面上增删网关无需重启服务**。MQTT 客户端内部维护「应当订阅的主题集合」，因此**断线重连后动态订阅不会丢**。

### 1.2 身份映射

| iotddc | 本平台 | 说明 |
| --- | --- | --- |
| `{pk}` / `{sn}` | `Gateway.Pk` / `Gateway.Sn` | `Gateway` 已有 `Pk` 字段（新增/修改网关接口可传 `pk`，列表与详情也会返回）。实测 pk 与 sn **取值相同**（都是 260112801），但协议上是两段，不能假设相等——它决定订阅主题能否拼对 |
| `dn` | `Device.DeviceKey` | 实测取值 `THmeter` / `system` / `user` |
| `tagCode` | 设备级测点的 `code` | 见 1.4 |

**一处要刻意偏离参考系统**：它按**设备名字**匹配（`d.getName().equals(dn)`，`PropertyPostProcessor.java:220`），改个名字即断链。本平台应把 `dn` 落进 `device_key` 作**匹配键**，`name` 只作展示——与 `gateway-device-provisioning-plan.md` 第 5.1 节一致。

### 1.3 实测数据报文与映射

```json
[{"dn":"THmeter","properties":{"AirRoomTemp1":"30.0000","DRoom Temp":"0.0000","Spare3":"-19108"},"time":1792122130},
 {"dn":"system","properties":{"TIME_MINUTE":"42","TIME_SECOND":"10"},"time":1792122130}]
```

| 源 | 处理 |
| --- | --- |
| JSON **数组** | 每项展开成一台设备的一条记录（现有上行管道是 per-device 的，正好逐个投递） |
| `properties` 扁平 map，**值全是字符串** | 保留原串；落库时尝试数值转换（Go 用 `strconv.ParseFloat`），失败则按字符串存 —— 与参考系统 `new BigDecimal()` → catch → 存字符串的语义一致 |
| `time` **Unix 秒**，整包一个 | ×1000 转毫秒；逐测点时间 = 整包时间（源数据无逐点时间） |
| QoS | 参考系统订阅用 QoS **0** |
| 测点名**可含空格** | 实测 `DRoom Temp`。解析/展示/告警规则都不能假设无空格 |
| `dn` = `system` / `user` | 网关自身时钟/占位设备，建议**可配置排除名单**，避免污染设备列表 |

### 1.4 设备清单：无物模型接入的关键

`/edge/connect/{pk}/{sn}` 报文里 `tags[]` 的字段全集（实测 + `DeviceServiceImpl.compareDevice/compareTags`）：

| 字段 | 落到 |
| --- | --- |
| `deviceCode` | 设备标识（→ `device_key`） |
| `deviceDescription` | 设备描述 |
| `tagCode` | 测点 code |
| `description` | 测点描述 |
| `objectType` | 推 `access_mode`：`I`→`r`（AI/DI）、`O`→`w`（AO/DO）、`V`→`rw`、其他→`r`；缺省 `AI` |

注册语义：**在一个元数据库事务内**做增/改/删比对（对齐参考系统的 `Db.tx` 语义）——新增缺失设备与测点、更新变化的、处理清单外的。

**清单外设备怎么处理是本设计最需要拍板的一点**：参考系统是**物理删除**（`delete from build_device where id in (...)`）。建议默认改为**标记摘除**（`device.provision_state = 'absent'`），因为时序历史在 DuckDB、元数据删了历史仍在，但**告警规则/组态的引用会断**。若产品语义要求"清单即权威、没了就是没了"，做成网关级配置项。

### 1.5 「允许没有物模型直接接入」为什么能成立，以及缺什么

**写入侧不需要物模型** —— 已核实：批量上报落库 `thinkmodel_lake.go:101-113` 直接把 `code` 写进 DuckDB，不做任何物模型校验。这是需求能成立的根本原因。

**缺的是读取侧** —— `propertyUnit` 从产品物模型取单位/类型（`thinkmodel_lake.go:327`）。设备无产品时取不到单位、推不出类型。

**因此需要一个设备级测点表**：

```
device_tag: device_id, code, name, type, unit, access_mode, source(auto|manual), discovered_at
```

- 为什么不建"隐式产品"：iotddc 报文里**没有产品维度**（`/edge/property` 只有 `dn`+`properties`，`/edge/connect` 的 tags 也没有 productKey），按产品归类无从下手；而本平台物模型是**产品级**的（`Product.Properties`/`Events`/`Actions`，`models/product.go:40-42`）。设备级测点表是唯一贴合源数据粒度的选择。
- 读路径统一成 resolver：**产品物模型（有产品时）> 设备级测点表（无产品时）**，上层调用方不需要知道设备有没有物模型。
- 类型推断：按首个值的形态（`"30.0000"`/`"-19108"`/`"42"` → 数值，其他 → 字符串）；**单位留空待人工补**。
- 推断结果必须**可见**：前端标记「自动发现 · 未校准」，不要静默写元数据。参考系统的做法更硬——它只落库已登记为 `DeviceProperty` 的测点，未登记的直接丢弃（`PropertyPostProcessor.java:204`），代价是新增测点必须先同步否则**静默丢数**。我们不做白名单丢弃。

### 1.6 幂等（与自有协议的关键差异）

**实测 iotddc 报文里没有 `seq` 字段**。所以 `gateway-device-provisioning-plan.md` 里那套 `(gateway_id, kind, seq)` 去重**不适用**于兼容路径。

- 数据去重键：`(gateway_id, device_key, ts)` —— 同一设备同一时间戳重复到达即重复。实测整包一个 `time`、周期 60 秒，这个粒度足够。
- `/edge/connect` 天然幂等：清单是全量比对，重复到达只是重复比对一次，无害。
- 保留 `seq` 机制给**新协议**（新装的网关走我们自己的 `iot/up/*` 时用）。

## 2. 路径 B：驱动镜像接入（现状，无需新建）

用户部署驱动镜像实例 → 授权 → 自己的设备经驱动接入。既有组件：

| 组件 | 位置 |
| --- | --- |
| 云服务名称枚举 | `internal/pkg/constants/cloudservice.go`（含参考系统平台枚举） |
| 驱动/云实例管理 | `application/dmi/docker/{app,driver,manager}.go`、`application/driverserviceapp/` |
| 驱动侧 RPC | `controller/rpcserver/driverserver/` |
| 实例类型 | `constants.CloudInstance = "cloudInstanceService"`（`internal/pkg/constants/common.go:22`） |
| 前端 | `iotserver-ui/src/api/cloud-service.ts`、`cloud-instance.ts`（含 `authorization_config`/`extend_template`/`log_switch`） |

这条路径**设备是显式创建的、带产品与物模型**，与路径 A 互补。本文不改动它，只在统一层（第 3 节）把它和路径 A 拉齐。

## 3. 统一层：设备 + 测点

### 3.1 `Device` 建议新增的字段

| 字段 | 用途 |
| --- | --- |
| `gateway_id`、`device_key` | **已存在**（`models/device.go:34,36`），但**目前没有任何 API 能写**（见 `gateway-device-provisioning-plan.md` 1.2 节第 3 条），路径 A 需要它 |
| `provision_source` | `gateway` / `driver` / `manual`，区分设备从哪条路径来 |
| `provision_state` | `active` / `absent`（清单摘除）/ `pending`（待确认） |
| 唯一索引 | `(tenant_id, gateway_id, device_key)` 部分唯一（`gateway_id != ''`）——否则重连重发产生重复设备 |

### 3.2 `Platform` 取值

路径 A 接入 iotddc 时**直接复用 `IotPlatform_LuomiIot`**，不要复用 `LocalIot`（本地），否则从设备上看不出它来自外部云、也无法与路径 B 的云实例对应。

### 3.3 上层无差别消费

时序写入、看板、组态、告警只依赖「设备 + 测点定义 + 时序数据」三样。只要 3.1 的字段齐备、3.2 的单位/类型能通过 resolver 取到，两条路径对上层就是同一件事。

## 4. 代码落点

| 改动 | 文件 |
| --- | --- |
| 订阅与解析 `/edge/...`（新 `ParseEdgeTopic`，注意 history 多一段） | `application/gatewayingest/service.go`（`Start`/`HandleMessage`/`ParseTopic`） |
| 清单比对注册（设备 + 测点，事务内增改删） | 新包 `application/edgeingest`，或扩展 `gatewayapp` |
| `Gateway` 加 `Pk`；唯一键 (tenant, pk, sn) | `models/gateway.go` |
| `Device` 加 `provision_source`/`provision_state` + 唯一索引；新增 `device_tag` | `models/device.go`、新 `models/device_tag.go` |
| 读路径测点回落（产品物模型 → 设备级测点表） | `application/persistence/thinkmodel_lake.go`（`propertyUnit` 与查询侧） |
| DB 接口与实现 | `core/interface/`、`infrastructure/sqlite/`（+ mysql 侧同步） |
| 主题常量 | `tools/gatewaymqtt/publisher.go`（或新包） |
| 建表/索引迁移 | `core/bootstrap/database`（参考 commit `67f9177` 的补表方式） |
| 前端：展示 `dn`/自动发现测点、标记未校准、排除名单配置 | `iotserver-ui/src/pages/gateway/DeviceManage`、`api/gateway.ts` |

## 5. 安全

- **接对方云**：生产 Broker 实测**允许匿名连接与匿名订阅**（不带凭证 CONNACK rc=0，带错误凭证 rc=5）。我们接它时仍应走签名凭证，并提醒对方核查 `allow_anonymous`。**不要把生产地址与 SN 写进仓库。**
- **我们自己对外暴露的 Broker 绝不能允许匿名**。现有 `gatewayingest` 校验的是"SN 是否已注册"（应用层），Broker 侧没有鉴权（见 `gateway-device-provisioning-plan.md` 1.2 节第 7 条）。参考系统的做法（`exhook/service/HookService.java`：username 形如 `{pk}|{sn}`、password 为 `HMAC-SHA256(secret, body)` 签名、ACL 按主题前缀）是我们要对齐的目标形态。

## 6. 待决策

1. **是否允许设备完全无产品接入？**（本文按需求假设"允许"，这也是 `device_tag` 存在的前提）
2. **清单外设备**：标记摘除（本文建议）还是参考系统的硬删？
3. **是否要兼容下行**（`/edge/db/{key}/{sn}/update` 工程下发、`/edge/upgrade` OTA）？若要替代参考系统对存量网关做配置下发/OTA，就必须做。
4. **`pk` 的语义**：产品标识、还是网关型号？（实测 `pk == sn`）决定唯一键与是否需要 `Pk` 字段之外的概念。
5. **是否长期双协议并存**：自有 `iot/up/*` + 兼容 `/edge/*`。本文建议并存——新网关走自有协议（有 seq、有逐点时间、可主动注册），存量走兼容层。