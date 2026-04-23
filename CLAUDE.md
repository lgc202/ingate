# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概览

Ingate 是一个用 Go 实现的网关控制面仓库，主要产出 5 个二进制：

- `ingate-apiserver`：基于 Kubernetes generic apiserver 的资源 API 服务
- `ingate-controller-manager`：监听网关与策略资源，并生成派生资源 `ResolvedGateway`
- `ingate-xds-server`：监听 `ResolvedGateway`，将其转换为运行时配置并通过 gRPC/xDS 发布
- `ingate-admin-api`：面向控制台的 Gin REST API
- `ingatectl`：用于查看 xDS/configsync 状态的运维 CLI

核心运行链路是：

1. 资源先写入 `ingate-apiserver`
2. `ingate-controller-manager` 监听 `Gateway`、`Route`、`Backend`、`Certificate`、`AuthPolicy`、`TrafficPolicy`
3. resolvedgateway controller 聚合这些资源，生成并持久化 `ResolvedGateway`
4. `ingate-xds-server` 监听 `ResolvedGateway`，翻译成运行时配置后通过 configsync/discovery/ADS 发布
5. `ingate-admin-api` 再把底层资源包装成控制台可直接使用的 REST API

理解这个仓库时，最重要的主线不是“单个资源怎么存”，而是“资源如何收敛到 `ResolvedGateway`，再如何发布到数据面”。

## 常用命令

### 工具与代码生成

- `make check-tools`：检查本地依赖工具是否齐全，包含 `go`、`etcd`、`protoc`、`protoc-gen-go`、`protoc-gen-go-grpc` 和 Kubernetes codegen helper
- `make generate`：生成 API helper、client/informer/lister、protobuf 产物
- `make verify-generated`：重新生成并校验仓库中的生成文件是否过期

### 构建

- `make build`：构建全部二进制到 `_output/<os>_<arch>`
- `make build-apiserver`
- `make build-admin-api`
- `make build-controller-manager`
- `make build-xds-server`
- `make build-ingatectl`
- `make version`：输出 `ingate-apiserver` 的构建版本信息

### 本地运行

- `make run-apiserver`：基于当前配置的 etcd 启动本地 apiserver
- `make run-admin-api`：连接本地 `ingate-apiserver` 启动 admin API
- `make write-apiserver-kubeconfig`：生成访问本地 apiserver 的 kubeconfig

### 验证与测试

这个仓库更依赖 `make verify-*` 脚本来覆盖多进程联调，而不是只靠一个总入口 `go test ./...`。

- `make verify-apiserver`：验证 apiserver 的 health/discovery/OpenAPI
- `make verify-apiserver-auth`：验证 public/admin/viewer 的认证鉴权行为
- `make verify-apiserver-admission`：验证 reserved metadata admission
- `make verify-apiserver-table`：验证自定义 Table 输出
- `make verify-apiserver-kubectl`：验证通过 kubeconfig 使用 kubectl 访问 apiserver
- `make verify-admin-api`：联动 apiserver + admin-api，验证 gateway/backend/route 产品 API
- `make verify-controller-manager`：验证 `ResolvedGateway` 调谐与状态更新
- `make verify-xds-server`：验证 xDS/config 发布与 discovery RPC
- `make verify-envoy`：验证真实 Envoy 在本地控制面下完成转发

如果只想跑单个包或单个测试，用标准 Go 命令：

- `go test ./...`
- `go test ./internal/controlplane/controller/resolvedgateway/...`
- `go test ./pkg/apis/gateway/validation/...`
- `go test ./cmd/controller-manager/... -run TestName`

## Docker Compose 演示环境

`deploy/compose/` 可以一键拉起完整本地环境：`etcd`、`apiserver`、`controller-manager`、`xds-server`、`admin-api`、console、Envoy、sample backend。

- `make compose-build`：构建 compose 使用的本地镜像
- `make compose-up`：后台启动整套 compose
- `make compose-up COMPOSE_ENV_FILE=deploy/compose/.env`：使用自定义 env 文件启动
- `make compose-ps`
- `make compose-logs`
- `make compose-down`
- `make verify-compose`：构建、启动、验证 Envoy 转发，然后清理

如果需要修改环境配置，先复制：

- `cp deploy/compose/.env.example deploy/compose/.env`

compose 中的 console 镜像直接复用相邻仓库 `../ingate-console/dist`。如果前端仓库有改动，需要先去那个仓库重新构建，再回到这里重启 compose。

## 架构速览

### API 定义与生成代码

`pkg/apis/` 定义了仓库的 API 面：

- `pkg/apis/gateway/v1alpha1/`：网关侧资源，如 `Gateway`、`Route`、`Backend`、`Certificate`、`Secret`、`ResolvedGateway`
- `pkg/apis/policy/v1alpha1/`：策略资源，如 `AuthPolicy`、`TrafficPolicy`
- `pkg/apis/*/validation/`：校验逻辑
- `pkg/scheme/`：汇总注册 scheme

