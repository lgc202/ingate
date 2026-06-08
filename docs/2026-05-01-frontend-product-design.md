# Ingate 前端产品设计

本文档记录 Ingate 控制台第一版前端产品设计。它是后续前端实现和 Admin API 设计的约束文档。文档里的图片是页面线框设计稿，只表达信息架构、字段归属和交互边界，不作为最终视觉稿。

## 设计定位

Ingate 是统一网关控制台，面向使用网关发布接口和 AI 能力的用户，不把 AI 单独拆成另一套系统。

主链路围绕四个对象组织：

```text
网关 -> 路由 -> 后端服务
             -> 插件策略
```

同时把 `证书` 作为一级配置资源，因为证书会被网关监听器和域名绑定复用，不能只藏在网关表单里。

## 第一版导航

```text
首页
网关
路由
后端服务
证书
插件
监控
系统设置
```

统一命名规则：

- `首页`：资源汇总、运行概览、最近变化
- `后端服务`：包含应用、模型服务、Agent 服务、MCP 服务
- `证书`：独立 TLS 凭据资源
- `系统设置`：平台配置，不是运行状态看板
- 不再使用偏技术化的传统接口命名，统一叫 `应用`
- 前端不出现底层配置导入入口、数据面配置协议、运行时快照这类实现词

## 全局交互规则

- 当前环境是全局上下文，在页面顶部切换，不在每个创建表单里重复配置
- 数据面组隶属于当前环境，表单中选择数据面组时只展示当前环境下的可用组
- 数据面组不是单个 Envoy，而是一组高可用数据面实例
- 列表页统一包含查询、重置、上一页、下一页
- 列表行操作统一为 `详情 / 编辑 / 删除`
- 详情页只读，不在详情页直接编辑核心配置
- 下拉选择用于所属网关、后端服务、数据面组、证书资源等引用关系
- 时间展示统一精确到秒，例如 `2025-04-30 14:20:35`
- 解释性文字只在关键点使用，不把大量说明铺在页面里

## 页面边界

### 首页

首页只做简单 overview，不承载复杂操作。

#### 首页概览

![首页 UI](images/frontend-home-overview-ui.svg)

### 网关

网关负责入口监听、数据面组和域名绑定。证书内容不在网关里维护，网关只引用证书资源。

创建或编辑网关时：

- 基础信息配置网关名称、数据面组和说明
- 监听器只定义协议、端口、TLS 模式
- 域名绑定单独成表，一个监听器可以绑定多个域名
- 域名绑定行选择证书资源，支持不同域名绑定不同证书
- HTTP 监听器可以不使用证书

#### 网关列表

![网关列表 UI](images/frontend-gateway-list-ui.svg)

#### 网关配置

![网关配置 UI](images/frontend-gateway-config-ui.svg)

#### 网关详情

![网关详情 UI](images/frontend-gateway-detail-ui.svg)

### 路由

路由负责把请求匹配、转发目标和策略串起来。这里的“策略”是控制台面向用户的产品语言，不等同于后端统一 `Policy` 资源。

概念边界：

- 匹配规则只决定请求是否命中路由
- 后端服务只决定命中后转发到哪里
- 策略配置决定命中后如何治理、改写、保护和观测；控制台可以是一体化向导，但保存时按资源边界拆成 Route、PolicyBinding 和对应强类型 Policy 多次调用
- 发布预览展示产品化影响范围，不展示底层配置明细

匹配规则包括：所属网关、匹配域名、匹配路径、请求方法、Header、Query、Cookie、来源 IP。

策略配置包括：认证、鉴权、限流、配额、超时、重试、路径改写、Header 改写、请求体限制、缓存、审计、访问日志、Token 配额、内容安全、Prompt 注入防护、PII 脱敏、AI 调用统计、模型 fallback。

策略不等于插件，也不等于后端 `Policy` 资源。策略是用户意图，底层分为三类实现：Route 原生能力、可复用治理 Policy、运行时 Plugin。超时、重试、路径改写、Header 改写属于 Route 原生能力；认证、限流、访问控制属于可复用治理 Policy；AI 内容安全、Token 统计、语义缓存等更适合 Plugin 或 AI 专用资源编译出的插件配置。

控制台策略能力目录属于前端产品配置，不由 admin-api 提供通用目录接口。保存请求必须使用稳定字段和资源 ID，`displayName` 只用于页面展示，不能用中文策略名驱动后端转换或编译逻辑。

#### 路由列表

![路由列表 UI](images/frontend-route-list-ui.svg)

#### 创建路由 - 第 1 步

![创建路由 01 基本信息与匹配](images/frontend-create-route-step1-match-ui.svg)

#### 创建路由 - 第 2 步

![创建路由 02 选择后端服务](images/frontend-create-route-step2-target-ui.svg)

#### 创建路由 - 第 3 步

![创建路由 03 配置策略](images/frontend-create-route-step3-policy-ui.svg)

#### 创建路由 - 第 4 步

![创建路由 04 发布预览](images/frontend-create-route-step4-preview-ui.svg)

#### 路由详情

![路由详情 UI](images/frontend-route-detail-ui.svg)

#### 路由策略抽屉

![路由策略配置抽屉 UI](images/frontend-route-policy-drawer-ui.svg)

### 后端服务

