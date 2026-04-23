# apiserver 源码阅读

这组文档只在你完成前两步以后再看：

1. 已经读完 `../01-quickstart.md` 到 `../05-generated-vs-manual.md`
2. 已经读完 `../k8s-details/README.md` 里的基础概念

源码阅读不要从某个随机函数开始。

应该按链路读。

## 推荐阅读顺序

1. [00-code-map.md](./00-code-map.md)
2. [01-startup-chain.md](./01-startup-chain.md)
3. [02-api-types-and-installation.md](./02-api-types-and-installation.md)
4. [03-registry-strategy-storage.md](./03-registry-strategy-storage.md)
5. [04-codegen-and-generated-code.md](./04-codegen-and-generated-code.md)
6. [05-build-scripts-and-makefile.md](./05-build-scripts-and-makefile.md)
7. [06-kube-onex-ingate-mapping.md](./06-kube-onex-ingate-mapping.md)
8. [07-openapi-generation-and-wiring.md](./07-openapi-generation-and-wiring.md)
9. [08-authentication-and-authorization.md](./08-authentication-and-authorization.md)
10. [09-admission-and-plugins.md](./09-admission-and-plugins.md)
11. [10-table-convertor-and-discovery-metadata.md](./10-table-convertor-and-discovery-metadata.md)
12. [11-static-policy-authz-and-kubeconfig.md](./11-static-policy-authz-and-kubeconfig.md)
13. [12-serving-certificates-and-secure-serving.md](./12-serving-certificates-and-secure-serving.md)

## 每篇文档解决什么

### 00 代码地图

先建立目录感。

你会知道：

- `cmd/apiserver` 放什么
- `internal/controlplane/apiserver` 放什么
- `pkg/apis` 放什么
- `pkg/generated` 放什么
- `tools/hack` 放什么

### 01 启动链

读懂：

```text
main -> command -> options -> config -> server
```

这是所有能力接入的主线。

### 02 API 类型和安装链

读懂资源类型怎么变成 API 路由。

重点是：

- API group
- version
- Scheme
- install

### 03 registry / strategy / storage

读懂资源 REST 行为怎么接到底层 store。

这篇是理解 Kubernetes apiserver 的关键。

### 04 codegen

读懂生成链：

- DeepCopy
- defaults
- model_name
- OpenAPI
- clientset
- informer
- lister
- proto

### 05 Makefile 和脚本

读懂工程入口为什么这样设计。

重点是：

- 顶层 `Makefile` 保持入口清晰
- `tools/hack` 承载复杂逻辑
- `_output` 承载构建产物

### 06 kube / OneX / Ingate 映射

建立参照系。

知道 Ingate 不是凭空设计，而是参考 Kubernetes apiserver 和 OneX 的工程模式。

### 07 OpenAPI

读懂 OpenAPI 怎么生成、怎么接入、为什么和 managedFields 有关系。

### 08 认证授权

读懂 token authenticator、static policy authorizer 怎么接进 generic apiserver。

### 09 admission

读懂 admission 插件注册、排序、启用、执行链路。

### 10 Table 和 discovery metadata

读懂 kubectl 表格展示、shortNames、categories 是怎么来的。

### 11 静态策略授权和 kubeconfig

读懂 viewer/admin 权限边界，以及 kubeconfig 怎么生成。

### 12 证书和 secure serving

读懂 HTTPS 证书为什么会出现，默认自签名证书怎么来。

## 阅读要求

读源码时不要只看“代码写了什么”。

要同时问：

- 这个文件属于哪一层
- 它的输入是什么
- 它的输出是什么
- 它依赖哪些 Kubernetes machinery
- 哪些代码是手写的
- 哪些代码是生成的
- 为什么这段逻辑不放在别的目录
