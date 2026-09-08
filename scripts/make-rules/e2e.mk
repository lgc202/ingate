.PHONY: adminapi-e2e als-e2e
adminapi-e2e: ## 通过 Console 会话验证核心 Admin API 旅程
	@$(GO_ENV) $(GO) run ./hack/adminapi-e2e

als-e2e: ## 在隔离的本地环境演练 ALS 可靠性与可观测性故障
	@$(GO_ENV) GO="$(GO)" bash hack/als-e2e/run.sh
