# 03 Scheme、GVK、Resource、REST path 是什么关系

这篇讲 Kubernetes API 最核心的一组概念。

如果这些概念不清楚，后面看 apiserver 会一直绕。

## 1. 先看一个资源 YAML

```yaml
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
metadata:
  name: demo-gateway
spec:
  listeners:
  - name: web
    protocol: HTTP
    port: 80
```

这里有两个非常关键的字段：

```text
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
```

它们合起来就是 GVK。

## 2. GVK 是什么

GVK 是：

```text
Group + Version + Kind
```

对上面的对象来说：

```text
Group:   gateway.ingate.io
Version: v1alpha1
Kind:    Gateway
```

GVK 表达的是：

**这个 JSON/YAML 对象应该被当成哪种 Go 类型处理。**

## 3. Resource 是什么

Resource 是 REST 路径里的资源名。

例如：

```text
/apis/gateway.ingate.io/v1alpha1/gateways
```

这里的 resource 是：

```text
gateways
```

注意：

- Kind 通常是单数大驼峰：`Gateway`
- Resource 通常是复数小写：`gateways`

它们不是同一个东西，但会对应起来。

## 4. GVR 是什么

GVR 是：

```text
Group + Version + Resource
```

例如：

```text
Group:    gateway.ingate.io
Version:  v1alpha1
Resource: gateways
```

你可以先这样区分：

- GVK 偏“对象类型”
- GVR 偏“REST 资源路径”

## 5. Scheme 是什么

Scheme 是 Kubernetes 里的类型登记表。

它负责记录：

```text
这个 GVK 对应哪个 Go struct
这个 Go struct 对应哪个 GVK
这个类型怎么创建空对象
这个类型怎么做默认值
这个类型怎么序列化/反序列化
```

当前项目的统一 Scheme 在：

```text
pkg/apis/scheme/scheme.go
```

各 API 包自己的注册在：

```text
pkg/apis/gateway/v1alpha1/register.go
pkg/apis/policy/v1alpha1/register.go
```

## 6. 请求进来以后怎么用这些信息

当 apiserver 收到请求：

```text
POST /apis/gateway.ingate.io/v1alpha1/gateways
```

再看到 body：

```json
{
  "apiVersion": "gateway.ingate.io/v1alpha1",
  "kind": "Gateway"
}
```

它会根据：

- URL 里的 group/version/resource
- body 里的 apiVersion/kind
- Scheme 里的类型注册

把 JSON 解码成 Go 对象。

也就是：

```text
JSON -> Gateway struct
```

## 7. 为什么 `TypeMeta` 必须存在

资源类型通常会嵌入：

```go
metav1.TypeMeta `json:",inline"`
```

它承载：

- `apiVersion`
- `kind`

如果没有这些信息，通用 apiserver 很难知道一个对象到底是什么类型。

## 8. 为什么 `ObjectMeta` 必须存在

资源类型通常还会嵌入：

```go
metav1.ObjectMeta `json:"metadata,omitempty"`
```

它承载通用元数据：

- name
- uid
- resourceVersion
- generation
- labels
- annotations
- creationTimestamp
- managedFields

这些不是 Ingate 自己发明的。

这是 Kubernetes API 对象的通用元信息。

## 9. REST path 和代码怎么对应

以 Gateway 为例：

```text
/apis/gateway.ingate.io/v1alpha1/gateways
```

大致对应到：

```text
pkg/apis/gateway/v1alpha1/types_gateway.go
internal/controlplane/apiserver/registry/gateway/gateway/storage
internal/controlplane/apiserver/registry/gateway/rest
```

你可以先记：

- `pkg/apis` 定义对象长什么样
- `register.go` 把对象注册进 Scheme
- registry 把 resource 接到 apiserver
- storage 把 resource 接到 etcd

## 10. 一个完整对应关系

```text
YAML apiVersion/kind
-> GVK
-> Scheme 找到 Go type
-> REST path 找到 Resource storage
-> Strategy 做校验和语义处理
-> Storage 写入 etcd
```

这就是 Kubernetes apiserver 和普通 HTTP CRUD 最大的不同之一。
