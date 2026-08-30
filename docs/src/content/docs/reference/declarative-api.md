---
title: 声明式 API
description: 资源结构、CRUD、List/Watch、版本和 Status 约定
---

Ingate API Server 提供独立的声明式资源 API，不要求 Kubernetes，也不兼容 Kubernetes CRD。API 支持 CRUD、List/Watch、乐观并发和 Status。

## 通用结构

```yaml
apiVersion: gateway.ingate.io/v1
kind: Gateway
metadata:
  name: 5cb83268-6e5c-42af-a4d0-3f40fd449b66
  generation: 3
  resourceVersion: "1287"
  creationTimestamp: "2026-08-10T10:00:00Z"
spec:
  displayName: public-gateway
  enabled: true
status:
  conditions:
    - type: Programmed
      status: "True"
      observedGeneration: 3
```

- `metadata.name`：不可变资源 ID
- `metadata.generation`：期望状态版本，只在 Spec 实际变化时增加
- `metadata.resourceVersion`：存储并发版本，只在 API Server 边界使用
- `spec`：用户期望状态
- `status`：Controller 观察与发布结果

Status 更新不会推动 generation。Controller 通过 `observedGeneration` 表明状态对应哪一版 Spec，避免把旧的“已生效”误认为当前编辑已经发布。

## Admin API 与声明式 API

Console 不直接操作资源对象。Admin API 对外提供平铺的产品协议，例如 `id`、`name`、`enabled` 和 `state`：

- `id` 映射声明式资源的 `metadata.name`
- `name` 映射面向用户的 `spec.displayName`
- `version` 映射 `metadata.generation`
- `state` 根据 enabled、generation 和 Status 计算

这层转换让 Console 不需要理解 metadata/spec/status，也避免把 etcd 或 Envoy 实现细节泄漏成产品协议。

## CRUD 与版本冲突

更新和删除必须提交读取到的 `version`。如果资源已经被其他用户修改，Admin API 返回 `409 Conflict`，客户端应重新加载资源，不应静默覆盖。

展示名称允许重复，资源身份只由 `id` 决定。引用存在性和跨资源冲突由后端业务层裁决；前端只做必填、格式、范围等当前表单内可以确定的校验。

## List 与 Watch

List 使用不透明 cursor 分页，调用方不能解析或拼接 cursor。Watch 从指定 `resourceVersion` 继续接收 Added、Modified、Deleted 事件；版本过旧时调用方重新 List，再建立 Watch。

Controller、Authz 和 AI ExtProc 都采用 List + Watch 维护自己需要的资源视图，不轮询 etcd，也不直接访问 etcd。

## Status

资源总体结果写入 `status.conditions`。策略的每个目标还可以在 `status.targets[]` 中独立表达引用解析和生效状态：一个无效目标不会阻止其他有效目标发布。

声明式资源是唯一持久化配置事实。Controller 派生的 Envoy 配置只保存在进程内，重启后重新全量编译，不持久化 Last Good。
