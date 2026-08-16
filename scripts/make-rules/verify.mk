.PHONY: lint
lint: go-lint proto-lint ## 运行 Go 和 Proto 静态检查

.PHONY: verify
verify: fmt lint verify-generated test build console-build prototype-build ## 执行提交前质量检查
