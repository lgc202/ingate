.PHONY: lint
lint: go-lint proto-lint workflow-lint ## 运行 Go、Proto 和 GitHub Actions 静态检查

.PHONY: workflow-lint
workflow-lint: $(TOOLS_DIR)/actionlint ## 检查 GitHub Actions 工作流
	@$(TOOLS_DIR)/actionlint

.PHONY: vuln
vuln: $(TOOLS_DIR)/govulncheck ## 检查 Go 代码实际可达的已知漏洞
	@$(GO_ENV) $(TOOLS_DIR)/govulncheck ./...

.PHONY: verify
verify: lint verify-generated test build console-build prototype-build ## 执行提交前质量检查
