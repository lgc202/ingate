# Ingate

Ingate 是面向 API 网关和 AI 网关的声明式 Envoy 控制面。应用服务、模型、MCP 和 Agent 统一建模为 `Upstream`，Envoy 是唯一数据平面。

## 架构

```text
Gateway / Route / Upstream / Policy
                  |
                  v
          ingate-apiserver
                  |
                  v
         ingate-controller
  Resource Watch -> Envoy Compiler
                 -> Config Delivery
                 -> xDS Snapshot Cache
                  |
                  v
                Envoy
```

一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例。一套 Ingate 可以声明多个逻辑 Gateway；所有资源会被全量编译成同一份 Envoy 配置。IP Upstream 使用 EDS，hostname Upstream 使用 Envoy `STRICT_DNS` cluster。

主要组件：

- `ingate`：CLI 和本地管理入口
- `ingate-admin-api`：控制台产品 API
- `ingate-apiserver`：声明式资源 API，也是持久化数据的唯一入口
- `ingate-controller`：资源收敛、Envoy 配置编译、Delivery 和 ADS xDS
- `Envoy`：唯一数据平面，二进制来自带 Redis 扩展 ABI 的 Higress Envoy
- `etcd`：由 apiserver 使用的声明式资源存储
- `Redis`：限流及未来 Token 配额等请求路径共享状态

内置限流和访问控制以强类型 Policy 与 PolicyBinding 对外提供。用户不需要安装内置 Wasm 插件，也不需要配置 Redis 地址；系统 Redis 由 Envoy bootstrap 中固定的 `ingate-system-redis` 使用。

## 目录

```text
cmd/                    服务和 CLI 入口
internal/               控制面内部实现
pkg/                    声明式 API 类型与生成客户端
plugins/                内置 Proxy-Wasm 插件和 Ingate Redis ABI
web/console/            控制台前端
deploy/all-in-one/      all-in-one 镜像与运行配置
hack/                   代码生成脚本
install.sh              本地安装脚本
```

## 构建

项目使用 Go 1.26。

```bash
make test
make build
make plugins-build
make console-build
make all-in-one-image
```

## 本地运行

```bash
./install.sh restart \
  --image ingate/all-in-one \
  --tag dev \
  --data-dir ./ingate-dev
```

默认入口：

```text
Console:      http://127.0.0.1:8001
Gateway HTTP: http://127.0.0.1:8080
```

all-in-one 内只运行产品必需组件：etcd、Redis、ingate-apiserver、ingate-controller、ingate-admin-api、Envoy 和 Console。测试后端使用独立容器，不进入产品镜像。

本地数据和日志默认保存在 `ingate-dev/`，不提交到 Git。
