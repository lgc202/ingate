---
title: 当前范围
description: Ingate 0.x 已实现能力和当前边界
---

Ingate 当前已经形成普通 API 与 AI 模型调用的最小完整链路。项目仍处于 `0.x`，资源协议和部署方式在 `1.0` 前可能调整。

## 已实现

- HTTP/HTTPS Gateway、证书和入口冲突检查
- API Route、加权目标、Host 转发、超时与重试
- HTTP Service 和模型 Service
- AI Route 对外模型名、OpenAI/Anthropic 线路与流式响应
- Caller 访问密钥、Route 授权和密钥轮换
- IP 访问限制、共享请求限流和 Token 额度
- 请求响应 Header 转换与模拟响应插件
- 插件源、安装、升级、依赖检查和卸载
- 请求记录、流量趋势、资源排行和 AI Token 用量
- 只读运维助手、在线模型连接、对话持久化和流式执行恢复
- 声明式 CRUD、List/Watch、版本、Status 和 xDS 发布
- Docker Compose 安装、升级、备份、恢复和卸载

## 当前不做

- 用户自定义 MCP 工具、写操作审批、多 Agent 编排和定时自动化
- Prompt 管理、知识库、数据集和模型训练
- 请求 Header、查询参数和正文的持久化或流量回放
- Kubernetes CRD、Helm Chart 和多数据平面适配
- 多租户控制面、组织、RBAC、OIDC 和复杂审批
- 模型价格目录、计费、充值和开票
- ACME 自动签发和证书自动续期

未实现能力不在 Console 中保留占位入口。新增能力需要先明确用户场景、数据模型和执行链路。

## 部署模型

一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例，其中可以创建多个逻辑 Gateway。

生产、测试、机房或租户需要控制面隔离时，应部署多套 Ingate。当前不在一套控制面内增加 Environment、Cluster 或 RuntimeGroup 等抽象。

## 数据平面

Envoy 是唯一数据平面，直接使用固定版本官方镜像。Ingate 不维护 Envoy 私有分支，也不为 Kong、Nginx 等数据平面预设适配接口。
