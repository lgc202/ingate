# 本地开发运行

开发环境将 MySQL、Redis、etcd 放在 Docker Compose 中；API Server、Controller、Admin API
和前端在本机由独立 tmux 服务托管，也可交给 IDE 调试。修改业务代码只需重新构建、重启对应进程。
本入口不改变生产配置，不安装 Kubernetes，不使用宿主的默认 kubeconfig。

## 准备

需要 Go 1.27、Docker Engine 与支持 `up --wait` 的 Compose、tmux 3.2+、OpenSSL、curl、kubectl、lsof；
前端需要 Node.js 24、npm 11/12 和 shasum。macOS 可以使用已有的 Docker Desktop 或 OrbStack，
先启动其 Docker daemon。脚本不会自动启动或切换 Docker Context。

在仓库根目录执行：

```sh
make dev-up
```

首次启动会生成配置、等待依赖健康、构建 Go 入口、执行数据库迁移，再按 API Server →
Namespace → Controller → Admin API → 前端的顺序启动并检查入口。前端依赖首次及锁文件
变化后自动 `npm ci --ignore-scripts`，要求 npm 和 Node 位于当前 PATH 中。
重复执行复用已经运行的托管组件，不会重启它们；修改代码后通过单组件 stop/start 恢复。

`dev-init` 也可单独生成 `.dev/`，目录权限为 0700，初始化文件权限为 0600，包含一年有效的本地 CA、
服务端/客户端证书、独立 kubeconfig、随机 MySQL 口令和两个进程的配置。重复执行会拒绝覆盖。
该目录已被 Git 忽略，不能提交、共享或用于生产；不要把开发 CA 加入系统信任库。
镜像版本固定在 [Compose](../deploy/dev/compose.yaml)，使用 `pull_policy: missing`，
本地有相同标签时直接复用，缺少时才拉取。当前采用本机已有的 `mysql:8.4.11`、
`redis:7.4.9-bookworm` 和 `registry.k8s.io/etcd:3.6.4-0`，不使用浮动 `latest`。

`dev-deps-up` 等待三个依赖健康；MySQL 健康检查实际执行 SQL，迁移仍通过现有
`ingate-admin-api --migrate` 显式执行。脚本不会清空数据库、自动改写旧表或重建已有卷。
一键启动中途失败立即返回失败，已启动的组件、依赖和日志保留用于排查；不会回滚数据库
或自动停止原来就运行的组件。排查后重试 `dev-up` 或执行 `dev-down`。

```sh
make dev-status                     # 组件进程、退出码与依赖容器状态
make dev-logs                       # 持续跟踪全部组件日志
make dev-logs COMPONENT=admin-api   # 只看 Admin API，Ctrl+C 结束看日志
make dev-down                       # 停止托管组件及依赖，保留数据和日志
```

日志追加写入 `.dev/logs/<component>.log`。每个检出目录使用自己的 `.dev/tmux.sock`，
不使用个人默认 tmux 服务，不保存 PID，不按端口或进程名执行 kill。
进程退出会保留会话和退出码，`dev-status` 不把进程存活等同于业务就绪。
停止时发送终端 Ctrl+C，等待组件退出后才移除会话；30 秒仍未退出会报错，保留会话和依赖。

## 单组件调试

已启动环境时，把一个组件交给 IDE：

```sh
make dev-stop COMPONENT=admin-api
# 在 IDE 中运行 cmd/ingate-admin-api，参数 --conf .dev/admin-api.yaml，工作目录为仓库根目录
```

其他组件继续运行。调试结束先停止 IDE 进程，再恢复托管实例：

```sh
make dev-start COMPONENT=admin-api
```

也可以从第一次启动就排除它：

```sh
make dev-up EXCLUDE=admin-api
# 然后在 IDE 中启动 Admin API
```

`COMPONENT` 支持 `apiserver`、`controller`、`admin-api`、`web`、`data-plane`；`EXCLUDE` 可用逗号分隔多个。
排除只表示本次不启动，不会停止已有的托管实例，已有实例仍需先 `dev-stop`。
`dev-start` 会重建对应 Go 二进制或准备前端依赖，再启动和检查该组件；不会重建其他组件。
端口被 IDE 或其他进程占用时拒绝启动，不会自动结束那个进程。

