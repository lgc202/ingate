# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概览

Ingate 是一个用 Go 实现的网关控制面仓库，主要产出 5 个二进制：

- `ingate-apiserver`：基于 Kubernetes generic apiserver 的资源 API 服务
- `ingate-controller-manager`：监听网关与策略资源，并生成派生资源 `ResolvedGateway`
- `ingate-xds-server`：监听 `ResolvedGateway`，将其转换为运行时配置并通过 gRPC/xDS 发布
- `ingate-admin-api`：面向控制台的 Gin REST API
- `ingatectl`：用于查看 xDS/configsync 状态的运维 CLI

理解这个仓库时，优先抓这条主线，而不是单个资源的 CRUD：

1. 资源先写入 `ingate-apiserver`
2. `ingate-controller-manager` 监听 `Gateway`、`Route`、`Backend`、`Certificate`、`AuthPolicy`、`TrafficPolicy`
3. 各资源 controller 负责把受影响的 gateway 入队，并维护依赖索引
4. `resolvedgateway` controller 拉全量依赖，生成并持久化 `ResolvedGateway`
5. `ingate-xds-server` 监听 `ResolvedGateway`，翻译成运行时配置并通过 configsync/discovery/ADS 发布
6. `ingate-admin-api` 再把底层资源包装成控制台可直接使用的 REST API

`ResolvedGateway` 是 controller 与 xDS 之间的核心交接对象，排查大多数链路问题时都应该先看它。

## 常用命令

### 查看可用目标

- `make help`

### 工具与代码生成

- `make check-tools`：检查本地依赖工具
- `make generate`：生成全部代码产物
- `make generate-apis`：生成 API helper，如 `DeepCopy`
- `make generate-clients`：生成 clientset、informer、lister
- `make generate-proto`：生成 protobuf 相关产物
- `make verify-generated`：校验生成产物是否最新

### 构建

- `make build`：构建全部二进制到 `_output/<os>_<arch>`
- `make build BINS="ingate-apiserver ingatectl"`：只构建指定二进制
- `make build-apiserver`
- `make build-admin-api`
- `make build-controller-manager`
- `make build-xds-server`
- `make build-ingatectl`
- `make version`

### 本地运行

- `make run-apiserver`
- `make run-admin-api`
- `make write-apiserver-kubeconfig`

### 验证与测试

这个仓库更依赖 `make verify-*` 脚本覆盖多进程联调，而不是只靠 `go test ./...`。

- `make verify-apiserver`
- `make verify-apiserver-auth`
- `make verify-apiserver-admission`
- `make verify-apiserver-table`
- `make verify-apiserver-kubectl`
- `make verify-admin-api`
- `make verify-controller-manager`
- `make verify-xds-server`
- `make verify-envoy`
- `make verify-compose`

包级或单个测试用标准 Go 命令：

- `go test ./...`
- `go test ./internal/controlplane/controller/gatewaycompiler/...`
- `go test ./pkg/apis/gateway/validation/...`
- `go test ./cmd/controller-manager/... -run TestName`
- `go test ./internal/adminapi/convert -run TestBackend`

### Lint / 格式化

- 当前 `Makefile` 没有暴露独立的 `make lint` 或 `make fmt` 目标，不要假设它们存在
- 修改生成相关输入后，补跑 `make verify-generated`

## Docker Compose 演示环境

`deploy/compose/` 可以一键拉起完整本地环境：`etcd`、`apiserver`、`controller-manager`、`xds-server`、`admin-api`、console、Envoy、sample backend。

- `make compose-build`
- `make compose-up`
- `make compose-up COMPOSE_ENV_FILE=deploy/compose/.env`
- `make compose-ps`
- `make compose-logs`
- `make compose-down`
- `make verify-compose`

如果需要修改环境配置，先复制：

- `cp deploy/compose/.env.example deploy/compose/.env`

compose 中的 console 镜像直接复用相邻仓库 `../ingate-console/dist`。如果前端仓库有改动，需要先去那个仓库重新构建，再回到这里重启 compose。

## 架构速览

### 改 API 类型、校验或生成产物时先看哪里

- `pkg/apis/gateway/v1alpha1/`：网关侧资源定义，如 `Gateway`、`Route`、`Backend`、`Certificate`、`Secret`
- `pkg/apis/policy/v1alpha1/`：策略资源定义，如 `AuthPolicy`、`TrafficPolicy`
- `pkg/apis/*/validation/`：校验逻辑
- `pkg/apis/scheme/`：scheme 注册
- `pkg/generated/`：生成产物；不要手改，改输入后走 `make generate`

