COMPOSE_FILE := $(PROJECT_ROOT)/deploy/docker-compose.yaml
COMPOSE := docker compose --project-directory $(PROJECT_ROOT) -f $(COMPOSE_FILE)

.PHONY: docker-up
docker-up: ## 构建并启动开发联调环境
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
