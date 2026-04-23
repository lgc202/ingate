# 00. 怎么运行和验证

这一篇先不读源码，只解决一个问题：怎么证明 admin-api 能跑、能访问 apiserver、能把资源写进去。

## admin-api 依赖什么

`ingate-admin-api` 自己不存数据。

它依赖：

- `ingate-apiserver`
- `etcd`
- generated clientset

链路是：

```text
curl /admin/v1/gateways
-> ingate-admin-api
-> generated clientset
-> ingate-apiserver
-> etcd
```

所以只启动 admin-api 是不够的。admin-api 后面必须有 apiserver，apiserver 后面必须有 etcd。

## 构建 admin-api

```bash
make build-admin-api
```

这个命令会构建：

```text
_output/<os>_<arch>/ingate-admin-api
```

在你的机器上通常是：

```text
_output/darwin_arm64/ingate-admin-api
```

## 手动运行

先启动 apiserver：

```bash
make run-apiserver
```

再开一个终端启动 admin-api：

```bash
make run-admin-api
```

默认地址：

```text
admin-api:        http://127.0.0.1:18080
ingate-apiserver: https://127.0.0.1:18443
```

默认 token：

```text
admin-api token:  ingate-dev-admin-api-token
apiserver token:  ingate-dev-admin-token
```

这里有两个 token：

```text
用户 -> admin-api 使用 admin-api token
admin-api -> apiserver 使用 apiserver token
```

这两个不要混淆。

## 健康检查

健康检查不需要 token：

```bash
curl http://127.0.0.1:18080/healthz
```

预期返回：

```text
ok
```

为什么健康检查不需要 token？

因为健康检查通常给负载均衡器、Kubernetes probe、运维系统用。如果健康检查也要业务 token，部署会变复杂。

## 访问业务接口

`/admin/v1/*` 需要 token：

```bash
curl \
  -H 'Authorization: Bearer ingate-dev-admin-api-token' \
  http://127.0.0.1:18080/admin/v1/gateways
```

如果不带 token，会返回 `401`。

## 自动验证

最重要的命令是：

```bash
make verify-admin-api
```

这个命令会自动做完整流程：

1. 构建 `ingate-apiserver`
2. 构建 `ingate-admin-api`
3. 临时启动 apiserver
4. 临时启动 admin-api
5. 验证没带 admin token 时返回 `401`
6. 创建 Gateway
7. 创建 Backend
8. 创建 Route
9. 创建 AuthPolicy
10. 创建 TrafficPolicy
11. 列表查询五类资源
12. 更新五类资源
13. 查询 Gateway topology
14. 查询 Route effective-status
15. 直接查 apiserver，确认资源真的写进去了
16. 删除五类资源
17. 删除后再查，确认返回 `404`

预期输出：

```text
ADMIN_API_HEALTHZ=ok
ADMIN_API_UNAUTH_CODE=401
ADMIN_API_GATEWAY_CREATE_CODE=201
ADMIN_API_BACKEND_CREATE_CODE=201
ADMIN_API_ROUTE_CREATE_CODE=201
ADMIN_API_AUTH_POLICY_CREATE_CODE=201
ADMIN_API_TRAFFIC_POLICY_CREATE_CODE=201
ADMIN_API_UPDATE_VERIFY=yes
ADMIN_API_TOPOLOGY_VERIFY=yes
ADMIN_API_DELETE_VERIFY=yes
ADMIN_API_APISERVER_RESOURCE_VERIFY=yes
```

## 为什么验证脚本要直接查 apiserver

如果只通过 admin-api 创建资源，再通过 admin-api 查询资源，有可能只是 admin-api 自己“看起来成功”。

但我们的设计是：

```text
admin-api 不存数据，最终数据必须在 apiserver/etcd 中
```

所以验证脚本会用 apiserver token 直接访问：

```text
/apis/gateway.ingate.io/v1alpha1/gateways/<name>
/apis/gateway.ingate.io/v1alpha1/backends/<name>
/apis/gateway.ingate.io/v1alpha1/routes/<name>
/apis/policy.ingate.io/v1alpha1/authpolicies/<name>
/apis/policy.ingate.io/v1alpha1/trafficpolicies/<name>
```

这样才能证明 admin-api 真的调用了 apiserver，而不是只在自己内部返回了一个 JSON。
