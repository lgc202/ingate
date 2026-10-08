BUF_VERSION := v1.73.0
BUF = $(GO) run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
GENERATE_TARGETS += generate-proto generate-kubernetes
LINT_TARGETS += proto-lint

.PHONY: generate-proto generate-kubernetes proto-lint

generate-proto: ## 按 buf.gen.yaml 重新生成 protobuf Go 代码
	$(BUF) generate

generate-kubernetes: ## 按现有脚本重新生成 Kubernetes API 与客户端代码
	bash scripts/generate-api.sh

proto-lint: ## 使用固定版本的 Buf 检查 protobuf 契约
	$(BUF) lint
