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

Console 当前没有登录认证，因此默认只绑定 `127.0.0.1`。远程访问建议使用 SSH 端口转发，不要在公网服务器上直接改为 `0.0.0.0`。

## 日常操作

```bash
./bin/status.sh       # 查看组件状态
./bin/logs.sh         # 查看全部日志
./bin/logs.sh envoy   # 只查看 Envoy 日志
./bin/stop.sh         # 停止容器并保留数据
```

如果确认不再需要任何数据，可以显式删除容器和 Volume：

```bash
docker compose --env-file .env -f compose.yaml down -v
```

该命令会删除 etcd、Kafka、ClickHouse、Redis 和 ALS 本地队列数据，不可恢复。
