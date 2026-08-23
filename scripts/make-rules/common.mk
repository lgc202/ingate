SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

VERSION_PACKAGE := github.com/lgc202/ingate/internal/pkg/version
# 插件发布使用独立 tag，主程序版本只能从 vX.Y.Z 形式的 Ingate tag 推导
GIT_VERSION ?= $(shell git describe --tags --match 'v[0-9]*' --always 2>/dev/null || echo v0.0.0-unknown)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
GIT_TREE_STATE ?= $(shell if test -z "$$(git status --porcelain 2>/dev/null)"; then echo clean; else echo dirty; fi)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
VERSION_LDFLAGS := -X $(VERSION_PACKAGE).gitVersion=$(GIT_VERSION) -X $(VERSION_PACKAGE).gitCommit=$(GIT_COMMIT) -X $(VERSION_PACKAGE).gitTreeState=$(GIT_TREE_STATE) -X $(VERSION_PACKAGE).buildDate=$(BUILD_DATE)

export GIT_VERSION GIT_COMMIT GIT_TREE_STATE BUILD_DATE

.PHONY: help
help: ## 显示可用命令
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_0-9.-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## 删除本地构建产物
	@rm -rf $(OUTPUT_DIR)
