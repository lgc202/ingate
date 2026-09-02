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

BUF_VERSION := v1.59.0
BUF_PACKAGE := github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_GO_PACKAGE := google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
PROTOC_GEN_GO_GRPC_PACKAGE := google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
PROTOC_GEN_GO_HTTP_VERSION := v3.0.0-20260526000039-30da04b769dc
PROTOC_GEN_GO_HTTP_PACKAGE := github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@$(PROTOC_GEN_GO_HTTP_VERSION)
PROTOC_GEN_GO_ERRORS_VERSION := v3.0.0-20260626125723-668db92c2c00
PROTOC_GEN_GO_ERRORS_PACKAGE := github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v3@$(PROTOC_GEN_GO_ERRORS_VERSION)
WIRE_VERSION := v0.7.0
WIRE_PACKAGE := github.com/google/wire/cmd/wire@$(WIRE_VERSION)
SQLC_VERSION := v1.31.1
SQLC_PACKAGE := github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
GOLANGCI_LINT_VERSION := v2.13.2
GOLANGCI_LINT_PACKAGE := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
ACTIONLINT_VERSION := v1.7.12
ACTIONLINT_PACKAGE := github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
GOVULNCHECK_VERSION := v1.7.0
GOVULNCHECK_PACKAGE := golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
GO_TOOL_PREREQUISITES := \
	$(PROJECT_ROOT)/scripts/make-rules/golang.mk \
	$(PROJECT_ROOT)/scripts/make-rules/tools.mk

.PHONY: tools
tools: $(TOOLS_DIR)/buf $(TOOLS_DIR)/protoc-gen-go $(TOOLS_DIR)/protoc-gen-go-grpc $(TOOLS_DIR)/protoc-gen-go-http $(TOOLS_DIR)/protoc-gen-go-errors $(TOOLS_DIR)/wire $(TOOLS_DIR)/sqlc $(TOOLS_DIR)/golangci-lint $(TOOLS_DIR)/actionlint $(TOOLS_DIR)/govulncheck ## 安装项目开发工具
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(KUBE_CODEGEN_PACKAGES)

$(TOOLS_DIR)/buf: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(BUF_PACKAGE)

$(TOOLS_DIR)/protoc-gen-go: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(PROTOC_GEN_GO_PACKAGE)

$(TOOLS_DIR)/protoc-gen-go-grpc: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(PROTOC_GEN_GO_GRPC_PACKAGE)

$(TOOLS_DIR)/protoc-gen-go-http: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(PROTOC_GEN_GO_HTTP_PACKAGE)

$(TOOLS_DIR)/protoc-gen-go-errors: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(PROTOC_GEN_GO_ERRORS_PACKAGE)

$(TOOLS_DIR)/wire: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(WIRE_PACKAGE)

$(TOOLS_DIR)/sqlc: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(SQLC_PACKAGE)

$(TOOLS_DIR)/golangci-lint: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(GOLANGCI_LINT_PACKAGE)

$(TOOLS_DIR)/actionlint: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(ACTIONLINT_PACKAGE)

$(TOOLS_DIR)/govulncheck: $(GO_TOOL_PREREQUISITES)
	@mkdir -p $(TOOLS_DIR)
	@$(GO_ENV) GOBIN=$(TOOLS_DIR) $(GO) install $(GOVULNCHECK_PACKAGE)

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
