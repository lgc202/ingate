# 01 快速启动和自动验证

这篇只解决一件事：

**怎么把当前 `ingate-apiserver` 跑起来，并确认它不是假的。**

## 1. 进入项目

```bash
cd /Users/guangcaili/workplace/code/lgc202/ingate
```

## 2. 确认工具入口

```bash
make help
```

你应该重点看这些目标：

- `check-tools`
- `generate`
- `verify-generated`
- `build-apiserver`
- `run-apiserver`
- `verify-apiserver`
- `verify-apiserver-auth`
- `verify-apiserver-kubectl`
- `verify-apiserver-admission`
- `verify-apiserver-table`

为什么先看 `make help`？

因为成熟工程不应该让你记一堆散命令。

顶层 `Makefile` 是给人的入口，`tools/hack` 是给脚本和 CI 的入口。

## 3. 准备 etcd

当前 apiserver 使用 etcd 存储资源。

如果本机 etcd 没启动，可以用类似命令启动：

```bash
etcd \
  --listen-client-urls=http://127.0.0.1:2379 \
  --advertise-client-urls=http://127.0.0.1:2379
```

为什么需要 etcd？

因为这个 apiserver 不是内存假服务。

资源创建以后会进入 Kubernetes apiserver storage 链路，最终存到 etcd。

## 4. 检查工具

```bash
make check-tools
```

这一步会检查本地是否具备必要工具。

如果缺工具，先补工具，不要急着跑服务。

## 5. 校验生成物

```bash
make verify-generated
```

这一步做三件事：

1. 重新生成 API / client / proto / OpenAPI 相关代码
2. 检查关键生成文件是否存在
3. 检查生成结果有没有过期

为什么启动前要关心生成物？

因为 Kubernetes 风格项目里，很多关键代码不是手写的。

如果生成物过期，服务可能还能编译，但行为和类型定义已经不一致。

## 6. 构建 apiserver

```bash
make build-apiserver
```

默认输出到：

```text
_output/<os>_<arch>/ingate-apiserver
```

例如 macOS arm64 是：

```text
_output/darwin_arm64/ingate-apiserver
```

## 7. 一键验证基础能力

```bash
make verify-apiserver
```

这个脚本会临时启动一个 apiserver，然后验证：

- `/healthz`
- `/readyz`
- `/apis`
- `gateway.ingate.io/v1alpha1` discovery
- `policy.ingate.io/v1alpha1` discovery
- `/openapi/v2`
- `/openapi/v3`

看到类似输出说明基础链路正常：

```text
HEALTHZ=ok
READYZ=ok
GATEWAY_DISCOVERY_OK=yes
POLICY_DISCOVERY_OK=yes
OPENAPI_V2={"swagger":"2.0"...
OPENAPI_V3={"paths"...
```

## 8. 启动一个长期运行的本地服务

如果你想自己手工 curl，可以执行：

```bash
make run-apiserver
```

默认连接：

```text
http://127.0.0.1:2379
```

默认 HTTPS 地址通常是：

```text
https://127.0.0.1:18443
```

你可以这样验证：

```bash
curl --noproxy '*' -k https://127.0.0.1:18443/healthz
```

预期：

```text
ok
```

为什么 curl 要加 `-k`？

因为当前默认使用本地自签名证书。

`-k` 表示跳过客户端证书信任校验，只用于本地开发。
