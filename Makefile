.PHONY: build generate test console-install console-build all-in-one-binaries all-in-one-image

GO_CACHE_DIR ?= /tmp/ingate-gocache
ALL_IN_ONE_IMAGE ?= ingate/all-in-one:dev
ALL_IN_ONE_GOOS ?= linux
ALL_IN_ONE_GOARCH ?= arm64
CONSOLE_DIR ?= web/console

build:
	mkdir -p _output/bin
	GOCACHE=$(GO_CACHE_DIR) go build -o _output/bin/ ./cmd/...

generate:
	bash hack/update-codegen.sh

test:
	GOCACHE=$(GO_CACHE_DIR) go test ./...

console-install:
	cd $(CONSOLE_DIR) && npm ci

console-build: console-install
	cd $(CONSOLE_DIR) && npm run build

all-in-one-binaries:
	mkdir -p _output/all-in-one/bin
	GOOS=$(ALL_IN_ONE_GOOS) GOARCH=$(ALL_IN_ONE_GOARCH) CGO_ENABLED=0 GOCACHE=$(GO_CACHE_DIR) go build -o _output/all-in-one/bin/ ./cmd/...

all-in-one-image: all-in-one-binaries console-build
	docker build -f deploy/all-in-one/Dockerfile -t $(ALL_IN_ONE_IMAGE) .