`pkg/generated/` 下都是生成产物。只要修改了 API type、defaulting、protobuf 或客户端相关输入，通常都要执行 `make generate`，然后再跑 `make verify-generated`。

### apiserver

`ingate-apiserver` 不是普通的 Gin 服务，而是基于 Kubernetes generic apiserver 体系搭出来的。

关键目录：

- `cmd/apiserver/`：Cobra 启动入口与参数装配
- `internal/controlplane/apiserver/`：generic apiserver 配置与 API 安装
- `internal/controlplane/apiserver/registry/`：各资源的 REST storage 与 strategy
- `internal/controlplane/apiserver/auth/`：认证、鉴权与策略辅助逻辑
- `internal/controlplane/apiserver/admission/`：admission plugin，如 reserved metadata

凡是涉及资源持久化、REST 语义、表格输出、认证鉴权、准入控制的改动，通常不会只落在一个文件里，而会分散在 `options`、`registry`、`auth`、`admission` 这几层。

### controller-manager

`ingate-controller-manager` 是典型 informer + queue 驱动结构。

关键目录：

- `cmd/controller-manager/app/run.go`：创建共享 informer factory 和共享 gateway work queue
- `internal/controlplane/controller/{gateway,route,backend,certificate,authpolicy,trafficpolicy}`：各资源 controller，职责主要是把受影响的 gateway 入队
- `internal/controlplane/controller/index/`：维护资源关系索引，用于从依赖资源反查 gateway
- `internal/controlplane/controller/resolvedgateway/`：加载完整资源 bundle，构建 `ResolvedGateway`，写回存储并更新状态
- `internal/controlplane/controller/status/`：负责 Accepted/Resolved 等状态写入

这里最重要的设计点是：各资源 controller 更多像“触发器”和“索引维护者”，真正的多资源收敛逻辑集中在 `resolvedgateway` controller。

### xDS 发布链路

`ingate-xds-server` 消费的是 `ResolvedGateway`，不是直接消费原始 Gateway/Route/Backend 资源。

关键目录：

- `cmd/xds-server/app/server.go`：启动 `ResolvedGateway` informer、runtime cache、publisher 和 health server
- `internal/controlplane/xds/watch/`：监听 `ResolvedGateway` 变化并触发重新发布
- `internal/controlplane/xds/translate/`：把 `ResolvedGateway` 翻译成运行时配置结构
- `internal/controlplane/xds/cache/`：缓存已发布配置
- `internal/controlplane/xds/publish/`：通过 configsync/discovery/ADS 暴露 gRPC 服务
- `internal/controlplane/xds/ads/`：ADS 资源类型和打包辅助逻辑

调试 xDS 行为时，优先顺序通常应该是：

1. 先看 `ResolvedGateway` 是否正确
2. 再看 `translate/` 是否把它翻对了
3. 最后看 cache/publish 是否把结果按预期发布出去

### admin-api

`ingate-admin-api` 是单独的 Gin 服务，面向控制台和产品接口，不等同于底层 apiserver。

关键目录：

- `internal/adminapi/server/routes.go`：`/admin/v1` 路由面
- `internal/adminapi/handler/`：HTTP handler 与 DTO
- `internal/adminapi/biz/`：按领域划分的业务逻辑
- `internal/adminapi/convert/`：API 对象与 HTTP DTO 的转换
- `internal/adminapi/store/`：admin-api 访问底层资源的存储抽象

如果一个需求同时影响“控制台展示/交互语义”和“底层控制面资源语义”，通常需要同时修改 `internal/adminapi/*` 和底层 API/controller 相关代码。

### ingatectl

`ingatectl` 主要用于本地排查 `ingate-xds-server` 的发布状态，而不是普通资源管理。

常见子命令：

- `ingatectl xds list|config|summary|check`：查看已发布 configsync 状态
- `ingatectl xds resolve`：查询后端 endpoint 发现结果
- `ingatectl xds ads`：抓取某个 gateway key 的原始 ADS 资源

排查发布链路时，优先用 `ingatectl` 看实际输出，通常比直接翻 gRPC 代码路径更快。

## 在这个仓库里工作的注意点

- 优先使用现成的 `make verify-*` 目标，它们已经编码了这个项目需要的多进程启动顺序和联调方式
- 把 `ResolvedGateway` 当成 controller 与 xDS 之间的核心交接对象来理解和排查问题
- 非必要不要手改 `pkg/generated/`，这类改动通常应该通过 `make generate` 产出
- 当前工作目录不是一个 git repo，因此某些依赖 `git diff` 的校验脚本在这里可能只执行生成，不执行基于 git 的差异校验
