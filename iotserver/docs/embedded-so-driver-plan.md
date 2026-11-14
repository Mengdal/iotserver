# 嵌入式部署 · iotpGW `.so` 驱动安装设计

> 范围：只针对 `Deployment.Mode = embedded`。`server` 部署继续使用现有 Docker 镜像下载，不改。
>
> 目标：在嵌入式部署下，驱动市场/本地安装把 `.so` 驱动库安装到 iotpGW 的 `drivers/` 目录，并完成 reload、状态展示、升级与卸载。

## 0. 实施状态

Phase 1（同机直接写目录）已实现：

- 后端：`driverapp/soinstaller.go`、`driverapp/sodriver.go`
- 接口：`POST/GET/DELETE /api/v1/drivers/so`
- 前端：驱动市场“安装 .so 驱动”、`iotpgw-so` 卡片状态与卸载
- 配置：`[CollectEngine] DriverDir / BackupDir / ReloadMode / ReloadCommand / DriverABI`
- 校验：文件名、架构、ABI、SHA256、100MB 上限、同名文件去重
- 安装：`.staging` 临时文件 → 备份旧文件 → 原子替换 → reload；失败回滚

与下文设计的差异：

- Phase 1 直接上传裸 `.so` + 表单元数据，未引入 `manifest.json` tar.gz 包。
- 未新增 `driver_artifact` 表；元数据存在 `DeviceLibrary.PackagePath/PackageHash/Manifest`。
- `ReloadMode` 代码实际支持 `none`、`command`；`api` 返回未实现，`signal` 未实现。
- 升级流程复用“卸载后再上传”，同一 `.so` 文件名只允许一条记录。
- 并发锁、操作审计、启动对账尚未实现。

## 1. 现状

当前代码的实际行为：

- `Deployment.Mode` 只影响 UI 和登录信息；协议驱动下载仍然走 Docker。
- 驱动市场“下载”链路固定为：
  `DriveMarket → WebSocket 10001 → UpgradeDeviceLibrary → DMI.DownApp → docker pull`。
- `dmi.New()` 永远返回 Docker 实现，没有 embedded 分支。
- `embedded` 只是隐藏了“自定义镜像”页签。
- 内置的 TCP / MQTT / HTTP 驱动是 Docker 运行时，已在嵌入式市场中过滤掉。
- “安装插件包”是另一套机制：上传 `tar.gz`，解压后由 iotserver 启动独立进程，不是 iotpGW 的 `.so`。
- iotpGW 自己从 `drivers/` 目录 `dlopen` `.so`，iotserver 目前不参与文件安装和 reload。

所以本功能需要新增一条独立的安装链路，不能复用 Docker DMI，也不能复用原生插件包运行时。

## 2. 总体设计

```text
嵌入式驱动市场 / 本地安装
        │
        ▼
DriverInstaller（按部署模式选择）
        │
        ├── DockerInstaller     server：现有 Docker 镜像
        └── IotpGWSOInstaller   embedded：下载/上传 .so
                │
                ├── 同机：直接写 iotpGW DriverDir
                └── 远程：调用 iotpGW Driver API
                        │
                        ▼
                iotpGW drivers/*.so
                        │
                        ▼
                reload（API 或重启 iotpGW）
```

### 2.1 安装器抽象

新增接口，避免把 Docker 和 `.so` 逻辑混在 `deviceLibraryManager` 里：

```go
type DriverInstaller interface {
    Install(ctx context.Context, artifact DriverArtifact, target Target) error
    Uninstall(ctx context.Context, libraryID string, target Target) error
    Status(ctx context.Context, libraryID string, target Target) (DriverStatus, error)
    Reload(ctx context.Context, target Target) error
}
```

- `server` → `DockerInstaller`，包装现有 `DMI`。
- `embedded` → `IotpGWSOInstaller`。

`deviceLibraryManager` 只负责状态机（installing/installed/default）和数据库更新，实际安装动作下沉到 installer。

## 3. 驱动产物模型

现有 `DeviceLibrary` 只有 Docker 字段，需要增加 `.so` 产物信息。推荐单独建表，避免把多架构/多版本产物塞进 `SupportVersions` JSON：

```go
type DriverArtifact struct {
    ID          string
    LibraryID   string // 对应 device_library.id
    Version     string
    Arch        string // arm64 / amd64
    Runtime     string // iotpgw-so
    FileName    string // ModbusClient.so，iotpGW 识别的文件名
    DownloadURL string
    SHA256      string
    Size        int64
    ABI         string // 驱动 ABI 版本，必须和 iotpGW 匹配
    Created     int64
    Modified    int64
}
```

