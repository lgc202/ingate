# 前端数据架构

## 目标

前端页面不直接依赖 mock 数据，也不直接拼接后端请求。所有页面通过统一的数据仓储接口读取资源，当前默认使用 mock adapter，后续接入真实后端时只替换 adapter，不重写页面组件。

## 分层

- `src/domain/*`：产品领域模型，描述网关、路由、服务等用户能理解的资源。
- `src/api/contracts.ts`：前端需要的数据仓储接口。
- `src/api/client.ts`：选择当前运行模式下的数据仓储实现。
- `src/api/useResource.ts`：页面读取异步资源的通用 hook，统一处理 loading 和 error。
- `src/mocks/*`：mock adapter，实现与真实后端相同的仓储接口。
- `src/features/*`：页面和业务视图，只消费 domain model，不关心数据来自 mock 还是后端。

## 当前接入范围

已接入统一数据边界的页面：

- 首页
- 网关
- 路由
- 服务
- 策略
- 插件
- 观测
- 设置

所有现有主导航页面都已经通过 repository 获取数据。后续新增页面也必须按同样模式接入。

## 后端接入方式

后续接入真实后端时，新增 live adapter，例如：

- `src/api/liveConsoleRepository.ts`
- 保持 `ConsoleRepository` 方法签名不变
- 在 `src/api/client.ts` 中根据 `VITE_INGATE_API_MODE=live` 切换实现

页面不应该直接使用 `fetch`、`axios` 或后端 URL。请求错误、字段错误、权限错误都应先在 `src/api/errors.ts` 归一化，再交给页面展示。

## 命名约定

- 面向用户的模型使用产品语义，例如 `Gateway`、`RouteResource`、`ServiceResource`。
- 不在 UI 层暴露后端实现细节或协议名。
- 页面文案统一使用中文。
