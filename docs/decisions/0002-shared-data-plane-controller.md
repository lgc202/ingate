# 0002：共享数据面的资源汇总与裁决

> 历史决策保留当时的 Route/Upstream 契约；2026-10-10 的 HTTPRoute/Backend 替换和行为变更见 [0011](0011-http-resource-contract.md)。

> 2026-10-19：用户确认增量编译与资源级增量下发方案并授权开始实现，见[控制面增量设计](../design/control-plane-incremental-design.md)；代码按阶段 review。本页全量协调仍描述当前编译路径，不再作为长期计算方式；共享输出所有权、现有裁决和最终一致性边界保留。

- 状态：已确认裁决策略，分步实施中

## 背景

Gateway 定义 Listener，Route 引用 Gateway 的 Listener 和 Upstream，Upstream 定义后端地址。目前 API Server 已分别保存三种资源，但单个资源的写入不能证明跨资源引用成立，也不能判断多个 Gateway 是否争用同一个监听入口。它们最终由同一组 Envoy 执行，因此数据面只有一份需要统一发布的期望配置。

## 协调边界

一个 Gateway Controller 观察 Gateway、Route、Upstream。三种资源的变更都进入同一个固定队列键；一个 Worker 每次从已同步的 Cache 列出三种资源，按稳定顺序重新计算完整配置。启动时主动入队一次，资源全部删除时也发布空配置。Reconciler 不保存跨次调用的可变配置，不自行启动轮询 goroutine。

```text
Gateway ──┐
Route ────┼──> Gateway Controller ──> 完整数据面配置 ──> xDS 发布
Upstream ─┘
```

这个 Controller 按「独占修改共享 xDS 配置」划分，而不是按 API 资源数划分。以后出现证书签发、Envoy 工作负载编排等独立外部状态时，可以增加对应 Controller，但不能让多个 Controller 分别修改同一份 xDS 配置。

三次 Cache List 不构成跨资源事务。一次计算可能读到短暂的新旧版本组合；相关事件会再次入队并收敛。构建和发布都以完整配置为单位，不把单个资源事件直接翻译成对已发布 xDS 资源的原地修改。系统性读取或资源构建失败时保留上一版并重试；`go-control-plane` 的 `SetSnapshot` 在通知已有 Watch 前就替换内存配置，因此若通知报错，不能断言 Cache 仍是上一版，但 Reconcile 会重试该完整配置。「写入 xDS Cache 成功」不等于 Envoy 已 ACK 或开始承载流量。

controller-runtime 通常建议按对象做小范围 Reconcile。这里选择全量重算，是因为多个资源共同决定一份共享配置；在没有构建耗时和资源规模数据前，不增加反向索引或增量缓存。后续由实际指标判断是否需要改变构建方式。

## 校验与引用

API Server 继续负责单个资源的字段和元数据校验，不在写入请求中查询其他资源。Controller 根据本轮列出的资源检查以下关系：

- Route 引用的 Gateway、Listener 和 Upstream 是否存在；Listener 必须属于该 Gateway。
- 不同 Gateway 的 Listener 是否占用相同端口且 Host 范围重叠。空 Host 与该端口所有 Host 重叠；两个相同的非空 Host 重叠。
- 同一 Gateway、Listener 下，两条 Route 的匹配类型和路径值是否相同。PathPrefix 的尾斜杠不参与比较，`/api` 与 `/api/` 是同一规则；Exact 保留尾斜杠。完整路径与前缀路径是不同匹配；其它优先级由后续路由编译明确排序。

跨资源问题不倒退为 API Server 的创建错误，因为合法的创建顺序可以先有 Route、后有被引用资源。每轮重新计算引用，不保存需要与资源变化同步维护的反向索引。

## 裁决策略

只排除受影响的资源，继续用其它有效资源生成一份完整配置：

- Gateway Listener 冲突时，按创建时间、名称稳定排序，先出现者优先；被拒绝的 Gateway 整体不参与配置，其 Route 也不能参与。
- Route 引用缺失时，只排除该 Route；Gateway 和其它 Route 继续参与。引用补齐后，下一轮自动接纳。
- 两条 Route 匹配条件完全相同时，同样按创建时间、名称决定优先者，只排除后者。
- Upstream 本身由 API Server 保证字段有效；只有被有效 Route 引用的 Upstream 进入数据面配置。

