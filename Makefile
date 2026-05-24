.PHONY: all build clean run dev test swagger swagger-check swagger-fmt swagger-clean swagger-rebuild swagger-install help

# 变量定义
BINARY_API=bin/api-server
CMD_API=cmd/api-server/main.go
GOBIN=$(shell go env GOPATH)/bin

# 默认目标
all: build

# 创建bin目录
bin:
	@mkdir -p bin

# ==================== 编译 ====================
build-api: bin
	@echo "🔨 Building API Server..."
	@go build -o $(BINARY_API) $(CMD_API)
	@echo "✅ API Server built: $(BINARY_API)"

build: build-api
	@echo "✅ All services built"

# ==================== 运行 ====================
run-api:
	@echo "🚀 Starting API Server..."
	@$(BINARY_API)

dev-api:
	@echo "🔧 Starting API Server (dev mode)..."
	@go run $(CMD_API)

dev: build
	@echo "🚀 Starting all services..."
	@$(BINARY_API) &
	@echo "✅ Services started (use 'make stop' to stop)"

# ==================== 控制 ====================
stop:
	@echo "🛑 Stopping services..."
	@-pkill -f "$(BINARY_API)" || true
	@echo "✅ Services stopped"

restart: stop dev

status:
	@echo "📊 Service Status:"
	@pgrep -fl "$(BINARY_API)" || echo "API Server: Not running"

# ==================== 数据库 ====================
migrate:
	@echo "🗄️  Running migrations..."
	@mysql -u root -p < sql/init.sql
	@echo "✅ Migrations completed"

# ==================== 测试 ====================
test:
	@echo "🧪 Running tests..."
	@go test -v ./...

test-cover:
	@echo "🧪 Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

# ==================== 代码质量 ====================
fmt:
	@echo "💅 Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

vet:
	@echo "🔍 Running go vet..."
	@go vet ./...
	@echo "✅ Vet completed"

lint:
	@echo "🔍 Running linter..."
	@golangci-lint run || echo "⚠️  Install: https://golangci-lint.run/usage/install/"

check: fmt vet test

# ==================== Swagger ====================
swagger-check:
	@test -f $(GOBIN)/swag || (echo "❌ swag not found. Run 'make swagger-install' first" && exit 1)

swagger: swagger-check
	@echo "📝 Generating Swagger docs..."
	@$(GOBIN)/swag init -g cmd/api-server/main.go -o docs --parseDependency --parseInternal
	@echo "✅ Swagger docs generated at docs/"
	@echo "📄 Files: docs.go, swagger.json, swagger.yaml"
	@echo "🌐 Visit: http://localhost:8080/swagger/index.html"

swagger-fmt: swagger-check
	@echo "💅 Formatting Swagger annotations..."
	@$(GOBIN)/swag fmt -g cmd/api-server/main.go
	@echo "✅ Swagger annotations formatted"

swagger-clean:
	@echo "🧹 Cleaning old Swagger docs..."
	@rm -f docs/docs.go docs/swagger.json docs/swagger.yaml
	@echo "✅ Old Swagger docs cleaned"

swagger-rebuild: swagger-clean swagger
	@echo "✅ Swagger docs rebuilt"

swagger-install:
	@echo "📦 Installing swag..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "✅ Swag installed at $(GOBIN)/swag"
	@echo "💡 Tip: Run 'make swagger' to generate docs"

# ==================== 依赖 ====================
deps:
	@echo "📦 Downloading dependencies..."
	@go mod download
	@echo "✅ Dependencies downloaded"

tidy:
	@echo "📦 Tidying dependencies..."
	@go mod tidy
	@echo "✅ Dependencies tidied"

# ==================== Docker ====================
docker-build:
	@echo "🐳 Building Docker image..."
	@docker build -t keyops:latest .
	@echo "✅ Docker image built"

docker-up:
	@echo "🐳 Starting containers..."
	@docker-compose up -d
	@echo "✅ Containers started"

docker-down:
	@echo "🐳 Stopping containers..."
	@docker-compose down
	@echo "✅ Containers stopped"

# ==================== 清理 ====================
clean:
	@echo "🧹 Cleaning..."
	@rm -rf bin/ coverage.out coverage.html
	@rm -f docs/docs.go docs/swagger.json docs/swagger.yaml
	@echo "✅ Clean completed (kept docs/*.md files)"

# ==================== 帮助 ====================
help:
	@echo "╔══════════════════════════════════════════════╗"
	@echo "║     KeyOps Backend - Makefile Commands      ║"
	@echo "╚══════════════════════════════════════════════╝"
	@echo ""
	@echo "🔨 编译:"
	@echo "  make build         - 编译所有服务"
	@echo "  make build-api     - 编译 API Server"
	@echo ""
	@echo "🚀 运行:"
	@echo "  make run-api       - 运行 API Server"
	@echo "  make dev-api       - 开发模式运行 API"
	@echo "  make dev           - 启动所有服务"
	@echo ""
	@echo "🔄 控制:"
	@echo "  make stop          - 停止所有服务"
	@echo "  make restart       - 重启所有服务"
	@echo "  make status        - 查看服务状态"
	@echo ""
	@echo "📝 Swagger:"
	@echo "  make swagger         - 生成 API 文档"
	@echo "  make swagger-fmt     - 格式化 Swagger 注释"
	@echo "  make swagger-clean   - 清理旧的 Swagger 文档"
	@echo "  make swagger-rebuild - 清理并重新生成文档"
	@echo "  make swagger-install - 安装 swag 工具"
	@echo ""
	@echo "🧪 测试:"
	@echo "  make test          - 运行测试"
	@echo "  make test-cover    - 运行测试并生成覆盖率报告"
	@echo "  make check         - 格式化+检查+测试"
	@echo ""
	@echo "🗄️  数据库:"
	@echo "  make migrate       - 运行数据库迁移"
	@echo ""
	@echo "📦 依赖:"
	@echo "  make deps          - 下载依赖"
	@echo "  make tidy          - 整理依赖"
	@echo ""
	@echo "🐳 Docker:"
	@echo "  make docker-build  - 构建镜像"
	@echo "  make docker-up     - 启动容器"
	@echo "  make docker-down   - 停止容器"
	@echo ""
	@echo "🧹 清理:"
	@echo "  make clean         - 清理编译产物"
	@echo ""
