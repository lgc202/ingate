# 2026-10-29 本地功能验证

本轮先将 `028a3d5a` 提交并推送至 `codex/gocanon-template-refactor`，随后在真实开发环境
进行功能验证。测试发现两个问题，修复与本记录一同提交，验证结果如下。
这是一次有范围的本地验证，不代表全部验证计划已完成。

## 环境与隔离

API Server、Controller、Admin API 和 Vite 在本机运行；MySQL 8.4.11、Redis
7.4.9-bookworm、etcd 3.6.4-0、Envoy 1.39.1 和 AI Proxy 通过 Docker Compose 运行。
浏览器验证使用真实 Chrome。请求经过实际 Envoy 和 AI Proxy，上游使用可记录请求的本地
HTTP 服务，未调用外部模型厂商或 MCP 产品。

资源操作使用独立 Namespace。改密和会话撤销使用独立 MySQL 数据库、数据库用户、Redis
容器及 Admin API 实例；未修改原有管理员密码。测试结束已删除临时数据库、用户、容器和
Namespace，开发环境继续运行，原有两个 Gateway 保留。

## 真实接口和数据面检查

最终一次完整执行为 **53 项通过、0 项失败**。每项可能包含多个请求和反例，计数不是
请求数，也不与单元测试数量合并。

| 范围 | 项数 | 主要验证内容 |
| --- | ---: | --- |
| 账户和 Session | 8 | 缺失认证、错误密码、登录、并存会话、改密校验、改密撤销旧会话、登出撤销 |
| 浏览器认证接口 | 7 | 响应隐藏 Token、Cookie 属性、Cookie 认证、跨来源/缺失 Origin 拒绝、重复 Cookie、混用认证、登出清理 |
| 管理端 CRUD | 8 | 服务端生成标识、端点权重投影、允许重复展示名、未知字段/非法端口/重复端点拒绝、版本冲突、删除收敛 |
| 列表查询 | 3 | 名称包含匹配与不匹配、`%`/`_` 按文字查询、分页无重复且总数一致 |
| Namespace | 4 | 查询隔离、禁止向不存在的空间创建、URL 范围优先于请求体、删除内容并完成 finalize |
| HTTP 路由 | 4 | 段前缀和查询串、精确路径、Host 不匹配、多 Listener 独立匹配 |
| 模型请求 | 9 | Chat/Responses 模型名映射、扩展 JSON 保留、内部头覆盖及客户端凭据剥离、非法正文、后台模式拒绝、正文限制、SSE、429、重定向和取消传播 |
| MCP | 5 | 合法请求及上游尾斜杠、缺失版本/方法不一致拒绝、不支持的版本拒绝、GET 返回 405、跨来源拒绝 |
| 控制面重连 | 1 | 停止 Controller 后 HTTP 和模型请求仍使用已有配置；恢复 Controller 后重新连接 |
| 配置热更新 | 4 | 修改模型映射、修改 AIBackend 前缀、Backend 失效后恢复、删除 ModelRoute 不影响 HTTP 路由 |

配置更新通过 API Server 写入，并等待实际请求结果收敛；没有把写入成功、Conditions 或
进程存活当作数据面生效证据。重连之后继续执行热更新检查。

## 浏览器交互

使用真实管理 API 验证 Gateway、Backend、AIBackend、HTTPRoute、ModelRoute、MCPRoute
六类资源各自的创建、编辑和删除后列表刷新，共 18 条主操作路径；每类删除前还验证取消保留资源。
另外验证：

- 编辑后连续读取旧投影时，继续等待并在配置收敛后刷新。
- 删除第二页最后一行后，回到第一页，并刷新总数和翻页状态。
- 投影持续不收敛时，有限等待后提示，不无限轮询；手动刷新后恢复。
- 390px 宽度下页面整体无横向溢出，测试过程中没有未处理 JavaScript 错误。

这些结果验证了交互行为；视觉是否符合产品期望仍需用户 review。

## 发现与修复

### Namespace 清理的内容协商错误

Namespace 删除内核使用 Kubernetes 原生 metadata 客户端。它的批量删除请求优先接受
Protobuf，而 Gateway API Group 使用完整 CodecFactory 宣告支持该编码；批量删除返回的
业务 List 没有实现 Protobuf 编码，导致请求报错和 Namespace 滞留 Terminating。

修复将 Gateway 组支持的媒体类型限制为实际可编码的类型，保留上游版本转换与流式能力；
核心组仍支持原生 Protobuf。没有修改官方 metadata 客户端或引入自定义删除协议。

新增 `TestMetadataDeleteCollection` 使用真实 HTTPS 和隔离 etcd，修复前稳定失败，修复后
通过，同时检查标签筛选、其他 Namespace 保留和清空后的 metadata List。真实环境中，
旧测试空间在 Controller 恢复后完成清理，新空间的删除也验证通过。

### 开发脚本过早判定 API Server 停止失败

带活跃 Watch 的 API Server 实际使用了默认 60 秒 HTTP 关闭期限，而脚本只等待 30 秒。
开发脚本现为 API Server 等待 90 秒，其余组件仍为 30 秒；超时继续保留会话和日志。
脚本回归用确定的探测次数验证超过旧期限的正常退出，以及真正超时不移除会话。

## 自动检查与证据边界

以下检查全部通过：

```sh
gofmt -l cmd internal pkg
git diff --check
golangci-lint run --timeout=5m ./...
go vet ./...
go test ./...
go build ./...
go test -tags=integration ./internal/apiserver
go test -race ./internal/aiproxy/... ./internal/controllermanager/dataplane/...
bash scripts/dev/processes_test.sh
```

前端 `npm run format:check`、16 项单元测试和生产构建通过。手写改动另行检查声明顺序、
错误路径及语义空行，未以 lint 代替人工审查。

2026-10-30 补查增量行为：重新执行 Gateway Controller、xDS、AI 配置和 Envoy 适配器四个
包的测试，全部通过。断言覆盖依赖重算次数、无关转换和索引复用、Delta 响应资源范围、
删除与按已知版本重连；尚未对真实部署逐包统计或进行大规模性能验证。

临时功能脚本、结果 JSON、浏览器截图及运行日志留在已忽略的 `.dev/` 中，未纳入产品代码。
可合入的回归用例位于 `internal/apiserver/metadata_integration_test.go`、
`internal/apiserver/gateway_test.go` 和 `scripts/dev/processes_test.sh`。

本轮没有验证外部厂商兼容性、实际 MCP 产品互通、可信公网证书的 TLS 成功链路、长时间负载、
多副本故障恢复和生产身份隔离。MCP 检查针对当前实现的 `2026-07-28` 契约，不代表所有版本。
当前客户端未发现 GoCanon MCP，因此未核验本轮运行服务的构建版本与规则快照，不能宣称
通过实时 GoCanon 核验或达到生产就绪。
