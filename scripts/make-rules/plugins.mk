PLUGIN_OUTPUT_DIR := $(OUTPUT_DIR)/plugins

.PHONY: plugins-build
plugins-build: ## 构建全部内置 Wasm 插件
	@mkdir -p $(PLUGIN_OUTPUT_DIR)
	@$(GO_ENV) GOOS=wasip1 GOARCH=wasm $(GO) build -trimpath -buildmode=c-shared -o $(PLUGIN_OUTPUT_DIR)/ratelimit.wasm ./plugins/ratelimit
	@$(GO_ENV) GOOS=wasip1 GOARCH=wasm $(GO) build -trimpath -buildmode=c-shared -o $(PLUGIN_OUTPUT_DIR)/tokenquota.wasm ./plugins/tokenquota
	@$(GO_ENV) GOOS=wasip1 GOARCH=wasm $(GO) build -trimpath -buildmode=c-shared -o $(PLUGIN_OUTPUT_DIR)/iprestriction.wasm ./plugins/iprestriction
