CONSOLE_DIR := $(PROJECT_ROOT)/web/console
PROTOTYPE_DIR := $(PROJECT_ROOT)/web/prototype
DOCS_DIR := $(PROJECT_ROOT)/docs

.PHONY: console-build
console-build: ## 构建 Console 前端
	@cd $(CONSOLE_DIR) && npm ci --no-audit --no-fund
	@cd $(CONSOLE_DIR) && npm run build

.PHONY: prototype-build
prototype-build: ## 构建独立产品原型
	@cd $(PROTOTYPE_DIR) && npm ci --no-audit --no-fund
	@cd $(PROTOTYPE_DIR) && npm run build

.PHONY: docs-build
docs-build: ## 构建正式文档站点
	@cd $(DOCS_DIR) && npm ci --no-audit --no-fund
	@cd $(DOCS_DIR) && npm run build
