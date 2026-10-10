#  ----------------------------------------------------------------------------------
#  Copyright 2018 iotserver
#  Copyright 2018 Cavium
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
#
#  ----------------------------------------------------------------------------------
#
#  注意：ducklake 数据层依赖官方 DuckDB Go 驱动（github.com/duckdb/duckdb-go/v2），
#  该驱动要求 Go >= 1.24 且需要 CGO，因此：
#    1) 构建镜像必须是 Go >= 1.24 且带 C 工具链（bookworm 系列自带 gcc）；
#    2) 运行镜像需要 glibc —— DuckDB 的静态库按 glibc 构建，不要用 alpine/musl。
#  内网仓库若有对应的 Go 1.24 / Debian 镜像，覆盖下面的 ARG 即可。

ARG BUILDER_BASE=golang:1.24-bookworm
ARG RUNTIME_BASE=debian:bookworm-slim

FROM ${BUILDER_BASE} AS builder

WORKDIR /edge

COPY . .

RUN make build

#Next image - Copy built Go binary into new workspace
FROM ${RUNTIME_BASE}

ENV APP_PORT=58081
ENV WEB_PORT=3000
EXPOSE $APP_PORT
EXPOSE $WEB_PORT

RUN set -eux; \
    if command -v apt-get >/dev/null 2>&1; then \
      apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && \
      rm -rf /var/lib/apt/lists/*; \
    fi

WORKDIR /var/bin/


COPY --from=builder /edge/iotserver-core /bin/
COPY --from=builder /edge/cmd/iotserver-ui/build /var/bin/cmd/iotserver-ui/build

CMD ["/bin/sh", "-c", "/bin/iotserver-core --confdir=/etc/iotserver-core/configuration.toml"]
