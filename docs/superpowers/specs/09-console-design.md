# Ingate v1 Console 设计

## 1. 目标

这份文档定义 `Ingate Console` 的第一版产品与前端设计。

第一版目标不是把所有管理能力一次做完，而是先做出一个有企业级产品感的控制台原型，让后续开发时可以明确回答：

- 用户进入控制台后先看什么
- `Gateway / Route / Backend / Policy` 在界面上怎么组织
- 哪些能力是资源列表，哪些能力是产品化聚合视图
- 前端怎么和 `admin-api` 解耦
- 前端代码应该放在哪里，怎么逐步演进

## 2. 总结论

`Ingate Console` 第一版采用：

```text
左侧资源导航 + 顶部全局栏 + 中间资源工作区 + 右侧详情抽屉
```

整体风格参考 `Higress Console` 的企业控制台形态，但不直接复刻它的传统后台质感。

第一版先做：

- 独立前端应用：`web/console`
- 静态高保真原型
- mock 数据
- 不直接接入真实 `admin-api`
- 页面结构、组件边界和产品信息架构先稳定下来

后续再把 mock 数据替换为 `admin-api` 调用。

## 3. 为什么先做静态原型

现在后端已经有 `ingate-apiserver` 和 `admin-api` 的基础能力，但产品控制台还没有定型。

如果前端一开始就直接接真实接口，会出现几个问题：

- 页面结构容易被接口形状牵着走
- 接口字段变化会频繁影响 UI
- 还没想清楚用户视角，就开始堆 CRUD 页面
- 很难判断这个控制台是不是有企业级产品感

所以第一版应该先解决“长什么样、怎么用、信息怎么组织”。

正确顺序是：

```text
静态产品原型
  -> 确认页面和交互
  -> 抽象前端数据模型
  -> 接入 admin-api
  -> 增加真实错误处理和状态流转
```

## 4. 产品定位

`Ingate Console` 面向的是网关平台使用者，而不是底层资源开发者。

它应该让用户优先看到产品语义：

- 网关入口是否正常
- 路由是否已经生效
- 后端服务是否被引用
- 认证策略和流量策略影响了哪些路由
- 当前配置链路是否存在未解析引用

它不应该要求用户一开始就理解：

- Kubernetes-style `metadata/spec/status`
- apiserver 资源 URL
- watch/list 语义
- controller reconcile 细节

这些底层概念仍然存在，但应该在高级视图或 YAML/JSON 视图里出现。

## 5. 总体布局

第一版采用典型企业控制台布局：

```text
┌─────────────────────────────────────────────────────────┐
│ 顶部栏：Logo / 环境 / 集群 / 文档 / 当前用户              │
├────────────────┬────────────────────────────────────────┤
│ 左侧导航        │ 主工作区                                 │
│                │                                        │
│ Overview       │ 页面标题                                 │
│ Gateways       │ 筛选区                                   │
│ Routes         │ 数据表格 / 卡片 / 拓扑                     │
│ Backends       │                                        │
│ Policies       │ 右侧详情抽屉                              │
│ Topology       │                                        │
│ Diagnostics    │                                        │
│ Settings       │                                        │
└────────────────┴────────────────────────────────────────┘
```

### 5.1 顶部栏

顶部栏负责全局上下文：

- `Ingate` 标识
- 当前环境，例如 `dev`
- 当前控制面地址，例如 `https://127.0.0.1:9443`
- 文档入口
- 当前用户

第一版先用静态展示，不做真实登录。

### 5.2 左侧导航

左侧导航是主要信息架构：

```text
概览
网关
路由
后端服务
策略中心
拓扑视图
事件与诊断
系统设置
```

`策略中心` 下后续可以拆分：

```text
认证策略
流量策略
```

第一版可以先做一级导航，页面内部再区分策略类型。

### 5.3 主工作区

主工作区要支持三类页面：

- 仪表盘页面
- 资源列表页面
- 详情与聚合视图页面

列表页面应该统一包含：

