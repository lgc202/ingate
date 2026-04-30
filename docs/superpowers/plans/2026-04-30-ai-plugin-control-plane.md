# AI Plugin Control Plane Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 打通 AI Gateway Plugin-first 设计的第一层控制面资源模型

**Architecture:** 先补齐 `AIProvider / AIModel / AIRoute / AIPolicy / Plugin / PluginBinding` 的 API 类型和生成链路。暂时不实现 Wasm 插件、不接入真实 AI Provider 调用，只让 apiserver、client、informer、lister 能识别这些资源。

**Tech Stack:** Go 1.26、k8s.io/code-generator、Kubernetes generic apiserver、Envoy xDS target

---

### Task 1: API Types

**Files:**
- Modify: `pkg/apis/gateway/v1/types.go`
- Modify: `pkg/apis/gateway/v1/register.go`

- [ ] 为 `AIProvider / AIRoute / Plugin / PluginBinding` 增加 `+genclient` 和 `+genclient:nonNamespaced`
- [ ] 新增 `AIModel`、`AIModelList`、`AIPolicy`、`AIPolicyList`
- [ ] 新增 `AIExecutionTargetType`、`PluginPhase`、`PluginFailurePolicy`
- [ ] 在 `Bundle` 和 `addKnownTypes` 中加入新增资源

### Task 2: Code Generation

**Files:**
- Generated: `pkg/generated/**`
- Generated: `pkg/apis/gateway/v1/zz_generated.deepcopy.go`

- [ ] 运行 `make generate`
- [ ] 确认生成 clientset、informer、lister

### Task 3: Apiserver Storage

**Files:**
- Create: `internal/apiserver/registry/aiprovider/*`
- Create: `internal/apiserver/registry/aimodel/*`
- Create: `internal/apiserver/registry/airoute/*`
- Create: `internal/apiserver/registry/aipolicy/*`
- Create: `internal/apiserver/registry/plugin/*`
- Create: `internal/apiserver/registry/pluginbinding/*`
- Modify: `internal/apiserver/server/config.go`

- [ ] 按现有 Gateway/Route/Upstream storage pattern 增加 REST storage
- [ ] 接入 apiserver storage map

### Task 4: Verification

**Files:**
- No direct source change

- [ ] 运行 `make test`
- [ ] 运行 `make build`

### Current Session Scope

本轮先做 Task 1 和 Task 2。Storage 和 controller watch 留到下一轮，避免一次改动过大。
