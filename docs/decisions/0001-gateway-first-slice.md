# 0001：Gateway、Route、Upstream 首轮资源契约

- 状态：实施中，资源契约待最终评审

## 本轮范围

Ingate 使用 Gateway、Route 和 Upstream 描述网关配置。Gateway 定义对外监听入口，Route 负责请求匹配与转发，Upstream 定义后端地址。同一组 Envoy 可以承载多个 Gateway；资源数量不决定 Envoy 的部署数量。本轮只定义资源契约，不涉及数据面的部署和扩缩容。

Gateway、Route、Upstream 使用 `gateway.ingate.io/v1alpha1`，按集群级资源建模。`metadata.name` 是稳定标识；Gateway 的 `spec.displayName` 供人阅读，不参与引用或匹配。一条 Gateway 可以声明多个 Listener。第一阶段只接受 HTTP、端口和精确 Host；同一端口上的空 Host 与任何 Host 冲突，不同的精确 Host 可以共用端口。

Route 指向一个 Gateway 的一个 Listener，并把匹配的请求转发到一个 Upstream。首轮只支持完整路径与按路径段匹配的前缀（`/api` 不匹配 `/apix`），以及转发行为。Upstream 保存后端地址、端口及相对权重；可被多条 Route 引用。Route 和 Upstream 的引用存在性、监听入口匹配关系及跨资源冲突，不由单个资源的字段校验函数推断，留给接入存储和发布流程时处理。三种资源的 `status.conditions` 预留给 Controller 记录观测结果，具体 Condition 及其确认边界在实现发布流程时确定。

本轮不定义 `enabled`、HTTPS、证书引用、健康检查、复杂路由匹配、重定向或直接响应。它们都需要对应的校验与数据面处理，不能只增加字段。

## 后续验收

Gateway、Route 和 Upstream 已接入 Generic API Server 的基础存储。下一步由 Controller 汇总这些资源，通过 xDS 发布到数据面。编译共享配置时需要保留 Route 对 Gateway 和 Listener 的引用关系，并处理监听与路由冲突。xDS Cache 写入、Envoy ACK 和实际可承载流量是不同阶段；状态字段的含义必须随发布实现明确，不能在本轮预先等同。

资源能够保存还不能证明网关业务成立。需要用实际 HTTP 请求验证 `Gateway → Route → Upstream` 的转发路径。Admin API、控制台、TLS 和治理策略不属于这条最小链路。

本决策中的字段选择和阶段划分仍待最终评审，不能视为已经定稿的产品契约。
