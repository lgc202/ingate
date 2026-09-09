---
title: 安装与运维
description: 管理 Ingate Docker Compose 安装、升级、备份和运行状态
---

Ingate 的服务配置、健康检查、结构化日志和优雅退出保持部署方式中立。当前完整交付方式是 Docker Compose；systemd 和 Kubernetes 尚未提供正式安装清单。

## 安装目录

安装脚本默认把版本固定的 Compose 文件、配置与操作脚本放在执行安装命令时所在目录的 `ingate` 子目录。也可以把目标目录作为第一个参数传给安装脚本。安装目录中的 `.env` 保存当前 Ingate 版本、管理凭据、内部控制面令牌和宿主机端口，安装器会将它限制为当前用户可读写。该文件不应提交到版本库或提供给外部客户端。

常用命令：

```bash
cd <安装目录>
./bin/status.sh
./bin/logs.sh
./bin/stop.sh
./bin/start.sh
```

容器镜像来自 `ghcr.io/lgc202`，业务端口默认只发布 Console、HTTP Gateway 和 HTTPS Gateway。

## 管理入口安全

安装脚本生成随机管理密码和独立会话签名密钥。Console 使用 HttpOnly、SameSite=Strict 的签名 Cookie 维护会话，并在代理 Admin API 时注入可信身份。

当前是一套 Ingate 一个管理员凭据，不包含用户表、角色或 OIDC。Admin API 是内部组件，不应绕过 Console 暴露到外部网络。远程访问 Console 时应在前面配置 HTTPS，并启用安全 Cookie。

## 健康检查

各组件按职责提供以下运维端点：

| 组件 | 健康与指标端点 |
| --- | --- |
| Console、Admin API | `/healthz` |
| Assistant | `/healthz`、`/readyz` |
| Controller、Authz、AI ExtProc、Analytics | `/healthz`、`/readyz`、`/metrics` |
| ALS | `/livez`、`/healthz`、`/readyz`、`/metrics` |
| API Server | `/healthz`、`/livez`、`/readyz`；内部指标端点需要认证 |

Compose 对 Controller、Authz 等具有依赖就绪语义的组件使用 `/readyz` 判断健康状态，对 Console 和 Admin API 使用 `/healthz`。健康检查请求属于高频基础探测，不会按普通业务请求输出 INFO 访问日志。

查看容器状态：

```bash
./bin/status.sh
docker compose ps
```

## 日志

默认使用 JSON 结构化日志并输出到容器 stdout/stderr，由容器运行环境负责收集和轮转：

```bash
./bin/logs.sh
docker compose logs -f ingate-controller
```

业务错误、配置发布失败和外部依赖异常应保留；健康探测、正常 Watch 心跳和成功的内部轮询不应淹没日志。

## 可选本地观测栈

源码部署可以叠加 `deploy/docker/observability/compose.yaml`，一次启动 OpenTelemetry Collector、Prometheus、Loki、Tempo、Grafana 和 Alertmanager：

```bash
docker compose --project-directory . \
  -f deploy/docker-compose.yaml \
  -f deploy/docker-compose.dev.yaml \
  -f deploy/docker/observability/compose.yaml \
  up -d --build --wait
```

安装包会持久化 Overlay 的启用状态。将 `.env` 中的开关改为 `true` 后，通过统一入口启动：

```dotenv
INGATE_OBSERVABILITY_ENABLED=true
```

```bash
./bin/start.sh
```

该 Overlay 只负责系统遥测，不参与请求记录投递。ALS 的 JSON stdout 由 Docker Fluentd logging driver 异步发送到 Collector，Trace 使用 OTLP；两条链路都有有界缓冲。Collector 或后端停止后可能丢失系统遥测，但不会阻塞 ALS 写 Kafka 或本地队列。可以单独停止整个观测栈验证该边界：

