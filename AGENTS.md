# Ingate 开发约定

本文件只约束代码修改方式，不重复记录产品功能和组件清单。开始修改前先阅读相关代码及正式文档：

- [系统架构](docs/src/content/docs/concepts/architecture.md)
- [资源模型](docs/src/content/docs/concepts/resources.md)
- [当前能力](docs/src/content/docs/reference/current-scope.md)

## 不可破坏的边界

- Envoy 是唯一数据平面，使用官方镜像；不要维护私有 Envoy 分支或为其他数据平面预留抽象
- 一套 Ingate 对应一个环境和配置域，可以包含多个逻辑 Gateway
- 声明式资源是控制面的持久化事实，配置链路为 `Resource -> Compiler -> Delivery -> xDS -> Envoy`
- Controller 与 xDS 位于同一进程；可发布配置只保存在内存，重启后重新编译
- 产品流量模型统一为 `Gateway -> Route -> Service`；API、AI 和 MCP 是 Route 与 Service 的类型
- etcd 只由 API Server 访问；Redis、Kafka 和 ClickHouse 是系统基础设施，不是用户资源
- 外部 API 和产品页面不得暴露 Envoy filter、xDS、ext_authz、ext_proc 等实现细节

## 编码原则

- 从真实调用方和可验收场景出发，不创建占位接口、空目录、无人使用的状态或提前抽象
- 优先使用标准库、已有依赖和上游官方实现；参考开源项目时应能定位到实际源码
- 当前没有生产数据兼容要求，错误设计直接重写，废弃代码和文档直接删除
- 保持主流程直接、调用层级浅；只有真实复用或清晰职责边界才值得拆函数和类型
- 文件内通常按常量、变量、类型、导出函数或方法、私有帮助函数组织；类型专属常量应先定义类型，再紧跟对应的 `const`，接口实现断言也与被断言类型保持相邻
- 同一文件包含多个结构体时，优先将没有构造函数的结构体集中前置；其余结构体在定义后紧跟对应的 `New...` 构造函数，避免不同结构体的定义与构造函数交错排列
- 校验外部输入、配置、网络响应和持久化数据，不对包内已确定的值重复防御
- 接口应来自真实的外部边界或依赖反转，定义在消费者侧；不要只为测试增加接口
- 名称要表达领域职责，避免自定义 `runner`、`runtime`、`snapshot`、`engine`、`manager`、`processor`
- Go 标识符、日志和内部错误使用英文；代码注释使用中文，解释领域约束和设计原因
- 每个 Go 包和导出的顶级标识符、导出类型的方法都使用完整句式注释；不为清晰的接口方法和私有实现重复名称本身
- 敏感内容、内部错误和完整请求不得返回前端或写入日志；同一错误只在责任边界记录一次
- 不为健康检查和正常轮询记录 INFO 请求日志

## 代码边界

- Admin API 保持 Kratos `server / service / biz / data` 分层：装配、协议、业务和外部访问各归其位
- `biz` 中跨资源、仓储或外部边界编排业务规则的入口统一命名为 `Usecase`、`NewUsecase` 和 `usecase.go`；具有明确领域角色的类型则直接使用 `Compiler`、`Delivery`、`Authorizer`、`Limiter`、`Recorder` 等角色名和对应文件名
- `Service`、`NewService` 和 `service.go` 只表示协议实现或适配，通常位于 `service` 层；不得用它们泛指业务编排器、存储或后台任务
- 文件名应与文件内的主要领域角色或职责一致；除 `service/service.go` 层入口或主类型确为协议 `Service` 外，不使用 `service.go`、`handler.go` 等无法表达真实职责的泛化文件名
- 各层公开的 Wire `ProviderSet` 统一声明在包入口文件中，例如 `biz/biz.go`、`data/data.go`、`server/server.go` 和 `service/service.go`
- `api/admin/v1` Proto 是 Console 的产品协议，使用 `id` 表达主键、`name` 表达展示名称
- 对外产品协议、Console 和用户文档统一使用 `Service`；`Upstream` 只用于声明式资源和数据面内部表达，不得泄漏到 Admin API 或产品界面
- 请求格式在 service 校验；同名、引用、版本和运行状态等系统规则在 biz 校验
- 强类型 Policy 表达用户意图，具体由 Envoy、插件或外部服务执行，但执行细节不能进入用户协议
- 插件页面只管理来源和生命周期，插件提供的业务配置仍由对应策略承载
- `web/console` 只使用真实 Admin API；`web/prototype` 只验证产品设计，两者不能互相依赖
- 生成代码必须通过现有 Make/Buf 流程更新，不手工修改

## 工作方式

- 使用 Go 1.27
- `make tools` 将锁定版本的开发工具安装到 `.tools/bin`；构建和代码生成不依赖全局 `GOPATH/bin`
- 当前不维护单元测试，不新增 `*_test.go` 或前端测试；使用编译、联调和端到端请求验证行为
- `make verify` 只在 CI 中运行，本地开发和 Agent 不执行；本地按改动范围运行必要的格式化、生成校验、编译或轻量检查
- 修改镜像或 Compose 时，本地运行 `make docker-up` 并检查组件状态
- 不覆盖用户已有改动，不修改工作区外的旧项目，不提交本地工具、缓存和构建产物
- 一个提交只处理一类事情，不混入无关重构或格式化
