#!/bin/sh
# 把镜像内置的 Kafka / InfluxDB 插件复制到 eKuiper 的持久化插件目录。
# 使用 -n 避免覆盖用户后来手动安装/升级的同名插件。
set -e

mkdir -p /kuiper/plugins/sinks /kuiper/etc/sinks
for f in /opt/kuiper-plugins/*.so; do
    [ -e "$f" ] || continue
    cp -n "$f" /kuiper/plugins/sinks/ 2>/dev/null || true
done
for f in /opt/kuiper-plugins/*.json; do
    [ -e "$f" ] || continue
    cp -n "$f" /kuiper/etc/sinks/ 2>/dev/null || true
done

exec /usr/bin/docker-entrypoint.sh "$@"