- 标题
- 主操作按钮
- 搜索与筛选
- 状态摘要
- 表格
- 详情抽屉

### 5.4 右侧详情抽屉

企业控制台里，右侧详情抽屉比频繁跳转详情页更适合资源管理。

第一版的详情抽屉用于展示：

- 基本信息
- 状态
- 关联资源
- 最近变更
- YAML/JSON 预览

例如点击一个 `Route` 后，右侧展示：

```text
Route 基本信息
  -> 匹配规则
  -> 绑定 Gateway
  -> 指向 Backend
  -> 认证策略
  -> 流量策略
  -> 生效状态
  -> 原始资源对象
```

## 6. 页面范围

第一版建议只做以下页面。

### 6.1 Overview

`Overview` 是控制台首页，解决“系统现在怎么样”的问题。

展示内容：

- Gateway 数量
- Route 数量
- Backend 数量
- Policy 数量
- 异常引用数量
- 最近变更
- 控制面健康状态

这不是简单统计页，而是用户进入系统后的态势感知页。

### 6.2 Gateways

`Gateways` 展示网关入口。

表格字段：

- 名称
- 状态
- 监听端口
- 绑定路由数
- 异常数
- 最后更新时间

详情抽屉展示：

- Gateway 基本信息
- Listener 配置
- 关联 Route
- 拓扑摘要
- 原始资源对象

### 6.3 Routes

`Routes` 是第一版最重要的页面。

表格字段：

- 名称
- Host
- Path
- 绑定 Gateway
- 目标 Backend
- 认证策略
- 流量策略
- 生效状态
- 最后更新时间

详情抽屉展示：

- 匹配规则
- 后端服务
- 策略链
- 生效状态
- 未解析引用
- 原始资源对象

### 6.4 Backends

`Backends` 展示后端服务。

表格字段：

- 名称
- 地址
- 端口
- 被引用次数
- 状态
- 最后更新时间

详情抽屉展示：

- Endpoint 信息
- 引用它的 Routes
- 健康状态
- 原始资源对象

第一版健康状态可以来自 mock 数据。

### 6.5 Policies

`Policies` 展示认证策略和流量策略。

第一版可以用 Tab 区分：

```text
认证策略 | 流量策略
```

认证策略字段：

- 名称
- 类型
- 绑定对象
- 状态
- 最后更新时间

流量策略字段：

- 名称
- 策略类型
- 绑定对象
- 状态
- 最后更新时间

### 6.6 Topology

`Topology` 用来展示资源关系。

第一版目标不是做复杂图编辑器，而是展示清晰关系：

```text
Gateway -> Route -> Backend
             |
             +-> AuthPolicy
             +-> TrafficPolicy
```

它主要帮助用户理解：

- 某个 Gateway 下挂了哪些 Route
- 某个 Route 最终打到哪些 Backend
- 哪些 Policy 正在影响请求链路
- 哪些引用没有解析成功

### 6.7 Diagnostics

`Diagnostics` 是事件与诊断页。

第一版先展示静态事件：

- 配置创建
- 配置更新
- 未解析引用
- 控制面健康检查
- 策略生效失败

后续可以接入真实事件、日志或审计能力。

## 7. 视觉方向

第一版采用“企业级网关控制面”的视觉方向。

关键词：

```text
稳重
清晰
高密度
可观察
有产品感
```

避免：

- 纯白低对比度表格后台
- 泛紫色渐变
- 只有卡片没有信息密度
- 过度装饰导致像营销页
- 完全照搬 Higress Console

建议视觉元素：

- 深墨蓝或石墨灰作为主导航底色
- 青绿表示正常状态
- 琥珀表示风险状态
- 红色表示错误状态
- 蓝色用于主操作
- 浅灰白背景叠加细微网格或分层卡片
- 资源状态用 badge 明确表达
- 页面加载、抽屉打开、拓扑连线使用克制动画

## 8. 前端目录

第一版放在：

```text
web/console/
```

推荐目录：

