# Makefile for sshwarden

# Go 相关变量
GO := go
GOPATH := $(shell go env GOPATH)
GOBIN := $(GOPATH)/bin

# 项目变量
APP_NAME := sshwarden
CMD_DIR := ./cmd/sshwarden

# 构建变量
OUTPUT_DIR := ./bin
OUTPUT_BIN := $(OUTPUT_DIR)/$(APP_NAME)

# 测试变量
TEST_DIRS := ./internal/...

# 格式化变量
FMT_DIRS := ./cmd ./internal

# 目标
.PHONY: all build test clean fmt vet lint run help install uninstall

# 默认目标
all: build

# 构建项目
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(OUTPUT_DIR)
	@$(GO) build -o $(OUTPUT_BIN) $(CMD_DIR)
	@echo "Build completed successfully!"

# 运行测试
test:
	@echo "Running tests..."
	@$(GO) test -v $(TEST_DIRS)

# 清理构建产物
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(OUTPUT_DIR)
	@echo "Clean completed!"

# 格式化代码
fmt:
	@echo "Formatting code..."
	@$(GO) fmt $(FMT_DIRS)
	@echo "Format completed!"

# 运行 go vet
vet:
	@echo "Running go vet..."
	@$(GO) vet $(FMT_DIRS)
	@echo "go vet completed!"

# 运行 golangci-lint (如果安装了)
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# 运行应用
run:
	@echo "Running $(APP_NAME)..."
	@$(GO) run $(CMD_DIR)

# 显示帮助信息
help:
	@echo "sshwarden Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  all     - Build the project"
	@echo "  build   - Build the project"
	@echo "  test    - Run tests"
	@echo "  clean   - Clean build artifacts"
	@echo "  fmt     - Format code"
	@echo "  vet     - Run go vet"
	@echo "  lint    - Run golangci-lint"
	@echo "  run     - Run the application"
	@echo "  help    - Show this help message"
	@echo "  install - Install as system service"
	@echo "  uninstall - Uninstall system service"

# 安装为系统服务
install:
	@echo "Installing $(APP_NAME) as system service..."
	@sudo $(OUTPUT_BIN) install

# 卸载系统服务
uninstall:
	@echo "Uninstalling $(APP_NAME) system service..."
	@sudo $(OUTPUT_BIN) uninstall
