# 本地部署（ARM64 + Mosquitto）

iotserver 以本机进程方式运行：

```text
Mosquitto → eKuiper → iotserver-core
```

由 systemd（服务器发行版）或 BusyBox init（Buildroot 嵌入式）托管。
时序数据写入外部 iotp，不编译 DuckDB。

## 1. 概览

### 目录

```text
/opt/iotserver/
  bin/iotserver-core
  bin/mosquitto              随包携带；未携带时链接到系统 /usr/sbin/mosquitto
  bin/mosquitto_pub
  bin/mosquitto_sub
  lib/libmosquitto.so.1      客户端工具依赖
  lib/libcjson.so.1          客户端工具依赖
  ui/                        前端构建产物
  ekuiper/                   eKuiper 解压目录（bin/kuiperd）
  plugins/                   原生驱动插件版本目录

/etc/iotserver/
  configuration.toml
  mosquitto.conf
  iotserver.env              运行用户

/var/lib/iotserver/
  core.db                    SQLite 元数据
  mosquitto/                 持久化
  plugin-data/               插件实例配置、日志、PID

/var/log/iotserver/
```

### 端口

| 端口 | 监听 | 用途 |
|---|---|---|
| 3000 | 0.0.0.0 | Web / 前端与 API |
| 58081 | 0.0.0.0 | HTTP 服务、eKuiper 规则回调 |
| 57081 | 0.0.0.0 | RPC |
| 58090 | 127.0.0.1 | MQTT 内部总线 |
| 9081 | 127.0.0.1 | eKuiper REST |
| 20498 | 127.0.0.1 | eKuiper CLI |

## 2. 构建

### 2.1 后端

使用纯 Go 构建，不含 DuckDB：

```bash
make build-iotp-linux TARGET_ARCH=arm64
# 产物：iotserver-core-linux-arm64-iotp
```

等价命令：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -tags noduckdb -trimpath -ldflags "-s -w" \
  -o iotserver-core-linux-arm64-iotp ./cmd/iotserver-core
```

好处：不需要交叉 C 工具链、静态链接、体积小（约 40 MB），armv7 也能编：

```bash
make build-iotp-linux TARGET_ARCH=armv7
```

该构建会在启动时校验数据层，`Type` 必须是 `iotp`，否则 panic：

```text
当前构建未包含 DuckDB，请将 [Databases.Data].Type 配置为 iotp，或使用默认构建
```

如果确实需要在设备上跑本地 DuckDB，改用 CGO 构建（配置 `Type='duckdb'`）：

```bash
make build-linux-arm64                      # 在 ARM64 设备上直接编
make build-linux-arm64 CC_ARM64=aarch64-linux-gnu-gcc   # 有交叉工具链时
```

### 2.2 前端

```bash
make build-ui
# 产物：../iotserver-ui/build
```

### 2.3 Mosquitto

**方式一：包管理器（Debian / Ubuntu）**

```bash
sudo apt install mosquitto mosquitto-clients
# 系统自带的 mosquitto.service 与 iotserver-mqtt 冲突
sudo systemctl disable --now mosquitto
```

安装脚本会把 `/usr/sbin/mosquitto` 链接到 `/opt/iotserver/bin/mosquitto`。

**方式二：随包携带（Buildroot 等无包管理器的设备）**

在 ARM64 机器上从源码构建：

```bash
sudo apt install build-essential libcjson-dev
curl -LO https://mosquitto.org/files/source/mosquitto-2.0.20.tar.gz
tar xf mosquitto-2.0.20.tar.gz && cd mosquitto-2.0.20
make WITH_TLS=no WITH_DOCS=no WITH_WEBSOCKETS=no
```

需要收集的产物：

```text
src/mosquitto
client/mosquitto_pub
client/mosquitto_sub
lib/libmosquitto.so.1
系统里的 libcjson.so.1
```

`mosquitto` 本体会把 cJSON 静态编入，只有客户端工具需要 `libcjson.so.1`。

### 2.4 eKuiper

下载官方 ARM64 包：

```text
https://github.com/lf-edge/ekuiper/releases/download/v2.4.2/kuiper-2.4.2-linux-arm64.tar.gz
```

官方包的可执行文件是 `bin/kuiperd`（`bin/kuiper` 是命令行工具）：

```text
ekuiper/bin/kuiperd
ekuiper/etc/kuiper.yaml
```

安装脚本会修改 `etc/kuiper.yaml`：

- `restIp` / `ip` 改成 `127.0.0.1`，只在回环提供 REST 与 CLI
- `pprof` 改成 `false`，避免和设备上已有服务抢 6060

启动脚本注入环境变量：

```text
MQTT_SOURCE__DEFAULT__SERVER=tcp://127.0.0.1:58090
KUIPER__BASIC__ENABLEPRIVATENET=true    # 规则回调 127.0.0.1 必需
KUIPER__BASIC__CONSOLELOG=false
```

不需要流处理时，产物里不放 `ekuiper/`，或安装时加 `SKIP_EKUIPER=1`。

### 2.5 组装发布目录

```text
dist/iotserver-local/
  bin/iotserver-core
  bin/mosquitto            （可选，随包携带时）
  bin/mosquitto_pub        （可选）
  bin/mosquitto_sub        （可选）
  lib/libmosquitto.so.1    （可选）
  lib/libcjson.so.1        （可选）
  ui/                      ← iotserver-ui/build 的内容
  ekuiper/                 ← eKuiper 解压目录（可选）
