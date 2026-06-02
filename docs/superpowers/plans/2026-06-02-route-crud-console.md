# Route CRUD Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让控制台路由页真实对接 `/api/v1/routes`，保存、删除、启停后刷新后端真实数据，并完成网关到服务的代理验证

**Architecture:** 继续使用 admin-api 的 Route DTO，不在前端构造临时 Route 列表。路由页面只维护 UI 草稿、筛选、弹窗和提交中状态；资源状态以后端返回为准。

**Tech Stack:** React + TypeScript + Vite, Gin admin-api, Ingate Gateway v1 Route/Upstream/Gateway resources

---

### Task 1: 移除路由页本地伪状态

**Files:**
- Modify: `web/console/src/features/routes/RoutePage.tsx`

- [x] 移除 `savedRoutes`、`hiddenRouteIds`、`enabledOverrides`
- [x] 路由列表只使用 `workspace.data.routes`
- [x] 保存成功后调用 `workspace.reload()`
- [x] 删除成功后调用 `workspace.reload()`
- [x] 启停成功后调用 `workspace.reload()`

### Task 2: 补齐提交中交互

**Files:**
- Modify: `web/console/src/features/routes/RoutePage.tsx`

- [x] 保存中禁用保存、取消、返回按钮
- [x] 删除中禁用删除确认弹窗按钮
- [x] 启停中禁用确认弹窗按钮
- [x] 保存失败、删除失败、启停失败展示后端错误

### Task 3: 保持真实路由表单数据

**Files:**
- Modify: `web/console/src/features/routes/RoutePage.tsx`
- Modify if needed: `web/console/src/features/routes/composer.ts`

- [x] 目标服务使用 `composer.targets`
- [x] 网关候选使用 `composer.gatewayNames` 和现有路由网关
- [x] 编辑时带上 route `id` 与 `version`
- [x] 新建保存后按后端命名规则选中新路由

### Task 4: 验证

**Commands:**
- `npm run build --prefix web/console`
- `make test`
- `make build`
- `git diff --check`

- [x] all-in-one 内创建临时服务指向 `127.0.0.1:19090`
- [x] 创建临时路由指向该服务
- [x] 验证 `curl -sSf http://127.0.0.1:8080/get`
- [x] 删除临时路由和临时服务