约束：

- `(LibraryID, Version, Arch)` 唯一。
- 只允许 `Runtime = iotpgw-so` 的产物走这条链路。
- `FileName` 必须匹配 `^[A-Za-z0-9._-]+\.so$`，禁止路径分隔符。
- 安装前校验 `Arch` 和 `ABI`。

如果暂时不做多版本，也可以先只加 `DeviceLibrary.ArtifactURL / ArtifactHash / ArtifactFileName / ABI` 四个字段；但后续多架构会不够用，建议直接建表。

## 4. 驱动包格式

推荐统一成 `.tar.gz`，而不是直接传裸 `.so`：

```text
modbus-driver-1.2.0-arm64.tar.gz
├── manifest.json
└── ModbusClient.so
```

`manifest.json`：

```json
{
  "schemaVersion": 1,
  "name": "modbus-driver",
  "displayName": "Modbus 驱动",
  "version": "1.2.0",
  "category": "driver",
  "runtime": "iotpgw-so",
  "arch": ["arm64"],
  "protocols": ["Modbus TCP", "Modbus RTU"],
  "library": "ModbusClient.so",
  "abi": "1"
}
```

好处：

- 文件名、架构、ABI、版本、协议都能校验。
- 市场下载和本地上传共用同一套格式。
- 后续可加签名。

如果市场只能提供裸 `.so`，则把元数据放在 `DriverArtifact` 里，平台只下载 `.so` 并校验 SHA256；本地上传时用表单填写名称/版本/协议/ABI。两种都支持也可以，但优先推 tar.gz。

## 5. 安装位置与写入方式

**默认按同机部署设计，不需要 iotpGW 提供任何写目录接口。**

iotserver 直接写配置好的 `DriverDir`，因为部署、目录权限、服务重启都由我们自己控制。

### 5.1 同机部署（默认，Phase 1）

iotserver 和 iotpGW 在同一台设备：

```toml
[CollectEngine]
DriverDir = '/opt/iotpgw/drivers'
BackupDir = '/var/lib/iotserver/driver-backups'
ReloadMode = 'command'   # none | command（api 预留；signal 未实现）
ReloadCommand = '/etc/init.d/S93iotserver-iotpgw restart'
```

部署时保证运行用户可写：

```sh
chown -R iotserver:iotserver /opt/iotpgw/drivers
```

安装流程：

1. 下载/上传到临时目录。
2. 校验 SHA256、架构、ABI、文件名。
3. 写入 `DriverDir/.staging/<file>`。
4. 备份旧文件到 `BackupDir`。
5. `rename` 原子替换 `DriverDir/<file>`。
6. 按 `ReloadMode` 触发 reload。
7. 失败则恢复备份并再次 reload。

这是最简单、最可控的方式，不需要改 iotpGW。

### 5.2 不同机部署（可选，Phase 3）

只有当 iotserver 和 iotpGW 不在同一台机器、并且没有共享目录时，平台才无法直接写文件。

这时按优先级：

1. **共享目录**：把 iotpGW 的 `drivers/` 通过 NFS/SSHFS/共享卷挂到 iotserver 本机，仍按同机方式写。
2. **部署脚本**：平台只负责下载/校验产物，由现有部署脚本把 `.so` 拷过去并重启 iotpGW。
3. **iotpGW 提供安装 API**：这是最后一种选择，不是前置条件。

如果确实需要远程 API，再单独设计：

```text
POST   /api/v1/drivers/install
DELETE /api/v1/drivers/{name}
GET    /api/v1/drivers
POST   /api/v1/drivers/reload
```

远程 API 需要额外做鉴权、文件名白名单、固定目录、哈希校验和回滚；这些在 Phase 1 同机方案里由平台本地完成，不需要 iotpGW 参与。

## 6. Reload 设计

iotpGW 二进制里已有 `gateway.(*Runtime).Reload`，但它不一定暴露成 HTTP 接口。同机部署下优先用本地方式：

```toml
ReloadMode = 'command'
ReloadCommand = '/etc/init.d/S93iotserver-iotpgw restart'
```

支持的模式：

| ReloadMode | 行为 | 适用 |
|---|---|---|
| `none` | 只替换文件，提示用户手动重启 | 最保守，先跑通安装 |
| `command` | 执行配置的本机重启命令 | 有 init/systemd 脚本时 |
| `api` | 预留，当前返回“尚未实现” | 远程部署/iotpGW 支持 API 时 |

