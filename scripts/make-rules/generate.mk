.PHONY: generate
generate: $(TOOLS_DIR)/buf $(TOOLS_DIR)/protoc-gen-go $(TOOLS_DIR)/protoc-gen-go-grpc $(TOOLS_DIR)/protoc-gen-go-http $(TOOLS_DIR)/wire ## 生成 API、Client 和依赖装配代码
	@mkdir -p $(TOOLS_DIR)
	@PATH="$(TOOLS_DIR):$$PATH" $(TOOLS_DIR)/buf generate --template buf.gen.yaml
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) bash hack/update-codegen.sh
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/apiserver
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/adminapi
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/als
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/analytics
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/controller

.PHONY: verify-generated
verify-generated: ## 验证生成代码没有漂移
	@before="$$( { find api -name '*.pb.go'; find internal/apiserver/conf -name '*.pb.go'; find internal/adminapi/conf -name '*.pb.go'; find internal/als/conf -name '*.pb.go'; find internal/analytics/conf -name '*.pb.go'; find internal/controller/conf -name '*.pb.go'; find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; find internal/apiserver internal/adminapi internal/als internal/analytics internal/controller -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	$(MAKE) generate >/dev/null; \
	after="$$( { find api -name '*.pb.go'; find internal/apiserver/conf -name '*.pb.go'; find internal/adminapi/conf -name '*.pb.go'; find internal/als/conf -name '*.pb.go'; find internal/analytics/conf -name '*.pb.go'; find internal/controller/conf -name '*.pb.go'; find pkg/generated -name '*.go'; find pkg/apis -name 'zz_generated*.go'; find internal/apiserver internal/adminapi internal/als internal/analytics internal/controller -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	[[ "$$before" == "$$after" ]] || { echo 'generated files are out of date'; exit 1; }
