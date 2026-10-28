# Ingate 控制台

基于 React、TypeScript 和 Vite，页面按“网关—路由—服务”组织。协议以
`api/admin/v1/*.proto` 为准，不复用 main 分支旧版 Service/Route 数据模型。

## 本地开发

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

**浏览器登录尚未接通。** 当前 Admin API 只校验 Bearer Session；前端只使用同源
Cookie，不读写 Token，不写入 localStorage/sessionStorage，也不发送后台身份头。
未认证返回 401 时会明确显示接入缺口，不回退到示例数据。浏览器 Cookie 认证、CSRF、
HTTPS 和登录限流需要后端及部署侧单独实现；本轮没有增加 BFF 或修改认证协议。

服务页将 Backend 展示为“网络服务（应用/MCP 共用）”，AIBackend 展示为“模型服务”。
Backend 暂无用途分类字段，因此不虚构可持久化的应用/MCP 分类。模型服务通过选择已有
网络服务来配置连接；不在前端串联创建、更新或级联删除两个资源。

地址、端口、主机名、路径与客户端模型名的独立筛选仍需要后端查询契约，当前不开放。
不沿用旧版的启停、健康状态、流量统计或凭据配置，不把保存成功解释为数据面生效。

写入响应来自 API Server，列表和详情查询来自 MySQL 异步投影。保存后展示写入响应，
刷新列表可能暂时看到旧值；页面提示投影延迟，不通过无限轮询或前端伪造状态掩盖它。
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
