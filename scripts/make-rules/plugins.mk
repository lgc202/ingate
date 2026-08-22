WASM_PLUGIN_OUTPUT_DIR := $(OUTPUT_DIR)/plugins

.PHONY: wasm-plugins
wasm-plugins: ## 构建 Ingate 维护的标准 Proxy-Wasm 插件
	@mkdir -p $(WASM_PLUGIN_OUTPUT_DIR)
	@$(GO_ENV) GOOS=wasip1 GOARCH=wasm $(GO) build -buildmode=c-shared -trimpath \
		-o $(WASM_PLUGIN_OUTPUT_DIR)/transformer.wasm ./plugins/transformer
