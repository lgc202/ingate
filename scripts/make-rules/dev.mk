.PHONY: dev-init dev-deps-up dev-deps-down dev-deps-status dev-migrate dev-apiserver dev-namespace dev-controller dev-admin dev-web

# 通过环境传递并在脚本中校验，避免把参数插入 Shell 命令字符串。
export COMPONENT
export EXCLUDE

.PHONY: dev-up dev-down dev-start dev-stop dev-status dev-logs

dev-up: ## 一键准备依赖、迁移并启动控制面和前端；EXCLUDE=admin-api 留给 IDE
	bash scripts/dev.sh up

dev-down: ## 停止托管组件和开发依赖，保留数据及日志
	bash scripts/dev.sh down

dev-start: ## 构建并恢复单个托管组件，指定 COMPONENT=admin-api
	bash scripts/dev.sh start

dev-stop: ## 停止单个托管组件，释放端口给 IDE，指定 COMPONENT=admin-api
	bash scripts/dev.sh stop

dev-status: ## 查看本项目托管进程和依赖状态
	bash scripts/dev.sh status

dev-logs: ## 持续查看日志；可指定 COMPONENT=admin-api，Ctrl+C 只停止看日志
	bash scripts/dev.sh logs

dev-init: ## 生成本地开发证书、kubeconfig 和连接配置，不启动服务
	bash scripts/dev.sh init

dev-deps-up: ## 启动开发用 MySQL、Redis、etcd，等待健康检查通过
	bash scripts/dev.sh deps-up

dev-deps-down: ## 停止本检出目录的开发依赖，保留数据卷
	bash scripts/dev.sh deps-down

dev-deps-status: ## 查看开发依赖容器状态
	bash scripts/dev.sh deps-status

dev-migrate: ## 在本地开发数据库执行现有迁移
	bash scripts/dev.sh migrate

dev-apiserver: ## 前台启动本地 API Server，供 IDE 或独立终端调试
	bash scripts/dev.sh apiserver

dev-namespace: ## 等待本地 API Server 就绪并创建 dev Namespace
	bash scripts/dev.sh namespace

dev-controller: ## 前台启动本地 Controller Manager（单副本）
	bash scripts/dev.sh controller

dev-admin: ## 前台启动本地 Admin API
	bash scripts/dev.sh admin

dev-web: ## 前台启动控制台 Vite 开发服务器，依赖需先 npm ci
	bash scripts/dev.sh web
