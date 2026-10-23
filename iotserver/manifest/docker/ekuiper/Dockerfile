# 带 Kafka / InfluxDB sink 插件的 eKuiper 镜像。
#
# 官方 2.4.2 的预编译插件下载地址返回 403，因此这里在构建阶段从 v2.4.2 源码
# 编译同版本插件，保证 Go plugin ABI 与运行镜像一致。
FROM golang:1.25.4-bookworm AS plugin-builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends git zip unzip ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --depth 1 --branch v2.4.2 https://github.com/lf-edge/ekuiper.git .
RUN ./build-plugins.sh sinks kafka \
    && ./build-plugins.sh sinks influx \
    && mkdir -p /plugins \
    && unzip -o _plugins/debian/sinks/kafka_*.zip -d /plugins \
    && unzip -o _plugins/debian/sinks/influx_*.zip -d /plugins \
    && rm -f /plugins/*.go

FROM lfedge/ekuiper:2.4.2-slim

COPY --from=plugin-builder /plugins/ /opt/kuiper-plugins/
COPY entrypoint.sh /usr/local/bin/iotserver-ekuiper-entrypoint.sh
RUN chmod +x /usr/local/bin/iotserver-ekuiper-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/iotserver-ekuiper-entrypoint.sh"]
CMD ["./bin/kuiperd"]
