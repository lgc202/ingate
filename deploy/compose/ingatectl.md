# `ingatectl` Usage Guide

`ingatectl` 现在主要是控制面和 xDS 排障工具，不是资源编排工具。

它最适合回答这类问题：

- 当前发布了哪些 gateway
- 某个 gateway 最终被整理成了什么配置
- 某个 backend 被解析成了哪些 endpoint
- `xds-server` 是否处于可发布状态
- Envoy 理论上应该拿到哪些标准 xDS 资源

## 构建

在仓库根目录执行：

```bash
make build-ingatectl
```

默认产物位置：

```text
_output/darwin_arm64/ingatectl
```

## 帮助

```bash
./_output/darwin_arm64/ingatectl --help
./_output/darwin_arm64/ingatectl xds --help
```

## 默认连接地址

默认会连接：

```text
127.0.0.1:19090
```

这不是 Envoy，也不是 backend。

它代表的是：

- 本机暴露出来的 `xds-server` gRPC 地址

如果你的 `xds-server` 不在这个地址，用 `--server` 覆盖。

## 最常用命令

### 1. 列出当前已发布的 gateway

```bash
./_output/darwin_arm64/ingatectl xds list
./_output/darwin_arm64/ingatectl xds list --output text
```

适合先看“控制面现在到底发布了什么”。

### 2. 看某个 gateway 的摘要

```bash
./_output/darwin_arm64/ingatectl xds summary --gateway compose-gateway
./_output/darwin_arm64/ingatectl xds summary --gateway compose-gateway --output text
```

适合快速看：

- listener 数量
- route 数量
- backend 数量
- endpoint 数量
- host 和 prefix 摘要

### 3. 读取某个 gateway 的完整 effective config

```bash
./_output/darwin_arm64/ingatectl xds config --gateway compose-gateway
./_output/darwin_arm64/ingatectl xds config --gateway compose-gateway --output text
```

适合看完整配置细节。

### 4. 解析某个 backend 的 endpoint

```bash
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend --output text
```

示例输出里的：

- `serverAddress`
  - 是 `ingatectl` 连到的 `xds-server`
- `endpoints`
  - 是控制面最终解析出的上游地址列表

### 5. 做一站式检查

```bash
./_output/darwin_arm64/ingatectl xds check --gateway compose-gateway --backend compose-backend
./_output/darwin_arm64/ingatectl xds check --gateway compose-gateway --backend compose-backend --output text
```

这个命令适合值班视角，快速判断：

- gateway 有没有发布
- config 能不能读
- backend 能不能解析

### 6. 拉标准 ADS/xDS 资源

```bash
./_output/darwin_arm64/ingatectl xds ads --gateway compose-gateway --type lds
./_output/darwin_arm64/ingatectl xds ads --gateway compose-gateway --type rds
./_output/darwin_arm64/ingatectl xds ads --gateway compose-gateway --type cds
./_output/darwin_arm64/ingatectl xds ads --gateway compose-gateway --type eds
```

适合确认 Envoy 理论上应该拿到什么资源。

## 常用参数

- `--server`
  - `xds-server` gRPC 地址
- `--gateway`
  - gateway key，当前 demo 通常用 `compose-gateway`
- `--backend`
  - backend 名称，当前 demo 通常用 `compose-backend`
- `--output`
  - `json` 或 `text`
- `--timeout`
  - RPC 超时时间

## 当前 demo 的推荐用法

先看发布列表：

```bash
./_output/darwin_arm64/ingatectl xds list --output text
```

再看摘要：

```bash
./_output/darwin_arm64/ingatectl xds summary --gateway compose-gateway --output text
```

再看 backend 解析：

```bash
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend --output text
```

最后跑检查：

```bash
./_output/darwin_arm64/ingatectl xds check --gateway compose-gateway --backend compose-backend --output text
```

## 如何理解它和其他接口的区别

- `admin-api`
  - 看的是“资源对象是什么”
- `ingatectl`
  - 看的是“控制面最终解析成什么”
- Envoy admin
  - 看的是“Envoy 实际加载和运行成什么”

排障时通常按这个顺序看：

1. `admin-api`
2. `ingatectl`
3. Envoy admin

## 当前限制

- 还没有资源 create/update/delete 子命令
- 重点是 xDS 和控制面可观测性
- 适合值班排障，不是完整日常运维 CLI
