.PHONY: build generate test managed-rate-limit-plugin-test managed-rate-limit-plugin-build console-install console-build all-in-one-binaries all-in-one-image dev-image dev-restart dev-reset

GO_CACHE_DIR ?= /tmp/ingate-gocache
ALL_IN_ONE_IMAGE ?= ingate/all-in-one:dev
ALL_IN_ONE_GOOS ?= linux
ALL_IN_ONE_GOARCH ?= arm64
CONSOLE_DIR ?= web/console
DEV_IMAGE ?= ingate/all-in-one
DEV_TAG ?= dev
DEV_DATA_DIR ?= ./ingate-dev
MANAGED_RATE_LIMIT_PLUGIN_DIR ?= plugins/managed/rate-limit
MANAGED_RATE_LIMIT_PLUGIN_OUT ?= _output/plugins/rate-limit.wasm

build:
	mkdir -p _output/bin
	GOCACHE=$(GO_CACHE_DIR) go build -o _output/bin/ ./cmd/...

generate:
	bash hack/update-codegen.sh

test:
	GOCACHE=$(GO_CACHE_DIR) go test ./...

managed-rate-limit-plugin-test:
	cd $(MANAGED_RATE_LIMIT_PLUGIN_DIR) && GOCACHE=$(GO_CACHE_DIR) go test ./...

managed-rate-limit-plugin-build:
	mkdir -p _output/plugins
	cd $(MANAGED_RATE_LIMIT_PLUGIN_DIR) && tinygo build -o ../../../$(MANAGED_RATE_LIMIT_PLUGIN_OUT) -scheduler=none -target=wasi -gc=leaking -tags='proxy_wasm_version_0_2_100' ./main.go

console-install:
	cd $(CONSOLE_DIR) && npm ci

console-build: console-install
	cd $(CONSOLE_DIR) && npm run build

all-in-one-binaries:
	mkdir -p _output/all-in-one/bin
	GOOS=$(ALL_IN_ONE_GOOS) GOARCH=$(ALL_IN_ONE_GOARCH) CGO_ENABLED=0 GOCACHE=$(GO_CACHE_DIR) go build -o _output/all-in-one/bin/ ./cmd/...

all-in-one-image: all-in-one-binaries managed-rate-limit-plugin-build console-build
	docker build -f deploy/all-in-one/Dockerfile -t $(ALL_IN_ONE_IMAGE) .

dev-image:
	$(MAKE) all-in-one-image ALL_IN_ONE_IMAGE=$(DEV_IMAGE):$(DEV_TAG)

dev-restart: dev-image
	./install.sh restart --image $(DEV_IMAGE) --tag $(DEV_TAG) --data-dir $(DEV_DATA_DIR)

dev-reset: dev-image
	./install.sh delete --purge-data --image $(DEV_IMAGE) --tag $(DEV_TAG) --data-dir $(DEV_DATA_DIR)
	./install.sh start --image $(DEV_IMAGE) --tag $(DEV_TAG) --data-dir $(DEV_DATA_DIR)
