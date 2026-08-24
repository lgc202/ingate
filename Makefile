.DEFAULT_GOAL := help

PROJECT_ROOT := $(abspath .)
OUTPUT_DIR := $(PROJECT_ROOT)/_output
TOOLS_DIR := $(PROJECT_ROOT)/.tools/bin

include scripts/make-rules/common.mk
include scripts/make-rules/golang.mk
include scripts/make-rules/plugins.mk
include scripts/make-rules/tools.mk
include scripts/make-rules/generate.mk
include scripts/make-rules/console.mk
include scripts/make-rules/docker.mk
include scripts/make-rules/verify.mk
