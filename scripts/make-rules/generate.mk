BUF := $(TOOLS_DIR)/buf

.PHONY: proto-fmt
proto-fmt: $(BUF) ## 格式化 Proto 文件
	@$(BUF) format -w

.PHONY: proto-lint
proto-lint: $(BUF) ## 运行 Proto 静态检查
	@$(BUF) lint
	@$(BUF) format --diff --exit-code

.PHONY: generate
generate: $(BUF) $(TOOLS_DIR)/protoc-gen-go $(TOOLS_DIR)/protoc-gen-go-grpc $(TOOLS_DIR)/protoc-gen-go-http $(TOOLS_DIR)/wire $(TOOLS_DIR)/sqlc ## 生成 API、Client 和依赖装配代码
	@mkdir -p $(TOOLS_DIR)
	@PATH="$(TOOLS_DIR):$$PATH" $(BUF) generate --template buf.gen.yaml
	@$(TOOLS_DIR)/sqlc generate
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) bash hack/update-codegen.sh
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/apiserver
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/adminapi
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/assistant
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/console
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/als
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/analytics
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/aiextproc
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/authz
	@$(GO_ENV) $(TOOLS_DIR)/wire ./internal/controller

.PHONY: verify-generated
verify-generated: ## 验证生成代码没有漂移
	@before="$$( { find api -name '*.pb.go'; find internal/apiserver/conf -name '*.pb.go'; find internal/adminapi/conf -name '*.pb.go'; find internal/assistant/conf -name '*.pb.go'; find internal/console/conf -name '*.pb.go'; find internal/als/conf -name '*.pb.go'; find internal/analytics/conf -name '*.pb.go'; find internal/aiextproc/conf -name '*.pb.go'; find internal/authz/conf -name '*.pb.go'; find internal/controller/conf -name '*.pb.go'; find internal/assistant/data/mysql/db -name '*.go'; find internal/pkg/generated -name '*.go'; find internal/pkg/apis -name 'zz_generated*.go'; find internal/apiserver internal/adminapi internal/assistant internal/console internal/als internal/analytics internal/aiextproc internal/authz internal/controller -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	$(MAKE) generate >/dev/null; \
	after="$$( { find api -name '*.pb.go'; find internal/apiserver/conf -name '*.pb.go'; find internal/adminapi/conf -name '*.pb.go'; find internal/assistant/conf -name '*.pb.go'; find internal/console/conf -name '*.pb.go'; find internal/als/conf -name '*.pb.go'; find internal/analytics/conf -name '*.pb.go'; find internal/aiextproc/conf -name '*.pb.go'; find internal/authz/conf -name '*.pb.go'; find internal/controller/conf -name '*.pb.go'; find internal/assistant/data/mysql/db -name '*.go'; find internal/pkg/generated -name '*.go'; find internal/pkg/apis -name 'zz_generated*.go'; find internal/apiserver internal/adminapi internal/assistant internal/console internal/als internal/analytics internal/aiextproc internal/authz internal/controller -name 'wire_gen.go'; } | sort | xargs shasum 2>/dev/null || true)"; \
	[[ "$$before" == "$$after" ]] || { echo 'generated files are out of date'; exit 1; }
