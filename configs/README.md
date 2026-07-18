# 服务配置

源码目录 `configs` 保存 Ingate 各进程的默认 YAML 配置：

- `ingate-apiserver.yaml`
- `ingate-admin-api.yaml`
- `ingate-controller.yaml`

服务通过 `--config` 指定配置文件，通过 `--version` 输出构建版本。配置项可以用环境变量覆盖，前缀分别为：

- `INGATE_APISERVER_`
- `INGATE_ADMIN_API_`
- `INGATE_CONTROLLER_`

环境变量名称由 YAML 路径转为大写并将 `.` 替换为 `_`，例如 `logging.level` 对应 `INGATE_CONTROLLER_LOGGING_LEVEL`。

安装包和容器内的实际配置随各组件放在 `/opt/ingate/<component>/configs/config.yaml`。API Server 自身运行证书放在 `/opt/ingate/apiserver/certificates`；Gateway 使用的 Certificate 是声明式资源，由 API Server 持久化到 etcd，不从这个目录读取。

应用日志默认使用 JSON 格式，不向标准输出写入。源码开发默认写入 `_output/logs`；安装包和容器内写入 `/data/ingate/<component>/logs`。etcd 和 Redis 的持久化数据分别放在 `/data/ingate/etcd/data` 和 `/data/ingate/redis/data`。

源码开发不要求创建 `/opt/ingate` 或 `/data/ingate`，可以继续通过 `--config` 使用仓库内的相对路径。

服务会监听配置文件变化。`logging.level` 会立即生效；监听地址、存储连接、文件日志等其它配置需要重启服务。
