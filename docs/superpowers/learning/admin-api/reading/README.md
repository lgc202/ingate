# admin-api 源码阅读目录

这个目录专门用来读 `ingate-admin-api`。

阅读顺序不要反过来。先会运行、会验证，再看源码；先看一次请求怎么走通，再看分层和工具脚本。

## 推荐阅读顺序

1. [00-how-to-run-and-verify.md](00-how-to-run-and-verify.md)
2. [01-big-picture.md](01-big-picture.md)
3. [02-create-gateway-request-flow.md](02-create-gateway-request-flow.md)
4. [03-crud-update-delete.md](03-crud-update-delete.md)
5. [04-middleware-auth-request-id.md](04-middleware-auth-request-id.md)
6. [05-layer-by-layer.md](05-layer-by-layer.md)
7. [06-topology-effective-status.md](06-topology-effective-status.md)
8. [07-tools-makefile-generated-code.md](07-tools-makefile-generated-code.md)

## 读完后应该知道什么

读完后你应该能回答：

- admin-api 怎么启动？
- admin-api 怎么验证？
- 为什么 admin-api 不直接连 etcd？
- Gin handler 做什么，不做什么？
- DTO 是什么，为什么不直接暴露 `pkg/apis/...` 结构？
- biz 层现在为什么看起来薄？以后会放什么？
- convert 层为什么单独存在？
- generated clientset 是怎么被 admin-api 用起来的？
- 为什么 update 要先 get 再 update？
- middleware 是怎么保护 `/admin/v1/*` 的？
- topology 和 effective-status 为什么算业务接口？
- `make verify-admin-api` 到底验证了什么？

## 当前阶段边界

当前 admin-api 是“产品 API 层”的第一阶段完整闭环，不是最终企业后台。

已经有：

- 五类资源完整 CRUD
- Bearer Token 认证
- request id
- topology 聚合
- effective-status 聚合
- 端到端验证脚本

暂时没有：

- 多用户登录系统
- RBAC
- 多租户
- audit
- 请求幂等键
- OpenAPI/Swagger 输出
- 前端控制台适配
