# Ingate Docker Compose

该安装包使用固定版本的容器镜像启动完整 Ingate 环境，不需要 Go、Node.js 或源码。

## 启动

```bash
./bin/start.sh
./bin/status.sh
```

组件就绪后可以访问：

- Console：<http://127.0.0.1:8001>
- HTTP Gateway：<http://127.0.0.1:8080>
- HTTPS Gateway：<https://127.0.0.1:8443>

Gateway 端口只有在 Console 中创建并成功发布对应 Gateway 后才会承载业务流量。

## 可选观测栈

安装包附带可选观测配置。需要查看 ALS 的系统指标、JSON 日志和 Trace 时，将 `.env` 中的开关改为：

```dotenv
INGATE_OBSERVABILITY_ENABLED=true
```

随后执行 `./bin/start.sh`。启用状态保存在 `.env`，因此日常启停、备份、升级和卸载都会继续使用同一 Overlay。Grafana 默认地址为 <http://127.0.0.1:3000>。观测服务使用独立的 `INGATE_OBSERVABILITY_BIND_ADDRESS`，不会随 Gateway 的监听地址向外暴露。Prometheus、Loki、Tempo 和 Alertmanager 使用独立 Volume 保存本地观测数据；停止这些组件不会中断 ALS 向 Kafka 或本地队列投递请求记录。

观测栈不是核心运行依赖。不再需要时，先执行 `./bin/stop.sh`，将开关改回 `false`，再执行 `./bin/start.sh`；观测数据 Volume 会继续保留。

## 配置

`.env` 保存镜像版本、监听地址、对外端口和进程使用的密钥。`docker/configs` 保存各个 Ingate 组件的 YAML 配置。修改后执行 `./bin/start.sh` 重建对应容器。

安装脚本会在 `.env` 中生成 Console 管理密码、会话签名密钥、API Server 内部认证令牌和数据库密码，并将该文件限制为当前用户可读写。管理用户名固定为 `admin`，密码只在安装结束时显示；可以直接编辑 `INGATE_ADMIN_PASSWORD` 后执行 `./bin/start.sh` 修改。API Server 令牌由内部组件共享，不应发送给浏览器或外部客户端。即使已经启用登录认证，仍建议默认绑定 `127.0.0.1`，远程访问优先使用 HTTPS 反向代理或 SSH 端口转发。

## 日常操作

```bash
./bin/status.sh       # 查看组件状态
./bin/logs.sh         # 查看全部日志
./bin/logs.sh envoy   # 只查看 Envoy 日志
./bin/stop.sh         # 停止容器并保留数据
./bin/backup.sh       # 停止写入并备份配置和持久化数据
./bin/upgrade.sh vX.Y.Z # 备份后升级到指定版本
```

## 备份与恢复

`backup.sh` 会短暂停止所有组件，对 etcd、MySQL、Redis、Kafka、ClickHouse、ALS 队列、证书和 Wasm 缓存的 Docker Volume 做一致性归档，然后恢复原来的运行状态。备份包含管理凭据和内部密钥，因此归档文件仅允许当前用户读取：

```bash
./bin/backup.sh
./bin/restore.sh ./backups/ingate-YYYYMMDD-HHMMSS.tar.gz
```

恢复会覆盖当前持久化数据，必须在交互确认后执行。升级脚本会先创建同样的完整备份，再替换 Compose 文件和组件配置；升级失败时可以使用该备份回滚。

## 卸载

完整卸载会删除容器、网络、持久化数据和当前安装目录。脚本会展示删除范围，并要求输入 `uninstall` 确认：

```bash
./bin/uninstall.sh
```

如果需要保留 Docker Volume 以便后续重新安装：

```bash
./bin/uninstall.sh --keep-data
```

`--remove-images` 可以同时删除未被其他容器使用的 Ingate 组件镜像，`--yes` 可以在自动化环境中跳过交互确认。未指定 `--keep-data` 时，etcd、MySQL、Kafka、ClickHouse、Redis、ALS 本地队列、证书和 Controller Wasm 缓存都会被永久删除。