### 改资源存储、REST 语义、认证鉴权、准入时先看哪里

`ingate-apiserver` 基于 Kubernetes generic apiserver，而不是普通 Gin 服务。

- `cmd/apiserver/app/server.go`：CLI 入口，组装 options 并启动 apiserver
- `internal/controlplane/apiserver/`：apiserver 配置与 API 安装
- `internal/controlplane/apiserver/registry/`：各资源 REST storage 与 strategy
- `internal/controlplane/apiserver/auth/`：认证、鉴权与策略辅助逻辑
- `internal/controlplane/apiserver/admission/`：admission plugin

这类改动通常不会只落在一个文件里，而会跨 `registry`、`auth`、`admission` 等层一起改。

### 改控制器收敛逻辑、状态更新、依赖关系时先看哪里

真正的多资源收敛逻辑集中在 `gatewaycompiler` controller，其它资源 controller 更像触发器和索引维护者。

- `cmd/controller-manager/app/run.go`：创建 shared informer factory、依赖索引和共享 gateway work queue，并注册所有 controller
- `internal/controlplane/controller/{gateway,route,backend,certificate,authpolicy,trafficpolicy}`：把受影响 gateway 入队
- `internal/controlplane/controller/index/`：维护依赖索引
- `internal/controlplane/controller/gatewaycompiler/`：拉取网关依赖并构建 `LogicalGateway`
- `internal/controlplane/controller/status/`：写入 Accepted/Resolved 等状态

如果表现为“某个依赖资源变了但网关没重算”，先看资源 controller 与 `index/`；如果表现为“重算了但结果不对”，先看 `gatewaycompiler/`。

### 改 xDS 翻译、发布或排查数据面问题时先看哪里

`ingate-xds-server` 现在消费的是由 `Gateway` 事件触发重建出来的 `LogicalGateway`，不是中间 CRD 资源。

- `cmd/xds-server/app/server.go`：启动 watcher、runtime cache、publisher 和 health server
- `internal/controlplane/xds/watch/`：监听 `Gateway` 并重建 `LogicalGateway`
- `internal/controlplane/xds/translate/`：翻译为运行时配置
- `internal/controlplane/xds/cache/`：缓存已发布配置
- `internal/controlplane/xds/publish/`：通过 configsync/discovery/ADS 发布
- `internal/controlplane/xds/ads/`：ADS 资源打包辅助逻辑

调试顺序通常是：先看 `ResolvedGateway`，再看 `translate/`，最后看 cache/publish。

### 改控制台接口或产品语义时先看哪里

`ingate-admin-api` 是面向控制台的语义层，不等同于底层 apiserver。

- `internal/adminapi/server/routes.go`：`/admin/v1` 路由面
- `internal/adminapi/handler/`：HTTP handler 与 DTO
- `internal/adminapi/biz/`：业务逻辑
- `internal/adminapi/convert/`：底层对象与 HTTP DTO 转换
- `internal/adminapi/store/`：访问底层资源的存储抽象

如果一个需求同时影响控制台语义和底层资源语义，通常需要同时修改 `internal/adminapi/*` 和底层 API/controller 代码。

### 排查发布结果而不是读代码时先用什么

`ingatectl` 主要用于排查 `ingate-xds-server` 的发布状态。

- `ingatectl xds list|config|summary|check`
- `ingatectl xds resolve`
- `ingatectl xds ads`

排查发布链路时，优先用 `ingatectl` 看实际输出，通常比直接翻 gRPC 代码路径更快。

## 在这个仓库里工作的注意点

- 优先使用现成的 `make verify-*` 目标，它们已经编码了这个项目需要的多进程启动顺序和联调方式
- 把 `ResolvedGateway` 当成 controller 与 xDS 之间的核心交接对象来理解和排查问题
- 非必要不要手改 `pkg/generated/`

## 接口设计约束

- 先写具体实现，只有在出现真实多实现、真实外部协作边界或需要打破包循环依赖时，才提取接口
- 接口应优先定义在消费者侧，只暴露调用方真正需要的最小方法集，不要在实现侧预先铺大接口
- 不要为了“将来可能扩展”提前发明接口；接口应从真实依赖点和真实依赖需求中被发现
- 不要为了测试方便给单实现类型补接口；先考虑直接测试具体类型、使用轻量 fake object，或删除低价值 orchestration 测试
- 如果一个接口只是单实现加纯转调，主要作用是注入 mock/stub，而没有带来生产代码里的真实解耦价值，应优先回退到具体类型
- 真实边界上的小接口可以保留，例如发布、存储装配、框架注册这类稳定协作边界；但接口要小、专注、可组合，避免胖接口
