# Ingate

Ingate 是使用声明式资源管理 HTTP 与 AI 网关配置的 Go 项目。当前控制面采用
Generic APIServer 和 controller-runtime，数据面配置通过 Envoy xDS 发布；
AI Proxy 负责已实现的模型请求适配与流式响应，不替代 Envoy 的通用转发职责。

## 组件

| 二进制 | 作用 |
| --- | --- |
| `ingate-admin-api` | HTTP/gRPC 产品接口、管理员认证和 MySQL 查询模型读取 |
| `ingate-apiserver` | 期望状态的权威存储入口，复用 Kubernetes API 机制和 etcd |
| `ingate-controller-manager` | 资源协调、MySQL 查询投影、共享配置和 xDS 发布 |
| `ingate-ai-proxy` | 受保护 Unix Socket 上的模型请求入口与模型协议适配 |

写入路径是 Admin API → API Server → etcd；Controller 将资源投影到 MySQL，
Get/List 查询 MySQL，不回退读取 API Server。该查询链是最终一致，创建或更新成功
后立即查询可能读到旧视图。Controller 也将配置发布到 Envoy 和供 AI Proxy 读取的
ConfigMap；这些输出之间不是一个事务，发布成功不等于 Envoy 已 ACK。

## 构建与检查

需要 `go.mod` 指定的 Go 1.27 工具链。命令从仓库根目录执行：

```sh
make help
make build
./bin/ingate-admin-api --version
make verify
```

`make build BINS=ingate-admin-api` 可以只构建一个入口。`verify` 执行格式、静态检查、
普通测试和构建，不生成代码、不更新依赖，也不启动真实数据库。

修改 protobuf 后显式执行 `make generate-proto`；修改 Kubernetes API 类型后执行
`make generate-kubernetes`。生成文件不能手改。工具版本由 Make 规则和模块锁定。

## 运行前准备

完整运行需要 etcd、MySQL、Redis 和 Envoy，以及明确指向 Ingate API Server 的
kubeconfig；不应把宿主 Kubernetes 集群的连接误用为 Ingate 连接。
`configs/` 是配置示例，不是可以直接用于生产的安装包。

```sh
./bin/ingate-admin-api --migrate --conf /absolute/path/to/admin-api.yaml
./bin/ingate-admin-api --conf /absolute/path/to/admin-api.yaml
./bin/ingate-controller-manager --help
./bin/ingate-apiserver --help
./bin/ingate-ai-proxy --conf /absolute/path/to/ai-proxy.yaml
```

先准备和迁移 MySQL，再启动依赖查询模型的组件。Admin API 当前只允许监听回环地址，
内建初始账户为 `admin/admin`，提供修改密码接口；不要将默认凭证和未配置 TLS 的接口
直接暴露到外部网络。AI Proxy 的 Socket 目录及 Envoy 连接由部署方预先保护。

控制面 Namespace 的创建、启动顺序及失败边界见
[初始化说明](docs/control-plane-initialization.md)。组件设计与当前验证范围见
[文档索引](docs/README.md)；历史验证记录不能当作最新端到端验收。

## 代码组织

进程入口位于 `cmd/ingate-*`。组件实现隔离在 `internal/<component>`；普通 Web 采用
Server → Handler → Service → Repository，手写构造注入。共享基础设施在
`internal/pkg`；声明式 API 契约和生成客户端在 `pkg/apis`、`pkg/generated`。
Generic APIServer 和 Controller 保留各自原生开发形式，不套用 Web 分层。

开发和评审入口是 [AGENTS.md](AGENTS.md)，本轮对齐依据和进度见
[GoCanon 模板对齐](docs/decisions/0016-gocanon-template-alignment.md)。
