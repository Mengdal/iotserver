.PHONY: help build build-linux run start ui ui-ensure build-ui sync-ui clean test fmt generate/api docker
# 本地二进制名（go build 默认输出到当前目录）
BIN ?= iotserver-core
# 前端产物目录与后端托管目录
UI_SRC ?= ../iotserver-ui/build
UI_DST ?= cmd/iotserver-ui/build
# 镜像地址：推送前用 make docker IMAGE=<你的仓库>/iotserver-core:v1.0 覆盖
IMAGE ?= iotserver-core:v1.0

# Docker socket 地址：平台要靠它拉驱动镜像。
# macOS 上 Docker Desktop 的 socket 不在 /var/run/docker.sock，需要显式给 DOCKER_HOST；
# Linux 与容器内保持默认即可（下面自动探测）。要手动指定就 make start DOCKER_HOST=...
DOCKER_HOST ?= $(shell if [ -S /var/run/docker.sock ]; then echo unix:///var/run/docker.sock; else echo unix://$$HOME/.docker/run/docker.sock; fi)
export DOCKER_HOST

help:
	@echo "可用目标："
	@echo "  make run          本地编译并启动（推荐，产物为 ./$(BIN)；会自动保证前端最新）"
	@echo "  make build        编译当前平台二进制到 ./$(BIN)"
	@echo "  make build-linux  交叉编译 linux/amd64（需 linux C 工具链）"
	@echo "  make ui           构建前端并同步到 $(UI_DST)"
	@echo "  make sync-ui      只同步前端产物（已构建过、只差拷贝时用）"
	@echo "  make start        用 go run 直接启动（开发用；会自动保证前端最新）"
	@echo "  make test         运行单元/集成测试"
	@echo "  make fmt          格式化 Go 代码"
	@echo "  make docker       构建并推送 Docker 镜像（先跑 make ui；镜像 $(IMAGE)）"
	@echo "  make clean        清理二进制"

# 本地编译并启动（先保证前端产物最新）
run: ui-ensure $(BIN)
	./$(BIN) -c cmd/iotserver-core/res/configuration.toml

$(BIN):
	go build -o $(BIN) ./cmd/iotserver-core

build: $(BIN)

# 交叉编译 linux/amd64（部署用）。
# ducklake 依赖 CGO + DuckDB 静态库，需要 linux 版 C 工具链
# （本机 macOS 无法直接交叉编译，建议在容器里执行：见 Dockerfile）
build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o $(BIN)-linux-amd64 ./cmd/iotserver-core

# 前端：构建 + 同步到后端静态托管目录
ui: build-ui sync-ui

# 只构建（CRA 产物落在 $(UI_SRC)）
build-ui:
	cd ../iotserver-ui && npm run build

# 只同步（把 $(UI_SRC) 整个拷到 $(UI_DST)，与 make ui 的拷贝步骤一致）
sync-ui:
	rm -rf $(UI_DST) && mkdir -p $(UI_DST) && cp -R $(UI_SRC)/. $(UI_DST)/
	@echo "前端产物已同步到 $(UI_DST)"

# 启动前兜底：保证 $(UI_DST) 反映最新的前端源码。
#   源码比产物新（或产物缺失）→ 完整构建 + 同步；
#   产物比托管目录新           → 只同步（便宜）；
#   都最新                     → 跳过。
# 页面是服务运行时从磁盘读的：同步后浏览器强刷即可生效，后端代码变了才需要重启。
ui-ensure:
	@if [ ! -f "$(UI_SRC)/index.html" ] \
		|| [ -n "$$(find ../iotserver-ui/src ../iotserver-ui/public -type f -newer "$(UI_SRC)/index.html" 2>/dev/null | head -n 1)" ]; then \
		echo "前端源码有更新，重新构建并同步..."; \
		$(MAKE) --no-print-directory ui; \
	elif [ ! -f "$(UI_DST)/index.html" ] \
		|| [ -n "$$(find "$(UI_SRC)" -type f -newer "$(UI_DST)/index.html" 2>/dev/null | head -n 1)" ]; then \
		$(MAKE) --no-print-directory sync-ui; \
	else \
		echo "前端产物已是最新，跳过构建/同步"; \
	fi

start: ui-ensure
	go run cmd/iotserver-core/main.go -c cmd/iotserver-core/res/configuration.toml

test:
	# 覆盖全部后端包（含 application 层的 resourceapp/ruleengineapp/sceneapp 与 tools/healthcheck）
	go test ./internal/... -count=1

fmt:
	gofmt -w cmd internal tools

generate/api:
	cd cmd/iotserver-core && swag init --parseDependency --parseInternal --parseDepth 10

# 构建并推送镜像。注意：Dockerfile 会把 $(UI_DST) 复制进镜像，
# 因此首次构建前必须先执行 make ui（否则构建会失败）
docker:
	@test -f $(UI_DST)/index.html || (echo "缺少前端产物 $(UI_DST)，请先执行: make ui"; exit 1)
	docker buildx build --platform linux/amd64 -t "$(IMAGE)" -f cmd/iotserver-core/Dockerfile --push .

clean:
	rm -f $(BIN) $(BIN)-linux-amd64 cmd/iotserver-core/iotserver-core
