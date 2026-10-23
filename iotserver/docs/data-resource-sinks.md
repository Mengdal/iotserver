# 数据资源推送（HTTP / MQTT / Kafka / InfluxDB）

「高级能力 → 资源管理」中的资源最终作为 eKuiper 规则的 sink 使用。规则引擎把资源配置转换成对应的 eKuiper action。

## 支持情况

| 类型 | eKuiper sink | 依赖 | 说明 |
|---|---|---|---|
| HTTP推送 | 内置 `rest` | 无 | 需要 eKuiper 允许访问目标地址；内网地址需 `enablePrivateNet=true` |
| 消息对队列MQTT | 内置 `mqtt` | MQTT broker | `server` 必须是 **eKuiper 容器**能访问的地址 |
| 消息队列Kafka | 插件 `kafka` | Kafka broker + kafka 插件 | 镜像已内置插件 |
| InfluxDB | 插件 `influx`（v1.x） | InfluxDB 1.x + influx 插件 | 镜像已内置插件 |

## 字段映射

- HTTP：`url`、`method`、`headers` 等原样传给 eKuiper `rest` sink；默认 `method=POST`、`bodyType=json`、`format=json`、`sendSingle=true`。
- MQTT：`server`、`topic` 等原样传给 eKuiper `mqtt` sink。
- Kafka：`brokers`、`topic` 等原样传给 kafka 插件。
- InfluxDB：
  - `databasename` → eKuiper 的 `database`
  - `tagkey` / `tagvalue` → eKuiper 的 `tags: {tagkey: tagvalue}`
  - `measurement` 为必填
  - 当前只支持 InfluxDB 1.x；InfluxDB 2.x 需要 `influx2` 插件和不同字段（org/bucket/token），未接入

## 部署要求

1. 使用 `make docker` 构建自定义 eKuiper 镜像 `iotserver-ekuiper:2.4.2`，镜像内置 Kafka/InfluxDB sink 插件。
2. `docker-compose.yml` 已设置 `KUIPER__BASIC__ENABLEPRIVATENET=true`，否则规则无法向私网 HTTP 地址推送。
3. Kafka / InfluxDB 服务本身需要单独部署，并在资源里填写 **eKuiper 容器可达**的地址。
4. 修改 eKuiper 插件后需要重新构建镜像；容器启动时会把插件复制到持久化插件目录，已存在同名插件不覆盖。

## 已验证链路

| 类型 | 验证方式 | 结果 |
|---|---|---|
| HTTP | 本地 HTTP 服务接收规则 POST | 通过 |
| MQTT | 订阅目标 topic 收到消息 | 通过 |
| Kafka | Redpanda broker + `rpk topic consume` | 通过 |
| InfluxDB | mock InfluxDB v1 `/ping` + `/write` | 通过 |

> 安全提示：`enablePrivateNet=true` 会让租户配置的 HTTP 规则访问内网地址。若部署环境不允许，请改回 `false`，并限制 HTTP 资源只使用公网地址。
