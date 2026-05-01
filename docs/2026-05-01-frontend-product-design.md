# Ingate 前端产品设计

本文档记录 Ingate 控制台第一版前端设计结果。目标是先把产品信息架构定住，再逐步替换成真实前端实现。

## 设计定位

Ingate 是统一网关控制台，不把 AI 单独拆成另一套系统。

核心对象：

```text
网关
路由
后端服务
证书
插件
```

AI 能力融入网关主线，后端服务承载应用、模型、Agent 和 MCP。

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

## 页面边界

- 首页：只做资源汇总和运行概览
- 网关：入口监听、域名、证书绑定、数据面组
- 路由：匹配规则、后端转发、策略配置
- 后端服务：应用、模型服务、Agent、MCP
- 证书：TLS 凭据资源
- 插件：插件包、版本、部署范围、运行状态
- 监控：请求量、错误率、延迟、AI Token、调用日志
- 系统设置：环境、数据面组、用户、审计、通知、全局参数

## 核心交互约定

- 前端面向用户表达产品对象，不直接暴露 Kubernetes 资源对象
- 首页只做轻量汇总和 overview，不承载复杂配置
- 网关配置需要支持 `环境 -> 数据面组` 联动选择
- 数据面组是一组高可用 Envoy 实例，不是单个 Envoy
- 一个网关可以配置多个监听
- 一个监听可以配置多个访问域名
- HTTPS 监听必须选择证书，证书在证书页面作为独立资源管理
- 详情页只读，编辑动作进入独立配置页或抽屉
- 列表页统一提供查询、重置、详情、编辑、删除、上一页、下一页
- 配置型资源数量通常不大，列表分页第一阶段可以前端分页
- 时间显示使用完整时间，精确到秒，例如 `2025-04-30 14:20:35`

## 路由策略设计

路由配置分为四步：

```text
匹配条件 -> 后端服务 -> 策略配置 -> 发布预览
```

策略不直接全部塞在向导页里。向导页展示策略分类和启用状态，点击具体策略后进入右侧抽屉配置参数。

策略来源分两类：

- 原生策略：路径重写、Header 处理、超时、重试、限流等网关基础能力
- 插件策略：认证、AI Token 统计、内容安全、AI Proxy、工具治理等扩展能力

插件页只负责插件包、版本、部署范围和运行状态。路由页负责启用某个策略，并填写当前路由自己的参数。未在路由启用的策略不会因为插件已部署而自动生效。

## AI 融合方式

AI 不单独拆成一套导航。AI 能力通过下面方式融入主线：

- 后端服务承载模型服务、Agent 和 MCP
- 路由选择模型服务、Agent 或 MCP 作为目标
- 插件提供 AI Token 统计、内容安全、AI Proxy、工具治理等能力
- 监控页展示 AI Token、模型调用、Provider 维度数据

这样既保留网关主线，也能继续演进 AI Gateway。

## 列表页规范

- 查询条件区有明确 `查询` 和 `重置`
- 表格底部有 `上一页` / `下一页`
- 列表行操作统一是 `详情 / 编辑 / 删除`
- 时间列统一叫 `更新时间`
- 真实时间精确到秒

## 关键页面

### 首页

![首页 UI](images/frontend-home-overview-ui.svg)

### 网关

![网关列表 UI](images/frontend-gateway-list-ui.svg)
![网关配置 UI](images/frontend-gateway-config-ui.svg)
![网关详情 UI](images/frontend-gateway-detail-ui.svg)

### 路由

![路由列表 UI](images/frontend-route-list-ui.svg)
![创建路由 01 基本信息与匹配](images/frontend-create-route-step1-match-ui.svg)
![创建路由 02 选择后端服务](images/frontend-create-route-step2-target-ui.svg)
![创建路由 03 配置策略](images/frontend-create-route-step3-policy-ui.svg)
![创建路由 04 发布预览](images/frontend-create-route-step4-preview-ui.svg)
![路由详情 UI](images/frontend-route-detail-ui.svg)
![路由策略配置抽屉 UI](images/frontend-route-policy-drawer-ui.svg)

### 后端服务

![后端服务列表 UI](images/frontend-backend-service-list-ui.svg)
![后端服务详情 UI](images/frontend-backend-service-detail-ui.svg)
![创建后端服务 UI](images/frontend-create-backend-service-ui.svg)

### 证书

![证书列表 UI](images/frontend-certificate-list-ui.svg)
![证书详情 UI](images/frontend-certificate-detail-ui.svg)
![创建证书 UI](images/frontend-create-certificate-ui.svg)

### 插件

![插件列表 UI](images/frontend-plugin-list-ui.svg)
![插件详情 UI](images/frontend-plugin-detail-ui.svg)
![插件部署 UI](images/frontend-plugin-deploy-ui.svg)

### 监控

![监控首页 UI](images/frontend-monitor-overview-ui.svg)

### 系统设置

![系统设置 UI](images/frontend-system-settings-ui.svg)

## Admin API 方向

前端不直接感知 Kubernetes 风格资源，而使用产品 DTO。

```text
GatewayView
RouteView
BackendServiceView
CertificateView
PluginView
MonitorOverview
SystemSettingsView
```

## 插件化路线

Plugin-first，但不是 Plugin-only。插件负责能力扩展，路由负责启用和参数配置，插件页负责部署和运行状态。

```text
发布插件包 -> 部署到数据面组 -> 路由配置策略 -> 控制面下发 -> 数据面生效
```
