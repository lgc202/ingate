# Ingate v1 代码架构与生成链路

## 1. 目标

这份文档统一回答三个问题：

- 目录结构如何组织
- 各层代码放在哪
- 代码生成与构建链路如何收敛

## 2. 最终目录结构

```text
cmd/
  apiserver/
  admin-api/
  controller-manager/
  xds-server/
  gateway/
  ingatectl/

proto/
  ingate/
    configsync/
      v1/
    discovery/
      v1/

pkg/
  apis/
    gateway/
      v1alpha1/
    policy/
      v1alpha1/
  generated/
    clientset/
    informers/
    listers/
    proto/
  apiserver/
  controller/
  xds/
  discovery/
  config/

internal/
  controlplane/
    apiserver/
    controller/
    xds/
    discovery/
  adminapi/
    contract/
    handler/
    biz/
    view/
    mapping/
  gateway/
    model/
    policy/
    ir/
    translation/
  pkg/
    options/
    tls/
    middleware/
    conditions/
    idempotent/
```

## 3. 设计原则

### 3.1 组件实现聚拢

- `internal/controlplane/` 聚拢控制面组件实现
- `internal/adminapi/` 聚拢产品接口实现

### 3.2 共享网关核心单独抽出

- `internal/gateway/` 放共享模型、策略、IR、翻译

### 3.3 公共底座统一收口

- `pkg/` 放可复用底座
- `internal/pkg/` 放项目内共享辅助件

### 3.4 顶层概念尽量少

顶层只保留：

- `cmd/`
- `proto/`
- `pkg/`
- `internal/`

不再单独抬高：

- `api/`
- `client/`
- `openapi/`

## 4. 为什么不用顶层 `api/` 和 `client/`

### 4.1 不用顶层 `api/`

因为这些类型本质上是：

- API machinery 资源定义
- 不是对外 HTTP API

所以更适合放在：

- `pkg/apis/`

### 4.2 不用顶层 `client/`

因为这些代码本质上是：

- 生成物
- 不是手写业务代码

所以更适合统一收进：

- `pkg/generated/`

## 5. 依赖方向

推荐依赖方向：

```text
cmd -> internal/* + pkg/*
internal/controlplane -> internal/gateway + internal/pkg + pkg/*
internal/adminapi -> internal/gateway + internal/pkg + pkg/generated + pkg/apis
internal/gateway -> internal/pkg + pkg/*
pkg/generated -> pkg/apis + proto
```

约束：

1. `internal/gateway/` 不反向依赖 `internal/controlplane/`
2. `internal/pkg/` 不依赖具体组件实现目录
3. `pkg/` 不依赖 `internal/`
4. `pkg/generated/` 只承载生成结果，不承载手写逻辑

## 6. 代码生成对象分层

### 6.1 资源层生成物

来源：

- `pkg/apis/gateway/v1alpha1`
- `pkg/apis/policy/v1alpha1`

输出：

- `pkg/generated/clientset/...`
- `pkg/generated/listers/...`
- `pkg/generated/informers/...`

v1 先不把 `applyconfigurations` 作为主设计的一部分。

### 6.2 Proto 契约生成物

来源：

- `proto/ingate/discovery/v1/*.proto`
- `proto/ingate/configsync/v1/*.proto`

输出：

- `pkg/generated/proto/.../*.pb.go`
- `pkg/generated/proto/.../*_grpc.pb.go`

### 6.3 非生成代码

这些代码不应被 codegen 覆盖：

- `internal/gateway/*`
- `internal/controlplane/*`
- `internal/adminapi/*`
- `pkg/xds/*`
- `pkg/apiserver/*`

## 7. 脚本与 Make 入口

建议将生成逻辑收敛到：

```text
tools/hack/
  generate-apis.sh
  generate-clients.sh
  generate-proto.sh
  generate-all.sh
  verify-generated.sh
```

根 `Makefile` 至少提供：

```make
make generate
make generate-apis
make generate-clients
make generate-proto
make verify-generated
```

## 8. 推荐主线

开发者主线：

```text
修改 pkg/apis/ 或 proto/
  -> make generate
  -> make verify-generated
  -> make lint
  -> 单元测试 / 集成测试
```

CI 主线：

```text
make generate
make verify-generated
make lint
make test
```

## 9. 一句话结论

`Ingate` 的代码组织应当收敛为：

**用 `internal/controlplane/` 聚拢控制面实现，用 `internal/adminapi/` 聚拢产品接口实现，用 `internal/gateway/` 承载共享网关核心，用 `pkg/apis/` 与 `pkg/generated/` 组织资源和生成物，用 `proto/` 管理内部契约源。**
