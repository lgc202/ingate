---
title: 资源关系
description: Gateway、Route、Service、Caller、Certificate 和治理策略之间的关系
---

Ingate 的配置以少量明确资源组成。资源之间通过不可变 ID 引用，不使用用户可编辑名称建立关系。

## 核心流量资源

```text
Certificate --> Gateway --> Route --> Service
                              ^
                              |
                           Caller
```

| 资源 | 职责 | 主要引用 |
| --- | --- | --- |
| Gateway | 请求从哪个协议、端口和域名进入 | HTTPS Listener 引用 Certificate |
| Route | 哪些请求允许进入，以及转发到哪里 | 引用 Gateway、Service；可要求 Caller |
| Service | 如何连接真实 HTTP 服务或模型厂商 | 被 Route 目标引用 |
| Certificate | HTTPS Listener 使用的证书与私钥 | 被 Gateway 引用 |
| Caller | 哪个应用或服务正在调用网关 | 授权 Route，并持有访问密钥 |

Console 使用 **Service** 作为产品名称。声明式 API 当前使用 `Upstream` 表达同一个对象，不存在两套平行配置。

## Route 和 Service 类型

普通 API 与 AI 使用相同资源关系：

| 类型 | Route 负责 | Service 负责 |
| --- | --- | --- |
| API | HTTP 匹配、访问方式和转发行为 | HTTP 端点、TLS、负载均衡与健康检查 |
| AI | 发布客户端模型名、选择模型线路 | 厂商协议、服务端点和凭据 |

模型不单独建模为顶层资源。Route 目标线路保存真实模型名，Service 保存连接模型厂商所需的协议和凭据。

## 治理策略

策略通过 `targetRefs[]` 引用目标：

| 策略 | 可引用目标 | 执行位置 |
| --- | --- | --- |
| IPRestrictionPolicy | Gateway、Route | Envoy 原生 RBAC |
| RateLimitPolicy | Gateway、Route | Envoy ext_authz + Authz + Redis |
| TokenQuotaPolicy | Caller | AI ExtProc + Redis |
| HeaderTransformationPolicy | Route | Envoy Wasm 插件 |
| MockResponsePolicy | Route | Envoy Wasm 插件 |

`targetRefs[]` 可以为空，表示策略已经保存但没有应用到流量。删除目标前会检查资源引用，Controller Status 负责表达声明式并发写入后的最终结果。

## ID、名称和版本

- `metadata.name`：声明式资源的不可变 ID；普通资源由 Admin API 生成 UUID，WasmPlugin 按插件包名派生稳定 ID，由 API Server 的原子 Create 保证同一配置域只安装一个版本
- `spec.displayName`：用户可编辑的展示名称，不参与引用、匹配或唯一性判断
- `metadata.generation`：期望状态版本，Admin API 映射为 `version`
- `status.conditions`：Controller 对当前 generation 的接收、引用解析和发布结果

Admin API 面向 Console 返回产品对象，可以按职责组织配置字段，但不直接暴露 `metadata/spec/status`。声明式 API 保留这些字段，以支持 CRUD、Watch、Status 和乐观并发。

## 生效状态

Console 的状态由启用开关、当前 version 和 Status 共同计算：

- **已生效**：当前期望版本已经进入 Envoy 有效配置
- **发布中**：资源已保存，但 Controller 尚未确认当前版本
- **异常**：引用、配置或发布校验失败
- **已停用**：资源被显式停用，不参与数据面配置
- **未应用**：策略已经保存，但没有任何有效目标

状态不是独立存储的业务字段，刷新页面后始终根据资源和 Status 重新计算。

## 配置域的原子发布

Controller 每次读取当前全部资源，编译出一份完整候选配置，只有编译成功且 Envoy 接受后才切换为新的有效配置。因此：

- 任一资源的阻塞级编译错误会阻止整份候选配置发布
- 发布失败时，Envoy 继续使用上一份已经确认的有效配置
- 修复或停用异常资源后，Controller 会用最新资源集合重新编译
- 支撑资源刚创建但尚未形成可发布流量链路时，可能暂时显示“发布中”

排查长期“发布中”时，不要只看当前资源；还要在 Gateway、Route、Service、Certificate、策略和插件中筛选“异常”。一套 Ingate 是一个配置域，不能假设无关页面中的错误一定与本次发布无关。
