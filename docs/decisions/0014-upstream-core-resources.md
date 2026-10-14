# 0014：直接复用 Kubernetes 原生核心资源实现

- 状态：方案与实现已通过用户 review，已授权本地提交
- 日期：2026-10-13
- 基线：`e3ced8e0`
- 参考：OneX `10119504e12c316569337b0dbca04fb192c7d7ea`，Kubernetes `v1.37.0`

源码依据为 OneX [核心存储装配](https://github.com/onexstack/onex/blob/10119504e12c316569337b0dbca04fb192c7d7ea/internal/registry/core/rest/storage_core.go)、[依赖锁定](https://github.com/onexstack/onex/blob/10119504e12c316569337b0dbca04fb192c7d7ea/go.mod)，以及 Kubernetes [核心安装](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/install/install.go)、[Namespace 存储](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/namespace/storage/storage.go)和 [ConfigMap 存储](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/configmap/storage/storage.go)。核对的是固定版本本地源码，不声称已验证 OneX 运行组件。

## 原因与范围

Namespace、ConfigMap 要求标准 Kubernetes 行为。此前为避免主仓库依赖而移植类型、校验、Strategy 与存储，导致这些上游语义需要由 Ingate 重复维护。用户已确认改用 OneX 的原生存储复用方式；本决策替代 0013 和 AI 编译配置文档中“不依赖 Kubernetes 主仓库”的取舍，不改变资源作用域和业务契约。

直接调用上游 Namespace、ConfigMap 的 NewREST，使用上游核心内部类型、转换、默认值、校验及 Table 打印器。删除被替代的本地核心 API 和 Registry 实现，不保留类型别名、转发包装或第二套校验。

API Server 仍拥有私有 Scheme，显式调用上游 core/install.Install 并注册 Ingate Gateway 类型。上游安装包会初始化其内置 legacyscheme，原生 Strategy 使用它；这是第三方核心实现的初始化边界，不向其中注册 Ingate 业务类型。使用完整核心类型注册不等于安装其全部 REST API：存储映射、资源启用和 Discovery 仍只开放 Namespace、ConfigMap 与原有网关资源。

OneX 的 Namespace 删除协调器在项目内适配 controller-runtime，并非全部直接引用上游。本步保留现有 Manager 协调器及删除内核，保留读取 Ingate 自身的 NamespaceLifecycle 准入数据源。不启用 NamespaceAutoProvision，不创建系统空间，不添加 Secret、Service 等 API，不修改认证授权、HTTP/xDS、MySQL 或 AI 配置编译。

## 依赖与维护责任

主仓库版本为 v1.37.0，相关发布模块保持 v0.37.0。只补入实际导入图需要的模块，不复制 OneX 的全部 replace。主仓库不属于官方支持的外部库；每次升级必须核对配套模块、构建、存储契约和原生 API 行为，不能视为普通无痛升级。

隔离探查补入 component-helpers、controller-manager、kubelet 三个配套模块后，两类上游存储可编译，核心编码往返成功，Gateway 注册未进入上游全局 Scheme。探查中的第一次测试因错误使用 IsUnversioned 返回值而编译失败；修正测试后重新执行通过，不将失败记录为验证成功。这个结果不证明 HTTP 存储或完整生命周期已经通过。

## 行为差异与验证目标

HTTP 资源路径、外部 core/v1 类型、存储版本、Namespace 范围、版本冲突、dry-run、字段保护及 finalizer 删除防护应保持。内部 Go 类型改为上游类型，不改变对外 JSON 契约，也不操作运行中的 etcd 数据。

上游 Namespace DELETE 会补写调用方传入的 DeleteOptions/Preconditions，不保留本地实现的参数深复制保证；直接调用测试按上游契约调整，HTTP dry-run 仍必须不写入存储。发生传播 finalizer 调整时，上游可能重新排列 metadata.finalizers；它们按集合表达，不承诺顺序。Table 使用上游标准列，错误说明文字与校验路径沿用上游。不能将这些差异称为逐字节无变化。

上游 ConfigMap 默认值还会将缺省 Data 初始化为空 map；此前本地副本遗漏了此项。首轮存储 Codec 用例因此失败，更新明确的默认值预期并增加缺省/非空 Data 断言，不使用 EquateEmpty 混淆 nil 与空 map。

上游 Strategy 包含原生声明式校验，直接调用 Store 的测试必须提供 RequestInfo 中的请求版本及子资源，不能只有 Namespace 上下文。首轮直接存储测试因缺少此信息失败；补齐与 HTTP 框架一致的信息后，筛选、status/finalize 字段保护、冲突、dry-run 和删除保护用例通过，没有关闭 Feature Gate 或另写校验绕过框架。

保留项目现有真实 HTTPS、隔离 etcd 的 Namespace/ConfigMap 契约测试，以及装配失败的资源清理测试。测试不能因内部实现被删除而一起丢失行为断言。Scheme 测试区分类型可识别与 API 已安装，Discovery/OpenAPI 和资源启用测试继续防止意外开放其他核心资源。

## 审查与验证记录

已同时检查工作区状态、已跟踪差异和未跟踪文件，逐文件审查手写改动及删除实现的替代路径。先按 GO-FUNC-012 检查声明顺序，再检查职责、签名、错误路径、注释和 GO-FUNC-013/014 的语义分段。项目存储及 HTTP 契约测试保留；随本地副本删除的纯上游实现细节测试不再重复维护。筛选不匹配、版本与 UID 冲突、dry-run 不落盘、子资源字段保护和业务 Scheme 隔离均保留明确断言。

本次已通过以下检查：

- `gofmt -l cmd internal pkg` 无输出；`git diff --check` 及两个未跟踪文件的独立空白检查通过。
- `golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1` 和 `go build ./...` 通过。
- `go test -tags=integration ./internal/apiserver -count=1` 通过，使用真实 HTTPS 入口和隔离 etcd；额外的 integration 标签 lint 通过。
- `bash scripts/generate-api.sh` 通过，剩余 29 个项目生成文件与生成前哈希一致。首次执行被共享缓存写入权限阻止，获准后原命令成功；未将失败尝试计为通过。

未执行真实 Manager 的 Namespace 内容删除联动、Envoy/AI Proxy 联调或部署权限验证；未单独验证上游 Table 响应的全部列。上述结果不等于模板级或生产就绪结论。2026-10-14 用户确认本地提交并继续下一小步；未授权推送或创建 PR。
