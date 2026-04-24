# ingate 去掉 ResolvedGateway 当前进度

## 当前做到的程度

这轮已经把 `ResolvedGateway` 从主链路和 API 面里实质性移除，控制面与 xDS 的主路径已经切到：

`Gateway/Route/Backend/... -> ResourceBundle -> LogicalGateway -> RuntimeConfig`

不再经过中间 CRD 资源。

## 已完成项

### 1. 主链路改造完成

#### controller-manager

- `internal/controlplane/controller/gatewaycompiler/` 现在负责：
  - 拉取 `Gateway` 相关依赖
  - 组装 `ResourceBundle`
  - 构建 `LogicalGateway`
- 不再持久化 `ResolvedGateway`
- controller 成功/失败状态统一写回 `Gateway.status.conditions`

#### xds-server

- `internal/controlplane/xds/watch/gateway.go` 现在直接 watch `Gateway`
- watcher 内直接复用 loader + `BuildLogicalGateway(...)`
- `internal/controlplane/xds/translate/logicalgateway.go` 直接把 `LogicalGateway` 翻译为 `RuntimeConfig`
- `internal/controlplane/xds/publish/server.go` 把 `Programmed` condition 写回 `Gateway.status.conditions`
- 不再 watch 或消费 `ResolvedGateway`

### 2. API 面删除完成

已删除或移除：

- `pkg/apis/gateway/v1alpha1/types_resolvedgateway.go`
- `pkg/apis/gateway/v1alpha1/register.go` 中的 `ResolvedGateway` 注册
- `pkg/apis/gateway/v1alpha1/resources.go` 中的 `resolvedgateways` 资源常量
- `internal/controlplane/apiserver/registry/gateway/resolvedgateway/`
- `internal/controlplane/apiserver/registry/gateway/rest/storage.go` 中的 storage 挂载
- `internal/controlplane/apiserver/registry/gateway/table/table.go` 中的 table 展示逻辑
- validation/defaulting 中与 `ResolvedGateway` 对应的入口
- generated client/informer/lister 中的 `ResolvedGateway` 生成产物

### 3. 包语义重命名完成

为避免代码里继续保留旧语义，本轮还做了命名收口：

- `internal/controlplane/controller/resolvedgateway/` -> `internal/controlplane/controller/gatewaycompiler/`
- `internal/controlplane/xds/watch/resolvedgateway.go` -> `internal/controlplane/xds/watch/gateway.go`
- `internal/controlplane/xds/translate/resolvedgateway.go` -> `internal/controlplane/xds/translate/logicalgateway.go`

因此现在代码层的语义已经变成：

- controller 侧：`gatewaycompiler`
- xds watch 侧：watch `Gateway`
- xds translate 侧：translate `LogicalGateway`

## 当前保留下来的东西

这轮是一次性切主链路，不是彻底重做全部概念层。

因此目前仍然保留：

- `RuntimeConfig` 作为 xDS 发布前的运行时结构
- `ResourceBundle` 作为 loader 到编译器之间的聚合输入
- `LogicalGateway` 作为当前稳定语义中心

也就是说，这一版已经完成了“去掉 ResolvedGateway”，但还没有继续往更远的 `RuntimeSnapshot` / 多 runtime / 插件机制做下一步抽象。

## 已验证项

本轮已经验证通过：

- `go test ./pkg/apis/gateway/validation ./internal/controlplane/controller/status ./internal/controlplane/controller/gatewaycompiler`
- `go test ./internal/controlplane/xds/... ./cmd/ingatectl/...`
- `go build ./cmd/controller-manager ./cmd/xds-server ./cmd/apiserver ./cmd/ingatectl`

## 还没继续做的部分

后续如果继续推进，优先级建议如下：

1. 清理历史文档里的 `ResolvedGateway` 描述
   - 包括 `docs/superpowers/` 和部分历史设计文档
   - 这些不影响代码，但会影响后续阅读体验

2. 继续统一仓库术语
   - 某些注释、文案、说明仍可能保留旧表述
   - 可以继续统一成 `LogicalGateway` / `gatewaycompiler`

3. 再决定是否推进下一阶段抽象
   - 是否把 `RuntimeConfig` 进一步收口为更通用的 runtime snapshot 语义
   - 是否拆分 compile pipeline
   - 是否为后续插件或 AI 网关能力预留更清晰边界

## 当前结论

截至这份文档为止：

- `ResolvedGateway` 已经不再参与运行时代码主链路
- `ResolvedGateway` API 资源已经从 apiserver 和生成代码中删除
- controller/xds 已经切到 `LogicalGateway`
- 残留主要在历史文档与术语层，不在运行时代码主链路
