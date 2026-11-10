#!/usr/bin/env bash
#
# iotserver 本地部署安装脚本（ARM64 + Mosquitto）
#
# 支持两类 init：
#   systemd  —— 服务器发行版（Debian/Ubuntu 等）
#   busybox  —— Buildroot / 嵌入式设备（/etc/init.d/S*）
# 自动探测，可用 INIT_SYSTEM=systemd|busybox 强制指定。
#
# 前置条件：root 权限。
#
# 发布产物目录（ARTIFACT_DIR）：
#   bin/iotserver-core          必填
#   ui/                         必填，前端 build 目录
#   ekuiper/                    选填，eKuiper 解压目录（含 bin/kuiperd）
#   bin/mosquitto               选填，随包携带的 Mosquitto（否则用系统已装的）
#   bin/mosquitto_pub
#   bin/mosquitto_sub
#   lib/libmosquitto.so.1       mosquitto_pub/sub 所需（与上同包）
#
# 用法：
#   ARTIFACT_DIR=/path/to/dist ./deploy/local/install.sh
#   START_NOW=1 ARTIFACT_DIR=/path/to/dist ./deploy/local/install.sh
#
set -euo pipefail

PREFIX="${PREFIX:-/opt/iotserver}"
CONFIG_DIR="${CONFIG_DIR:-/etc/iotserver}"
DATA_DIR="${DATA_DIR:-/var/lib/iotserver}"
LOG_DIR="${LOG_DIR:-/var/log/iotserver}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
INITD_DIR="${INITD_DIR:-/etc/init.d}"
# 服务运行用户：auto 表示按目标机能力选择，可用 SERVICE_USER=xxx 强制指定。
SERVICE_USER="${SERVICE_USER:-auto}"
ARTIFACT_DIR="${ARTIFACT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/dist/iotserver-local}"
START_NOW="${START_NOW:-0}"
INIT_SYSTEM="${INIT_SYSTEM:-auto}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "请使用 root 执行：ARTIFACT_DIR=... $0" >&2
  exit 1
fi

if [[ "$INIT_SYSTEM" == "auto" ]]; then
  if command -v systemctl >/dev/null 2>&1; then
    INIT_SYSTEM="systemd"
  elif [[ -d "$INITD_DIR" ]]; then
    INIT_SYSTEM="busybox"
  else
    echo "无法识别 init 系统，请用 INIT_SYSTEM=systemd|busybox 指定。" >&2
    exit 1
  fi
fi

# eKuiper 可选：
#   SKIP_EKUIPER=1 或产物里没有 ekuiper/ 时跳过安装与启动。
installed_ekuiper=1
if [[ ! -x "${ARTIFACT_DIR}/ekuiper/bin/kuiperd" && ! -x "${ARTIFACT_DIR}/ekuiper/bin/ekuiper" ]]; then
  installed_ekuiper=0
fi
if [[ "${SKIP_EKUIPER:-0}" == "1" ]]; then
  installed_ekuiper=0
fi

for required in "${ARTIFACT_DIR}/bin/iotserver-core" "${ARTIFACT_DIR}/ui/index.html"; do
  if [[ ! -e "$required" ]]; then
    echo "缺少发布产物：$required" >&2
    echo "请先构建 ARM64 二进制和前端。" >&2
    exit 1
  fi
done

# 运行用户选择：
#   os/exec 启动子进程时会打开 /dev/null，很多 Buildroot 镜像把 /dev/* 设成
#   root:root 0660，非 root 进程直接失败。此时退化为 root 运行（嵌入式设备常见做法）。
if [[ "$SERVICE_USER" == "auto" ]]; then
  dev_null_mode="$(stat -c %a /dev/null 2>/dev/null || echo 666)"
  if (( (dev_null_mode % 10) & 6 )); then
    SERVICE_USER="iotserver"
  else
    SERVICE_USER="root"
    echo "检测到 /dev/null 权限为 ${dev_null_mode}（others 不可读写），服务将以 root 运行。"
  fi
fi

# 用户：systemd 发行版用 useradd，Buildroot 只有 adduser。
if [[ "$SERVICE_USER" != "root" ]] && ! id "$SERVICE_USER" >/dev/null 2>&1; then
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --user-group --home-dir "$PREFIX" --shell /usr/sbin/nologin "$SERVICE_USER"
  elif command -v adduser >/dev/null 2>&1; then
    # BusyBox adduser -S 默认把组设成 nogroup，Buildroot 上没有这个组，
    # 先建同名组再显式指定。
    if ! grep -q "^${SERVICE_USER}:" /etc/group 2>/dev/null; then
      addgroup -S "$SERVICE_USER"
    fi
    adduser -S -D -H -G "$SERVICE_USER" -s /sbin/nologin "$SERVICE_USER"
  else
    echo "请先手动创建用户 $SERVICE_USER" >&2
    exit 1
  fi