```bash
docker compose --project-directory . \
  -f deploy/docker-compose.yaml \
  -f deploy/docker-compose.dev.yaml \
  -f deploy/docker/observability/compose.yaml \
  stop otel-collector prometheus loki tempo grafana alertmanager
```

Grafana 默认位于 <http://127.0.0.1:3000>，仅面向本机开放匿名只读访问；Prometheus 位于 <http://127.0.0.1:9090>，Alertmanager 位于 <http://127.0.0.1:9093>。它们使用独立的 `INGATE_OBSERVABILITY_BIND_ADDRESS`，不会在 Gateway 或 Console 改为外部监听时被连带暴露。Grafana 自动加载 **Ingate / Ingate ALS** Dashboard，告警处置见 [ALS SLO 与告警处置](../als/monitoring/)。在 Grafana Explore 中可以分别查询：

- Prometheus：`ingate_als_records_received_total`
- Loki：`{service_namespace="ingate", service_name="ingate-als"}`
- Tempo：Service Name 选择 `ingate-als`

Prometheus 指标保留 35 天，以覆盖 30 天 SLO 计算窗口；Loki 日志和 Tempo Trace 保留 7 天，Alertmanager 状态保留 7 天。对应 Volume 在重启后保留数据，但它们不包含在核心业务备份中。本地部署在配置 Trace 出口后默认采集全部 ALS 批次，正式环境可以按容量修改 `sample_ratio`。安装包不再需要观测栈时，先运行 `./bin/stop.sh`，将开关改回 `false`，再运行 `./bin/start.sh`；观测数据 Volume 会继续保留。

不启动该 Overlay 时，核心 Compose 行为不变。接入已有 Collector 时，只需在 `.env` 中设置 `INGATE_ALS_TRACING_ENDPOINT=<host>:<port>`；地址为空时 ALS 不创建 Trace 出口。使用 TLS 时还应在 ALS 配置中关闭 `insecure` 并配置证书。外部日志采集仍应直接读取 ALS stdout，不需要修改业务代码。

## 备份与恢复

```bash
./bin/backup.sh
./bin/restore.sh ./backups/ingate-YYYYMMDD-HHMMSS.tar.gz
```

备份会短暂停止写入，归档当前 Release 文件、`.env` 和 Ingate 持久化 Volume，再恢复原运行状态。备份包含管理凭据和内部密钥，脚本会将归档权限限制为当前用户可读。归档覆盖：

- etcd 声明式资源与证书
- MySQL 中的 Assistant 会话、执行记录和模型连接
- Redis 当前限流、额度计数和 Assistant 短期流式事件
- Kafka 待消费消息
- ClickHouse 请求明细与聚合
- ALS 本地 WAL
- Controller Wasm 缓存

恢复会覆盖当前 Ingate 数据。交互执行必须输入 `restore`，自动化恢复必须显式传入 `--yes`。

## 升级

```bash
./bin/upgrade.sh vX.Y.Z
```

升级流程会校验 Release、创建备份、替换组件配置并拉取新镜像。管理员密码、会话密钥和端口配置会保留，只更新 `INGATE_VERSION`。升级失败时脚本会输出对应的恢复命令。

插件拥有独立版本与发布节奏，升级 Ingate 不等于升级已安装插件；插件兼容版本由插件目录和当前 Ingate 版本共同判断。

## 卸载

```bash
./bin/uninstall.sh
```

卸载会明确要求输入 `uninstall`，停止容器并删除 Ingate 创建的 Volume。若 Docker 网络仍被其他容器使用，Compose 会提示网络未删除；脚本应在确认没有残留 Ingate 容器后清理该网络，而不是掩盖警告。

## 数据保留

Analytics 分别控制请求明细、模型调用明细和分钟聚合的 ClickHouse TTL。请求与模型调用明细默认保留 30 天，流量分钟聚合默认保留一年；长期模型用量聚合不设置 TTL。

缩短 TTL 后由 ClickHouse 后台合并异步清理数据，不保证配置修改后立即释放磁盘。
