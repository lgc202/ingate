# 控制面与 AI 数据面启动

Controller 通过 Ingate API Server 的 List/Watch 读取领域资源，为 Envoy 和 AI Proxy 发布 Delta ADS。
派生配置不再写入 ConfigMap，因此不需要专有配置 Namespace，也不再有 `--ai-config-namespace` 参数。
业务 Namespace 仍由管理员通过 Ingate API 创建，Gateway、路由和后端引用仍在同空间解析。
原生 Namespace、ConfigMap API 继续保留，不因更换下发通道而删除 API 存储。

## 部署边界

Manager 使用指向 Ingate API 的 kubeconfig，不能误用宿主 Kubernetes 的当前上下文。
AI Proxy 只连接 Manager 的 ADS 地址，不再需要 kubeconfig 或读取 ConfigMap 的权限。
Envoy 和 AI Proxy 可以是同一共享数据面组的多个副本；当前固定组名为 `ingate-shared-data-plane`。
每个实例的 Node ID 由部署侧分配，不能用 Node ID 代替身份认证。

当前 ADS 使用明文 gRPC，与现有 Envoy 控制连接一样依赖受保护的部署网络。
Unix 入口和出口目录必须由部署侧预先创建并限制权限。当前没有 ADS 通道 TLS、服务间鉴权或自动安装器；
Backend 的上游 TLS 是另一段连接，已支持显式 SNI 与系统 CA。
Manager 未配置可用选主，只能部署一个副本；多个 Envoy/AI Proxy 副本不等于控制面支持多活。

## 启动接线

先准备 Ingate API 连接和 Manager 的 MySQL 配置。仓库配置只作示例，不包含可直接用于生产的连接凭据。

```sh
go run ./cmd/ingate-controller-manager \
  --kubeconfig /absolute/path/to/ingate.kubeconfig \
  --config /absolute/path/to/controller-manager.yaml \
  --xds-bind-address 127.0.0.1:18000 \
  --ai-ingress-socket /run/ingate/ingress.sock \
  --ai-egress-socket /run/ingate/egress.sock
```

Envoy 的 bootstrap 使用 `DELTA_GRPC` 连接同一 ADS 地址，`node.cluster` 使用上述组名。
AI Proxy 的[启动配置](../configs/ai-proxy.yaml)通过以下字段连接该服务：

```yaml
discovery:
  address: 127.0.0.1:18000
  node_id: ingate-ai-proxy-1
  max_response_bytes: 16777216
```

```sh
go run ./cmd/ingate-ai-proxy --conf configs/ai-proxy.yaml
```

地址中的回环只适用于同主机部署；跨主机需替换为受保护网络内可达的地址。
入口、出口 UDS 路径必须与 Envoy 和 Controller 的下发配置一致；它们是数据面主机路径，不是 Manager 本机连接路径。
示例接收预算是 16 MiB，须结合初始全集大小选择，不是默认值或无限容量。

## 同步与就绪

Manager 等待六类源集合同步后发布第一份有效配置；没有 AI 路由时发布合法空集合。
AI Proxy 在完整批次校验、构建入口索引后交换运行视图并 ACK；首次成功之前，请求返回配置不可用。
完整撤销路由后仍有有效空视图，请求返回未匹配；断线或坏更新保留最近有效值，不解释为资源删除。
进程重启不继承内存版本，会重新完整同步；普通重连通过已接受版本补发变化和删除。

HTTP 关闭时先停止接收并排空在途请求，再取消 ADS Loader，最后关闭连接。
已捕获视图不会在处理中改变。Manager ready 仅说明本地监听和缓存状态，不表示代理已 ACK 或真实后端可用。
原生 ADS 联合测试不替代实际 Envoy、API Server/etcd、MySQL 和跨进程部署验证；这些仍留待统一验收。
