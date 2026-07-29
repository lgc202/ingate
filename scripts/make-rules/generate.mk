.PHONY: generate
generate: ## 生成 API、Client 和 OpenAPI 代码
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) bash hack/update-codegen.sh

.PHONY: verify-generated
verify-generated: ## 验证生成代码没有漂移
	@before="$$( { find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	$(MAKE) generate >/dev/null; \
	after="$$( { find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	[[ "$$before" == "$$after" ]] || { echo 'generated files are out of date'; exit 1; }