后端服务是路由的转发目标，统一承载：

```text
应用
模型服务
Agent 服务
MCP 服务
```

AI 能力不拆成独立导航，而是通过模型服务、Agent 服务、MCP 服务和插件策略融入网关主线。

#### 后端服务列表

![后端服务列表 UI](images/frontend-backend-service-list-ui.svg)

#### 后端服务详情

![后端服务详情 UI](images/frontend-backend-service-detail-ui.svg)

#### 创建后端服务

![创建后端服务 UI](images/frontend-create-backend-service-ui.svg)

### 证书

证书是独立资源，负责统一管理 TLS 凭据、绑定域名、证书链、有效期、续期记录和最近错误。

证书创建支持：

- 手动上传证书和私钥
- 通过 ACME 自动签发
- 引用已有 Secret 或外部证书

网关域名绑定只选择证书资源，不在网关页粘贴证书内容。

#### 证书列表

![证书列表 UI](images/frontend-certificate-list-ui.svg)

#### 证书详情

![证书详情 UI](images/frontend-certificate-detail-ui.svg)

#### 创建证书

![创建证书 UI](images/frontend-create-certificate-ui.svg)

### 插件

插件页是企业级能力管理页面，不是轻量展示页。

插件页负责：

- 插件包
- 插件版本
- 包来源和校验摘要
- 部署范围
- 数据面组部署状态
- 运行状态
- 被哪些路由使用

路由策略页负责启用策略和配置当前路由参数。插件部署不会自动给未配置的路由套默认限制。

一个插件可以提供多个策略能力，不同路由可以配置不同参数。插件私有配置可以是 JSON，但插件的生效目标、阶段、优先级和失败策略必须由后端结构化字段表达。

#### 插件列表

![插件列表 UI](images/frontend-plugin-list-ui.svg)

#### 插件详情

![插件详情 UI](images/frontend-plugin-detail-ui.svg)

#### 插件部署

![插件部署 UI](images/frontend-plugin-deploy-ui.svg)

### 监控

监控页只看流量运行态，不做系统设置。

第一版范围：

- 请求量
- 错误率
- P95 / P99 延迟
- 路由维度统计
- 后端服务维度统计
- 插件运行异常
- AI Token 用量
- 最近调用日志

监控数据不进入 etcd。后端应从 Envoy、插件运行时和 AI Runtime 打点，经 Prometheus、ClickHouse、Loki 或 OpenTelemetry 聚合后由 Admin API 提供产品化接口。

#### 监控首页

![监控首页 UI](images/frontend-monitor-overview-ui.svg)

### 系统设置

系统设置只放平台配置，不展示运行看板。

第一版范围：

- 环境管理
- 数据面组管理
- 用户与角色
- 审计设置
- 通知设置
- 全局安全
- 系统参数

不放流量趋势、调用日志、异常摘要、证书列表、配置发布记录和数据面运行看板。

#### 系统设置

![系统设置 UI](images/frontend-system-settings-ui.svg)

## 最终口径核对

| 页面 | 最终口径 | 不应该出现 |
| --- | --- | --- |
| 首页 | 简单 overview，只展示资源汇总、运行概览和最近变化 | 复杂配置入口、产品介绍大图 |
| 网关 | 入口、监听器、域名绑定和数据面组；证书只引用独立证书资源 | 在网关表单里粘贴证书内容 |
| 路由 | 匹配规则、后端服务、策略配置三段分离；Route 保存接口只提交 Route 原生能力，治理策略和插件通过独立绑定接口保存 | 把超时、重试、路径改写放进匹配条件；把所有策略都保存成一坨 JSON；让 Route 保存接口聚合 Policy 和 Plugin |
| 后端服务 | 应用、模型服务、Agent 服务、MCP 服务统一作为转发目标 | 把 AI 拆成独立系统 |
| 证书 | 一级资源，可被多个网关域名绑定复用 | 藏在网关配置页里维护 |
| 插件 | 管理插件包、版本、发布范围和运行状态 | 路由未配置时自动套默认策略 |
| 监控 | 只展示运行态流量、错误、延迟、Token 和日志 | 系统设置、证书管理、数据面配置 |
| 系统设置 | 环境、数据面组、用户角色、审计、通知、安全和全局参数 | 运行看板和调用日志 |

## Admin API 方向

前端不直接感知内部声明式资源，而使用产品 DTO。

```text
GatewayView
RouteView
UpstreamServiceView
CertificateView
PluginView
MonitorOverview
SystemSettingsView
```

后端再把产品 DTO 翻译成内部声明式资源和数据面配置。Route DTO 只覆盖 Route 原生能力；治理策略通过 PolicyBinding 和对应强类型 Policy 的 DTO 保存。控制台中文展示名不能进入 compiler 作为协议判断依据。

## 插件化路线

Plugin-first，但不是 Plugin-only。

```text
发布插件包 -> 部署到数据面组 -> 路由配置策略 -> 控制面下发 -> 数据面生效
```

策略是用户视角，插件是运行实现。超时、重试、路径改写、Header 改写这类能力应由 Route 原生能力实现；认证、限流、访问控制这类可复用治理能力应由 Policy 资源和 PolicyBinding 实现；AI 内容安全、Token 统计、语义缓存、RAG、审计等能力更适合插件化实现。
