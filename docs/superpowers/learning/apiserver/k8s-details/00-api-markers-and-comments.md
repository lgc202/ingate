# 00 API 注解和代码标记怎么看

Kubernetes 代码里很多注释不是给人看的。

它们是给代码生成器看的。

这类注释通常叫 marker。

## 1. 当前项目里 marker 在哪里

包级 marker 在：

```text
pkg/apis/gateway/v1alpha1/doc.go
pkg/apis/policy/v1alpha1/doc.go
```

类型级 marker 在：

```text
pkg/apis/gateway/v1alpha1/types_gateway.go
pkg/apis/gateway/v1alpha1/types_route.go
pkg/apis/gateway/v1alpha1/types_backend.go
pkg/apis/policy/v1alpha1/types_authpolicy.go
pkg/apis/policy/v1alpha1/types_trafficpolicy.go
```

## 2. `+k8s:deepcopy-gen=package`

例子：

```go
// +k8s:deepcopy-gen=package
```

含义：

**这个包里的类型要生成 DeepCopy 方法。**

DeepCopy 是 Kubernetes API 对象非常基础的能力。

为什么需要？

因为 apiserver、cache、client、watch 都会在不同地方传递对象。

如果只是浅拷贝，slice、map、指针字段可能被多个地方共享，后面会出现很隐蔽的状态污染。

所以 Kubernetes 要求对象能安全深拷贝。

## 3. `+k8s:deepcopy-gen:interfaces=runtime.Object`

例子：

```go
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Gateway struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec GatewaySpec `json:"spec,omitempty"`
    Status GatewayStatus `json:"status,omitempty"`
}
```

含义：

**给这个类型生成 `DeepCopyObject()`，让它实现 `runtime.Object`。**

为什么资源对象要实现 `runtime.Object`？

因为 Kubernetes apiserver 处理的是一套通用对象模型。

它不能只认识 `Gateway`。

它要能统一处理：

- Pod
- Deployment
- Gateway
- Route
- AuthPolicy
- 任意注册进 Scheme 的资源对象

`runtime.Object` 就是这套通用对象模型的基础接口。

## 4. `+genclient`

例子：

```go
// +genclient
```

含义：

**给这个资源生成 clientset 方法。**

比如 `Gateway` 会生成类似这样的调用入口：

```go
client.GatewayV1alpha1().Gateways().Create(...)
client.GatewayV1alpha1().Gateways().Get(...)
client.GatewayV1alpha1().Gateways().List(...)
client.GatewayV1alpha1().Gateways().Watch(...)
```

为什么要生成？

因为每个资源的 CRUD/list/watch 客户端代码都很像。

手写没有价值，而且容易写错路径、版本、参数。

## 5. `+genclient:nonNamespaced`

例子：

```go
// +genclient:nonNamespaced
```

含义：

**这个资源不是 namespace 级资源，而是集群级资源。**

所以路径是：

```text
/apis/gateway.ingate.io/v1alpha1/gateways
```

而不是：

```text
/apis/gateway.ingate.io/v1alpha1/namespaces/<namespace>/gateways
```

为什么当前资源先做成 non-namespaced？

因为现在 Ingate 还没有完整租户和 namespace 模型。

先把资源主链路打通，比过早引入 namespace 更稳。

## 6. `+groupName=...`

例子：

```go
// +groupName=gateway.ingate.io
```

含义：

**告诉生成器这个包属于哪个 API group。**

这个值会影响生成 clientset 的包路径和 group/version 映射。

它和资源对象里的 `apiVersion` 对应。

例如：

```yaml
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
```

## 7. `+k8s:openapi-gen=true`

例子：

```go
// +k8s:openapi-gen=true
```

含义：

**这个包里的类型要参与 OpenAPI schema 生成。**

OpenAPI 用来告诉外部工具：

- 资源有哪些字段
- 字段是什么类型
- 哪些结构可以被客户端识别

kubectl、客户端校验、文档生成、字段管理都会间接受它影响。

## 8. `+k8s:openapi-model-package=...`

例子：

```go
// +k8s:openapi-model-package=com.github.lgc202.ingate.pkg.apis.gateway.v1alpha1
```

含义：

**给 OpenAPI model name 一个稳定包名。**

为什么需要？

因为 generic apiserver 需要把 OpenAPI schema 和 Scheme 里的 GVK 对起来。

如果 model name 对不上，字段管理会找不到对应类型，日志里可能出现：

```text
failed to update managedFields
no corresponding type for gateway.ingate.io/v1alpha1, Kind=Gateway
```

## 9. `+k8s:defaulter-gen=TypeMeta`

例子：

```go
// +k8s:defaulter-gen=TypeMeta
```

含义：

**让 defaulter 生成链处理带 TypeMeta 的类型。**

当前默认值逻辑写在：

```text
pkg/apis/gateway/v1alpha1/defaults.go
pkg/apis/policy/v1alpha1/defaults.go
```

生成器会把这些默认值函数接成统一入口。

## 10. 怎么判断 marker 给谁看

简单记：

- `deepcopy-gen` 看 `+k8s:deepcopy-gen...`
- `client-gen` 看 `+genclient...` 和 `+groupName`
- `defaulter-gen` 看 `+k8s:defaulter-gen...`
- `openapi-gen` 看 `+k8s:openapi-gen...` 和 `+k8s:openapi-model-package...`

这些 marker 不直接改变 Go 编译结果。

它们改变的是“生成器会生成什么代码”。
