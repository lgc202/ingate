---
title: 运维助手
description: 启动运维助手骨架并检查 MySQL、Temporal 与模型端点状态
---

运维助手当前交付的是新的最小后端运行骨架，在一个 `ingate-assistant --role=all` 进程中同时运行 Kratos HTTP 服务与 Temporal Worker。

对话、诊断、审批、自动执行和 Console 页面尚未接入这套新骨架。旧 Assistant 的会话 API、Redis Stream、执行状态机、数据库表和 Console 入口已经移除。

Assistant 是可选的控制面辅助组件，不参与配置发布或业务流量。它不可用时，其他 Console 页面、控制面和 Envoy 转发不受影响。

## 依赖

当前进程把以下组件视为必需依赖：

| 组件 | 检查方式 | 用途 |
| --- | --- | --- |
| MySQL | 在请求期限内执行连接池 Ping | 为后续持久状态预留事实来源 |
| Temporal | 调用 Temporal Frontend 健康检查 | 承载持久工作流和任务队列 |
| 模型端点 | 对配置的无凭据健康 URL 发起 HTTP GET | 确认模型服务入口可达 |
| Temporal Worker | 检查进程内 Worker 已启动且未停止 | 确认 HTTP 与 Worker 同时运行 |

任一项不可用时，整体状态为 `Unavailable`。检查结果只返回组件名称和状态，不把数据库错误、网络响应或模型响应正文暴露给浏览器。

## 本地启动

先启动 MySQL、Temporal 和本地模型服务，再执行：

```bash
go run ./cmd/ingate-assistant --config configs/ingate-assistant.yaml --role all
```

默认模型健康地址是 Ollama 的 `http://127.0.0.1:11434/api/version`。使用其他本地模型服务时，通过配置文件中的 `${ASSISTANT_MODEL_HEALTH_URL}` 占位符设置 `INGATE_ASSISTANT_MODEL_HEALTH_URL`；该地址必须是无需凭据且不会产生模型调用的 HTTP 或 HTTPS 端点。

`--role` 当前只接受 `all`。API 与 Worker 的独立角色会在出现真实部署需求后再增加。

## 查看状态

运维端点包括：

| 端点 | 行为 |
| --- | --- |
| `GET /healthz` | 只表示进程仍存活 |
| `GET /readyz` | 所有必需组件可用时返回 `200`，否则返回 `503` |
| `GET /assistant/v1/system/readiness` | 返回整体和逐组件状态 |

本地可直接检查：

```bash
curl -sS http://127.0.0.1:8083/healthz
curl -sS -i http://127.0.0.1:8083/readyz
curl -sS http://127.0.0.1:8083/assistant/v1/system/readiness
```

后续 Console 接入会继续使用当前应用外壳和登录身份，不创建独立前端或第二套登录。

停止 MySQL、Temporal 或模型端点后，下一次检查会把对应组件和整体状态改为不可用。高频健康检查不会写 INFO 请求日志。

## 退出行为

收到 `SIGINT` 或 `SIGTERM` 后，Kratos 先停止 HTTP 与 Worker。Temporal Worker 停止领取新任务，并在 `worker_stop_timeout` 内等待已领取任务到达安全点；进程随后关闭 Temporal 客户端和 MySQL 连接池。`worker_stop_timeout` 必须短于进程的 `shutdown_timeout`。
