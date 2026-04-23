# 05 哪些是手写的，哪些是生成的

这篇必须搞清楚。

否则你后面很容易手改生成代码，然后下一次 `make generate` 又被覆盖。

## 1. 手写代码

### 入口和启动

```text
cmd/apiserver/...
internal/controlplane/apiserver/config.go
internal/controlplane/apiserver/server.go
internal/controlplane/apiserver/install.go
```

这些代码决定服务怎么启动、怎么组装 generic apiserver、怎么安装 API group。

### API 类型源

```text
pkg/apis/gateway/v1alpha1/*.go
pkg/apis/policy/v1alpha1/*.go
```

排除 `zz_generated.*.go`。

这些文件定义资源语义：

- `Gateway`
- `Route`
- `Backend`
- `AuthPolicy`
- `TrafficPolicy`
- `spec`
- `status`
- 字段类型
- 默认值入口
- 注册信息

### 资源实现

```text
internal/controlplane/apiserver/registry/...
```

这里是：

- storage
- strategy
- REST storage provider
- TableConvertor

### 工具链

```text
Makefile
tools/hack/*.sh
```

这里是工程入口。

它决定怎么构建、怎么生成、怎么验证。

### proto 源

```text
proto/...
```

这里是组件通信契约源文件。

## 2. 生成代码

不要手改这些：

```text
pkg/apis/**/zz_generated.deepcopy.go
pkg/apis/**/zz_generated.defaults.go
pkg/apis/**/zz_generated.model_name.go
pkg/generated/openapi/zz_generated.openapi.go
pkg/generated/clientset/...
pkg/generated/informers/...
pkg/generated/listers/...
pkg/generated/proto/...
```

## 3. 为什么要生成

因为这些代码都有共同特点：

- 结构固定
- 重复量大
- 人写容易漏字段
- 和类型定义强绑定
- 需要跟 Kubernetes 工具链兼容

比如 `DeepCopy`。

如果手写，很容易把 slice、map、指针字段复制错。

比如 `clientset`。

如果手写，每个资源都要写 create、update、delete、list、watch。

这类代码不体现业务判断，但必须机械正确。

所以应该生成。

## 4. 怎么生成

总入口：

```bash
make generate
```

分入口：

```bash
make generate-apis
make generate-clients
make generate-proto
```

校验入口：

```bash
make verify-generated
```

## 5. 开发时怎么判断该改哪里

如果你要改资源字段，改：

```text
pkg/apis/<group>/v1alpha1/types_*.go
```

然后跑：

```bash
make generate
make verify-generated
```

如果你要改默认值，改：

```text
pkg/apis/<group>/v1alpha1/defaults.go
```

然后跑：

```bash
make generate-apis
```

如果你要改资源校验，改：

```text
pkg/apis/<group>/validation/...
```

如果你要改资源存储和 REST 行为，改：

```text
internal/controlplane/apiserver/registry/...
```

如果你要改客户端调用方式，不要手改 `pkg/generated/clientset`。

应该改 API 类型或生成脚本，然后重新生成。