```text
web/console/
  package.json
  tsconfig.json
  vite.config.ts
  index.html
  src/
    main.tsx
    app/
      App.tsx
      routes.tsx
    layout/
      ConsoleLayout.tsx
      Sidebar.tsx
      Topbar.tsx
    pages/
      OverviewPage.tsx
      GatewaysPage.tsx
      RoutesPage.tsx
      BackendsPage.tsx
      PoliciesPage.tsx
      TopologyPage.tsx
      DiagnosticsPage.tsx
      SettingsPage.tsx
    features/
      gateways/
      routes/
      backends/
      policies/
      topology/
    components/
      DataTable.tsx
      StatusBadge.tsx
      DetailDrawer.tsx
      MetricCard.tsx
      EmptyState.tsx
    api/
      client.ts
      mock.ts
      types.ts
    styles/
      tokens.css
      global.css
```

目录原则：

- `app/` 放应用入口和路由
- `layout/` 放控制台外壳
- `pages/` 放页面编排
- `features/` 放业务资源组件
- `components/` 放通用 UI 组件
- `api/` 放数据访问边界
- `styles/` 放设计变量和全局样式

## 9. 技术栈

第一版建议：

```text
Vite
React
TypeScript
CSS Modules 或普通 CSS
```

不建议第一版引入很重的 UI 框架。

原因：

- 我们现在需要先建立自己的产品气质
- 太早引入组件库容易变成普通后台
- 当前页面范围不大，手写基础组件成本可控
- 后续真要企业级表格、表单、弹窗，可以再评估组件库

第一版可以先手写：

- Layout
- Sidebar
- Topbar
- Table
- Badge
- Drawer
- Metric Card
- Topology View

## 10. 数据流

第一版数据来自 mock：

```text
Page
  -> api/mock.ts
  -> mock resource list
  -> components
```

后续接入真实接口后：

```text
Page
  -> api/client.ts
  -> admin-api
  -> ingate-apiserver
```

前端不直接访问 `ingate-apiserver`。

正确调用方向：

```text
Console -> admin-api -> ingate-apiserver
```

禁止：

```text
Console -> ingate-apiserver
Console -> etcd
```

## 11. 与 admin-api 的关系

`Console` 面向用户，`admin-api` 面向产品接口。

第一版原型里的页面应该对应后续这些接口：

```text
GET /admin/v1/gateways
GET /admin/v1/gateways/:name
GET /admin/v1/gateways/:name/topology

GET /admin/v1/routes
GET /admin/v1/routes/:name
GET /admin/v1/routes/:name/effective-status

GET /admin/v1/backends
GET /admin/v1/auth-policies
GET /admin/v1/traffic-policies
```

创建、更新、删除能力可以后续再做。

第一版控制台先重点验证：

- 信息架构
- 列表体验
- 详情体验
- 拓扑体验
- 状态表达

## 12. 第一版不做什么

第一版明确不做：

- 登录系统
- 权限系统
- 国际化
- 真实监控指标
- 真实事件流
- 复杂表单编排
- YAML 编辑器
- 拖拽式拓扑编辑
- 多集群管理

这些能力都可以后续扩展，但不应该阻塞第一版原型。

## 13. 验收标准

第一版原型完成后，应满足：

- 可以通过 `make run-console` 或等价命令启动
- 可以通过浏览器访问控制台
- 左侧导航完整
- 顶部栏完整
- `Overview / Gateways / Routes / Backends / Policies / Topology / Diagnostics / Settings` 页面可访问
- 页面在桌面端布局正常
- 移动端至少不破版
- 资源列表、状态 badge、详情抽屉、拓扑视图都有可感知的企业级质感
- 使用 mock 数据即可完整浏览主要页面

## 14. 一句话结论

`Ingate Console` 第一版应该先作为 `web/console` 下的静态高保真企业控制台原型出现，采用左右栏企业控制台结构，重点展示 `Gateway / Route / Backend / Policy / Topology` 的产品视图，后续再通过清晰的 `api/` 边界接入 `admin-api`。
