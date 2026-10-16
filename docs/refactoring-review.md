# Ingate 重构审查记录

- 日期：2026-10-07
- 状态：用户于 2026-10-08 确认提交；保留下述测试缺口与未验证项

## 规则与范围

GoCanon MCP 的规则来源为 `/Users/lgc202/workspace/source/lgc202/gocanon`，启动快照修订为 `f0f27ed03bcbb8bf278813ead9c0aa03a25922a5`，规则工作区为 clean。MCP 二进制报告 `a27f6cf-dirty`，不将二进制标识当作规则版本。开发入口、规范索引与任务相关规则已完整读取；草案规范结合仓库已有契约和用户确认范围使用，不把参考项目的业务搬入 Ingate。

本次检查覆盖三个 Go 组件及其调用链。Admin API 继续采用 Handler → Service → Repository；写入经过 API Server，查询来自异步 MySQL 投影。API Server 保留原生 Generic API Server，Controller Manager 保留原生 controller-runtime。没有增加通用 CRUD、跨框架运行时、配置加载框架或业务接口，也没有新增应用依赖模块。

## 累计改动

| 范围 | 具体问题与调整 | 行为边界 |
| --- | --- | --- |
| 投影验证 | 三种 Reconciler 补齐状态版本、启动对账、读取失败、部分失败及重试测试 | 不改 Watch、队列、投影字段或事务 |
| MySQL | 连接初始化从业务 Store 移至原生客户端包，DSN 解析统一供客户端与迁移使用 | 保留版本查询、自动 Ping、UTC、parseTime、禁止多语句及原始错误原因；连接池由入口关闭 |
| Redis | 业务 Store 不再构造和关闭客户端，入口使用原生 UniversalClient | 按已确认要求移除旧 URL 配置，支持单节点、Sentinel、Cluster；原有会话键、JSON、过期和删除语义不变 |
| 入口与账户 | API Server 的命令、配置、服务组装分离；账户密码比较统一错误分类 | 保留原生框架生命周期、认证结果和改密后的会话版本校验 |
| 版本与开发命令 | 三个二进制接入构建版本，共用 Make 开发入口 | `--version` 先于依赖启动；Kubernetes `/version` 与 EffectiveVersion 不变 |
| 日志 | Admin API 使用可配置原生 slog，保留组合输出、文件轮转、备份清理、压缩；Manager 清理日志归入 klog | 已确认默认改为 JSON/stdout；不增加请求日志、审计、远程输出或热更新；复用原有 lumberjack 锁定版本 |
| 声明组织 | MySQL 资源类型集中声明；REST Strategy 的两个类型和构造入口集中，方法按接收者排列；ADS 类型及辅助函数按调用路径排列 | 只移动声明，不改变表名、SQL、主资源与 Status 隔离、Generation 或 resourceVersion 语义 |
| 路由裁决 | 将本轮引用索引和引用检查从排序、冲突占用、结果收集拆开 | 仍按 Gateway → Listener → Upstream 检查，再裁决匹配冲突；被拒绝 Route 不占用规则或引入 Upstream |
| xDS 转换 | 将端点克隆、排序、地址分配与 DNS 识别从 Cluster 策略和校验中拆开 | 保留纯 IP 的 EDS、含 DNS 的 STRICT_DNS、地址/端口/权重排序及先 Cluster 后端点的校验顺序 |

MySQL、Redis、版本和日志的已确认选择分别见 [0006](decisions/0006-mysql-client-lifecycle.md)、[0007](decisions/0007-redis-client-lifecycle.md)、[0008](decisions/0008-build-version.md)、[0009](decisions/0009-logging-initialization.md)。DSN 统一与最后一轮内部职责整理随本次重构确认提交；确认提交不代表全仓库合规或全部真实组件验证已经完成。

已有的三种资源 CRUD、投影协调、独立组件配置加载和框架 REST 适配保留各自职责。代码相似但输入契约或业务含义不同，不仅为去重引入泛型、接口或公共加载器。单 Worker 全量裁决、发布后再写 Status、异步查询模型以及明文 ADS 等既有取舍保持不变，详见 [0002](decisions/0002-shared-data-plane-controller.md) 和 [0005](decisions/0005-admin-resource-management.md)。

## 审查与验证

同时核对 `git status --short`、`git diff --name-only` 和全部未跟踪文件清单。新增与修改的手写文件逐个审查：先检查 GO-FUNC-012 的类型、入口、接收者方法和辅助函数顺序，再检查职责、命名、签名、错误传播、清理、注释与测试断言。赋值和错误检查相邻，空行按语义阶段保留；没有用猜测命名或注释语义的扫描器替代人工审查。

生成的配置 protobuf 不按手写代码逐行审查；核对其源契约、生成器与可重复性。公开资源 API 类型未修改，已有 Kubernetes 生成代码保持不变。

