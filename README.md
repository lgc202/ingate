# Ingate

Ingate 是面向 API 网关、AI 网关、流量分析和治理的一体化产品工程。当前仓库采用单仓结构，同时包含控制台前端、admin-api、apiserver、controller、xDS 服务和 all-in-one 交付配置。

## 项目方向

- 普通 API 网关：管理网关、路由、服务和策略，先跑通代理闭环
- AI 网关：把模型服务、Agent 服务、MCP 服务作为一类后端服务接入
- 数据分析：基于访问日志和 AI 用量识别 API、账号、用户、应用、源 IP 等资产
- 治理处置：围绕 API、风险事件、用户和服务执行限流、封禁、下线等治理动作

当前阶段优先保证网关主链路稳定：

```text
Gateway / Route / Upstream -> Compiler -> RuntimeSnapshot -> xDS -> Envoy
```

## 目录结构

```text
cmd/                    后端进程入口
internal/               后端内部实现
pkg/                    API 类型、客户端和生成代码
web/console/            控制台前端
deploy/all-in-one/      all-in-one 镜像和运行配置
docs/                   设计文档
hack/                   代码生成脚本
install.sh              all-in-one 本地安装和运行脚本
Makefile                统一构建入口
```

前端和后端在同一仓库内管理，但工程边界保持隔离：前端只通过 `/api/v1` 调用 admin-api，不直接依赖 Go 代码；all-in-one 只消费 `web/console/dist` 构建产物。

## 本地构建

构建后端：

```bash
make build
```

构建前端：

```bash
make console-build
```

构建 all-in-one 镜像：

```bash
make all-in-one-image
```

`make all-in-one-image` 会依次构建 Go 二进制、`web/console` 前端产物，并把它们打进 `ingate/all-in-one:dev` 镜像。

## 本地运行

启动或重启 all-in-one：

```bash
./install.sh restart --image ingate/all-in-one --tag dev --data-dir ./ingate-dev
```

默认访问入口：

```text
控制台: http://127.0.0.1:8001
网关 HTTP: http://127.0.0.1:8080
网关 HTTPS: https://127.0.0.1:8443
```

all-in-one 内部包含：

```text
etcd
ingate-apiserver
ingate-controller
ingate-xds
ingate-admin-api
Envoy
Console
```

## 网关入口模型

all-in-one 默认固定暴露运行入口端口：

```text
HTTP  8080
HTTPS 8443
```

多个业务 Gateway 共享同一组运行入口，通过 Host、SNI、Path、Method 和 Header 等规则区分流量。网关不是 Docker 端口映射对象，端口属于运行时部署。

同一个运行入口下可以有多个指定 Host 的 Gateway，但只允许一个启用状态的“不限制 Host”Gateway 作为默认入口。

## 常用验证

```bash
go test ./internal/xds/server ./internal/adminapi/...
npm --prefix web/console run build
curl -sSf http://127.0.0.1:8001/api/v1/gateways
curl -sSf -D - http://127.0.0.1:8080/ -o /dev/null
```

网关入口响应里应该能看到：

```text
server: envoy
```

## 本地数据

`ingate-dev/` 是 all-in-one 本地运行数据目录，包含 etcd 数据和日志，只用于开发验证，不提交到 Git。

停止容器但保留数据：

```bash
./install.sh stop
```

删除容器并保留数据：

```bash
./install.sh delete
```

删除容器并清理数据：

```bash
./install.sh delete --purge-data
```
