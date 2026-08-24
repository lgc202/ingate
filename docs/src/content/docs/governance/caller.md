---
title: 调用方与访问密钥
description: 标识调用网关的应用，并限制其可访问路由
---

Caller 表示调用 Ingate 的应用或服务。它不是控制台用户，也不用于管理面登录。

## 适用范围

Route 设为“需要调用方密钥”后，只有满足以下条件的请求才会被放行：

1. 请求携带有效访问密钥
2. 密钥所属 Caller 已启用
3. Caller 被授权访问当前 Route
4. Route 内进一步配置了模型或工具权限时，请求也满足这些限制

普通公开 API 可以不使用 Caller。对于需要网关统一签发密钥、归属用量或限制 AI Token 的场景，应使用受保护 Route 和 Caller。

## 创建与使用密钥

创建 Caller 时可以同时签发首个访问密钥，并选择可访问 Route。密钥只在创建成功时完整展示一次，之后只展示名称、前缀、有效期和状态。

客户端通过 Bearer Token 调用：

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer <access-key>' \
  -H 'Content-Type: application/json' \
  -d '{"model":"assistant","messages":[{"role":"user","content":"hello"}]}'
```

Ingate 消费自己的访问密钥后，不会把它继续转发给上游。上游业务自己使用的认证 Header 应通过公开 Route 或明确的转发设计处理，不能与 Ingate Caller 密钥混为一谈。

## 轮换

一个 Caller 可以持有多个密钥，便于无中断轮换：

1. 创建新密钥
2. 更新客户端配置
3. 确认流量已经使用新密钥
4. 停用或删除旧密钥

停用 Caller 会使其全部密钥失效，但不会删除历史请求与用量归属。

## 与额度的关系

TokenQuotaPolicy 引用 Caller，并按调用方独立计数。请求记录和模型用量同时记录 Caller ID，用于按应用归属 Token 用量，而不是仅按 Route 或厂商统计。

当前 Caller 面向应用身份，不包含控制台用户、角色、组织、OAuth2 客户端或 OIDC 身份联合。