fi

install -d -m 0755 "$PREFIX" "$PREFIX/bin" "$PREFIX/lib" "$PREFIX/ui"
install -d -m 0755 "$CONFIG_DIR"
install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_USER" \
  "$DATA_DIR" "$DATA_DIR/duckdb" "$DATA_DIR/mosquitto" "$DATA_DIR/plugin-data" \
  "$PREFIX/plugins" "$LOG_DIR"

install -m 0755 "${ARTIFACT_DIR}/bin/iotserver-core" "${PREFIX}/bin/iotserver-core"

# 随包携带的 Mosquitto（可选）
if [[ -x "${ARTIFACT_DIR}/bin/mosquitto" ]]; then
  install -m 0755 "${ARTIFACT_DIR}/bin/mosquitto" "${PREFIX}/bin/mosquitto"
  for c in mosquitto_pub mosquitto_sub; do
    [[ -x "${ARTIFACT_DIR}/bin/$c" ]] && install -m 0755 "${ARTIFACT_DIR}/bin/$c" "${PREFIX}/bin/$c"
  done
  for lib in "${ARTIFACT_DIR}"/lib/*.so*; do
    [[ -f "$lib" ]] || continue
    install -m 0755 "$lib" "${PREFIX}/lib/$(basename "$lib")"
  done
  ldconfig 2>/dev/null || true
elif [[ -x /usr/sbin/mosquitto ]]; then
  # 未随包携带时，用系统已装的 Mosquitto，启动脚本统一走 $PREFIX/bin。
  ln -sf /usr/sbin/mosquitto "${PREFIX}/bin/mosquitto"
fi

rm -rf "${PREFIX}/ui"
cp -R "${ARTIFACT_DIR}/ui" "${PREFIX}/ui"
# macOS 打包会带 AppleDouble 元数据（._*），eKuiper 会把它们当配置解析报错。
find "${PREFIX}/ui" -name '._*' -exec rm -f {} \; 2>/dev/null || true

if [[ "$installed_ekuiper" == "1" ]]; then
  rm -rf "${PREFIX}/ekuiper"
  cp -R "${ARTIFACT_DIR}/ekuiper" "${PREFIX}/ekuiper"
  find "${PREFIX}/ekuiper" -name '._*' -exec rm -f {} \; 2>/dev/null || true
  install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_USER" "$PREFIX/ekuiper/data" "$PREFIX/ekuiper/log"
  # 内网单机部署：只监听回环，关闭与设备已有服务冲突的 pprof。
  if [[ -f "${PREFIX}/ekuiper/etc/kuiper.yaml" ]]; then
    sed -i -e 's/^\([[:space:]]*restIp:\).*/\1 127.0.0.1/' \
           -e 's/^\([[:space:]]*ip:\).*/\1 127.0.0.1/' \
           -e 's/^\([[:space:]]*pprof:\).*/\1 false/' \
           "${PREFIX}/ekuiper/etc/kuiper.yaml"
  fi
else
  echo "未提供 eKuiper 产物，跳过 eKuiper 安装。"
fi

# 配置不覆盖已有文件：生成 .new 供人工对比。
install_config() {
  local src="$1" dst="$2"
  if [[ -f "$dst" ]]; then
    install -m 0644 "$src" "${dst}.new"
    echo "已存在 $dst，新配置写入 ${dst}.new"
  else
    install -m 0644 "$src" "$dst"
  fi
}
# 优先使用本地覆盖配置（含 iotp Token 等凭据，已被 .gitignore 忽略）。
CONFIG_SRC="${SCRIPT_DIR}/configuration.toml"
if [[ -f "${SCRIPT_DIR}/configuration.local.toml" ]]; then
  CONFIG_SRC="${SCRIPT_DIR}/configuration.local.toml"
fi
install_config "${CONFIG_SRC}" "${CONFIG_DIR}/configuration.toml"
install_config "${SCRIPT_DIR}/mosquitto.conf" "${CONFIG_DIR}/mosquitto.conf"

# 启动脚本从这里读取运行用户。
printf 'RUN_USER=%s\n' "$SERVICE_USER" > "${CONFIG_DIR}/iotserver.env"
chmod 0644 "${CONFIG_DIR}/iotserver.env"

# Mosquitto 以 root 启动时会按 user 指令降权（默认找 mosquitto 用户），
# 这里显式写成实际运行用户，避免落到 nobody 导致持久化目录不可写。
if grep -q '^user ' "${CONFIG_DIR}/mosquitto.conf" 2>/dev/null; then
  sed -i "s/^user .*/user ${SERVICE_USER}/" "${CONFIG_DIR}/mosquitto.conf"
