# GoCanon 模板对齐

- 日期：2026-10-15
- 状态：独立审查与本地验证完成，用户已授权审查通过后本地提交

## 范围与依据

本轮在 `8a7b28f8` 上重构已有产品，不重建示例产品。开发分支为
`codex/gocanon-template-refactor`。用户授权自主完成设计、实现和验证；提交仍遵守
仓库审查边界，不自动推送或部署。

通过运行中的 GoCanon MCP 读取规则并生成隔离参考项目，规则快照为
`6f63ad2a7eb341a15ab99453619a907422bcdc7a`，工作区 clean。
参考项目不覆盖现有代码；草案规范按已确认决策和真实业务边界采用。

以下产品契约保持不变：

- `ingate-*` 二进制名称、公开 API 路径、资源字段和公开错误语义。
- 声明式资源写入 API Server，经 Controller 投影到 MySQL 后查询。
- Generic APIServer 与 controller-runtime 的原生扩展方式。
- 管理员、Redis Session、改密使旧会话失效的认证行为。
- AI Proxy 的原生 HTTP Client、受保护的 Unix Socket、编译配置与组件私有能力包。

Controller 直接复用 `adminapi/repository/mysql` 的资源投影 Store 是本产品已确认的
共享边界，不因通用剖面禁止跨组件实现依赖而复制两套表模型或增加转发包。

## 运行边界调整

Admin API 请求完成日志采用 GoCanon 0057 的固定字段、级别及唯一所有者：
Server 最外层 Middleware 使用显式注入的组件 Logger；成功和 4xx 为 Debug，
5xx 为 Error。Handler、Service 和 Repository 不重复记录。
不记录请求、响应、Header、完整 URL 或 4xx 原因；5xx 内部原因仍由错误生产方避免携带秘密。
这不覆盖解码、404、响应写出或 panic，也不提供可靠审计及日志采样。

Admin API 的 HTTP/gRPC 关闭使用 Kratos 原生 StopTimeout，默认十秒；没有新增
goroutine 或第二套关闭控制器。客户端与文件日志的独立清理错误由 Run 聚合返回到
进程入口，而不是记录后吞掉。普通 HTTP 和 AI Proxy 的业务入口均不再回退到
全局 DefaultServeMux。Admin API 先检查本地 kubeconfig，再创建数据库客户端和初始
管理员；一次性迁移模式单独拥有取消信号，服务模式继续由 Kratos 处理信号。

不从资源名推导业务，不添加空接口、通用 CRUD 框架或第二套生命周期。
已有合理实现无需为目录对称而重写。生成文件仅通过其源契约和生成流程修改。

## 实施顺序

1. 固定基线、隔离生成参考骨架，记录验证边界。
2. 对齐普通 Web Handler 与 errno：同包资源文件、独立类型、显式注册；保留公开协议。
3. 审查组合根、客户端与有界退出；对照基础包示例整理日志、版本和构建入口。
4. 复查 Service、Repository、校验、投影和声明式组件的所有权，不引入无用中间层。
5. 复查 AI Proxy 的现有契约、并发所有权与关闭路径。
6. 独立审查声明顺序、函数签名、语义空行、注释和验证门禁，汇总已验证与未验证项。

## 审查约束

结构重构与业务变化分开。每一阶段先运行相关测试，再运行全仓库检查。
检查通过不等于真实 MySQL、Redis、Envoy 联动或生产就绪；没有执行的验证明确列出。
禁止用机械空行或猜测语义的扫描器代替函数级审查。

原仓库仅启用少量空白规则，不能阻止块后黏连。本轮明确修订该旧约定，采用通过 MCP
生成参考项目中的 lint 配置：固定 v2.13.2、0050 的五项空白规则、0051 的接收者与
Context 首参、0052 的存储导入边界和 0053 的直接 Context 字段检查。
不启用所有默认风格规则，不将草案的语义规则冒充自动检查。
自动排版前后比较包含注释和字面量的 Go token；只允许排版变化。
规则自检样本保留在 GoCanon，Ingate 仅运行自身 lint 与业务测试。
带 integration 标签的手写文件通过独立 `go-lint-integration` 目标纳入 `make lint`
和 CI；这里只执行静态分析，不读取测试配置、不运行测试或启动外部依赖。

## 已复现的协议缺口

原 JSON 解码器仅对三种早期资源使用严格 ProtoJSON，其余类型走 Kratos 的
`DiscardUnknown: true`。新增 AIBackend、ModelRoute、MCPRoute 因而会忽略
用户提供的未知约束并继续写入。先用有效创建请求作正例，再加入未知字段，
六个 application/json、application/protojson 反例均在旧实现返回 200 且产生写调用。

解码器改为按 protobuf 消息与 JSON subtype 选择严格 ProtoJSON，不再维护资源类型
白名单。已有合法字段、枚举和 Timestamp 不变；含未知字段的请求现在返回中文
400/INVALID_ARGUMENT，不进入业务层。这是有意修复非法请求行为，不属于纯排版。

