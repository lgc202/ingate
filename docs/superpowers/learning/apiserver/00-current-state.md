# 00 当前 apiserver 实现到了什么程度

先说结论：

当前 `ingate-apiserver` 已经不是占位服务。

它已经是一个真实的、最小完整的 Kubernetes 风格 `generic apiserver`。

## 已经具备的能力

### 服务形态

- 基于 `k8s.io/apiserver` 的 `generic apiserver`
- HTTPS secure serving
- 默认自签名证书
- 命令行参数启动
- 版本信息输出
- 本地构建和验证脚本

### 存储能力

- 使用 etcd 作为后端存储
- 支持 create
- 支持 get
- 支持 list
- 支持 watch
- 支持 delete
- 支持 `/status` 子资源

### 当前 API group

```text
gateway.ingate.io/v1alpha1
policy.ingate.io/v1alpha1
```

### 当前资源

```text
Gateway
Route
Backend
AuthPolicy
TrafficPolicy
```

### Kubernetes 风格能力

- `apiVersion/kind`
- `metadata/spec/status`
- Scheme 注册
- APIGroup 安装
- discovery
- OpenAPI v2
- OpenAPI v3
- Table 输出
- shortNames
- categories
- kubectl 访问
- kubeconfig 生成

### 准入和安全

- token 认证
- admin 内置身份
- viewer 内置身份
- 静态策略授权
- 匿名公共路径
- admission 插件骨架
- 当前 admission 插件会拒绝系统保留 metadata

### 代码生成

已经接入：

- `deepcopy-gen`
- `defaulter-gen`
- `client-gen`
- `lister-gen`
- `informer-gen`
- `openapi-gen`
- `protoc`
- `protoc-gen-go`
- `protoc-gen-go-grpc`

对应生成物包括：

- `zz_generated.deepcopy.go`
- `zz_generated.defaults.go`
- `zz_generated.model_name.go`
- `pkg/generated/openapi/zz_generated.openapi.go`
- `pkg/generated/clientset/...`
- `pkg/generated/informers/...`
- `pkg/generated/listers/...`
- `pkg/generated/proto/...`

## 为什么说它是“最小完整”

因为它已经打通了 apiserver 的主链路：

```text
资源类型定义
-> Scheme 注册
-> APIGroup 安装
-> registry / strategy / storage
-> generic apiserver
-> HTTPS API
-> etcd
-> client / kubectl / watch
```

很多 demo 只做到 HTTP CRUD。

当前这个不是。

它已经开始进入 Kubernetes 风格资源系统：

- 用户提交期望状态
- apiserver 做准入、校验、默认值、存储
- controller 后续可以 watch 资源
- controller 后续可以回写 status
- kubectl 可以作为调试入口

## 当前还没做什么

明确暂缓：

- audit
- controller-manager 完整调谐逻辑
- admin-api 产品接口
- xDS server
- 生产级证书管理
- 生产级 RBAC
- 多副本 leader election
- 灾备和恢复策略

这些不是忘了。

只是当前阶段先把 `apiserver` 这条主线学明白。
