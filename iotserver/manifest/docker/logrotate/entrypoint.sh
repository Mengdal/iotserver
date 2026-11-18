#!/bin/sh
set -eu

# logrotate 自己按文件大小判断是否轮转；这里每小时触发一次即可。
while true; do
  logrotate /etc/logrotate.conf || true
  sleep 3600
done
