# 安装与运维

Ingate 的进程配置、健康检查和优雅退出不依赖具体部署方式。本页说明当前完整支持的 Docker Compose 交付；systemd 和 Kubernetes 尚未提供正式安装清单。

## 管理入口安全

安装脚本生成一个随机管理密码和独立的会话签名密钥。Console 使用 HttpOnly、SameSite=Strict 的签名 Cookie 维护会话，并在反向代理到 Admin API 前注入可信管理员身份；浏览器不能直接伪造该身份 Header。

当前是一套 Ingate 一个管理员凭据，不包含用户表、角色权限或 OIDC。Admin API 是内部组件，不应绕过 Console 暴露到外部网络。远程访问 Console 时应使用 HTTPS 反向代理，并把 `secure_cookie` 配置为 `true`。

## 备份、恢复与升级

安装目录中的脚本负责组件级交付操作：

```bash
./bin/backup.sh
./bin/restore.sh ./backups/ingate-YYYYMMDD-HHMMSS.tar.gz
./bin/upgrade.sh vX.Y.Z
```

备份会短暂停止写入，归档当前 Release 文件、`.env` 和所有 Ingate 持久化 Volume，然后恢复备份前的运行状态。归档包含：

- etcd 声明式资源和证书
- Redis 实时计数
- Kafka 待消费消息
- ClickHouse 明细和聚合
- ALS 本地待投递队列
- Controller Wasm 缓存

恢复会先停止并移除当前容器，再覆盖已知 Ingate Volume 和安装文件。它不会从归档中创建任意名称的 Docker Volume。自动化恢复必须显式传入 `--yes`；交互执行要求输入 `restore`，防止误覆盖。

升级先下载目标 Release 和 SHA-256 校验文件，创建完整备份，再替换组件配置包并拉取新镜像。`.env` 中的管理员密码、会话密钥和端口配置会保留，只更新 `INGATE_VERSION`。升级失败时脚本会输出对应的恢复命令。

## ClickHouse 数据保留

Analytics 配置分别控制三类 TTL：

```yaml
data:
  clickhouse:
    retention:
      request_records: "2592000s"
      request_metrics: "31536000s"
      model_calls: "2592000s"
```

Proto Duration 只接受秒数与小于秒的单位，因此配置使用秒数表达 30 天和 365 天。

- `request_records`：排障请求明细，默认 30 天
- `model_calls`：模型线路调用明细，默认 30 天
- `request_metrics_1m`：分钟流量聚合，默认 1 年
- `model_usage_1m`：累计 Token 与调用量账本，不设置 TTL

TTL 由显式 `ingate-analytics migrate` 命令修改，正常 Analytics 服务账号只需要数据读写权限时可以与迁移账号分开。缩短 TTL 后，ClickHouse 在后台合并时异步清理数据，不保证配置保存后立即释放磁盘。

## 健康检查与指标

各组件 HTTP 服务统一提供：

- `/healthz`：进程能否响应 HTTP，不探测所有外部依赖
- `/readyz`：是否已经具备接收业务流量或查询的条件
- `/metrics`：Prometheus 文本指标

关键指标包括：

| 组件 | 指标 |
| --- | --- |
| Controller | 当前有效资源数、策略挂载数、最近发布是否失败 |
| Authz | 检查总数、放行、鉴权拒绝、限流拒绝、执行错误 |
| AI ExtProc | 处理流数量、流错误、正在关联的 AI 请求 |
| ALS | 接收、Kafka 投递、WAL 写入与积压字节 |
| Analytics | 消费、写入、查询和失败计数 |

Compose 健康检查使用 `/readyz`，因此 Redis、Kafka、ClickHouse 或资源 Watch 尚未就绪时，相关容器不会被标记为 healthy。Controller 和 Envoy 的内部端口只监听 loopback 或共享网络命名空间；其他组件的运维端口也只留在 Compose 网络中，不默认发布到宿主机。

Redis 故障时，请求限流和 Token 额度采用失败关闭，受影响请求不会绕过治理规则。Kafka 或 ClickHouse 故障不会改变同步转发结果，但会延迟请求记录，并可能增加 ALS 本地 WAL 占用。
