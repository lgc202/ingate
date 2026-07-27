CONSOLE_DIR := $(PROJECT_ROOT)/web/console

.PHONY: console-build
console-build: ## 构建 Console 前端
	@cd $(CONSOLE_DIR) && npm ci --no-audit --no-fund
	@cd $(CONSOLE_DIR) && npm run build
