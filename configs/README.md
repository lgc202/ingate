# 服务配置

`configs` 保存 Ingate 各进程的默认 YAML 配置：

- `ingate-apiserver.yaml`
- `ingate-admin-api.yaml`
- `ingate-controller.yaml`

服务通过 `--config` 指定配置文件，通过 `--version` 输出构建版本。配置项可以用环境变量覆盖，前缀分别为：

- `INGATE_APISERVER_`
- `INGATE_ADMIN_API_`
- `INGATE_CONTROLLER_`

环境变量名称由 YAML 路径转为大写并将 `.` 替换为 `_`，例如 `logging.level` 对应 `INGATE_CONTROLLER_LOGGING_LEVEL`。

应用日志默认使用 JSON 格式，不向标准输出写入，而是分别写入 `_output/logs` 下的服务日志文件。all-in-one 部署会将应用日志路径覆盖到 `/var/log/ingate`；各子进程自身的标准输出和错误输出保存在同目录的 `*.process.log` 文件中。

服务会监听配置文件变化。`logging.level` 会立即生效；监听地址、存储连接、文件日志等其它配置需要重启服务。
