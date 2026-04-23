# 05 OpenAPI 和 managedFields 为什么有关

很多人会以为 OpenAPI 只是接口文档。

在 Kubernetes apiserver 里，它不只是文档。

它还会影响字段管理。

## 1. OpenAPI 在当前项目里做什么

当前 apiserver 会暴露：

```text
/openapi/v2
/openapi/v3
```

这些接口告诉客户端：

- 有哪些资源类型
- 每个资源有哪些字段
- 字段是什么结构

这对 kubectl、客户端、调试工具都有价值。

## 2. managedFields 是什么

你看资源对象时，metadata 里可能会出现：

```yaml
managedFields:
- manager: kubectl
  operation: Update
  apiVersion: gateway.ingate.io/v1alpha1
```

它记录的是：

**哪些字段由哪个管理者写过。**

这是 Kubernetes server-side apply 和字段冲突管理的基础。

## 3. 为什么 managedFields 需要 schema

字段管理要知道：

- 哪些字段存在
- 哪些字段是 map
- 哪些字段是 list
- 哪些字段是 struct
- 字段路径怎么解析

这些信息来自结构化 schema。

OpenAPI schema 是重要来源。

## 4. 为什么需要 `zz_generated.model_name.go`

只有 schema 还不够。

apiserver 还要知道：

```text
这个 schema 对应哪个 GVK
```

比如：

```text
gateway.ingate.io/v1alpha1, Kind=Gateway
```

`zz_generated.model_name.go` 提供 Go 类型到 OpenAPI model name 的稳定映射。

然后 `DefinitionNamer` 才能把它和 Scheme 里的 GVK 对起来。

## 5. 如果对不起来会怎样

之前出现过这类日志：

```text
failed to update managedFields
no corresponding type for gateway.ingate.io/v1alpha1, Kind=Gateway
```

这说明：

- 资源能创建
- OpenAPI 也可能有内容
- 但字段管理找不到 GVK 对应的结构化类型

这不是业务字段校验问题。

这是 OpenAPI、model name、Scheme 之间的接线问题。

## 6. 当前是怎么修好的

当前做法是：

1. 在 API 包 `doc.go` 里加 `+k8s:openapi-model-package=...`
2. 在 `generate-apis.sh` 里先生成 `zz_generated.model_name.go`
3. 再生成完整 `pkg/generated/openapi/zz_generated.openapi.go`
4. 在 apiserver config 里用 Scheme 创建 `DefinitionNamer`
5. 把生成的 OpenAPI definitions 接进 generic apiserver

## 7. 为什么 OpenAPI 分两段生成

第一段：

```text
只处理 Ingate 自己的 API 包
生成 pkg/apis/**/zz_generated.model_name.go
```

第二段：

```text
处理 Ingate API 包 + Kubernetes 依赖包
生成 pkg/generated/openapi/zz_generated.openapi.go
```

为什么要分？

因为 model name 应该落在我们自己的 API 包里。

完整 schema 又需要 Kubernetes 的 `metav1`、`runtime`、`version` 等依赖类型。

两个职责不同，所以分开更清楚。

## 8. 怎么验证这条链没坏

至少跑：

```bash
make verify-generated
make verify-apiserver
make verify-apiserver-table
```

再扫日志：

```bash
rg -n "failed to update managedFields|SHOULD NOT HAPPEN|no corresponding type" _output/*/ingate-apiserver-*.log
```

没有命中，才说明这类问题没有复发。