```

```bash
mkdir -p dist/iotserver-local/bin dist/iotserver-local/lib

# 构建产物名带 -iotp，安装脚本要的是 iotserver-core
cp iotserver-core-linux-arm64-iotp dist/iotserver-local/bin/iotserver-core

cp -R ../iotserver-ui/build dist/iotserver-local/ui
cp -R /path/to/kuiper-2.4.2-linux-arm64 dist/iotserver-local/ekuiper
cp mosquitto mosquitto_pub mosquitto_sub dist/iotserver-local/bin/
cp libmosquitto.so.1 libcjson.so.1 dist/iotserver-local/lib/
```

## 3. 安装

```bash
sudo ARTIFACT_DIR=$PWD/dist/iotserver-local ./deploy/local/install.sh
```

脚本会：

- 探测 init 系统（有 `systemctl` → systemd，否则 `/etc/init.d` → BusyBox）
- 创建运行用户并决定权限
- 创建 `/opt/iotserver`、`/etc/iotserver`、`/var/lib/iotserver`、`/var/log/iotserver`
- 安装二进制、前端、eKuiper、随包 Mosquitto
- 安装配置与启动脚本

已有配置不会被覆盖，会生成 `.new` 供对比。

### 3.1 配置

`configuration.toml` 与 `mosquitto.conf` 会和安装脚本放在同一目录。
带凭据的覆盖配置（iotp Token）放在 `configuration.local.toml`，
安装脚本优先使用它：

```bash
cp deploy/local/configuration.toml deploy/local/configuration.local.toml
# 填入 [Databases.Data].Token
```

`configuration.local.toml` 已被 `.gitignore` 忽略。

数据层配置：

```toml
[Databases.Data]
Type = 'iotp'
BaseURL = 'http://<iotp 地址>'
Token = '<token>'
Name = 'iotserver'
Timeout = 10000
```

其它本地部署相关项：

```toml
[WebUI]
BuildPath = '/opt/iotserver/ui'

[Deployment]
Mode = 'embedded'          # 隐藏镜像仓库入口，驱动以本机进程运行
```

### 3.2 运行用户

默认按目标机能力选择（`SERVICE_USER=auto`）：

- `/dev/null` 对 others 可读写 → 用 `iotserver` 普通用户
- `/dev/null` 是 `root:root 0660`（Buildroot 常见）→ 用 `root`

原因：`os/exec` 启动子进程时会打开 `/dev/null`，非 root 进程在这类镜像上
会直接失败（原生驱动实例起不来）。选择结果写入
`/etc/iotserver/iotserver.env` 的 `RUN_USER`，三个启动脚本共用。

强制指定：

```bash
SERVICE_USER=iotserver ARTIFACT_DIR=... ./deploy/local/install.sh
INIT_SYSTEM=busybox  ARTIFACT_DIR=... ./deploy/local/install.sh
START_NOW=1          ARTIFACT_DIR=... ./deploy/local/install.sh
```

## 4. 启停

### systemd

```bash
sudo systemctl start iotserver-mqtt
sudo systemctl start iotserver-ekuiper
sudo systemctl start iotserver-core

