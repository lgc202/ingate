.DEFAULT_GOAL := help

include scripts/make-rules/common.mk
include scripts/make-rules/tools.mk
include scripts/make-rules/version.mk
include scripts/make-rules/golang.mk
include scripts/make-rules/generate.mk
include scripts/make-rules/dev.mk

.PHONY: generate lint verify

generate: $(GENERATE_TARGETS) ## 重新生成 protobuf 与 Kubernetes API 代码

lint: go-lint $(LINT_TARGETS) ## 执行 Go 与 protobuf 静态检查

verify: fmt-check vet lint test build ## 检查格式、静态分析、普通测试与构建，不更新源码