## 已复现的资源清理缺口

锁定版本的 RecommendedOptions 在 Config 阶段准备 HTTPS Listener，
GenericAPIServer.Destroy 不自动销毁这个尚未运行的 Listener。
通过真实本地随机端口和原生扩展点注入初始化器失败、存储构造失败，旧实现都保留了
未关闭的 Listener；测试不连接 etcd、MySQL 或宿主 Kubernetes。

Config 应用失败时直接回收；Generic Server 构造失败时回收尚未移交的资源；成功创建
Generic Server 后用上游 RegisterDestroyFunc 统一接管，API 安装失败与运行退出均走
该入口。正常 HTTP 关闭后的重复 Close 只容忍 net.ErrClosed，不吞掉其他错误。
这不增加 Server 包装器、后台清理任务或第二套信号处理。

## 进度

- 基线：工作区 clean，普通 `go test -mod=readonly ./...` 通过。
- MCP：已连接当前 HTTP 服务，并生成三组件隔离参考项目。
- Web Handler / errno：七个独立入口已迁入同包，公开 code/reason/message 保持不变；
  真实 HTTP/gRPC 协议回归及未知错误隐藏原因的测试通过。
- 运行与日志：请求完成日志收敛到 Server，取消使用进程级默认 Logger 的错误转换；
  客户端清理错误聚合，Admin API 原生关闭有界，Manager 先完成配置再打开 MySQL。
- 空白门禁：五项 wsl 修复已完成；345 个 Go 文件修复前后 token 一致。
  新增字段检查只命中一次测试的 io.Writer 适配，已记录 Context 所有权与局部豁免。
- CI：复用 Make 检查入口；生成结果检查补齐原来遗漏的 AI Proxy 配置。
- 其他层：完成 Service、Repository、查询投影、API Strategy、Controller、xDS 和
  AI Proxy 的职责与所有权审查。保留真实业务边界，不为目录对称新增接口或中间层。
- 最终差异：同时检查跟踪与未跟踪文件，清理迁移后的空目录；没有修改模块依赖、
  protobuf 契约、资源 Schema 或迁移 SQL。生成客户端目录中的手写测试仅调整空行。

## 本轮验证结果

以下检查在本轮完成，不沿用历史审查记录作为通过依据：

- 目标工作区 `make -j4 verify`：格式、普通及 integration 标签下的 lint、vet、
  全仓库普通测试与四个二进制构建通过。
- 目标工作区 `go test -race -mod=readonly ./...` 通过。
- 隔离副本 `go test -tags=integration -mod=readonly ./internal/apiserver -count=1`
  通过；副本代码已逐文件哈希核对后同步到目标工作区。测试使用临时目录、随机
  回环端口和内嵌 etcd，不连接部署中的控制面。
- `make generate` 在隔离副本通过，执行前后全部源码及模块文件哈希一致。
- 固定版本 actionlint v1.7.12 检查 CI 工作流通过；未触发远端 CI。
- 四个实际构建的二进制均可执行 `--version` 后退出，不启动数据库或服务。
- 跟踪及未跟踪文件的空白检查通过。

协议回归覆盖既有 HTTP/gRPC 错误语义、严格 JSON 正反例、认证拒绝后的调用阻断、
请求日志级别与敏感内容边界，以及 DefaultServeMux 不暴露到业务入口。
Listener 两个失败路径的测试先在旧实现失败，修复后通过。

没有执行真实 MySQL、Redis 拓扑、Envoy 与模型上游的完整联调，也没有验证多副本
协调或生产部署。默认凭证、回环监听、最终一致查询及单 Manager 的既有部署约束
继续有效。上述检查证明本轮可验证范围内的代码与协议结果，不构成生产就绪承诺。

## 2026-10-16 独立复查

用户完成重构后，重新通过 MCP 确认同一规则快照并读取开发入口和相关规范。
同时检查跟踪、删除和未跟踪文件，逐文件核对手写改动；另行复查声明顺序与语义空行。
没有发现阻断本次提交的功能问题。迁移后的 Backend Handler 测试仍使用 `input`
命名，按已有约定改为 `endpoints`，不改变断言或业务行为。

本次独立执行并通过 `gofmt -l cmd internal pkg api`、跟踪及未跟踪文件空白检查、
普通及 integration 标签的 golangci-lint、`go vet ./...`、`go test ./...`、
`go build ./...`、全仓库非缓存竞态测试，以及目标工作区 API Server 的非缓存
integration 测试。后者使用内嵌 etcd 和本地临时端口，不是部署组件联调。
生成复现、actionlint 和四个二进制版本入口的结果仍属于上节原重构记录，未在本次
独立复查中重复执行。真实外部组件和多副本验证的边界不变。
