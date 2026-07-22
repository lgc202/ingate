.PHONY: build generate test ratelimit-plugin-test ratelimit-plugin-build tokenquota-plugin-test tokenquota-plugin-build acl-plugin-test acl-plugin-build ai-proxy-plugin-test ai-proxy-plugin-build plugins-test plugins-build console-install console-build all-in-one-binaries all-in-one-image dev-image dev-restart dev-reset

GO_CACHE_DIR ?= /tmp/ingate-gocache
GO_MOD_CACHE_DIR ?= /tmp/ingate-gomodcache
ALL_IN_ONE_IMAGE ?= ingate/all-in-one:dev
ALL_IN_ONE_COMMANDS := ./cmd/ingate-apiserver ./cmd/ingate-admin-api ./cmd/ingate-controller
CONSOLE_DIR ?= web/console
DEV_IMAGE ?= ingate/all-in-one
DEV_TAG ?= dev
DEV_DATA_DIR ?= ./ingate-dev
RATELIMIT_PLUGIN_DIR ?= plugins/ratelimit
RATELIMIT_PLUGIN_OUT ?= _output/plugins/ratelimit.wasm
TOKENQUOTA_PLUGIN_DIR ?= plugins/tokenquota
TOKENQUOTA_PLUGIN_OUT ?= _output/plugins/tokenquota.wasm
ACL_PLUGIN_DIR ?= plugins/acl
ACL_PLUGIN_OUT ?= _output/plugins/acl.wasm
AI_PROXY_PLUGIN_DIR ?= plugins/aiproxy
AI_PROXY_PLUGIN_OUT ?= _output/plugins/ai-proxy.wasm
VERSION_PACKAGE := github.com/lgc202/go-kit/version
GIT_VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo v0.0.0-unknown)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
GIT_TREE_STATE ?= $(shell if test -z "$$(git status --porcelain 2>/dev/null)"; then echo clean; else echo dirty; fi)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
VERSION_LDFLAGS := -X $(VERSION_PACKAGE).gitVersion=$(GIT_VERSION) -X $(VERSION_PACKAGE).gitCommit=$(GIT_COMMIT) -X $(VERSION_PACKAGE).gitTreeState=$(GIT_TREE_STATE) -X $(VERSION_PACKAGE).buildDate=$(BUILD_DATE)

build:
	mkdir -p _output/bin
	GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go build -ldflags "$(VERSION_LDFLAGS)" -o _output/bin/ ./cmd/...

generate:
	bash hack/update-codegen.sh

test:
	GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go test ./...

ratelimit-plugin-test:
	cd $(RATELIMIT_PLUGIN_DIR) && GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go test ./...

ratelimit-plugin-build:
	mkdir -p _output/plugins
	cd $(RATELIMIT_PLUGIN_DIR) && GOOS=wasip1 GOARCH=wasm GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go build -buildmode=c-shared -o ../../$(RATELIMIT_PLUGIN_OUT) .

tokenquota-plugin-test:
	cd $(TOKENQUOTA_PLUGIN_DIR) && GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go test ./...

tokenquota-plugin-build:
	mkdir -p _output/plugins
	cd $(TOKENQUOTA_PLUGIN_DIR) && GOOS=wasip1 GOARCH=wasm GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go build -buildmode=c-shared -o ../../$(TOKENQUOTA_PLUGIN_OUT) .

acl-plugin-test:
	cd $(ACL_PLUGIN_DIR) && GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go test ./...

acl-plugin-build:
	mkdir -p _output/plugins
	cd $(ACL_PLUGIN_DIR) && GOOS=wasip1 GOARCH=wasm GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go build -buildmode=c-shared -o ../../$(ACL_PLUGIN_OUT) .

ai-proxy-plugin-test:
	cd $(AI_PROXY_PLUGIN_DIR) && GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go test ./...

ai-proxy-plugin-build:
	mkdir -p _output/plugins
	cd $(AI_PROXY_PLUGIN_DIR) && GOOS=wasip1 GOARCH=wasm GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go build -buildmode=c-shared -o ../../$(AI_PROXY_PLUGIN_OUT) .

plugins-test: ratelimit-plugin-test tokenquota-plugin-test acl-plugin-test ai-proxy-plugin-test

plugins-build: ratelimit-plugin-build tokenquota-plugin-build acl-plugin-build ai-proxy-plugin-build

console-install:
	cd $(CONSOLE_DIR) && npm ci

console-build: console-install
	cd $(CONSOLE_DIR) && npm run build

all-in-one-binaries:
	mkdir -p _output/all-in-one/bin
	GOOS=linux GOARCH=$(shell go env GOARCH) CGO_ENABLED=0 GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR) go build -ldflags "$(VERSION_LDFLAGS)" -o _output/all-in-one/bin/ $(ALL_IN_ONE_COMMANDS)

all-in-one-image: all-in-one-binaries plugins-build console-build
	docker build -f deploy/all-in-one/Dockerfile -t $(ALL_IN_ONE_IMAGE) .

dev-image:
	$(MAKE) all-in-one-image ALL_IN_ONE_IMAGE=$(DEV_IMAGE):$(DEV_TAG)

dev-restart: dev-image
	./install.sh restart --image $(DEV_IMAGE) --tag $(DEV_TAG) --data-dir $(DEV_DATA_DIR)

dev-reset: dev-image
	./install.sh delete --purge-data --image $(DEV_IMAGE) --tag $(DEV_TAG) --data-dir $(DEV_DATA_DIR)
	./install.sh start --image $(DEV_IMAGE) --tag $(DEV_TAG) --data-dir $(DEV_DATA_DIR)
