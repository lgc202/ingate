# Ingate All-in-One 部署设计

本文档描述 Ingate 第一阶段 all-in-one 部署方案。目标是让用户用一个 Docker 容器快速跑通完整代理链路，同时保持内部组件边界清晰，后续可以平滑扩展到 VM/systemd、Docker 多容器和 Kubernetes 部署。

## 背景

Ingate 当前产品演进路径是：

```text
API 网关 -> AI 网关 -> 分析 -> 治理
```

第一阶段最重要的不是一次性做完所有部署形态，而是让用户用最少步骤验证核心闭环：

```text
控制台创建 Gateway
控制台创建 Upstream
控制台创建 Route
配置自动生效
Envoy 成功代理请求
```

因此 all-in-one 的定位不是最终生产架构，也不是把所有组件揉成一个单体进程，而是把多组件打包进一个容器，提供低门槛体验和稳定的本地/单机部署入口。

## 设计目标

- 一个命令启动完整 Ingate 核心链路
- 保持 `frontend -> ingate-admin-api -> ingate-apiserver -> etcd` 作为管理主链路
- 保持 `Resource -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot -> xDS -> Envoy` 作为运行主链路
- 控制台保存网关、路由、服务后自动生效，不引入独立发布页作为必经步骤
- 单容器内仍按组件进程运行，避免把架构变成不可拆分的单进程
- 默认只暴露控制台和网关入口，内部控制面端口尽量绑定到 `127.0.0.1`
- 数据、配置、日志持久化到宿主机目录
- 后续可以复用同一套组件拆到 VM/systemd、Docker 多容器和 Kubernetes

## 非目标

第一阶段 all-in-one 不做以下内容：

- 不引入 MySQL 承载网关配置主链路
- 不引入 Kafka、ClickHouse、OpenSearch 等分析链路组件
- 不实现多节点高可用部署
- 不实现 Kubernetes operator
- 不实现插件运行时完整生命周期
- 不实现 AI Provider 自动配置向导
- 不在安装脚本里提供 route/config 子命令直接修改运行配置
- 不把用户、权限、审计、安装任务等管理面业务一次性做完

这些能力可以后续扩展，但不应阻塞第一条代理链路。

## 参考取舍

Higress all-in-one 有参考价值，主要参考这些点：

- 单容器多进程，降低用户启动成本
- 控制台、网关入口和内部端口分层
- 安装脚本提供 `start`、`stop`、`restart`、`delete`、`status`、`logs` 等生命周期命令
- 本地数据目录和 env-file 持久化
- 容器已存在但停止时可以用 `docker start` 重新拉起
- 启动完成后输出控制台地址、网关入口、日志位置和停止命令

不建议照搬这些点：

- 大量 LLM Provider 参数和交互式 wizard
- 通过脚本子命令直接修改容器内 YAML
- 自动选择插件镜像仓库
- Higress 绑定 Istio/pilot 的组件模型
- 过早暴露 xDS、Envoy admin、apiserver 等内部概念给普通用户

Ingate 第一阶段应该更像一个清爽的产品安装入口，而不是一个把所有高级配置都塞进 shell 的运维工具。

## 总体形态

默认命令：

```bash
./install.sh start
```

默认输出：

```text
Console:      http://localhost:8001
Gateway HTTP: http://localhost:8080
Data dir:     ./ingate
Logs:         ./ingate/logs
Stop:         ./install.sh stop
```

容器名默认使用：

```text
ingate
```

镜像默认使用：

```text
ingate/all-in-one:latest
```

## 组件边界

all-in-one 容器内包含这些进程：

```text
ingate-all-in-one
  ├── etcd
  ├── ingate-apiserver
  ├── ingate-admin-api
  ├── ingate-controller
  ├── ingate-xds
  ├── envoy
  └── console frontend
```

组件职责保持不变：

- `etcd`：保存声明式资源和资源版本
- `ingate-apiserver`：提供资源 API、校验、watch 和版本冲突语义
- `ingate-admin-api`：提供控制台产品 API，做 DTO 转换，不直接暴露 CR 结构
- `ingate-controller`：监听资源变化并生成 RuntimeSnapshot
- `ingate-xds`：把 RuntimeSnapshot 转为 Envoy 可消费的 xDS 配置
- `envoy`：接收用户流量并代理到 Upstream
- `console frontend`：提供用户操作界面

控制台静态资源可以由 `ingate-admin-api` 直接托管，也可以由容器内轻量 HTTP server 托管。第一阶段推荐由 `ingate-admin-api` 托管，减少容器内进程数量和端口映射。

## 数据流

管理链路：

```text
Browser
  |
  v
Console frontend
  |
  v
ingate-admin-api
  |
  v
ingate-apiserver
  |
  v
etcd
```

配置生效链路：

```text
etcd
  |
  v
ingate-apiserver watch
  |
  v
ingate-controller
  |
  v
RuntimeSnapshot
  |
  v
ingate-xds
  |
  v
Envoy
```

