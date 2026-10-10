.PHONY: help build build-linux run start ui clean test fmt generate/api docker

# 本地二进制名（go build 默认输出到当前目录）
BIN ?= iotserver-core
# 前端产物目录与后端托管目录
UI_SRC ?= ../iotserver-ui/build
UI_DST ?= cmd/iotserver-ui/build
# 镜像地址：推送前用 make docker IMAGE=<你的仓库>/iotserver-core:v1.0 覆盖
IMAGE ?= iotserver-core:v1.0

help:
	@echo "可用目标："
	@echo "  make run          本地编译并启动（推荐，产物为 ./$(BIN)）"
	@echo "  make build        编译当前平台二进制到 ./$(BIN)"
	@echo "  make build-linux  交叉编译 linux/amd64（需 linux C 工具链）"
	@echo "  make ui           构建前端并同步到 $(UI_DST)"
	@echo "  make start        用 go run 直接启动（开发用）"
	@echo "  make test         运行单元/集成测试"
	@echo "  make fmt          格式化 Go 代码"
	@echo "  make docker       构建并推送 Docker 镜像（先跑 make ui；镜像 $(IMAGE)）"
	@echo "  make start-shaper 读取 .env.shaper 注入 Shaper 配置后启动"
	@echo "  make clean        清理二进制"

# 本地编译并启动
run: $(BIN)
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
ui:
	cd ../iotserver-ui && npm run build
	rm -rf $(UI_DST) && mkdir -p $(UI_DST) && cp -R $(UI_SRC)/. $(UI_DST)/
	@echo "前端产物已同步到 $(UI_DST)"

start:
	go run cmd/iotserver-core/main.go -c cmd/iotserver-core/res/configuration.toml

# 带 Shaper 配置启动（从本地未入库的 .env.shaper 注入 API Key / DashboardId）
.PHONY: start-shaper
start-shaper:
	./scripts/start-with-shaper.sh

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