API Server 被排除且 IDE 尚未启动时，Controller 可以先运行并等待连接；一键入口会提示
该依赖未就绪。IDE 启动 API Server 后再执行 `make dev-namespace` 完成初始化。
Controller 被排除时没有查询投影更新；这两种调试方式暂时影响管理功能，不能解释为全链路可用。

## 独立终端启动（可选）

保留以下前台入口供手工调试。使用前先单独执行 `dev-init`（配置不存在时）、`dev-deps-up`、
`make build` 和 `dev-migrate`，并停止同组件的托管实例，避免端口冲突。

终端一：

```sh
make dev-apiserver
```

另一个终端等待 API Server 就绪并创建业务 Namespace；然后分别启动两个长驻进程：

```sh
make dev-namespace
```

```sh
# 终端二
make dev-controller
```

```sh
# 终端三
make dev-admin
```

`dev-namespace` 使用显式的 Ingate kubeconfig，有限重试 `/readyz`，成功后幂等创建 `dev`。
不会创建宿主 Kubernetes Namespace。Controller 只运行一个副本，Get/List 的 MySQL 投影
仍是异步的；Admin 写入成功不表示列表立即刷新或数据面已接收配置。

| 入口 | 本地地址 |
| --- | --- |
| Ingate API Server（HTTPS） | `127.0.0.1:18443` |
| Admin HTTP / gRPC | `127.0.0.1:8000` / `127.0.0.1:9000` |
| Controller Delta ADS | `127.0.0.1:18000` |
| Controller metrics / health | `127.0.0.1:18080` / `127.0.0.1:18081` |
| MySQL / Redis / etcd | `127.0.0.1:13306` / `127.0.0.1:16379` / `127.0.0.1:12379` |
| 前端 | `127.0.0.1:5173` |
| Envoy 网关入口 / 本地管理口 | `127.0.0.1:18088` / `127.0.0.1:19000` |

所有宿主入口限定回环。依赖端口避开数据库常用端口，但同时运行第二套仍会端口冲突，
启动报错时不要借用其他项目的库。Compose 项目名按检出目录的绝对路径计算，网络和卷不与
其他检出目录复用；移动目录前先停止旧项目，否则新路径会对应另一套卷。

## API Server 的独立运行方式

开发环境直接使用现有 Generic API Server 参数，不增加开发专用认证绕过代码：

- 使用本地 CA 签发的客户端证书。开发客户端属于上游 `system:masters` 组，具有完整权限，
  供本地 Admin、Controller 和 kubectl 共用；这不是生产工作负载身份隔离方案。
- `--kubeconfig`、`--authentication-kubeconfig` 和 `--authorization-kubeconfig` 都显式指向
  Ingate 自身；证书认证与上游特权组授权不需要宿主 Kubernetes 的 TokenReview/SAR。
  Ingate 没有安装这两类 API，该环境不支持委托 Token 或普通组的授权。
- `--authentication-skip-lookup` 跳过宿主认证 ConfigMap 查询，客户端 CA 显式提供。
  没有启用请求头身份代理，也没有关闭认证或设置全路径放行。
- 保留 NamespaceLifecycle；关闭需要完整 Kubernetes API 的四个 Webhook/Policy 插件及 APF。
  这些开关只在开发脚本中生效，不改变二进制默认值或准入实现。

检查本地资源时始终显式指定 kubeconfig：

```sh
kubectl --kubeconfig .dev/ingate.kubeconfig get namespaces
kubectl --kubeconfig .dev/ingate.kubeconfig -n dev get gateways
```

## 前端与 IDE

```sh
cd web/console
npm ci --ignore-scripts
npm run dev
```

手工调试时可从仓库根目录执行 `make dev-web`。访问
`http://127.0.0.1:5173/gateways?namespace=dev`，Vite 将管理请求代理到本机 Admin API。

