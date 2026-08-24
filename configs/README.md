# 服务配置

源码目录 `configs` 保存 Ingate 各进程的默认 YAML 配置：

- `ingate-apiserver.yaml`
- `ingate-admin-api.yaml`
- `ingate-assistant.yaml`
- `ingate-controller.yaml`
- `ingate-console.yaml`
- `ingate-ai-extproc.yaml`
- `ingate-als.yaml`
- `ingate-analytics.yaml`

服务通过 `--config` 指定配置文件，通过 `--version` 输出构建版本。所有组件直接读取完整 YAML，不为每个字段维护环境变量覆盖规则。不同部署环境应提供对应配置文件，敏感配置由部署系统生成或挂载到配置文件中。

Docker Compose 的容器专用配置放在 `deploy/docker/configs`，它们使用容器服务名连接依赖。不要把容器地址写回这里的直接运行配置。

容器内的实际配置随各组件放在 `/opt/ingate/<component>/configs/config.yaml`。API Server 自身运行证书放在 `/opt/ingate/apiserver/certificates`；Gateway 使用的 Certificate 是声明式资源，由 API Server 持久化到 etcd，不从这个目录读取。

所有 Go 服务的应用日志由 Kratos Log 输出到标准错误流，容器、systemd 或日志采集器负责收集、落盘和轮转；etcd、MySQL、Redis、Kafka 和 ClickHouse 数据分别保存在独立 Volume。Assistant 的 MySQL 密码和模型凭据通过配置文件中的显式环境变量占位符注入；模型地址和名称均为空时，组件可以启动，但不会执行对话。

源码开发不要求创建 `/opt/ingate`，可以继续通过 `--config` 使用仓库内的相对路径。

配置文件修改后需要重启对应服务才能生效。