sudo systemctl status iotserver-mqtt iotserver-ekuiper iotserver-core
journalctl -u iotserver-core -f
```

### Buildroot / BusyBox

脚本装到 `/etc/init.d/`，`rcS` 按序号开机启动：

```text
S90iotserver-mqtt
S91iotserver-ekuiper
S92iotserver-core
```

```bash
/etc/init.d/S90iotserver-mqtt start|stop|restart|status
/etc/init.d/S91iotserver-ekuiper start|stop|restart|status
/etc/init.d/S92iotserver-core start|stop|restart|status
```

日志：

```bash
tail -f /var/log/iotserver/iotserver-core.log
tail -f /var/log/iotserver/mosquitto.log
tail -f /opt/iotserver/ekuiper/log/stream.log
```

## 5. 网关接入

默认 Mosquitto 只监听回环，供平台内部总线和 eKuiper 使用：

```text
58090 127.0.0.1
```

需要局域网网关接入时，编辑 `/etc/iotserver/mosquitto.conf`，启用外部监听：

```conf
listener 1883 0.0.0.0
allow_anonymous false
password_file /etc/iotserver/mosquitto.passwd
acl_file /etc/iotserver/mosquitto.acl
```

```bash
mosquitto_passwd -c /etc/iotserver/mosquitto.passwd <username>
/etc/init.d/S90iotserver-mqtt restart     # systemd: systemctl restart iotserver-mqtt
```

网关上行主题：

```text
iot/up/{sn}/data      实时数据
iot/up/{sn}/batch     批量补传
iot/up/{sn}/status    心跳 / 状态
iot/up/{sn}/offline   遗嘱
```

`sn` 必须是在平台登记过的网关，否则日志会报“未注册的网关”。

## 6. 驱动

原生驱动插件：

- `manifest.json` 的 `arch` 必须包含 `arm64`（或 `any`）
- 在“驱动组件 → 驱动市场 → 安装插件包”上传
- 版本目录 `/opt/iotserver/plugins`
- 实例数据 `/var/lib/iotserver/plugin-data`

串口驱动需要访问 tty 设备：

- systemd：在 `iotserver-core.service` 加 `SupplementaryGroups=dialout`
- Buildroot：放开对应设备节点权限，或把运行用户加入设备所属组

## 7. 验证

```bash
# 进程
ps | grep -E 'mosquitto|kuiperd|iotserver-core'

# 端口
netstat -lnt | grep -E '58090|9081|3000|58081'

# MQTT 内部总线
LD_LIBRARY_PATH=/opt/iotserver/lib /opt/iotserver/bin/mosquitto_sub \
  -h 127.0.0.1 -p 58090 -t 'eventbus/#' -v

# 页面
curl -I http://127.0.0.1:3000/

# 登录（默认 admin / admin123）
curl -s -X POST http://127.0.0.1:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

# eKuiper
curl -s http://127.0.0.1:9081/
```

启动日志里应能看到数据层：

```text
时序数据层: iotp (http://..., db=iotserver)
iotp 元数据同步已启用
```

重启设备后再确认三个服务自动启动。

## 8. 升级与回滚

替换后端二进制：

```bash
/etc/init.d/S92iotserver-core stop
cp /opt/iotserver/bin/iotserver-core /opt/iotserver/bin/iotserver-core.bak
install -m 0755 iotserver-core /opt/iotserver/bin/iotserver-core
/etc/init.d/S92iotserver-core start
```

前端和 eKuiper 直接替换 `/opt/iotserver/ui`、`/opt/iotserver/ekuiper` 后重启对应服务。
回滚把 `.bak` 盖回并重启即可。

## 9. 资源与排障

实测（ARM64 网关，4 核 / 950 MB / eMMC）：

```text
Mosquitto         2 MiB
eKuiper          35 MiB
iotserver-core   66 MiB（iotp 模式）
合计           约 103 MiB
```

设备内存紧张时可以只部署 Mosquitto + iotserver-core，不装 eKuiper。
systemd 下可限制资源：

```ini
MemoryMax=256M
CPUQuota=50%
```

### 常见问题

| 现象 | 原因 / 处理 |
|---|---|
| 原生驱动启动失败，`open /dev/null: permission denied` | `/dev/null` 是 0660 root，安装时用 `SERVICE_USER=root` |
| 启动 panic `当前构建未包含 DuckDB` | 纯 Go 构建必须配 `Type='iotp'` |
| 启动 panic `Type=iotp 需要 BaseURL 非空` | 补 `BaseURL` |
| eKuiper 起不来，6060 被占用 | 确认 `kuiper.yaml` 里 `pprof: false` |
| 日志 `Item mqtt_stream already exists` | 正常，平台复用已有流 |
| `未注册的网关 sn=...` | 网关 SN 没在平台登记 |
| Mosquitto 以 nobody 运行、持久化失败 | `mosquitto.conf` 里补 `user <运行用户>` |
| eKuiper 日志里出现 `._xxx.json` 解析报错 | 打包时带了 macOS 的 AppleDouble 文件，删掉 `._*` |
