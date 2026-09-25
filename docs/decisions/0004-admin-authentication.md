# 0004：Admin API 的首版认证边界

- 状态：已接受（实现细节仍需逐项评审）
- 日期：2026-09-25

## 背景

Admin API 使用自身的 API Server 凭据创建 Gateway。当前 `CreateGateway` 没有调用方认证，因此启动配置暂时只接受本机回环地址。回环监听限制了网络暴露范围，但不能证明本机调用者的身份，也不能作为正式部署的认证方案。

GoCanon 的 User Account 示例已经实现注册、登录和 Redis Session；Ingate 是网关管理产品，首个管理员的创建流程不能直接照搬公开注册接口。

## 已确认的产品边界

- 首版使用内建管理员账户，不要求部署外部 OIDC 服务。
- 首版先服务 CLI 和 API 客户端，通过 `Authorization: Bearer <session_token>` 传递登录态。
- HTTP 和 gRPC 都是公开 Admin API；gRPC 不因传输协议不同而绕过认证。

## 本决策建议的第一条实现链路

安装时通过离线管理操作创建首个管理员；公开 API 不提供匿名 `Register`。离线操作应从交互输入或受保护的文件读取密码，不允许把密码放在命令行参数中。具体命令形式与安装流程一起评审，不在当前代码中预建入口。

管理员账户及密码哈希保存在 MySQL。密码哈希由成熟密码库生成和校验，不自定义摘要算法。Redis 保存多实例 Admin API 共享的不透明 Session：登录成功后生成随机 Token，只向调用方返回一次；Redis 使用 Token 摘要定位 Session，并设置固定的绝对有效期。原始密码、哈希和 Token 不进入日志、Trace 或指标。

首个公开协议只需要 `Login`、`Logout` 和读取当前账户；`CreateGateway` 等现有及后续管理操作默认要求有效 Session。只有 `Login` 被显式列为匿名操作。认证中间件区分「Session 不存在或过期」与「Redis 无法裁决」，后者不得伪装成未登录。`Logout` 删除当前 Session；已通过认证并进入 Handler 的并发请求不因此回滚。

第一版只有安装时创建的管理员，不提供公开的新增账户、角色或授权管理接口。因此认证通过即可执行当前管理操作；增加第二类用户之前，必须先定义授权规则。会话认证与业务 Handler 分离，Handler 接收已认证的账户身份，Service 不从 Context Value 隐式读取身份。

## 部署与客户端约束

包含密码或 Bearer Token 的请求必须通过 TLS 保护。接入认证、TLS 和登录限流之前，保留现有的回环监听限制；不能只因有了 `Login` 就允许监听所有网卡。登录响应设置 `Cache-Control: no-store`。

首版 Bearer Session 面向 CLI/API 客户端，不要求浏览器 JavaScript 保存 Token。浏览器控制台接入时，应另行确定 BFF，或使用 `HttpOnly`、`Secure`、`SameSite` Cookie 并处理 CSRF；不得把 Session Token 放入 `localStorage` 或 `sessionStorage`。外部自动化客户端需要 PAT 或委托凭据时，再定义签发、权限范围、到期和撤销；不把人工 Session 当作长期机器凭据。内部服务则使用独立的工作负载身份，不走公开 Session 白名单。这些后续凭据不在首版实现范围内。

## 尚待评审的实现细节

本决策只固定身份来源和首版客户端边界。开始实现前，还需逐项确认：

1. 首个管理员的离线创建命令、重复执行语义和密码输入方式。
2. 账户表字段、迁移方式，以及密码策略与密码哈希参数。
3. Session 的有效期、Redis Key 与值格式、故障转移下的撤销语义。
4. `AccountService` 的 proto 字段、公开错误和登录限流规则。
5. TLS 在进程内还是可信入口终止，以及 Admin API 如何识别直接调用方地址。

这五项确认前，不增加空的认证中间件、占位账户表或固定共享 Token。当前回环监听仍是临时保护，不能视为生产就绪。

## 依据

- GoCanon `0031-user-account-authentication.md` 提供技术参考，但其匿名注册用例不适用于 Ingate。
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)说明随机 Session ID、Cookie 属性与浏览器存储风险。
- [OWASP REST Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/REST_Security_Cheat_Sheet.html)说明 API 凭据的传输保护。
