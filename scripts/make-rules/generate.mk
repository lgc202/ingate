.PHONY: generate
generate: $(TOOLS_DIR)/sqlc $(TOOLS_DIR)/wire ## 生成 API、Client、OpenAPI 和依赖装配代码
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) bash hack/update-codegen.sh
	@$(GO_ENV) $(TOOLS_DIR)/sqlc generate
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/admin

.PHONY: verify-generated
verify-generated: ## 验证生成代码没有漂移
	@before="$$( { find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; find internal/admin/store/accesskey/sqlc -name '*.go'; find internal/admin -maxdepth 1 -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	$(MAKE) generate >/dev/null; \
	after="$$( { find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; find internal/admin/store/accesskey/sqlc -name '*.go'; find internal/admin -maxdepth 1 -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	[[ "$$before" == "$$after" ]] || { echo 'generated files are out of date'; exit 1; }
