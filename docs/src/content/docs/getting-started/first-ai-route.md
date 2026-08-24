---
title: 发布第一个模型
description: 接入模型 Service，发布稳定模型名并使用 OpenAI Chat Completions 调用
---

AI Route 向客户端发布稳定模型名，并把请求转发到一个或多个模型 Service。客户端不需要感知真实厂商协议、API Key 和真实模型名。

## 1. 创建模型 Service

进入 **流量配置 → 服务**，创建模型服务：

1. 选择厂商协议，例如 OpenAI 兼容或 Anthropic
2. 填写厂商 API 地址
3. 填写 API Key
4. 保存并等待状态变为“已生效”

API Key 保存在 Service 中，不会出现在 Route 或客户端请求中。

## 2. 创建 AI Route

进入 **流量配置 → 路由**：

| 配置 | 示例 |
| --- | --- |
| 路由类型 | `AI` |
| 路径 | `/v1/chat/completions` |
| 对外模型名 | `assistant` |
| 目标线路 | 模型 Service + 真实模型名 |
| 访问方式 | 公开访问或需要调用方密钥 |

同一个对外模型可以配置多条线路。权重用于正常情况下的流量分配，失败切换用于当前线路请求失败后的重试选择。

![AI Route 详情，展示请求匹配、对外模型名与两条模型线路](/ingate/images/screenshots/ai-route.jpg)

## 3. 调用模型

客户端统一使用 OpenAI Chat Completions 格式：

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <access-key>' \
  -d '{
    "model": "assistant",
    "messages": [{"role": "user", "content": "你是谁"}],
    "stream": false
  }'
```

公开 Route 不需要 `Authorization` Header。受保护 Route 的访问密钥通过[调用方](../../governance/caller/)签发。

## 请求如何到达厂商

- OpenAI 兼容线路保留 Chat Completions 请求结构，并改写真实模型名和凭据
- Anthropic 线路转换为 Messages 请求，返回时再转换为 OpenAI 兼容响应
- `stream: true` 时同步转换 SSE 流，不缓存完整响应

模型线路尝试、Token 用量和最终响应可以在[AI 用量](../../observability/ai-usage/)与请求详情中查看。
