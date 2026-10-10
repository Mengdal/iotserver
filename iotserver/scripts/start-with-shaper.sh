#!/usr/bin/env bash
# 启动 iotserver-core，并把本地（未入库）的 .env.shaper 里的 Shaper 配置注入环境变量。
# Shaper 的 API Key 只走环境变量，不写进 configuration.toml，避免密钥进仓库。
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE=".env.shaper"
if [ -f "${ENV_FILE}" ]; then
  set -a
  # shellcheck disable=SC1090
  . "./${ENV_FILE}"
  set +a
  APIKEY="${IOTSERVER_SHAPER_APIKEY:-}"
  DASHID="${IOTSERVER_SHAPER_DASHBOARDID:-<unset>}"
  echo "[start-with-shaper] loaded ${ENV_FILE}: apiKey=${#APIKEY} chars, dashboardId=${DASHID}"
else
  echo "[start-with-shaper] ${ENV_FILE} not found; use configuration.toml values instead (dashboard page will show setup hints)"
  echo "                    copy .env.shaper.example to .env.shaper and fill ApiKey / DashboardId"
fi

CONFIG="cmd/iotserver-core/res/configuration.toml"
if [ -x "./iotserver-core" ]; then
  exec ./iotserver-core -c "${CONFIG}"
fi
echo "[start-with-shaper] ./iotserver-core not found, falling back to go run"
exec go run cmd/iotserver-core/main.go -c "${CONFIG}"
