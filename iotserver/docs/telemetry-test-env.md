# 持续测试数据环境

用于长时间观察时序库、规则引擎与看板：一个常驻容器按固定间隔模拟网关上报真实数据。

## 组成

| 组件 | 位置 | 说明 |
| --- | --- | --- |
| `mqtt-broker` | Docker（已有） | `58090`，消息总线 |
| `ekuiper` / `ekuiper-manager` | Docker（已有） | `9081` / `9082`，规则引擎 |
| `iotserver-core` | **本机进程** | `3000` / `58081` / `57081`，独占 DuckDB 文件 |
| `iotserver-telemgen` | Docker（新增） | 持续上报测试数据 |

> core 是本机进程而不是容器：`docker-compose.yml` 里虽然有 `iotserver-core` 服务，
> 但那份编排一旦 `up` 会同时拉起 core 容器，与本机进程**端口冲突**且**争抢 DuckDB 单写者文件锁**。
> 因此造数服务放在独立的 `docker-compose.telemetry.yml` 里，不碰 core。

## 启动 / 停止

```bash
cd iotserver/manifest/docker

# 启动（首次会先编译，约 30–60 秒）
docker compose -f docker-compose.telemetry.yml up -d

# 看上报心跳
docker compose -f docker-compose.telemetry.yml logs -f

# 停止
docker compose -f docker-compose.telemetry.yml down
```

`restart: unless-stopped`：Docker 重启后会自动拉起。

## 造数工具（telemgen）

`tools/telemgen` 走**真实的网关上行走廊**，而非直接写数据层，因此落库、租户归属、
推 eKuiper 告警链路都会被真实触发。

首次运行会自动建好（幂等，重复运行只复用）：

- 产品「监控联调产品」，物模型：`temp`(float/℃)、`humidity`(float/%RH)、`mode`(text)
- 5 台设备「监控设备-1..5」，`device_sn` 为 `TELEMGEN-DEV-<n>`
- 一个网关 `TELEMGEN-GW1`（上行主题 `iot/up/TELEMGEN-GW1/data`）

上报内容：每台设备每轮 3 个测点，数值为日内正弦波 + 噪声，`mode` 在
`auto/manual/off/idle` 之间轮转。默认 5 秒一轮、5 台设备。

本机直接跑（不经 Docker）：

```bash
export PATH=$HOME/sdk/go1.27.2/bin:$PATH
export GOPATH=$(pwd)/../.gopath GOMODCACHE=$(pwd)/../.gopath/pkg/mod GOFLAGS=-mod=mod

go run ./tools/telemgen -once                      # 冒烟：只上报一轮
go run ./tools/telemgen -devices 20 -interval 1s   # 加压
```

常用参数：`-api`、`-broker`、`-devices`、`-interval`、`-product`、`-device-prefix`、
`-gateway-sn`、`-tenant`（平台管理员代管租户时传 `x-tenant-id`）、`-once`。

## 验证数据在流动

```bash
TOKEN=$(curl -s -X POST http://127.0.0.1:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r .result.token)

DEV=$(curl -s "http://127.0.0.1:3000/api/v1/devices?page=1&pageSize=200" \
  -H "x-token: $TOKEN" | jq -r '.result.list[]|select(.name|startswith("监控设备-1")).id')

# 最新值（应随时间推进）
curl -s "http://127.0.0.1:3000/api/v1/device/$DEV/thing-model/property" -H "x-token: $TOKEN" | jq .result

# 历史曲线
NOW=$(python3 -c 'import time;print(int(time.time()*1000))')
curl -s "http://127.0.0.1:3000/api/v1/device/$DEV/thing-model/history-property?code=temp&page=1&pageSize=20&range=$((NOW-600000)),$NOW" \
  -H "x-token: $TOKEN" | jq '.result.total'
```

## 注意

- **容器挂载了源码**，改了 `tools/telemgen` 后需要 `docker compose ... restart telemgen` 才生效。
- **不产生事件类数据**：`mode` 等是属性。若要用事件/告警规则，需先给产品加物模型事件，
  再让工具在上报里带 `events`。
- DuckDB 主文件在 `manifest/docker/db-data/duckdb/core-data.duckdb`，写入先进 `.wal`，
  由 `CHECKPOINT` 合并（保留期清理与正常退出都会触发）。
