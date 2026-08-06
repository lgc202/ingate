NODE_MAJOR := 24
NPM_MAJOR := 11
DOCKER_COMPOSE_MIN_MAJOR := 2

# 不追加 @version，让 go install 直接使用 go.mod 锁定的模块版本
KUBE_CODEGEN_PACKAGES := \
	k8s.io/code-generator/cmd/applyconfiguration-gen \
	k8s.io/code-generator/cmd/client-gen \
	k8s.io/code-generator/cmd/conversion-gen \
	k8s.io/code-generator/cmd/deepcopy-gen \
	k8s.io/code-generator/cmd/defaulter-gen \
	k8s.io/code-generator/cmd/informer-gen \
	k8s.io/code-generator/cmd/lister-gen \
	k8s.io/code-generator/cmd/validation-gen \
	k8s.io/kube-openapi/cmd/openapi-gen

SQLC_VERSION := v1.31.1
SQLC_PACKAGE := github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

.PHONY: tools
tools: $(TOOLS_DIR)/sqlc ## 安装项目代码生成工具
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(KUBE_CODEGEN_PACKAGES)

$(TOOLS_DIR)/sqlc: $(PROJECT_ROOT)/scripts/make-rules/tools.mk
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(SQLC_PACKAGE)

.PHONY: check-tools
check-tools: ## 检查本地开发工具
	@command -v bash >/dev/null || { echo 'bash is required'; exit 1; }
	@command -v $(GO) >/dev/null || { echo 'go is required'; exit 1; }
	@command -v node >/dev/null || { echo 'node is required'; exit 1; }
	@command -v npm >/dev/null || { echo 'npm is required'; exit 1; }
	@command -v docker >/dev/null || { echo 'docker is required'; exit 1; }
	@command -v shasum >/dev/null || { echo 'shasum is required'; exit 1; }
	@$(GO_ENV) $(GO) version >/dev/null
	@node_version="$$(node --version)"; \
	node_major="$${node_version#v}"; \
	node_major="$${node_major%%.*}"; \
	[[ "$$node_major" == "$(NODE_MAJOR)" ]] || { \
		echo "node $(NODE_MAJOR).x is required, found $$node_version"; \
		exit 1; \
	}
	@npm_version="$$(npm --version)"; \
	npm_major="$${npm_version%%.*}"; \
	[[ "$$npm_major" == "$(NPM_MAJOR)" ]] || { \
		echo "npm $(NPM_MAJOR).x is required, found $$npm_version"; \
		exit 1; \
	}
	@compose_version="$$(docker compose version --short)"; \
	compose_version="$${compose_version#v}"; \
	compose_major="$${compose_version%%.*}"; \
	(( compose_major >= $(DOCKER_COMPOSE_MIN_MAJOR) )) || { \
		echo "docker compose $(DOCKER_COMPOSE_MIN_MAJOR).x or newer is required, found $$compose_version"; \
		exit 1; \
	}
	@echo 'development tools are ready'
