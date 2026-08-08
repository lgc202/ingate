.PHONY: generate
generate: $(TOOLS_DIR)/sqlc $(TOOLS_DIR)/buf $(TOOLS_DIR)/protoc-gen-go $(TOOLS_DIR)/protoc-gen-go-http $(TOOLS_DIR)/wire ## 生成 API、Client、依赖装配和数据访问代码
	@mkdir -p $(TOOLS_DIR)
	@PATH="$(TOOLS_DIR):$$PATH" $(TOOLS_DIR)/buf generate --template buf.gen.yaml
	@PATH="$(TOOLS_DIR):$$PATH" $(TOOLS_DIR)/buf generate --template buf.gen.config.yaml
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) bash hack/update-codegen.sh
	@$(GO_ENV) $(TOOLS_DIR)/sqlc generate
	@$(GO_ENV) $(TOOLS_DIR)/wire ./cmd/ingate-admin-api

.PHONY: verify-generated
verify-generated: ## 验证生成代码没有漂移
	@before="$$( { find api -name '*.pb.go'; find internal/adminapi/conf -name '*.pb.go'; find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; find internal/adminapi/data/dao/accesskey/sqlc -name '*.go'; find cmd/ingate-admin-api -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	$(MAKE) generate >/dev/null; \
	after="$$( { find api -name '*.pb.go'; find internal/adminapi/conf -name '*.pb.go'; find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; find internal/adminapi/data/dao/accesskey/sqlc -name '*.go'; find cmd/ingate-admin-api -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	[[ "$$before" == "$$after" ]] || { echo 'generated files are out of date'; exit 1; }
