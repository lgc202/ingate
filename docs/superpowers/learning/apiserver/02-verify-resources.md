# 02 验证资源 API、CRUD、status、watch

这篇验证资源系统主链路。

你要确认的不是“HTTP 能通”。

你要确认的是：

**Gateway 这类资源真的进入了 Kubernetes 风格 apiserver 存储链路。**

## 1. 准备变量

如果你用 `make run-apiserver` 启动服务，可以在另一个终端准备：

```bash
BASE_URL=https://127.0.0.1:18443
ADMIN_TOKEN=ingate-dev-admin-token
AUTH_HEADER="Authorization: Bearer ${ADMIN_TOKEN}"
```

当前内置 admin token 来自：

- `internal/controlplane/apiserver/auth/constants.go`

## 2. 创建 Gateway

```bash
curl --noproxy '*' -k -X POST "${BASE_URL}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H "${AUTH_HEADER}" \
  -H 'Content-Type: application/json' \
  -d '{
    "apiVersion": "gateway.ingate.io/v1alpha1",
    "kind": "Gateway",
    "metadata": {
      "name": "demo-gateway"
    },
    "spec": {
      "listeners": [
        {
          "name": "web",
          "protocol": "HTTP",
          "port": 80,
          "hostname": "api.example.com"
        }
      ]
    }
  }'
```

这里最重要的是这几个字段：

- `apiVersion`：告诉 apiserver 这个对象属于哪个 group/version
- `kind`：告诉 apiserver 这个对象是什么类型
- `metadata.name`：资源名
- `spec`：用户期望状态

## 3. 查询 Gateway

```bash
curl --noproxy '*' -k "${BASE_URL}/apis/gateway.ingate.io/v1alpha1/gateways/demo-gateway" \
  -H "${AUTH_HEADER}"
```

你应该能看到：

- `apiVersion`
- `kind`
- `metadata`
- `spec`
- `status`

为什么会有 `status`？

因为声明式系统通常把对象拆成两部分：

- `spec`：用户想要什么
- `status`：系统观察到什么

当前 status 后续会由 controller 回写。

## 4. 列表查询

```bash
curl --noproxy '*' -k "${BASE_URL}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H "${AUTH_HEADER}"
```

如果能看到 `items`，说明 list 路径是通的。

## 5. watch

```bash
curl --noproxy '*' -k "${BASE_URL}/apis/gateway.ingate.io/v1alpha1/gateways?watch=true" \
  -H "${AUTH_HEADER}"
```

然后另开一个终端创建或删除资源。

watch 终端会收到事件。

为什么 watch 重要？

因为 controller-manager 后续不是轮询数据库。

它会 watch apiserver 的资源变化。

这就是 Kubernetes 控制器模型的基础。

## 6. status 子资源

`/status` 是一条单独入口。

它的意义是：

- 普通用户主要改 `spec`
- 控制器主要回写 `status`
- 两者不要互相覆盖

这也是 Kubernetes 资源模型里非常重要的一条边界。

## 7. 自动验证入口

除了手工 curl，也可以直接跑：

```bash
make verify-apiserver
```

这个脚本验证基础 discovery 和 OpenAPI。

资源 CRUD 更详细的手工验证可以先按本文走。
