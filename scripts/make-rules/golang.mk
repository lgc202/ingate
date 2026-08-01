GO ?= go
GOFMT ?= gofmt
GO_TOOLCHAIN := go1.26.0
GO_ENV := GOTOOLCHAIN=$(GO_TOOLCHAIN)

.PHONY: fmt
fmt: ## 格式化 Go 代码
	@git ls-files -co --exclude-standard -z -- '*.go' | xargs -0 $(GOFMT) -w

.PHONY: vet
vet: ## 运行 go vet
	@$(GO_ENV) $(GO) vet ./...

.PHONY: test
test: ## 编译检查全部 Go package
	@$(GO_ENV) $(GO) test ./...

.PHONY: build
build: ## 构建全部 Go 服务
	@mkdir -p $(OUTPUT_DIR)/bin
	@$(GO_ENV) $(GO) build -trimpath -ldflags "$(VERSION_LDFLAGS)" -o $(OUTPUT_DIR)/bin/ ./cmd/...
