# Keep recipe shells portable on hosts that do not provide C.UTF-8.
# This must be set before any $(shell ...) call below starts bash.
export LC_ALL := C
export LANG := C
export LC_CTYPE := C

$(shell mkdir -p _output)
$(shell LC_ALL=C LANG=C LC_CTYPE=C $(CURDIR)/tools/hack/setup_env.sh envfile > _output/.env)
include _output/.env
export

SHELL := /bin/bash

.DEFAULT_GOAL := help

APISERVER_BIN ?= $(BUILD_DIR)/ingate-apiserver
ADMIN_API_BIN ?= $(BUILD_DIR)/ingate-admin-api
CONTROLLER_MANAGER_BIN ?= $(BUILD_DIR)/ingate-controller-manager
XDS_SERVER_BIN ?= $(BUILD_DIR)/ingate-xds-server
COMPOSE_FILE ?= deploy/compose/compose.yaml
COMPOSE_ENV_FILE ?= deploy/compose/.env.example
COMPOSE_PARALLEL_LIMIT ?= 1
COMPOSE_TARGET_OS ?= linux
COMPOSE_TARGET_ARCH ?= $(TARGET_ARCH)

.PHONY: help check-tools generate generate-apis generate-clients generate-proto verify-generated build build-apiserver build-admin-api build-controller-manager build-xds-server build-ingatectl clean run-apiserver run-admin-api verify-apiserver verify-apiserver-auth verify-apiserver-admission verify-apiserver-table verify-admin-api verify-controller-manager verify-xds-server verify-envoy compose-build compose-up compose-down compose-logs compose-ps verify-compose write-apiserver-kubeconfig verify-apiserver-kubectl version

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<TARGET>\033[0m [BINS=\"...\"] [BUILD_DIR=...]\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf '\nOptions:\n'
	@printf '  \033[36mBINS\033[0m        Space-separated components or binary names. Example: make build BINS="ingate-apiserver ingatectl"\n'
	@printf '  \033[36mBUILD_DIR\033[0m   Build output directory. Default: %s\n' "$(BUILD_DIR)"
	@printf '  \033[36mTARGET_OS\033[0m   Target operating system. Default: %s\n' "$(TARGET_OS)"
	@printf '  \033[36mTARGET_ARCH\033[0m Target architecture. Default: %s\n' "$(TARGET_ARCH)"
	@printf '  \033[36mCOMPOSE_FILE\033[0m Compose file used by compose-* targets. Default: %s\n' "$(COMPOSE_FILE)"
	@printf '  \033[36mCOMPOSE_ENV_FILE\033[0m Env file used by compose-* targets. Default: %s\n' "$(COMPOSE_ENV_FILE)"
	@printf '  \033[36mCOMPOSE_PARALLEL_LIMIT\033[0m Max parallel compose builds. Default: %s\n' "$(COMPOSE_PARALLEL_LIMIT)"
	@printf '  \033[36mCOMPOSE_TARGET_OS\033[0m Target OS for compose image binaries. Default: %s\n' "$(COMPOSE_TARGET_OS)"
	@printf '  \033[36mCOMPOSE_TARGET_ARCH\033[0m Target arch for compose image binaries. Default: %s\n' "$(COMPOSE_TARGET_ARCH)"

check-tools: ## Verify required local development tools.
	./tools/hack/check-tools.sh

generate: ## Generate all current code artifacts.
	./tools/hack/generate-all.sh

generate-apis: ## Generate API helper code such as DeepCopy.
	./tools/hack/generate-apis.sh

generate-clients: ## Generate versioned clientset, informers, and listers.
	./tools/hack/generate-clients.sh

generate-proto: ## Generate proto-related code.
	./tools/hack/generate-proto.sh

verify-generated: ## Verify generated files are up to date.
	./tools/hack/verify-generated.sh

build: ## Build selected binaries into _output/<os>_<arch>.
	BUILD_DIR="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))" BINS="$(or $(BINS),ingate-apiserver ingate-admin-api ingate-controller-manager ingate-xds-server ingatectl)" ./tools/hack/build.sh

build-apiserver: ## Build only ingate-apiserver into BUILD_DIR.
	BUILD_DIR="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))" BINS="ingate-apiserver" ./tools/hack/build.sh

build-admin-api: ## Build only ingate-admin-api into BUILD_DIR.
	BUILD_DIR="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))" BINS="ingate-admin-api" ./tools/hack/build.sh

build-controller-manager: ## Build only ingate-controller-manager into BUILD_DIR.
	BUILD_DIR="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))" BINS="ingate-controller-manager" ./tools/hack/build.sh

build-xds-server: ## Build only ingate-xds-server into BUILD_DIR.
	BUILD_DIR="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))" BINS="ingate-xds-server" ./tools/hack/build.sh

build-ingatectl: ## Build only ingatectl into BUILD_DIR.
	BUILD_DIR="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))" BINS="ingatectl" ./tools/hack/build.sh

version: build-apiserver ## Print ingate-apiserver build version metadata.
	"$(APISERVER_BIN)" --version

clean: ## Remove current build outputs.
	rm -rf $(CURDIR)/_output

run-apiserver: build-apiserver ## Run the local apiserver against the configured etcd.
	APISERVER_BIN="$(APISERVER_BIN)" ./tools/hack/run-apiserver.sh

run-admin-api: build-admin-api ## Run the local admin-api against the configured ingate-apiserver.
	ADMIN_API_BIN="$(ADMIN_API_BIN)" ./tools/hack/run-admin-api.sh

verify-apiserver: build-apiserver ## Start a local apiserver and verify public health, discovery, and OpenAPI endpoints.
	APISERVER_BIN="$(APISERVER_BIN)" ./tools/hack/verify-apiserver.sh

verify-apiserver-auth: build-apiserver ## Start a local apiserver and verify authn/authz behavior for public, admin, and viewer access.
	APISERVER_BIN="$(APISERVER_BIN)" ./tools/hack/verify-apiserver-auth.sh

verify-apiserver-admission: build-apiserver ## Start a local apiserver and verify reserved metadata admission behavior.
	APISERVER_BIN="$(APISERVER_BIN)" ./tools/hack/verify-apiserver-admission.sh

verify-apiserver-table: build-apiserver ## Start a local apiserver and verify custom Table output for all five resources.
	APISERVER_BIN="$(APISERVER_BIN)" ./tools/hack/verify-apiserver-table.sh

verify-admin-api: build-apiserver build-admin-api ## Start local apiserver/admin-api and verify gateway/backend/route product APIs.
	APISERVER_BIN="$(APISERVER_BIN)" ADMIN_API_BIN="$(ADMIN_API_BIN)" ./tools/hack/verify-admin-api.sh

verify-controller-manager: build-apiserver build-controller-manager ## Start local apiserver/controller-manager and verify ResolvedGateway reconciliation plus Accepted/Resolved status updates.
	APISERVER_BIN="$(APISERVER_BIN)" CONTROLLER_MANAGER_BIN="$(CONTROLLER_MANAGER_BIN)" ./tools/hack/verify-controller-manager.sh

verify-xds-server: build-apiserver build-controller-manager build-xds-server build-ingatectl ## Start local apiserver/controller-manager/xds-server and verify Programmed status updates plus discovery RPC.
	APISERVER_BIN="$(APISERVER_BIN)" CONTROLLER_MANAGER_BIN="$(CONTROLLER_MANAGER_BIN)" XDS_SERVER_BIN="$(XDS_SERVER_BIN)" INGATECTL_BIN="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))/ingatectl" ./tools/hack/verify-xds-server.sh

verify-envoy: build-apiserver build-controller-manager build-xds-server build-ingatectl ## Start local control-plane plus Dockerized Envoy and verify real xDS-driven HTTP forwarding.
	APISERVER_BIN="$(APISERVER_BIN)" CONTROLLER_MANAGER_BIN="$(CONTROLLER_MANAGER_BIN)" XDS_SERVER_BIN="$(XDS_SERVER_BIN)" INGATECTL_BIN="$(or $(BUILD_DIR),$(CURDIR)/_output/$(TARGET_OS)_$(TARGET_ARCH))/ingatectl" ./tools/hack/verify-envoy.sh

compose-build: ## Build Docker images for the demo stack without relying on compose buildx bake.
	COMPOSE_TARGET_OS="$(COMPOSE_TARGET_OS)" COMPOSE_TARGET_ARCH="$(COMPOSE_TARGET_ARCH)" ./tools/hack/build-compose-images.sh

compose-up: ## Start the Docker Compose demo stack in the background.
	COMPOSE_TARGET_OS="$(COMPOSE_TARGET_OS)" COMPOSE_TARGET_ARCH="$(COMPOSE_TARGET_ARCH)" ./tools/hack/build-compose-images.sh
	docker compose -f "$(COMPOSE_FILE)" --env-file "$(COMPOSE_ENV_FILE)" up -d --no-build

compose-down: ## Stop and remove the Docker Compose demo stack.
	docker compose -f "$(COMPOSE_FILE)" --env-file "$(COMPOSE_ENV_FILE)" down -v --remove-orphans

compose-logs: ## Stream logs from the Docker Compose demo stack.
	docker compose -f "$(COMPOSE_FILE)" --env-file "$(COMPOSE_ENV_FILE)" logs -f

compose-ps: ## Show the current Docker Compose demo stack state.
	docker compose -f "$(COMPOSE_FILE)" --env-file "$(COMPOSE_ENV_FILE)" ps

verify-compose: ## Build and verify the Docker Compose demo stack proxies traffic through Envoy.
	COMPOSE_FILE="$(COMPOSE_FILE)" COMPOSE_ENV_FILE="$(COMPOSE_ENV_FILE)" COMPOSE_PARALLEL_LIMIT="$(COMPOSE_PARALLEL_LIMIT)" COMPOSE_TARGET_OS="$(COMPOSE_TARGET_OS)" COMPOSE_TARGET_ARCH="$(COMPOSE_TARGET_ARCH)" ./tools/hack/verify-compose.sh

write-apiserver-kubeconfig: build-apiserver ## Write a kubeconfig for talking to the local ingate apiserver.
	./tools/hack/write-apiserver-kubeconfig.sh

verify-apiserver-kubectl: build-apiserver ## Start a local apiserver and verify kubectl access with admin and viewer contexts.
	APISERVER_BIN="$(APISERVER_BIN)" ./tools/hack/verify-apiserver-kubectl.sh