else
  printf '\nuser %s\n' "$SERVICE_USER" >> "${CONFIG_DIR}/mosquitto.conf"
fi

chown -R "$SERVICE_USER:$SERVICE_USER" "$PREFIX" "$DATA_DIR" "$LOG_DIR" "${CONFIG_DIR}/iotserver.env"

if [[ ! -x "${PREFIX}/bin/mosquitto" && ! -x /usr/sbin/mosquitto ]]; then
  echo "警告：未找到 Mosquitto（${PREFIX}/bin/mosquitto 或 /usr/sbin/mosquitto）。" >&2
fi

if [[ "$INIT_SYSTEM" == "systemd" ]]; then
  install -m 0644 "${SCRIPT_DIR}/systemd/iotserver-mqtt.service" "${SYSTEMD_DIR}/iotserver-mqtt.service"
  install -m 0644 "${SCRIPT_DIR}/systemd/iotserver-ekuiper.service" "${SYSTEMD_DIR}/iotserver-ekuiper.service"
  install -m 0644 "${SCRIPT_DIR}/systemd/iotserver-core.service" "${SYSTEMD_DIR}/iotserver-core.service"
  for unit in iotserver-mqtt iotserver-ekuiper iotserver-core; do
    sed -i -e "s/^User=.*/User=${SERVICE_USER}/" -e "s/^Group=.*/Group=${SERVICE_USER}/" \
      "${SYSTEMD_DIR}/${unit}.service"
  done

  systemctl daemon-reload
  systemctl enable iotserver-mqtt.service
  systemctl enable iotserver-core.service
  if [[ "$installed_ekuiper" == "1" ]]; then
    systemctl enable iotserver-ekuiper.service
  fi

  if [[ "$START_NOW" == "1" ]]; then
    systemctl restart iotserver-mqtt.service
    if [[ "$installed_ekuiper" == "1" ]]; then
      systemctl restart iotserver-ekuiper.service
    fi
    systemctl restart iotserver-core.service
  fi
else
  install -m 0755 "${SCRIPT_DIR}/init.d/S90iotserver-mqtt" "${INITD_DIR}/S90iotserver-mqtt"
  install -m 0755 "${SCRIPT_DIR}/init.d/S91iotserver-ekuiper" "${INITD_DIR}/S91iotserver-ekuiper"
  install -m 0755 "${SCRIPT_DIR}/init.d/S92iotserver-core" "${INITD_DIR}/S92iotserver-core"

  if [[ "$installed_ekuiper" != "1" ]]; then
    # 没有 eKuiper 产物时移除启动脚本，避免开机启动失败。
    rm -f "${INITD_DIR}/S91iotserver-ekuiper"
  fi

  if [[ "$START_NOW" == "1" ]]; then
    "${INITD_DIR}/S90iotserver-mqtt" stop || true
    "${INITD_DIR}/S90iotserver-mqtt" start
    if [[ "$installed_ekuiper" == "1" ]]; then
      "${INITD_DIR}/S91iotserver-ekuiper" stop || true
      "${INITD_DIR}/S91iotserver-ekuiper" start
    fi
    "${INITD_DIR}/S92iotserver-core" stop || true
    "${INITD_DIR}/S92iotserver-core" start
  fi
fi

cat <<EOF

安装完成（init: $INIT_SYSTEM）。

目录：
  $PREFIX
  $CONFIG_DIR
  $DATA_DIR
  $LOG_DIR

EOF

if [[ "$INIT_SYSTEM" == "systemd" ]]; then
  cat <<EOF
启动：
  systemctl start iotserver-mqtt
  systemctl start iotserver-ekuiper
  systemctl start iotserver-core

查看状态：
  systemctl status iotserver-mqtt iotserver-ekuiper iotserver-core
  journalctl -u iotserver-core -f
EOF
else
  cat <<EOF
启动：
  $INITD_DIR/S90iotserver-mqtt start
  $INITD_DIR/S91iotserver-ekuiper start
  $INITD_DIR/S92iotserver-core start

查看状态：
  $INITD_DIR/S90iotserver-mqtt status
  $INITD_DIR/S91iotserver-ekuiper status
  $INITD_DIR/S92iotserver-core status

日志：
  tail -f $LOG_DIR/iotserver-core.log
  tail -f $PREFIX/ekuiper/log/stream.log
EOF
fi

cat <<EOF

如果需要网关从局域网接入，请编辑 $CONFIG_DIR/mosquitto.conf
启用 1883 监听，并用 mosquitto_passwd 创建密码文件。

EOF
