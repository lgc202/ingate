# Service CRUD Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让控制台服务页真实对接 `/api/v1/upstreams`，用户可以创建、编辑、删除服务，并用 all-in-one 内置 httpbin 验证代理链路

**Architecture:** 保持现有 admin-api `Upstream` 资源边界，前端通过 repository 调用后端，不再用本地 saved/hidden 状态模拟保存结果。服务表单继续承载控制台视角的 DTO，保存后刷新真实列表并展示后端错误。

**Tech Stack:** React + TypeScript + Vite, Gin admin-api, Kubernetes-style generated client, Ingate Gateway v1 resources

---

### Task 1: 服务页状态模型改为真实远端数据

**Files:**
- Modify: `web/console/src/api/useResource.ts`
- Modify: `web/console/src/features/services/ServicePage.tsx`

- [x] 给 `useResource` 增加 `reload()`，让页面保存/删除后重新请求后端列表
- [x] 移除 `ServicePage` 里的 `savedServices` / `hiddenServiceIds` 本地合并逻辑
- [x] 新建服务时默认名称为空，不再填 `new-service`
- [x] 保存成功后调用 `reload()`，再回到列表
- [x] 删除成功后调用 `reload()`，再回到列表

### Task 2: 服务表单补齐产品化交互

**Files:**
- Modify: `web/console/src/features/services/form.ts`
- Modify: `web/console/src/features/services/ServicePage.tsx`

- [x] 服务端点默认地址为空，端口默认 `80`
- [x] 表单提交期间禁用保存按钮，避免重复提交
- [x] 保存失败时展示后端错误
- [x] 删除失败时展示后端错误
- [x] 保留端点地址、端口、权重、启用状态、健康检查的前端即时校验

### Task 3: 后端服务接口做必要补强

**Files:**
- Modify if needed: `internal/adminapi/handler/upstream/dto/request.go`
- Modify if needed: `internal/adminapi/service/upstream/service.go`

- [x] 确认创建时依赖 apiserver 返回 AlreadyExists 作为重名校验，不在前端伪造
- [x] 确认更新时禁止改名，并通过资源版本处理并发冲突
- [x] 确认删除时仍有关联路由会拒绝删除
- [x] 只补真正缺失的边界校验，不做额外抽象

### Task 4: 验证

**Commands:**
- `npm run build --prefix web/console`
- `go test ./...`
- `make build`
- `git diff --check`

- [x] all-in-one 内创建服务指向 `127.0.0.1:19090`
- [x] 创建路由指向该服务
- [x] 验证 `curl -sSf http://127.0.0.1:8080/get`
