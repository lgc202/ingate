.PHONY: lint
lint: go-lint proto-lint workflow-lint ## 运行 Go、Proto 和 GitHub Actions 静态检查

.PHONY: go-declaration-order
go-declaration-order: ## 检查手写 Go 文件的顶层声明组织
	@$(GO_ENV) $(GO) run ./hack/verify-go-declarations .

.PHONY: go-docs
go-docs: ## 检查手写 Go 代码的包与导出声明文档
	@$(GO_ENV) $(GO) run ./hack/verify-go-docs .

.PHONY: workflow-lint
workflow-lint: $(TOOLS_DIR)/actionlint ## 检查 GitHub Actions 工作流
	@$(TOOLS_DIR)/actionlint

.PHONY: vuln
vuln: $(TOOLS_DIR)/govulncheck ## 检查 Go 代码实际可达的已知漏洞
	@$(GO_ENV) $(TOOLS_DIR)/govulncheck ./...

.PHONY: verify
verify: lint verify-generated test build wasm-plugins console-build prototype-build docs-build ## 执行提交前完整质量检查
