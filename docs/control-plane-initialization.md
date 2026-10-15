# 控制面 Namespace 初始化

本说明配套共享配置协调接线提交 `fcc9650b`。它提供手动安装入口，不是已完成的自动安装器，也不表示 AI Proxy 已实现或可以接流量。

## 对象与责任

安装者通过 Ingate API Server 创建控制面 Namespace；Controller Manager 随后在其中生成共享 AI 配置 ConfigMap。业务用户继续在各自的 Namespace 中管理 Gateway、路由和后端，不需要创建这份派生配置。

| 对象 | 示例位置 | 创建与维护者 |
| --- | --- | --- |
| 控制面 Namespace | `ingate-system` | 安装流程或部署者 |
| 共享 AI 配置 ConfigMap | `ingate-system/ingate-shared-data-plane-ai` | Controller Manager |
| 业务资源 | `team-a`、`team-b` 等业务空间 | 业务管理流程；引用仍在同空间解析 |

这里的 Namespace 属于 Ingate API，不是宿主 Kubernetes 的 Namespace。即使 Ingate 服务运行在 Kubernetes 中，也不能把宿主集群里创建的同名空间当作 Ingate 已完成初始化。

`ingate-system` 是清单中的示例选择，不是服务的默认值。需要使用其他名称时，先修改[清单](../configs/control-plane-namespace.yaml)的 `metadata.name`，再让 `--ai-config-namespace` 使用同一名称。Controller Manager 不自动创建 Namespace；Namespace Controller 负责删除阶段的内容清理与终结，不负责安装初始化。

## 安装顺序

前提是 Ingate API Server 已运行，已注册 Namespace/ConfigMap API；Controller 的 MySQL 查询模型数据库结构与连接已按现有约定准备。下面的命令从仓库根目录执行，需要本机已有 kubectl。

先提供一个明确指向 Ingate 的 kubeconfig，并核对其中的 API 地址。不要省略 `--kubeconfig` 而使用可能指向宿主集群的当前上下文。

```sh
INGATE_KUBECONFIG=/absolute/path/to/ingate.kubeconfig

kubectl --kubeconfig "$INGATE_KUBECONFIG" config view --minify \
  -o jsonpath='{.clusters[0].cluster.server}{"\n"}'
```

确认地址属于 Ingate 后，应用标准 Namespace 清单并检查对象。已经存在的空间应核对用途和删除状态，不通过删除重建来处理安装冲突。

```sh
kubectl --kubeconfig "$INGATE_KUBECONFIG" apply \
  -f configs/control-plane-namespace.yaml

kubectl --kubeconfig "$INGATE_KUBECONFIG" get namespace ingate-system -o yaml
```

空间存在且未进入删除或 Terminating 后，启动单副本 Controller Manager。配置文件中的 MySQL DSN 需要替换为实际连接；[仓库配置](../configs/controller-manager.yaml)只是占位示例。此处保留独立 API 连接配置，不自动借用宿主集群身份。

```sh
go run ./cmd/ingate-controller-manager \
  --kubeconfig "$INGATE_KUBECONFIG" \
  --config /absolute/path/to/controller-manager.yaml \
  --ai-config-namespace ingate-system
```

即使尚无模型或 MCP 资源，也必须指定 `--ai-config-namespace`。首次完整协调会创建携带完整空 AI 配置的 ConfigMap，用于建立明确的期望状态；安装清单不提前创建空 ConfigMap，也不伪造管理标签或 payload。

模型出口切片另提供 `--ai-egress-socket /run/ingate/ai-egress.sock`。它指定下发给 Envoy 的本地出口 UDS 路径，不是 Controller 自己连接的路径，也不是公开入口。无模型目标时可以省略；存在有效模型目标却未配置时，AI xDS 发布会报错并重试，HTTP 与 ConfigMap 输出仍独立尝试。目录权限、Go/Envoy 身份与后续 Go 启动配置必须按同一数据面部署对齐。本参数只配置出口，不能单独启动尚未装配的 AI Proxy；实现与未验证范围见[模型操作与部署设计](design/ai-model-operations-and-deployment.md#受限模型出口目标绑定与两路发布)。

启动后的对象检查使用同一个 kubeconfig：

```sh
kubectl --kubeconfig "$INGATE_KUBECONFIG" get configmap \
  ingate-shared-data-plane-ai --namespace ingate-system -o yaml
```

对象存在只证明 API 中存在配置，不能据此认定 Proxy 已加载、Envoy 已 ACK 或请求已可转发。Controller 的 xDS readiness 不覆盖这些 AI 条件。

## 失败与维护边界

- 参数缺失或 Namespace 名称不合法：Controller Manager 配置阶段失败。
- Namespace 不存在或正在删除：AI 配置写入失败，协调错误交给工作队列重试；Controller 不补建 Namespace。
- 同名 ConfigMap 不具备约定的 managed-by 标签：报告冲突，不静默接管。已有受管对象的 owned key 被修改或对象被删除时，Controller 按完整输入重新协调。
- 共享 ConfigMap 写入失败：保留旧对象；HTTP 发布与 AI 写入不是事务，不能据此承诺即时撤销或同时切换。
- 控制面 Namespace 被删除：Namespace 生命周期会清理其中的内容。这是整个控制面的维护操作，不能把它当作某个租户 Gateway 的删除方式；恢复仍由部署者创建 Namespace。

安装身份需要能够创建/读取 Namespace，Controller 身份需要相应资源读写能力。当前没有完成部署身份与资源授权验收，managed-by 标签不是权限控制；本清单不新增或假定一套 RBAC 策略。API/监听地址的暴露、TLS 和多副本选主也不由本说明补齐。

## 实现与验证依据

[核心存储](../internal/apiserver/registry/core/storage.go)直接复用上游 Namespace/ConfigMap REST 实现；[启动参数](../internal/controllermanager/options.go)明确要求配置空间；[写入器](../internal/controllermanager/controller/gateway/ai_config.go)检查 Namespace 存在及删除状态，并拥有共享 ConfigMap 的配置 key。

本轮只增加初始化清单与说明，不修改 Go 运行逻辑、API 类型或存储，也不执行上述命令创建外部对象。交付时只核对 YAML 结构、代码/文件链接、命令参数与空白；真实安装、权限、Manager Watch 和 etcd 写入仍未验证。