已通过格式检查、跟踪及未跟踪文件的空白检查、golangci-lint、go vet、全仓库普通测试与构建，以及 `make -j4 verify`。Admin API、API Server、Controller Manager 和共享客户端包的竞态测试通过。三个实际构建的二进制可输出版本并退出，不启动数据库或服务。

最后一轮新增测试先在原实现上通过，再在拆分后的实现上通过。路由测试同时固定多重缺失的拒绝顺序、匹配冲突、只收集已接纳 Upstream，以及全部输入不被修改；端点测试同时检查 IP/DNS 混合输入、IPv6、排序、权重、输入所有权、错误结果和校验上下文。临时 Go overlay 分别禁用 Listener 校验、匹配冲突、DNS 识别、输入克隆和端点校验，五个故障版本均使对应断言失败；仓库源码没有替换为故障版本。DSN 三项规范化和两个调用入口的故障验证见 0006。

`go test -tags=integration ./internal/apiserver -count=1` 已通过。测试启动临时目录和随机端口的隔离嵌入式 etcd 与真实 HTTPS API Server，验证三种资源 CRUD、匹配与不匹配的筛选、Watch、过期 resourceVersion 冲突、Generation 和 Status 隔离。它不连接用户部署的 etcd，也不证明完整网关链路可用。

## 追加审查发现与提交范围

真实 `RequireSession` 缺少有效回归测试。现有中间件测试只检查 Bearer 字符串解析，HTTP 鉴权选择测试使用替代中间件。临时 Go overlay 将会话认证调用替换为直接返回身份后，`go test ./internal/adminapi/... -count=1` 仍然通过，因而现有测试不能发现这一类认证绕过回归。这是已验证的测试缺口，不是当前实现存在绕过漏洞的证明。

用户获知这一缺口后于 2026-10-08 确认提交。本次只提交已完成的重构，不改变鉴权实现或补充测试；后续需直接验证真实中间件的有效会话、失效或伪造 Token、依赖失败、拒绝后的调用阻断和 Context 身份传递。

## 尚未验证的边界

此前 Redis 阶段已验证本地单节点、Sentinel、FailoverClusterClient 与三主节点 Cluster 的会话操作、认证反例和网络读截止时间，本轮未重建这些拓扑。TLS 握手、命名 ACL、真实故障转移及复制丢失仍未验证。

真实 MySQL 的初始化、事务、迁移 SQL 执行和服务联合退出没有验证；sqlmock 只覆盖期望 SQL 与调用结果。没有执行 API Server → Manager → MySQL → Admin API → Envoy 的完整联动，没有用真实 Envoy 验证转发、ACK 或故障恢复。

文件日志测试使用真实临时文件，但不能证明外部采集、磁盘故障或可靠审计。lumberjack.Close 只关闭当前文件，不等待后台压缩与清理。上述静态、单元及隔离组件检查不构成生产就绪结论。

## 2026-10-15：GoCanon 模板对齐

本节是新一轮审查，前文保留为历史记录。本轮基线为 `8a7b28f8`，分支为
`codex/gocanon-template-refactor`，实现与独立复查完成，用户已授权审查通过后本地提交。
范围、规则版本、改动原因及完整验证清单见
[0016：GoCanon 模板对齐](decisions/0016-gocanon-template-alignment.md)。

通过当前运行的 GoCanon MCP 读取规则，并在隔离目录生成参考骨架。普通 Web 的
七种 Handler 迁入同包，公开错误集中到 errno；Service 与 Repository 保留已有
业务边界。原生 Generic APIServer、controller-runtime、AI Proxy 流式转发及
Controller 共享查询 Store 的已确认形式保持不变。没有新增通用 CRUD 或依赖注入框架。

复现并修复两类非排版问题：新增资源的未知 JSON 字段被忽略并继续写入；API Server
初始化失败后已创建的 HTTPS Listener 未关闭。另调整组合根的依赖检查顺序、客户端
清理错误传播、请求完成日志和业务入口的 DefaultServeMux 隔离。空白检查采用固定
GoCanon lint 配置，integration 标签下的手写代码也纳入 CI 静态检查。

当前基线已有真实 RequireSession 回归，覆盖有效、缺失、伪造、过期、撤销会话与
Redis 失败，并检查拒绝后的调用阻断和 Context 身份。本轮保留并运行这些断言；
前文 2026-10-08 的测试缺口不能视作当前仍然缺少该测试。

目标工作区 `make -j4 verify`、全仓库竞态测试、四个二进制版本入口和 CI 工作流
语法检查通过。隔离副本的 API Server 内嵌 etcd 集成测试通过，代码生成前后文件
哈希一致。真实 MySQL/Redis/Envoy/模型上游联调、生产部署及多副本验证未执行；
不能据此声称所有运行条件下均达到模板级或生产就绪。
