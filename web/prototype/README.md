# Ingate 产品原型

这个目录是独立的 API、AI 与 MCP 网关产品原型，只用于验证信息架构和用户流程。

- 所有页面使用同一份 Mock 数据
- 不调用 Admin API，也不需要登录
- 不与 `web/console` 共享组件、状态或业务代码
- 原型能力不会自动成为后端资源或生产功能

本地运行：

```bash
npm ci
npm run dev
```

默认地址为 `http://127.0.0.1:5174`。
