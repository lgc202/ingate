# Ingate v1 管理接口与 API 分层

## 1. 目标

这份文档统一回答三个问题：

- `ingate-apiserver` 的资源接口是什么
- `admin-api` 的产品接口是什么
- 两者为什么必须分层

## 2. 总结论

`Ingate` 必须有两层 northbound：

1. **资源接口层**：`ingate-apiserver`
2. **产品接口层**：`ingate-admin-api`

两者的关系是：

```text
User / Console / CLI
  -> admin-api
  -> ingate-apiserver
  -> controller-manager
  -> xds-server
  -> Envoy
```

## 3. `ingate-apiserver`

`ingate-apiserver` 的接口风格应参考 `kube-apiserver`，但它不是给最终用户直接使用的产品接口。

它提供的是：

- 资源化 URL
- `metadata/spec/status`
- list/watch
- status subresource

### 3.1 资源分组

v1 约定：

- group：`gateway.ingate.io`
- version：`v1alpha1`

### 3.2 资源集合

- `gateways`
- `routes`
- `backends`
- `authpolicies`
- `trafficpolicies`

### 3.3 URL 风格

```text
/apis/gateway.ingate.io/v1alpha1/gateways
/apis/gateway.ingate.io/v1alpha1/routes
/apis/gateway.ingate.io/v1alpha1/backends
/apis/gateway.ingate.io/v1alpha1/authpolicies
/apis/gateway.ingate.io/v1alpha1/trafficpolicies
```

### 3.4 v1 范围

- 产品版本：`Ingate v1`
- 资源 API：`gateway.ingate.io/v1alpha1`
- v1 先做 cluster-scoped
- 支持 `status` 子资源
- 必须支持真正的 `list/watch` 语义

## 4. `admin-api`

`admin-api` 是正式组件，不是 console 附件。

它面向：

- 前端控制台
- CLI 的产品化能力
- 外部平台集成

它负责：

- 产品化写接口
- 聚合查询
- 多资源写入工作流
- 状态到产品视图的映射

它不负责：

- 成为资源真相源
- 执行 reconcile
- 执行 xDS 发布
- 绕过 apiserver 直接改配置

### 4.1 `admin-api` 内部建议

```text
internal/adminapi/
  contract/
  handler/
  biz/
  view/
  mapping/
```

- `contract/`：HTTP/JSON 请求与响应 DTO
- `handler/`：路由与接口入口
- `biz/`：产品工作流与业务编排
- `view/`：聚合查询与页面视图
- `mapping/`：产品模型和资源模型映射

## 5. 为什么必须分层

如果不分层，会出现两种坏结果：

### 5.1 直接把 apiserver 暴露给用户

问题：

- 用户体验差
- UI 难做
- 用户需要理解底层资源对象

### 5.2 只有用户友好的命令式接口

问题：

- 后面 controller/apiserver 语义会被绕开
- 很难接 K8s 风格自动化
- 很难保持声明式控制面稳定

所以正确做法是：

**产品接口在上，资源接口在下。**

## 6. `admin-api` 的工作流范围

v1 优先支持五类工作流：

1. 创建网关入口
2. 创建 HTTP 路由
3. 配置认证
4. 配置流量治理
5. 查看生效状态

写操作的语义应是：

- 返回“已受理”，不是“已生效”
- 不假装做跨资源强事务
- 要有请求级幂等

## 7. `admin-api` 接口风格

示例：

```text
POST /admin/v1/gateways
GET  /admin/v1/gateways/{gatewayId}
GET  /admin/v1/gateways/{gatewayId}/topology

POST /admin/v1/routes
GET  /admin/v1/routes/{routeId}
GET  /admin/v1/routes/{routeId}/effective-status

PUT  /admin/v1/routes/{routeId}/auth
PUT  /admin/v1/routes/{routeId}/traffic-policy
```

关键点不是 URL 本身，而是：

- 返回的是产品语义
- 底层资源对象由 `admin-api` 负责映射

## 8. 一句话结论

`Ingate` 的 northbound 必须分成两层：

**`ingate-apiserver` 提供声明式资源接口，`admin-api` 提供面向用户的产品接口与工作流。**
