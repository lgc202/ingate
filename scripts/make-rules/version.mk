# 递归变量延迟求值，make help 不调用 Go、Git 或 date。
MODULE = $(shell $(GO) list -m)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo devel)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
GIT_TREE_STATE ?= $(shell \
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		if test -z "$$(git status --porcelain)"; then echo clean; else echo dirty; fi; \
	else \
		echo unknown; \
	fi)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

VERSION_LDFLAGS = \
	-X '$(MODULE)/internal/pkg/version.gitVersion=$(VERSION)' \
	-X '$(MODULE)/internal/pkg/version.gitCommit=$(GIT_COMMIT)' \
	-X '$(MODULE)/internal/pkg/version.gitTreeState=$(GIT_TREE_STATE)' \
	-X '$(MODULE)/internal/pkg/version.buildDate=$(BUILD_DATE)'