这些裁决只决定本轮的接纳结果，不直接修改 API 对象。完整配置成功写入 xDS Cache 后，Controller 为 Gateway 和 Route 写回 `Accepted` Condition；未被接纳的资源写入对应 Reason，引用补齐或冲突解除后恢复为 `Accepted=True`。`Accepted` 只表示资源进入本轮共享配置，不表示 Envoy 已 ACK 或开始承载流量。

Upstream 目前仅由单资源校验保证字段有效，没有独立的跨资源拒绝结果。已有的 Status 字段和子资源暂时保留，但 Controller 不写入 Condition；未被 Route 引用不表示 Upstream 出错。跨资源错误不会阻断其它有效资源，系统性读取或构建失败时保留上一版。Status 写回失败会使 Reconcile 重试；重试时只更新发生变化的 Condition，不重复写入相同状态。配置发布与多个 Status 更新之间没有跨存储事务，短时间内可能看到部分 Status 滞后。

## 代码边界与实施顺序

第一步实现无 I/O 的引用解析和冲突裁决，并用输入顺序变化、引用缺失、冲突及恢复用例验证结果稳定。接着引入与当前 Kubernetes 依赖对应的 controller-runtime Manager，注册三种资源的 Watch、首次入队和单 Worker 全量协调。最后定义不含版本化 API 对象和 Envoy protobuf 的数据面配置，由 Envoy 适配层构建 LDS、RDS、CDS、EDS 并一次发布；用真实请求验证 `Gateway → Route → Upstream`。

当前已实现引用解析、冲突裁决、端口与 Host 汇总、三种资源 Watch、首次入队，以及完整配置的 xDS Snapshot Cache 发布。`dataplane.Config` 按端口组织 Host 和 Route，不包含版本化 API 对象或 Envoy 类型。Controller 在裁决和编译时统一去除 PathPrefix 的尾斜杠（根路径 `/` 除外），因此 `/api` 与 `/api/` 冲突并生成相同的路径段匹配；`/api` 不匹配 `/apix`。适配层为每个端口生成 LDS Listener 和 RDS RouteConfiguration；精确路径排在前缀路径之前，较长的前缀优先。纯 IP Upstream 使用 CDS + EDS；包含域名的 Upstream 使用 CDS 的 STRICT_DNS 和内嵌地址分配，因为普通 EDS 地址不能依靠 Envoy 自动解析域名。所有资源类型分别按确定性内容计算版本，单独修改 Endpoint 不改变 LDS、RDS、CDS 版本。发布空集合会删除旧资源；发布成功只表示 xDS Cache 更新，不代表 Envoy 已 ACK。

ADS gRPC Server 已实现为 Manager Runnable：`Start` 才监听端口，取消时停止活动流并等待服务退出，就绪检查要求已监听且首份配置已发布。`cmd/ingate-controller-manager` 用 `--kubeconfig` 或 `--master` 连接 API Server，并要求通过 `--config` 指定 MySQL 查询模型连接；controller-runtime Manager 统一运行 Gateway Controller、资源 MySQL 投影 Controller、ADS、健康探针和指标服务。MySQL 投影的读取边界见 [0005](0005-admin-resource-management.md)。当前已写回 Gateway 和 Route 的接纳状态，但尚未用 Envoy 进程验证真实请求；构造测试、protobuf、Snapshot 和服务生命周期单元测试都不等同于真实代理验证。后续需要验证 API Server → Controller → ADS → Envoy 的完整请求链路，并在有 Envoy ACK 依据后再讨论是否增加「已生效」状态。

当前未配置 Leader Election，按单个 Controller Manager 实例部署。启用主备之前，需要确认 API Server 提供 Leader Election 所需的 Lease 资源和权限，并验证备用实例接管后的 Envoy 重连行为；`NeedLeaderElection()` 本身不等于已经启用主备。

当前 ADS 使用明文 gRPC，`--xds-bind-address` 默认监听所有网卡的 `:18000`。这只适用于隔离的可信网络；在提供可跨网络访问的部署模板前，还需确定 xDS 服务认证与传输加密方式，不能把当前默认监听方式直接暴露到公网。

实现时才创建 `cmd/ingate-controller-manager`、`internal/controllermanager/controller/gateway`、`internal/controllermanager/dataplane` 和 `internal/controllermanager/dataplane/envoy` 中实际需要的文件，不预建空包或通用 Builder、Checker 接口。

参考：[controller-runtime 的 Manager、Watch 与 Reconcile 约定](https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/doc.go)、[GoCanon 的共享数据面协调决策](https://github.com/lgc202/gocanon/blob/main/docs/decisions/0023-global-data-plane-reconciliation.md)。
