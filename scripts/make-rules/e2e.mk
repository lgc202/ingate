.PHONY: adminapi-e2e
adminapi-e2e: ## 通过 Console 会话验证核心 Admin API 旅程
	@$(GO_ENV) $(GO) run ./hack/adminapi-e2e
