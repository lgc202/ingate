# 03 验证认证授权、kubectl、Table、admission

这篇把几个“更像正式 apiserver”的能力放在一起讲。

## 1. 认证授权怎么验证

直接跑：

```bash
make verify-apiserver-auth
```

预期关键输出：

```text
PUBLIC_HEALTHZ=ok
ANON_CREATE_CODE=403
AUTH_CREATE_CODE=201
AUTH_GET_CODE=200
VIEWER_GET_CODE=200
VIEWER_CREATE_CODE=403
```

这些结果分别证明：

- 匿名用户可以访问公共健康检查
- 匿名用户不能创建资源
- admin 可以创建资源
- admin 可以读取资源
- viewer 可以读取资源
- viewer 不能创建资源

为什么要区分 admin 和 viewer？

因为正式控制面不会只有“全放行”和“全拒绝”。

哪怕现在只是最小实现，也要先建立授权边界。

## 2. 当前内置 token

当前开发默认值：

```text
admin token:  ingate-dev-admin-token
viewer token: ingate-dev-viewer-token
```

它们定义在：

- `internal/controlplane/apiserver/auth/constants.go`

这只是开发默认值。

后续生产环境不应该依赖固定明文 token。

## 3. kubectl 怎么验证

直接跑：

```bash
make verify-apiserver-kubectl
```

预期关键输出：

```text
KUBECONFIG_WRITE_OK=yes
KUBECTL_GET_TABLE_OK=yes
KUBECTL_VIEWER_CREATE_FORBIDDEN=yes
```

这个脚本证明三件事：

- 项目能生成专用 kubeconfig
- `kubectl get gateways` 能看到资源表格
- viewer context 不能创建资源

## 4. kubeconfig 是什么

kubeconfig 本质是 kubectl 的连接配置。

它告诉 kubectl：

- server 地址是什么
- 用哪个 token
- 当前 context 是谁
- 是否跳过证书校验

当前生成入口是：

```bash
./tools/hack/write-apiserver-kubeconfig.sh
```

默认输出到：

```text
_output/<os>_<arch>/ingate-apiserver.kubeconfig
```

## 5. Table 输出怎么验证

直接跑：

```bash
make verify-apiserver-table
```

预期关键输出：

```text
GATEWAY_TABLE_OK=yes
ROUTE_TABLE_OK=yes
BACKEND_TABLE_OK=yes
AUTHPOLICY_TABLE_OK=yes
TRAFFICPOLICY_TABLE_OK=yes
```

Table 输出不是新 API。

它是同一个 list API 在不同 `Accept` 头下返回 `meta.k8s.io/v1 Table`。

kubectl 很依赖这个能力。

如果没有自定义 Table，`kubectl get` 的展示会很弱，不能体现网关业务关键字段。

## 6. admission 怎么验证

直接跑：

```bash
make verify-apiserver-admission
```

预期关键输出：

```text
NORMAL_CREATE_CODE=201
RESERVED_METADATA_CODE=403
```

这说明：

- 普通资源可以创建
- 带系统保留 metadata 的资源会被拒绝

当前保留前缀是：

```text
internal.ingate.io/
```

为什么要拦这个？

因为这类 metadata 应该由系统内部控制。

如果允许用户随便写，后续 controller、发布链路、状态标记都会被用户伪造。
