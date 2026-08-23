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

## 配置

`.env` 保存镜像版本、监听地址和对外端口。`docker/configs` 保存各个 Ingate 组件的 YAML 配置。修改后执行 `./bin/start.sh` 重建对应容器。

安装脚本会在 `.env` 中生成 Console 管理密码和会话签名密钥。管理用户名固定为 `admin`，密码只在安装结束时显示；可以直接编辑 `INGATE_ADMIN_PASSWORD` 后执行 `./bin/start.sh` 修改。即使已经启用登录认证，仍建议默认绑定 `127.0.0.1`，远程访问优先使用 HTTPS 反向代理或 SSH 端口转发。

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

`backup.sh` 会短暂停止所有组件，对 etcd、Redis、Kafka、ClickHouse、ALS 队列、证书和 Wasm 缓存的 Docker Volume 做一致性归档，然后恢复原来的运行状态：

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

`--remove-images` 可以同时删除未被其他容器使用的 Ingate 组件镜像，`--yes` 可以在自动化环境中跳过交互确认。未指定 `--keep-data` 时，etcd、Kafka、ClickHouse、Redis 和 ALS 本地队列数据都会被永久删除。