`signal` 模式未实现。

如果 iotserver 运行用户没有重启权限，可以：

- 给重启脚本配 sudoers 白名单；
- 或在部署时把 reload 命令包装成 iotserver 可执行的脚本；
- 或先用 `ReloadMode='none'`，安装完成后由部署流程统一重启。

不需要为了写目录去改 iotpGW；只有“跨机器 + 无共享目录”才需要远程接口。

## 7. 与驱动市场的关系

`embedded` 下驱动市场不再显示 Docker 驱动，而是显示 `.so` 驱动：

```text
驱动市场
├── 系统组件（builtin）
└── 协议驱动（iotpgw-so）
      ├── Modbus TCP / RTU   [安装] [卸载]
      └── ...
```

市场数据来源：

- 现有内置 TCP/MQTT/HTTP 是 Docker 驱动，embedded 下过滤掉（已实现）。
- 新增嵌入式驱动记录，`runtime = iotpgw-so`，并关联 `driver_artifact`。
- 若市场还没有 `.so` 产物，先提供“本地上传 `.so` 驱动”入口，保证功能可用。

UI 行为：

- 未安装：显示“安装”，并校验架构/ABI。
- 安装中：显示进度/状态。
- 已安装：显示版本、卸载、升级。
- 不兼容：显示“架构不匹配/ABI 不匹配”，禁用安装。
- 安装/卸载/升级按钮只对平台管理员开放。

## 8. 安全要求

- 只允许平台管理员操作。
- 只允许 `Deployment.Mode = embedded` 走 `.so` 安装；server 模式拒绝。
- 校验 SHA256；有条件时校验签名。
- 校验 `arch`、`abi`、文件名。
- 不允许路径穿越、符号链接、任意目录写入。
- 安装包大小限制（例如 100MB）。
- 并发安装同一驱动要加锁。
- 备份旧版本，reload 失败自动回滚。
- 日志记录：操作人、驱动、版本、文件哈希、结果。

## 9. 数据与状态

`device_library` 增加/复用：

- `Runtime = iotpgw-so`
- `Arch`
- `Version`
- `OperateStatus`：default / installing / installed / error

`driver_artifact` 保存可下载产物；`driver_install_status`（或复用 `device_library`）保存目标 iotpGW 的安装状态：

```text
library_id
target_url       iotpGW 地址
installed_version
installed_file
installed_at
last_error
```

启动时做一次对账：

- 同机部署：扫描本地 `DriverDir`，按文件名匹配已安装驱动。
- 远程部署：如已有 driver API，再调 `GET /api/v1/drivers`；没有 API 时不做远程对账。
- 更新已安装状态；不删除未知文件。

## 10. 分阶段实施

### Phase 1：同机 + 本地 `.so` 安装

- 新增 `DriverArtifact` 模型和 `IotpGWSOInstaller`。
- `embedded` 下支持本地上传 `.so` 包并安装到 `DriverDir`。
- 校验哈希/架构/ABI。
- reload 先用配置的 restart 命令。
- UI 增加“安装 `.so` 驱动”。

### Phase 2：驱动市场产物

- 市场返回 `iotpgw-so` 产物。
- 嵌入式驱动市场显示 `.so` 驱动，支持下载/安装/卸载/升级。
- 增加 SHA256/签名校验。

### Phase 3：远程 iotpGW（可选）

只有“不同机器 + 无共享目录”时才需要：

- 共享目录优先，直接复用 Phase 1。
- 或由部署脚本拷贝/重启。
- 最后才考虑 iotpGW 提供 driver API。
- 多引擎/多网关目标选择。

## 11. 需要确认

1. iotpGW 的 `.so` ABI 是什么？导出符号、初始化函数、参数结构。
2. `drivers/` 里的文件名是否固定？比如必须叫 `ModbusClient.so`。
3. iotpGW 是否支持 reload 信号/接口？如果只支持重启，重启命令是什么？
4. 嵌入式部署时 iotpGW 和 iotserver 是否同机？是否有共享目录？
5. `.so` 驱动产物从哪里来？市场 URL、包格式、是否有签名。
6. 一个驱动是否允许多个版本共存？还是同名 `.so` 只能保留一个？
7. 卸载驱动时，如果已有通道/设备在使用，是阻止卸载还是提示后强制？

这些确认后，Phase 1 可以直接开始实现。
