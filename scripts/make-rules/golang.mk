.PHONY: build $(BINS) test fmt fmt-check vet go-lint

build: $(BINS) ## 构建全部或 BINS 指定的二进制，输出到 bin/

$(BINS):
	mkdir -p bin
	$(GO) build -mod=readonly -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o bin/$@ ./cmd/$@

test: ## 运行不需要外部服务的普通测试
	$(GO) test -mod=readonly ./...

fmt: ## 使用 gofmt 格式化 Go 源码
	$(GOFMT) -w $(GO_DIRS)

fmt-check: ## 检查 Go 格式，不修改源码
	@files="$$($(GOFMT) -l $(GO_DIRS))"; \
	if test -n "$$files"; then \
		printf '以下文件需要执行 make fmt：\n%s\n' "$$files" >&2; \
		exit 1; \
	fi

vet: ## 运行 go vet 静态分析
	$(GO) vet -mod=readonly ./...

go-lint: ## 使用固定版本的 golangci-lint 检查 Go 代码
	$(GOLANGCI_LINT) run --allow-serial-runners --config .golangci.yml ./...
