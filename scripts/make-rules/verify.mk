.PHONY: verify
verify: fmt vet verify-generated test build plugins-build console-build ## 执行提交前质量检查
