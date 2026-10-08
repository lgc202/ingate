# macOS 自带的 GNU Make 3.81 不支持 .SHELLFLAGS；严格模式放入 Shell 命令。
SHELL := /bin/bash -eu -o pipefail
.SHELLFLAGS := -c

GO ?= go
GOFMT ?= gofmt
BINS ?= $(notdir $(patsubst %/,%,$(dir $(wildcard cmd/*/main.go))))
GO_DIRS := $(wildcard api cmd internal pkg)
GENERATE_TARGETS :=
LINT_TARGETS :=

.PHONY: help

help: ## 显示可用命令和构建参数
	@awk 'BEGIN {FS = ":.*## "; print "用法：make <目标> [变量=值]\n\n目标："} /^[a-zA-Z_0-9.-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\n构建参数：\n  BINS          要构建的入口，默认全部；多个名称以空格分隔\n  VERSION       版本标识，默认从 Git 获取\n  BUILD_DATE    UTC 构建时间，可由发布流水线固定\n  LDFLAGS       附加的 Go 链接参数\n  GO / GOFMT    Go 和 gofmt 命令\n'