浏览器通过登录页使用已有账户，初始账户为 `admin/admin`；已有数据库的账户不会被重置。
复用 Redis Session，凭据保存在 HttpOnly、Secure、SameSite=Strict Cookie，JavaScript
不接收或存储 Token。写操作要求自定义头与同源 Origin，不开放跨源 CORS。
Vite 代理显式保留 Host，部署代理也须保留公开 Host，不能改写为内部服务地址。
本机使用浏览器的 loopback 安全上下文例外；外部部署必须提供 HTTPS，登录限流仍未完成。
CLI/gRPC 继续使用原有 Bearer 登录响应和认证。浏览器会话失效后回到登录页。

IDE 可直接运行 `cmd/ingate-*`，工作目录设为仓库根目录；参数以
[启动脚本](../scripts/dev.sh)各分支为准，Admin 使用 `--conf .dev/admin-api.yaml`，
Controller 使用 `--config .dev/controller-manager.yaml` 和显式 kubeconfig。
修改 Go 代码后执行 `make build BINS=对应二进制` 并重启该进程即可，不需重建依赖容器。

## 数据面边界

默认启动 Envoy 与 AI Proxy 容器，使用独立命名卷共享 `/run/ingate/ingress.sock` 与
`egress.sock`。两者使用开发用户 root（只在该专用网络/卷中），Socket 目录为 0700，
避免跨 macOS 与 Linux 共享 UDS。AI Proxy 在本机交叉编译，镜像构建上下文只包含二进制，
不包含 `.dev/` 的证书或口令；运行镜像复用缓存的 `envoyproxy/envoy:v1.39.1` 及其系统 CA。

Controller 默认下发上述 Socket 路径。Envoy 和 AI Proxy 通过 `host.docker.internal:18000`
连接宿主的 Delta ADS；该接线已在 macOS/OrbStack 验证。其他容器运行时仍需确认能访问
宿主回环监听，不因容器内 `127.0.0.1` 与宿主相同而假设可用，脚本不把 ADS 暴露到所有网卡。

本地仅映射 Gateway Listener 的 `8080` 到宿主 `18088`；创建网关时选 HTTP 端口 8080。
添加其他入口端口需显式修改 Compose 的端口映射。Envoy 管理口 `19000` 只发布到回环，
`/ready` 是进程及配置初始化检查，不代表某个业务后端已经可达。

```sh
make dev-stop COMPONENT=data-plane
make dev-start COMPONENT=data-plane
make dev-logs COMPONENT=data-plane
make dev-up EXCLUDE=data-plane
```

数据面作为一个容器组操作；`dev-start` 重新编译 AI Proxy 并启动两容器。本机 IDE 不能直接
代替 Linux 容器中的 UDS 服务；远程调试需在同一 Linux 环境配置。模型凭据管理仍未接入。

## 停止、恢复与数据保留

托管运行执行 `make dev-down`。手工前台运行则先 Ctrl+C 停止各进程，再执行：

```sh
make dev-deps-status
make dev-deps-down
```

停止依赖保留三个数据卷，重新 `dev-deps-up` 后复用数据。没有自动删卷或全局 prune
命令。不要单独删除 `.dev/` 后重新初始化：已有 MySQL 卷不会自动更新随机口令，新的 CA
也会使旧 kubeconfig 失效。重置需先确认本套卷可丢弃，再协调清理配置和数据。

本机 Docker daemon 未启动时只能检查 Compose 结构与初始化产物，不能据此宣布真实依赖、
迁移、联合启动或请求链路已验证。实际结果需与普通测试、配置检查分开记录。
进程托管回归可单独执行 `bash scripts/dev/processes_test.sh`；它使用真实 tmux 与临时假组件，
不绑定网络端口、不启动数据库，也不操作现有 tmux 会话，不能替代真实组件联调。

## 参考

- [Docker Compose 启动和健康依赖](https://docs.docker.com/compose/how-tos/startup-order/)
- [MySQL 官方镜像初始化约定](https://hub.docker.com/_/mysql)
- [Redis 官方镜像](https://hub.docker.com/_/redis)
- [etcd 官方容器参数](https://etcd.io/docs/v3.6/op-guide/container/)
- [Kubernetes x509 客户端身份](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#x509-client-certs)
- [tmux 官方手册：独立 Socket、会话和退出状态](https://man.openbsd.org/tmux.1)