代理链路：

```text
Client
  |
  v
Envoy Gateway Port
  |
  v
Upstream
```

这个链路里，`RuntimeSnapshot` 只能说明控制面已经生成期望运行配置，不等于证明 Envoy 已经应用配置。后续如果要展示“已生效”，需要引入 xDS ACK、Envoy 状态或探测结果，不能只依赖控制面生成状态。

## 端口设计

默认暴露到宿主机：

| 宿主机地址 | 容器内地址 | 用途 |
| --- | --- | --- |
| `0.0.0.0:8001` | `0.0.0.0:8001` | 控制台和 admin API |
| `0.0.0.0:8080` | `0.0.0.0:8080` | 网关 HTTP 入口 |
| `0.0.0.0:8443` | `0.0.0.0:8443` | 网关 HTTPS 入口，第一阶段可选 |

网关入口端口属于运行时部署，不属于单个业务 Gateway。all-in-one 默认固定暴露 `8080` 和 `8443`，多个 Gateway 共享同一组入口，通过 Host、SNI、Path、Method、Header 等规则区分流量。

同一个运行入口下，可以有多个指定 Host 的 Gateway；但只允许一个启用状态的“不限制 Host”Gateway 作为默认入口。这样既支持 `curl http://localhost:8080/` 这类本地体验，也避免多个默认 Gateway 同时接收任意 Host 后产生不可解释的匹配结果。

默认只在容器内部访问：

| 容器内地址 | 用途 |
| --- | --- |
| `127.0.0.1:18443` | ingate-apiserver |
| `127.0.0.1:2379` | etcd |
| `127.0.0.1:18000` | ingate-xds |
| `127.0.0.1:15000` | Envoy admin |

后续可选暴露：

| 宿主机地址 | 用途 |
| --- | --- |
| `0.0.0.0:15021` | 健康检查 |
| `0.0.0.0:15090` | metrics |

默认不把内部控制面端口映射到宿主机，除非用户显式开启调试参数。

## 本地目录

默认数据目录：

```text
./ingate/
  default.env
  data/
  logs/
```

容器内目录：

```text
/etc/ingate/
  default.env

/var/lib/ingate/
  etcd/
  apiserver/
  runtime/
  envoy/

/var/log/ingate/
```

目录职责：

- `default.env`：保存用户选择的端口、镜像、绑定地址和数据目录等启动配置
- `data/`：挂载到 `/var/lib/ingate`，保存 etcd 数据和运行时持久化数据
- `logs/`：挂载到 `/var/log/ingate`，保存各组件日志

第一阶段可以先使用统一日志目录，日志文件按组件拆分：

```text
logs/
  apiserver.log
  admin-api.log
  controller.log
  xds.log
  envoy.log
  etcd.log
```

## 启动配置

`default.env` 示例：

```text
INGATE_MODE=all-in-one
INGATE_CONSOLE_ADDR=0.0.0.0:8001
INGATE_GATEWAY_HTTP_ADDR=0.0.0.0:8080
INGATE_GATEWAY_HTTPS_ADDR=0.0.0.0:8443
INGATE_APISERVER_ADDR=127.0.0.1:18443
INGATE_ETCD_ADDR=127.0.0.1:2379
INGATE_XDS_ADDR=127.0.0.1:18000
INGATE_ENVOY_ADMIN_ADDR=127.0.0.1:15000
INGATE_DATA_DIR=/var/lib/ingate
INGATE_LOG_DIR=/var/log/ingate
```

配置文件只保存运行必要配置，不保存网关、路由、Upstream 等业务配置。业务配置必须通过控制台或 API 写入 apiserver，避免出现脚本配置、控制台配置和运行配置三套来源。

## 安装脚本

第一阶段脚本命令：

```text
install.sh start
install.sh stop
install.sh restart
install.sh delete
install.sh status
install.sh logs
```

本地开发构建命令：

```bash
cd /Users/lgc202/workspace/source/lgc202/ingate
make all-in-one-image
./install.sh start --image ingate/all-in-one --tag dev
```

`make all-in-one-image` 会在当前仓库内完成 Go 二进制构建和 `web/console` 前端构建，Dockerfile 直接读取 `web/console/dist`。

推荐参数：

```text
--non-interactive
--container-name ingate
--image ingate/all-in-one
--tag latest
--data-dir ./ingate
--bind 127.0.0.1
--console-port 8001
--http-port 8080
--https-port 8443
```

参数语义：

- `--non-interactive`：非交互模式，适合 CI、脚本和文档复制执行
- `--container-name`：容器名
- `--image` / `--tag`：镜像和版本
- `--data-dir`：宿主机持久化目录
- `--bind`：对外绑定地址，默认本机体验可以用 `127.0.0.1`，需要局域网访问时改为 `0.0.0.0`
- `--console-port`：控制台端口
- `--http-port`：网关 HTTP 入口端口
- `--https-port`：网关 HTTPS 入口端口

