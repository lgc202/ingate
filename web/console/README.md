# Ingate 控制台

基于 React、TypeScript 和 Vite，页面按“网关—路由—服务”组织。协议以
`api/admin/v1/*.proto` 为准，不复用 main 分支旧版 Service/Route 数据模型。

## 本地开发

整套开发环境可在仓库根目录执行 `make dev-up`；依赖准备、单组件调试和停止方式见
[开发运行说明](../../docs/development.md)。

需要 Node.js 24 与 npm 11 或 12：

```sh
cd web/console
npm ci --ignore-scripts
npm run dev
```

开发服务器只监听 `127.0.0.1:5173`，把 `/api/admin/v1` 转发至
`127.0.0.1:8000`。这只是开发反向代理，不是 BFF。生产构建为静态文件，部署方需提供
SPA 回退及同源 `/api/admin/v1` HTTPS 入口。

打开 `/gateways?namespace=<已有命名空间>`。当前 Admin API 没有 Namespace 列表接口，
顶部输入框只选择已有 Namespace，不负责创建；命名空间改变后清空页面状态与在途查询。

## 当前接入范围

- Gateway、Backend、AIBackend、HTTPRoute、ModelRoute、MCPRoute 的列表、分页、详情、
  创建、完整更新和删除。左侧只显示网关、路由、服务，类型在页面内选择。
- 展示名称查询，以及三类路由的入口网关筛选，全部交给服务端分页。不加载全量资源后
  在浏览器筛选，也不把当前页筛选伪装成全局查询。
- 多 Listener、多端点及相对权重、有序规则、OR 匹配、父引用、TLS/System CA、模型名
  覆盖与 MCP 上游路径。编辑使用完整资源，保留未改动的规则；不只保存第一条摘要。
- 资源名称由后台生成；前端原样携带字符串版本，冲突时保留草稿，不自动覆盖。
- 资源选择器独立分页查找，展示名可重复，选项附带稳定标识用于辨认。缺失引用不自动
  指向其他对象；后端最终裁决引用、冲突和删除行为。

## 明确边界

浏览器直接访问 Admin API，使用已有管理员账户与 Redis Session，不增加 BFF。
登录响应不返回 Token，浏览器只发送 HttpOnly、Secure、SameSite=Strict Cookie；
前端不读写 localStorage/sessionStorage。写操作发送 `X-Ingate-Browser: 1`，服务端校验
同源 Origin，代理须保留公开 Host。会话失效回到登录页，退出撤销原 Session 并删除 Cookie。
外部部署须提供 HTTPS；本机 loopback 用于开发，登录限流仍未完成。

服务页将 Backend 展示为“网络服务（应用/MCP 共用）”，AIBackend 展示为“模型服务”。
Backend 暂无用途分类字段，因此不虚构可持久化的应用/MCP 分类。模型服务通过选择已有
网络服务来配置连接；不在前端串联创建、更新或级联删除两个资源。

地址、端口、主机名、路径与客户端模型名的独立筛选仍需要后端查询契约，当前不开放。
不沿用旧版的启停、健康状态、流量统计或凭据配置，不把保存成功解释为数据面生效。

写入响应来自 API Server，列表和详情查询来自 MySQL 异步投影。保存后关闭编辑抽屉回到列表，
最多等待 8 秒确认投影配置与保存结果一致（删除则确认 404），然后自动刷新列表。
不把资源版本当作数字比较，Status 更新也会改变版本；只比较可写配置，并遵守 Listener/
Endpoint 无序、路由规则有序的契约。超时或查询失败会说明写入已成功但刷新未确认，
不无限轮询、不重放写请求，也不把列表刷新解释为数据面生效。刷新保留原表格，末页最后
一条被删除时自动返回上一页；新建后清除旧筛选并返回第一页。
删除请求没有版本字段，前端不声称它具备条件删除或自动级联能力。

## 检查

```sh
npm run format:check
npm test
npm run build
```

单元测试检查 API 参数、字符串版本、错误和完整配置回写，使用隔离的 fetch 替身，
不操作真实数据。浏览器测试响应只能证明前端交互；没有实际运行 Admin API、API Server、
MySQL、Redis 与代理链路，就不能视为登录、查询投影或数据面端到端验证通过。
