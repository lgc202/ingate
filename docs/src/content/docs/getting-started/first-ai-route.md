---
title: 发布第一个模型
description: 接入模型 Service，发布稳定模型名并使用 OpenAI Chat Completions 调用
---

AI Route 向客户端发布稳定模型名，并把请求转发到一个或多个模型 Service。客户端不需要感知真实厂商协议、API Key 和真实模型名。本页需要一个已经创建的 HTTP Gateway；可以复用[第一个 API](../first-api/)中的 `演示网关`。

## 1. 创建模型 Service

进入 **流量配置 → 服务**，创建模型服务：

1. 服务类型选择“模型服务”
2. 选择厂商协议：OpenAI 兼容或 Anthropic Messages
3. 填写模型服务的地址和端口
4. 使用 HTTPS 时开启 TLS，并填写证书校验使用的服务名称
5. 填写 API Key；无需认证的模型服务可以留空
6. 保存 Service

API Key 保存在 Service 中，不会出现在 Route、列表响应或客户端请求中。新安装中 Service 可能暂时显示“发布中”，完成 AI Route 后再统一确认状态。

## 2. 创建 AI Route

进入 **流量配置 → 路由**：

| 配置 | 示例 |
| --- | --- |
| 路由类型 | `AI` |
| 生效网关 | 已创建的 HTTP Gateway |
| 路径 | `/v1/chat/completions` |
| 对外模型名 | `assistant` |
| 目标线路 | 模型 Service + 真实模型名 |
| 访问方式 | 公开访问或需要调用方密钥 |

同一个对外模型可以配置多条线路，权重用于线路之间的相对流量分配。需要重试失败请求时，在 Route 的转发设置中显式配置重试次数与单次超时；当前没有独立的主备或“失败切换”开关。

![AI Route 详情，展示请求匹配、对外模型名与两条模型线路](/ingate/images/screenshots/ai-route.jpg)

## 3. 调用模型

等待 Gateway、AI Route 和模型 Service 都显示“已生效”，再调用数据面。客户端统一使用 OpenAI Chat Completions 格式：

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
- Anthropic 线路把文本消息和两种协议共有的采样参数转换为 Messages 请求，返回时再转换为 OpenAI 兼容响应
- `stream: true` 时同步转换 SSE 流，不缓存完整响应

当前 Anthropic 转换要求 `n=1`，只支持文本消息，不支持 OpenAI 请求中的 `tools`、`tool_choice` 或 `response_format`；携带这些字段会返回明确的客户端错误。需要工具调用时选择 OpenAI 兼容模型 Service，或由客户端直连支持工具协议的模型端点。

模型线路尝试、Token 用量和最终响应可以在[AI 用量](../../observability/ai-usage/)与请求详情中查看。