脚本不提供 `route add`、`config add` 这类业务配置命令。业务配置统一从控制台或 admin API 进入。

## 生命周期行为

`start` 行为：

1. 检查 Docker 是否可用
2. 创建数据目录和日志目录
3. 生成或读取 `default.env`
4. 如果同名容器正在运行，输出当前访问地址并退出
5. 如果同名容器存在但已停止，执行 `docker start`
6. 如果容器不存在，执行 `docker run`
7. 等待控制台和网关健康检查通过
8. 输出访问地址、数据目录、日志目录和停止命令

`stop` 行为：

```text
docker stop ingate
```

`restart` 行为：

```text
stop -> start
```

`delete` 行为：

- 停止并删除容器
- 默认保留数据目录
- 只有用户显式传入 `--purge-data` 才删除数据目录

`status` 行为：

- 展示容器运行状态
- 展示控制台地址、网关地址、数据目录、日志目录
- 后续可以补充组件健康状态

`logs` 行为：

- 默认跟随 all-in-one 容器日志
- 后续可以支持 `--component admin-api` 查看指定组件日志

## 容器启动顺序

容器入口进程负责按顺序拉起组件：

```text
1. etcd
2. ingate-apiserver
3. ingate-controller
4. ingate-xds
5. envoy
6. ingate-admin-api
```

启动原则：

- 每个组件独立进程
- 组件启动失败时容器整体失败，避免用户看到半可用状态
- 每个组件日志写入独立文件，同时关键日志输出到容器 stdout
- `ingate-admin-api` 最后启动，因为控制台入口需要依赖 apiserver 已可用
- Envoy 可以先用空配置启动，再等待 xDS 下发

第一阶段可以用简单 supervisor 脚本实现，后续再评估是否引入 s6、supervisord 或自研轻量进程管理器。

## 第一阶段验收标准

all-in-one 第一阶段完成后，应能跑通：

1. 执行 `./install.sh start`
2. 打开 `http://localhost:8001`
3. 创建 Gateway
4. 创建 Upstream
5. 创建 Route，并选择 Gateway 和 Upstream
6. 控制面生成 RuntimeSnapshot
7. xDS 下发 Envoy 配置
8. 执行 `curl http://localhost:8080/<path>` 能代理到目标 Upstream
9. 修改 Route 或 Upstream 后无需独立发布，配置自动更新
10. 执行 `./install.sh logs` 能看到关键组件日志
11. 执行 `./install.sh stop` 后容器停止，数据保留
12. 再次执行 `./install.sh start` 后已有配置仍存在

## 与后续部署形态的关系

all-in-one 只是第一种包装方式，内部组件边界不能被 all-in-one 绑死。

后续 VM/systemd 形态：

```text
systemd
  ├── ingate-apiserver.service
  ├── ingate-admin-api.service
  ├── ingate-controller.service
  ├── ingate-xds.service
  ├── envoy.service
  └── etcd.service
```

后续 Docker 多容器形态：

```text
docker network ingate
  ├── etcd
  ├── ingate-apiserver
  ├── ingate-admin-api
  ├── ingate-controller
  ├── ingate-xds
  └── envoy
```

后续 Kubernetes 形态：

```text
Deployment / StatefulSet
  ├── ingate-apiserver
  ├── ingate-admin-api
  ├── ingate-controller
  ├── ingate-xds
  ├── envoy
  └── etcd 或外部存储
```

这三种形态都应该复用同一套二进制、配置项和资源模型。all-in-one 只负责把它们放进一个容器里启动。

## 后续扩展

AI Gateway 阶段可以在 all-in-one 里逐步增加：

- AI Provider / AI Model / AI Route 管理
- OpenAI-compatible 代理
- Token 用量观测
- 内容安全、Prompt 注入防护、PII 脱敏等策略

分析与治理阶段不建议塞进第一阶段 all-in-one 主链路。后续可以提供可选 profile：

```text
all-in-one-core
all-in-one-ai
all-in-one-analysis
```

其中 `all-in-one-core` 只包含网关主链路，`all-in-one-ai` 增加 AI Gateway 运行依赖，`all-in-one-analysis` 再增加日志上报、分析存储和治理能力。

Kafka 可以作为后续分析链路的一部分，用来接收 Envoy 或运行组件上报的访问日志、AI 用量和治理事件。但 Kafka 不应该进入第一阶段网关配置主链路，也不应该成为用户启动第一个代理链路的前置条件。

## 当前实现限制

第一阶段实现需要注意：

- all-in-one 镜像构建依赖当前仓库内的 `web/console`，不再依赖外部前端仓库或手工复制 `dist`
- 当前 all-in-one 不内置 MySQL、Kafka、ClickHouse、OpenSearch 和分析组件
- 当前脚本默认只暴露控制台、网关 HTTP 和网关 HTTPS 端口
- 内部 apiserver、xDS、Envoy admin 端口不默认暴露到宿主机
- Docker 镜像构建和容器启动需要在本机 Docker daemon 可用时验证
