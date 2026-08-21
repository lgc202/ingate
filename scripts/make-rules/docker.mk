COMPOSE_FILE := $(PROJECT_ROOT)/deploy/docker-compose.yaml
COMPOSE_DEV_FILE := $(PROJECT_ROOT)/deploy/docker-compose.dev.yaml
COMPOSE := docker compose --project-directory $(PROJECT_ROOT) -f $(COMPOSE_FILE) -f $(COMPOSE_DEV_FILE)
DOCKER_OUTPUT_DIR := $(OUTPUT_DIR)/docker
DOCKER_GOARCH ?= $(shell docker version --format '{{.Server.Arch}}')

.PHONY: docker-artifacts
docker-artifacts: console-build ## 构建开发 Compose 使用的 Linux 二进制和前端静态资源
	@rm -rf $(DOCKER_OUTPUT_DIR)
	@mkdir -p $(DOCKER_OUTPUT_DIR)/bin $(DOCKER_OUTPUT_DIR)/web
	@$(GO_ENV) CGO_ENABLED=0 GOOS=linux GOARCH=$(DOCKER_GOARCH) $(GO) build -p=4 -trimpath -ldflags "-s -w $(VERSION_LDFLAGS)" -o $(DOCKER_OUTPUT_DIR)/bin/ ./cmd/...
	@cp -R $(CONSOLE_DIR)/dist/. $(DOCKER_OUTPUT_DIR)/web/

.PHONY: docker-up
docker-up: docker-artifacts ## 构建并启动开发联调环境
	@$(COMPOSE) up -d --build --remove-orphans

.PHONY: docker-down
docker-down: ## 停止开发联调环境
	@$(COMPOSE) down

.PHONY: docker-logs
docker-logs: ## 持续查看开发联调环境日志
	@$(COMPOSE) logs -f

.PHONY: docker-ps
docker-ps: ## 查看开发联调环境组件状态
	@$(COMPOSE) ps
