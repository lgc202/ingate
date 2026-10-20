# AI 编译配置：沿用官方编译分层，使用标准 ConfigMap 保存非敏感结果

> 历史方案：本文记录 ConfigMap/JSON 阶段的契约与实现。当前运行通道已由[控制面增量设计](control-plane-incremental-design.md)替代，Controller 不再写入派生 ConfigMap，Proxy 不再通过 Kubernetes API 读取配置。下文旧路径和参数只用于理解当时决策，不作为当前启动或开发指引。

- 状态：契约已于 2026-10-13 经用户确认；原生 ConfigMap API、内部编解码、纯编译及写入边界已提交，完整读取、受限 Watch 与协调发布接线已提交为 `fcc9650b`，模型数据面已提交；MCP 现代透明代理在本轮接通，当前边界见文末
- 作者：Ingate 开发协作
- 日期：2026-10-13
- Ingate 基线：`cda0478e`，分支 `codex/rebuild-from-zero`
- 上游源码：Agent Router 固定提交 `daa9f891a8afcb18576d4593ad18870d1d18abac`
- 讨论入口：[资源契约](ai-gateway-resource-contract.md)、[配置与凭据草案](ai-credentials-and-runtime-config.md)
- 范围：非敏感配置的载体、字段、编译、存储和删除；首个代码切片只安装原生 ConfigMap API

## 摘要

采用官方的分层：Controller 解析领域资源，生成独立于 Kubernetes 类型的强类型配置；运行层只加载编译结果，不在请求中查询 ModelRoute、AIBackend 或 Backend。

Ingate 的模型和 MCP 都进入 Go HTTP Proxy，而官方模型使用 ExtProc，所以不能直接复制官方整个配置。我们保留官方按路由规则绑定后端、schema 和模型覆盖的语义，补入 Go 必须消费的模型匹配；MCP 只编译已确认的单后端和路径，不带入官方的聚合、工具过滤、CEL 或身份插件。

载体为一份标准 `core/v1 ConfigMap`，对应现有共享数据面；内部 payload 使用 JSON。它只保存当前完整期望配置，不保存密钥，也不声称一次写入使 Go 与 Envoy 原子切换。代码阶段先完成编译与存储，不发布 AI 入口，不实现 Proxy Watch。

用户已确认普通 HTTP 路由不进入这份配置。只被 HTTP 引用或无人引用的 Backend 也不下发给 Go；同一个 Backend 同时被 HTTP 和 AI 使用时，分别进入 xDS 与 AI 输出。为共享入口裁决读取普通路由，不等于将它们序列化给 Proxy。

## 背景：官方哪些设计可以直接借鉴

以下依据均来自固定提交的实际源码，不用当前 main 代替它。官网和 Kubernetes 文档属于读取日的可变资料。此次没有运行上游组件，也没有重新核验 release tag。

| 官方实现与依据 | Ingate 采用范围或必要差异 |
| --- | --- |
| [filterapi.Config](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/filterapi/filterconfig.go)：协议配置与 Kubernetes 类型分离 | 沿用这个边界；不直接依赖上游 `internal` 包，也不另建公开业务资源 |
| [Gateway 编译](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/gateway.go)：每个路由规则的 backendRef 生成独立绑定，附带 schema、modelNameOverride | 同一 AIBackend 被两个规则使用不同 override 时，仍生成两个绑定；不能只按 AIBackend.name 去重 |
| [派生 HTTPRoute](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/ai_gateway_route.go#L255)：模型 Header 条件和网络目标进入 Envoy 路由 | 我们由 Go 选择模型，所以必须把这些匹配编入 Go 配置；这是统一 HTTP Proxy 的必要适配，不是增加新的业务路由类型 |
| Config 的 Models/ModelsByHost/UnscopedModels 用于模型发现 | 不用这几个列表冒充模型匹配表；首步不增加 `/models` 接口 |
| [MCPConfig](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/filterapi/mcpconfig.go)与 [MCP 请求消费](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/mcpproxy/handlers.go)：路由/后端标识关联 Envoy 本地出口 | 沿用已解析目标绑定；本次不写死本地端口、socket 或内部 Header，也不复制多后端功能 |
| [配置分片写入器](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/controller/filter_config_bundle.go)及 [加载器](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/filterapi/watcher.go)：分片与索引 Secret，校验内容后加载 | 官方不是一份无限大的配置文件；我们先限定单 ConfigMap，超限拒绝，不提前照搬分片和 kubelet 文件投射 |
| 官方 Backend.Auth 可以包含编译后的实际凭据 | 不能把整个官方 Config 放入 ConfigMap；本稿没有 auth 字段，凭据另按 Secret 安全方案评审 |

当前 Ingate [共享协调器](../../internal/controllermanager/controller/gateway/controller.go)已接入六种领域资源，分别编译 HTTP/xDS 与 AI/ConfigMap 输出；[核心存储](../../internal/apiserver/registry/core/storage.go)已安装 Namespace 与 ConfigMap。编译存储接线已通过用户 review，但尚未验证真实 Manager → API Server/etcd 链路，没有 Secret 存储或 AI 转发能力。

## 设计：配置载体与所有者

固定对象名 `ingate-shared-data-plane-ai`，固定 key `config.json`；Namespace 由部署配置显式提供，不固定为某个租户空间、不自动创建。本文使用 `ingate-system` 作为示例值，不是已有 Namespace。对象名对应当前唯一的 `ingate-shared-data-plane`，不增加可由用户编辑的 dataPlaneId。

控制面 Namespace 由安装流程或部署者通过 Ingate 原生 API 预先创建；Controller Manager 只创建和更新其中的 ConfigMap，Namespace Controller 不承担初始化职责。[初始化清单与手动顺序](../control-plane-initialization.md)已提交为 `6979b0c5`，尚未执行真实安装。

一个共享协调器拥有该对象的 `data["config.json"]`；API Server 仍提供标准 ConfigMap 原生接口，没有 AI 配置 Admin CRUD、MySQL 投影表或 ConfigMap status。其他身份不得修改该 key；权限部署必须另行验证，`managed-by` 标签本身不是授权。建议标注 `app.kubernetes.io/managed-by: ingate-controller-manager` 用于诊断，已有同名对象若不是约定的受管对象，应报冲突而不是静默接管。

这里的单一写入者也包含进程边界：首步只运行一个活动 Manager。`MaxConcurrentReconciles=1` 只约束单进程，resourceVersion 也不能防止两个进程用各自陈旧输入反复覆盖结果。多 Manager HA 要使用框架标准 Leader Election 并确认其存储/部署前置能力，不在本稿假设 Ingate 已安装 Lease 或实现自制选主。

六种业务资源仍在自己的 Namespace 内解析引用。中央配置包含不同 Namespace 的编译结果，是共享数据面的汇总，不代表 ModelRoute 可以引用其他空间的 AIBackend。不能把整个共享 ConfigMap 的 ownerReference 设为其中一个 Gateway，也不能依赖跨空间 ownerReference 实现回收。控制面 Namespace 的删除和维护权限必须受限。

ConfigMap 标准只校验其自身字段，不会理解字符串里的路由表；payload 的结构、引用和 revision 由编译器及未来加载器校验。采用 [Kubernetes ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/) 的非敏感配置语义与 1 MiB 数据限制，不自建第七种业务 Kind。非敏感不等于可公开：网络地址和资源关联仍应只让必要身份读取。

![编译配置的边界与保存流程](../diagrams/ai-compiled-config.svg)

### payload 的具体字段

内部强类型配置放在 [internal/pkg/aidataplane](../../internal/pkg/aidataplane/types.go)，与服务启动配置 `conf.Bootstrap` 区分，不导入 Kubernetes API 类型；这是 Controller 与未来 Proxy 的共享数据契约，不属于任一组件的内部实现，也不创建空服务。沿用仓库的内部共享包目录，不增加 infra 分类层：本包定义数据契约，不承担基础设施适配。核心组 ConfigMap 存储仍在 `internal/apiserver/registry/core`，编译与写入留在现有共享协调器中，不按六种资源各建一个配置写入者。

| 字段 | 类型与有效值 | 来源、消费与边界 |
| --- | --- | --- |
| formatVersion | 必填字符串，首版 `v1` | 内部序列化格式，不是软件发布版本或 OpenAI 版本；未知值拒绝加载 |
| revision | 必填，64 位小写十六进制 SHA-256 | 当前有效配置的内容标识；不是 UID、metadata.resourceVersion 或 xDS version_info |
| listeners | 必填数组，可空 | 只包含附着了模型/MCP 匹配的已接纳 Listener；普通 HTTP 不进入这份 payload |
| listeners[].port / hosts | 端口与 Host 数组 | 沿用现有 HTTP 编译布局；汇总相同端口，保留不同有效 Host |
| hosts[].name / hostname | 编译标识 / 有效 Host，hostname 可空 | name 沿用 `gateway/{namespace}/{name}/listener/{listener}`；空 hostname 沿用当前无 Host 限制的语义，不新增 wildcard 能力 |
| hosts[].modelRoutes | 必填数组，可空 | 每项对应一条已展开的模型 match；Controller 已确定优先级，运行层按序选择首个命中项 |
| modelRoutes[].name | 必填编译标识 | 沿用 HTTP 编译习惯：`{namespace}/{route}/rule/{i}/match/{j}`；索引用于绑定和诊断，不是数据库 position 列 |
| modelRoutes[].headers | 必填数组，0 或 1 项 | 0 项表示所有模型；1 项固定 `name=x-ai-eg-model,type=Exact`，value 保留原值。它匹配 Go 提取的模型，不信任客户端同名 Header |
| modelRoutes[].backendName | 必填字符串，可空 | 非空引用下面的 backends；空字符串表示保留匹配并拒绝，不是匿名、兜底或动态解析 |
| backends | 必填数组，可空 | 沿用官方按路由规则生成的协议后端绑定；不是重新暴露 AIBackend CRUD |
| backends[].name | 必填编译标识 | 借鉴官方 [PerRouteRuleRefBackendName](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/internalapi/internalapi.go#L143) 的 `{ns}/{aiBackend}/route/{route}/rule/{i}/ref/0`；消费者按完整字符串查找，不解析字符串重建领域引用 |
| backends[].schema | 必填对象：name、prefix | 仅已确认的 `OpenAI`；prefix 保存有效默认值 `/v1` 或显式原值，不开放官方其他 schema/version |
| backends[].modelNameOverride | 必填字符串，可空 | 空值保留请求模型；非空只覆盖该绑定的出站模型名。沿用官方语义 |
| backends[].networkBackendName | 必填字符串 | 引用 networkBackends；为 Ingate 显式区分协议与网络的必要字段，不让 Go 自建 DNS/端点池 |
| hosts[].mcpRoutes | 必填数组，可空 | 只含当前单后端 MCPRoute，不含工具/提示词过滤、聚合或会话配置 |
| mcpRoutes[].name / path | 资源作用域标识 / 有效精确路径 | name 为 `{namespace}/{route}`；path 省略值已编译成 `/mcp`，保留尾斜杠 |
| mcpRoutes[].backendName / backendPath | 网络 Backend 编译标识 / 有效上游路径 | backendName 直接引用 networkBackends，空值拒绝；backendPath 已默认化为 `/mcp` 或保留显式路径，不从请求拼任意 URL |
| networkBackends | 必填数组，可空 | 只保存被有效模型绑定或 MCP 路由引用的网络快照；不是另一个领域资源，也不包含所有闲置 Backend |
| networkBackends[].tls | 可选对象；省略或 null 表示 HTTP | 存在时 sni 为固定 DNS 身份，wellKnownCACertificates 为 System；原样参与 revision 和模型/MCP 内容目标哈希，只由 Envoy 执行 TLS，不交给 Go 建连接 |
| networkBackends[].name / endpoints | `{namespace}/{backend}` / 非空端点数组 | address、port、weight 沿用现有 Backend 契约及排序；用于后续重建同 revision 的 Envoy 出口，不由 Go 负载均衡 |

所有数组输出 `[]`，不输出 null。除表中允许的空字符串外，标识、路径与引用不能为空；非空引用必须在本份配置内存在，标识在对应集合内唯一，端口与端点满足现有校验。加载层应拒绝未知字段、重复 JSON 成员、缺少必填字段和非法枚举；不能仅依赖 `DisallowUnknownFields` 就称为已检查重复成员和必填项。原生 ConfigMap API 不因此新增一套通用 JSON 校验。

不输出 displayName、领域 status、对象 resourceVersion、完整 metadata、请求数据、密钥、Token 或密钥哈希。排序需要的创建时间只在 Controller 输入中使用。配置中的内部 name 是已有资源绑定信息，不再添加一套用户必须管理的 ID。

### 完整示例：领域引用在编译后不再需要查询

以下为构造示例，不是仓库已有对象或真实提供商。业务 Namespace `team-a` 和控制面 Namespace `ingate-system` 都需预先存在；示例明文内网端点只解释编译关系，不表示允许用明文访问真实模型账号。

```yaml
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
metadata: {namespace: team-a, name: public}
spec:
  listeners:
    - {name: api, protocol: HTTP, port: 8080, hostname: gateway.example.com}
---
apiVersion: gateway.ingate.io/v1alpha1
kind: Backend
metadata: {namespace: team-a, name: model-network}
spec:
  endpoints:
    - {address: model.internal.example, port: 8081, weight: 1}
---
apiVersion: gateway.ingate.io/v1alpha1
kind: AIBackend
metadata: {namespace: team-a, name: model-service}
spec:
  backendRef: {name: model-network}
  schema: {name: OpenAI, prefix: /v1}
---
apiVersion: gateway.ingate.io/v1alpha1
kind: ModelRoute
metadata: {namespace: team-a, name: coding}
spec:
  parentRefs: [{name: public, sectionName: api}]
  rules:
    - matches:
        - headers: [{name: x-ai-eg-model, type: Exact, value: coding}]
      backendRefs: [{name: model-service, modelNameOverride: code-v2}]
---
apiVersion: gateway.ingate.io/v1alpha1
kind: Backend
metadata: {namespace: team-a, name: tools-network}
spec:
  endpoints:
    - {address: tools.internal.example, port: 8082, weight: 1}
---
apiVersion: gateway.ingate.io/v1alpha1
kind: MCPRoute
metadata: {namespace: team-a, name: tools}
spec:
  parentRefs: [{name: public, sectionName: api}]
  path: /mcp
  backendRefs: [{name: tools-network, path: /stream}]
```

对应的完整输出如下。revision 根据本稿下述字段顺序和紧凑 JSON 算法计算；展示缩进不参与哈希。ConfigMap 创建时不填写 resourceVersion，更新时使用 API 返回的原值。

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: ingate-system
  name: ingate-shared-data-plane-ai
  labels:
    app.kubernetes.io/managed-by: ingate-controller-manager
data:
  config.json: |
    {
      "formatVersion": "v1",
      "revision": "29e071533c8e5e4bb2f5c78460aeacd14bad01ffe4e7ee0a5450e5ab23a3cc0a",
      "listeners": [{
        "port": 8080,
        "hosts": [{
          "name": "gateway/team-a/public/listener/api",
          "hostname": "gateway.example.com",
          "modelRoutes": [{
            "name": "team-a/coding/rule/0/match/0",
            "headers": [{"name": "x-ai-eg-model", "type": "Exact", "value": "coding"}],
            "backendName": "team-a/model-service/route/coding/rule/0/ref/0"
          }],
          "mcpRoutes": [{
            "name": "team-a/tools",
            "path": "/mcp",
            "backendName": "team-a/tools-network",
            "backendPath": "/stream"
          }]
        }]
      }],
      "backends": [{
        "name": "team-a/model-service/route/coding/rule/0/ref/0",
        "schema": {"name": "OpenAI", "prefix": "/v1"},
        "modelNameOverride": "code-v2",
        "networkBackendName": "team-a/model-network"
      }],
      "networkBackends": [{
        "name": "team-a/model-network",
        "endpoints": [{"address": "model.internal.example", "port": 8081, "weight": 1}]
      }, {
        "name": "team-a/tools-network",
        "endpoints": [{"address": "tools.internal.example", "port": 8082, "weight": 1}]
      }]
    }
```

未来运行层只需：在可信 Listener 绑定中匹配 `model=coding` → 查协议绑定 → 得到 OpenAI、`/v1`、`code-v2` 和网络目标；MCP 命中 `/mcp` → 得到 tools-network 和 `/stream`。这里没有增加中间网络调用。`model=other` 没有命中且没有 catch-all，拒绝；`/mcpx` 不命中 MCP 精确路径。凭据、入口操作、内部传输与协议错误尚未实现，这份配置本身不能使请求可用。

### 编译、排序与内容版本

一次协调沿用共享 Gateway/Listener 裁决及同空间引用，得到原有 HTTP/xDS 配置与本稿 AI 配置。保留原有 Gateway、HTTPRoute、Backend 的读取、发布及状态路径；再补读 AIBackend、ModelRoute、MCPRoute。公共输入读取失败时两份结果都不更新，AI 专属输入读取失败只停止 AI 更新，不阻止 HTTP 路径完成。AI 结果必须基于六类完整输入，不能把读取失败当成空集合。多次 List 不是跨资源事务；短暂引用不一致通过重协调收敛，不增加自行实现的 etcd 事务协议。

HTTP 与 AI 输出分别执行并报告失败；某一路失败不回滚另一路已成功的发布，也不抑制另一路能完成的状态更新，最后汇总错误交给框架重试。HTTP status 继续只在其原有发布条件满足后写回；AI status 只报告自身可证明的解析结果。这是新增输出需要遵守的隔离边界，不新增独立协调器或后台发布任务。

领域 Watch 覆盖六类资源的 spec 变化和进入删除状态/最终删除，不能只依赖 Generation 变化处理撤销。受管 ConfigMap 的创建、删除、所属 key 或受管标签变化也应入队对账；仅监听显式 Namespace/name，不监听全部配置作为输入。ConfigMap 的内容更新没有业务 spec Generation 语义，不能直接套用当前全局 GenerationChangedPredicate；自己的幂等写入最多触发一次无写回的对账，不循环制造版本。

ModelRoute 的 OR matches 展开为多项 modelRoutes；每项 headers 仍是 AND。省略或空 matches 生成 `match/0` 且 headers 为 `[]`；显式空 match 同样为 catch-all。一条规则中同时有精确 match 和空 match 时分别排序，不能让 catch-all 排在其他资源的精确匹配之前。同一规则的各项引用同一个协议绑定，多个父 Listener 复用该绑定。

先按模型精确匹配优先于 catch-all 排序，再按资源创建时间、Namespace、name、rule 索引、match 索引稳定排序；可选 rule.name 不参与排序。保留较低优先级候选，不因后端失效改变排序。MCP 相同精确路径的同类候选沿用创建时间、Namespace、name 顺序，运行层取首个；这是已确认的编译契约，尚无 MCP 运行实现。跨 HTTP/模型/MCP 的完全相同网络匹配仍按后文的激活前置项处理。

listeners 按端口排序；hosts 沿用接纳 Gateway 与 Listener 声明顺序；MCP 路由按上述元数据顺序；backends 与 networkBackends 按完整 name 排序，endpoints 沿用现有确定排序。内容中没有无序 map。缺失引用的匹配保留，backendName 输出空字符串；只有有效且被引用的后端进入后端集合，不能输出悬空的非空名字。

建议 revision 算法为：稳定排序后的强类型值移除 revision 字段 → Go 标准 `encoding/json.Marshal` 紧凑编码、不附加换行 → SHA-256 → 小写十六进制。字段声明顺序固定为上面完整 JSON 示例顺序；revision 紧随 formatVersion，但计算时不输出。它不是对任意语言 JSON 文本的通用规范化算法，加载层按同一强类型契约重编码核对，不直接 hash 展示文本。以 golden case 固定编码结果，改字段顺序或格式需显式评审。

status、resourceVersion、displayName 和闲置 Backend 的改变不进入哈希；有效匹配、顺序、模型覆盖、schema、路径或网络目标改变才影响 revision。普通 HTTP 的无关变更不会改变 AI revision。内容标识不提供身份认证，授权与受保护传输仍是必要条件。

### 写入、失效和删除

Controller 先完整编译并校验，核对显式控制面 Namespace 存在且未进入删除/终止，再 Get/Create/Update 受管 ConfigMap；这是写入者保护，不声称原生准入会拒绝终止空间内的所有 Update。相同 key 内容及所属标签不写回；更新使用原生 resourceVersion，Conflict 或 Create 的 AlreadyExists 交由标准重试重新读取，不关闭版本校验。只维护所属 key 和约定标签，保留其他合法 metadata/data；按完整对象 data 与 binaryData 的总数据大小核对标准上限，API Server 仍作权威校验。若已有 immutable ConfigMap 阻止所需内容更新，不强删重建，明确报告发布失败。

| 输入或故障 | 编译/保存行为 | 后续加载要求与不能承诺的部分 |
| --- | --- | --- |
| 路由未成功附着 | 不生成该 Listener 的匹配 | 不凭名字猜测其他 Gateway |
| AIBackend 或 Backend 缺失/正在删除 | 保留仍附着的模型/MCP 匹配，backendName 为空；移除无有效使用者的后端 | 命中后拒绝，不跳到下一候选或普通 HTTP 兜底；响应 body/状态由协议切片确定 |
| 删除一条 ModelRoute/MCPRoute | 删除其编译匹配并重新汇总 | 该入口是否回到现存 HTTP 兜底，是删除路由的行为影响，不等于永久路径封禁 |
| 最后一条 AI 路由删除或其 Gateway 删除 | 仍写完整空配置：listeners/backends/networkBackends 均为 `[]`，重算 revision | 空配置必须清空旧视图，不能以 nil/空内容跳过加载 |
| ConfigMap 被删除 | Controller 对账重建当前结果 | 未来加载器收到确定删除后停止 AI 新请求；普通 HTTP 不受这份配置控制 |
| 某次领域 List 失败 | AI 所需输入不完整时不更新 AI 配置；HTTP 按其完整输入独立推进，最后报告错误供框架重试 | 保存旧配置不表示旧目标持续获授权；失联新鲜度/撤销另行定稿 |
| ConfigMap 超限、Namespace 不存在/终止、写权限失败、版本重试失败 | 不截断、不拆出半份结果，不另找 Namespace；报告写入失败 | 旧对象可能仍在，不能声称删除或撤销已生效；本阶段尚无运行消费者 |
| payload 格式、内容哈希或内部引用校验失败 | 编译端禁止发布；未来加载端禁止替换当前完整视图 | 初次加载失败不 ready；是否可暂留旧版及其期限由运行方案确定，禁止静默无限沿用 |

Namespace 生命周期应包含标准 ConfigMap，不增加一个「AI ConfigMap Controller」代替原生资源删除。控制面 Namespace 中的 ConfigMap 不进入六种领域资源的 MySQL 投影。AIBackend 的 BackendRef、ModelRoute 的 AIBackendReference 与 MCPRoute 的 MCPBackendReference 都在源对象的 Namespace 内解析；本轮同步修正内部/外部类型与 OpenAPI 中遗留的“集群级”说明，不改变现有解析行为。

持久化成功只证明 API Server 接受了当前对象，不证明 Proxy 已加载、Envoy 已 ACK 或真实服务可调用。本阶段不新写“运行成功”的 Ready，不在 ConfigMap 添加全局 loaded/status。领域 Conditions 如需扩展，只报告实际完成的附着和引用解析，不把写入配置当成数据面成功。

## 理由与取舍

| 选择 | 为什么采用，以及代价 |
| --- | --- |
| 标准 ConfigMap，而非 AIProxyConfig Kind | 减少公开类型、生成和存储维护；代价是 API Server 不做 payload 字段级校验，必须由生产者/消费者执行 |
| 一份共享配置，而非每个业务 Namespace 一份 | 与当前共享 Listener 裁决和 Envoy 输出所有者一致；代价是共享数据面拥有汇总读取范围，不提供租户进程隔离 |
| JSON，而非照搬官方 YAML | 复用 Go 标准序列化、显式字段和确定性内容版本；外层仍是标准 ConfigMap YAML 示例。此前 config.yaml 只是占位示例，不是已确认契约 |
| 独立协议绑定，而非只按 AIBackend 汇总 | 两个规则可以对同一服务使用不同模型覆盖；绑定数量随实际规则增长，但没有新增业务实体 |
| 保存网络快照，但 Go 不连接这些端点 | 未来可重建匹配 revision 的 Envoy 出口，不从当前可变 Backend 猜旧目标；少量重复数据换取明确版本边界 |
| 首步单对象、只保存当前版本 | 一次原生写入不会出现分片拼装混合；代价是 1 MiB 规模界限，且不能仅凭此实现并行双版本或无损更新 |

不复制官方所有分支，也不照搬个别直接赋值的运行配置交换方式。未来加载器先构建不可变视图，完整校验后同步交换；新请求捕获单版，不能一边匹配旧路由一边查最新网络目标。这是运行期必要约束，不在本轮增加 goroutine、Watch 或并发抽象。

## 兼容性与实施

这是新能力，没有旧 AI 配置兼容要求。格式 `v1` 的标识是解码契约，不是承诺保留旧方案。原有 HTTP 编译内容、接纳状态语义、业务资源与管理查询保持现有行为；协调接线新增必填控制面 Namespace 参数，Reconcile 也可能返回 AI 路径错误。本稿替代凭据草案中的“先建空载体、以后定 payload”实施顺序；AIProxyConfig、双版本和本地健康准入仍是历史候选/后续议题，不成为本轮新增资源的授权。

按小步推进，每步审查全部手写改动，等待用户 review 后再提交：

1. 原生 ConfigMap API：复用项目锁定 Kubernetes 版本的官方校验、转换和 Generic Store，核对 CRUD/List/Watch、resourceVersion、immutable、1 MiB 和 NamespaceLifecycle；本步已通过用户 review 并获准本地提交，不生成空 Proxy 服务、不新增依赖。
2. 内部强类型配置与编解码校验：按本稿 payload 实现，不导入 Kubernetes API 类型；生产端类型、结构/引用校验、确定性 revision、空配置与严格解码已实现，已通过用户 review 并获准本地提交。检查必填/未知/重复 JSON 成员及传入 revision，不以普通 json.Unmarshal 冒充完整加载校验。
3. 在现有共享协调器接入 AI 依赖解析、确定排序、完整编译与 ConfigMap 幂等写入。证明精确/catch-all、同名多 Namespace、不同 override、失效目标、全量删除、写冲突和超限不会被误当成功；核对受管对象漂移与删除状态 Watch。不改变 HTTP/xDS 内容及既有错误/状态行为，验证 AI 专属失败不阻止 HTTP 更新。
4. 编译存储完成后，再按已批准的协议和运行边界实现 Proxy 消费与 Envoy AI 入口/出口；不能因 payload 已存入 etcd 就提前开放真实流量。

前三步只提供配置存储和编译能力。原生 API 存储契约变更仍执行仓库要求的隔离 integration；真实 Manager/Envoy/Proxy 联调按用户指定的统一测试阶段进行，不混淆两者。

第三步已提交纯编译和 ConfigMap 写入边界；完整输入读取、受限 Watch、两路发布错误隔离与运行接线已通过用户 review 并获准本地提交，范围见下文。

### 接流量前必须另行确定的契约

这些项不要求先实现再保存配置，但未确认前阻止相应运行代码：

- 首批模型操作及 MCP 版本/SDK 的具体采用范围；本配置不添加 operation/provider 插件字段，`schema=OpenAI` 不自动开放所有接口。
- 普通 HTTP、模型与 MCP 完全相同的网络匹配如何裁决；不能在本次偷偷按 Kind 字典序决定，也不能抢占现有 HTTP 路径。
- Envoy → Go 的可信入口标识、Go → Envoy 的受限目标关联；公网标识必须清除后重写，不能允许客户端选择任意后端。2026-10-15 已确认[最终一致性取舍](ai-model-operations-and-deployment.md#配置更新最终一致按请求目标关联)，不把每请求整份 revision 一致作为准入条件，具体目标绑定仍待确定。
- 跨组件热更新、目标变更与删除、在途请求及重启恢复。本稿只存当前期望配置，未采用此前双版本发布建议；保存 networkBackends 不表示已冻结或保留旧 Envoy 出口，也不保证旧请求始终连接旧网络目标。
- 服务身份/授权、Secret 与政策的正式契约、新鲜度及撤销/排空期限。TLS 首版已按[资源契约](ai-gateway-resource-contract.md#上游-tls-首版2026-10-16本地实现待-review)接入本地源码，真实握手与部署仍未验收。超限或控制面断联时可能保留旧对象，尤其不能承诺即时撤销；首步不接真实凭据或匿名调用真实提供商。

## 原生 ConfigMap 实现记录

2026-10-13 迁移补充：用户已确认按 [0014](../decisions/0014-upstream-core-resources.md) 直接复用上游核心实现，并于 2026-10-14 完成 review、授权本地提交；此前移植 API 已提交为 `e3ced8e0`。payload、写入与后续 Proxy 边界不变。

此前 API 切片不创建示例 ConfigMap 或 Namespace，不接 Controller 写入，不修改 HTTP 编译或转发，也不新增 Admin API、MySQL 表或 Secret。本轮仅增加 0014 指定的上游依赖，其他边界保持。标准 ConfigMap 不包含 Spec/Status，不维护业务 Generation；原生 API 允许无条件 Update，后续配置写入者仍必须携带 resourceVersion 防止覆盖并发修改。

| 本地实现 | 官方来源与适配边界 |
| --- | --- |
| 内部类型与标准 core/v1 注册 | 直接使用 Kubernetes [v1.37.0 core/types.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/types.go)，在私有 Scheme 安装上游核心类型，不因此安装其它资源 REST API |
| 字段校验与写入策略 | 直接使用上游 [validation.go](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/apis/core/validation/validation.go) 与 [ConfigMap Strategy](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/configmap/strategy.go)；不维护本地副本，不解析 payload |
| [核心存储装配](../../internal/apiserver/registry/core/storage.go) | 直接调用上游 [ConfigMap storage.NewREST](https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/registry/core/configmap/storage/storage.go)，包括标准选择器及 Table 打印器 |
| Scheme、转换与 DeepCopy | 原生核心产物来自锁定版本依赖，Ingate 不再生成副本；`scripts/generate-api.sh` 只维护自有 Gateway 产物，调用方仍使用标准 core/v1 客户端 |

不增加第三方源码目录；主仓库锁定为 v1.37.0，公开模块保持 v0.37.0。保留 [KUBERNETES-LICENSE](../KUBERNETES-LICENSE)，因为 Namespace 删除内核等仍有受控移植实现。本轮构建与存储验证结果见 0014；下文记录的是此前移植切片的验证，不代替迁移验证。

## 内部配置类型与编解码切片

2026-10-14，原生核心资源迁移已提交为 `c2eac979`。后续小步新增 [types.go](../../internal/pkg/aidataplane/types.go)、[encode.go](../../internal/pkg/aidataplane/encode.go)、[decode.go](../../internal/pkg/aidataplane/decode.go) 与 [validation.go](../../internal/pkg/aidataplane/validation.go)，仅实现本稿共享编译数据契约，已通过用户 review 并获准本地提交。经用户确认，共享契约从 AI Proxy 组件目录迁入 internal/pkg/aidataplane，包名为 aidataplane；保留 JSON 字段与内容版本，不创建 conf 或服务入口。Controller 的领域编译和 ConfigMap 写入、Proxy 的加载及运行视图交换仍留在各自组件，不移入共享包；当前没有生产调用方，不保留旧路径兼容包。

Encode 要求调用方提供已默认化、已排序的 v1 强类型配置；检查非 nil 数组、集合内标识唯一性、协议/匹配枚举、路径、端口/权重、重复端点及非空引用存在性，再按固定字段顺序紧凑编码、排除 revision 计算 SHA-256，重新编码携带内容版本。传入旧 revision 不参与计算、不写回调用方；模型名、prefix 和 MCP 尾斜杠保持原值。编码器不排序、删路由或自动填默认值，空 backendName 的失效匹配仍保留。域内附着与引用解析、优先级、只汇总被 AI 使用的后端由后续编译器负责，不能由编码校验反推出已完成领域裁决。

已重新读取官方固定提交的 filterapi.Config 与 Backend/APISchema 原文，采用其类型分离和规则级协议绑定；不复制 Models、Auth、CEL 等未确认能力。DNS 校验复用已有发布模块的 Kubernetes validation 工具，不复制 DNS 正则，不增加模块依赖，也不导入核心/网关 API 类型。

[golden 用例](../../internal/pkg/aidataplane/testdata/v1.json) 使用缩进方便 review；测试读取后用标准 json.Compact 仅移除展示空白，再核对字段顺序、字面量和 revision，不重新序列化无序 map。空配置另有固定哈希。测试覆盖重复编码、输入不变、保留输入路由顺序、有效内容改变时 revision 改变，以及非法枚举、数组为 nil、标识重复、悬空引用、无效路径和端点的反例。失效目标/空模型条件用完整对象比较，名称反例同步调整引用，避免因另一项引用错误使名称校验失效仍被误判通过。

Decode 复用已有 sigs.k8s.io/json 的大小写敏感解码、重复字段和未知字段检查；同时处理普通解析错误与额外 strictErrors，不因解析成功就忽略严格检查结果。不编写通用 JSON 解析器、反射校验框架或自定义字段 DSL。模块版本不变，仅从间接依赖调整为直接依赖，go.sum 不变。

强类型语义校验拒绝必填字段的非法零值及 nil 集合。允许空字符串的 hostname、两类 backendName 和 modelNameOverride 在原配置类型中使用 *string：nil 表示缺失/null，指向空字符串的值合法。只保留这一份字段定义，编码端和解码端共用 validate；删除函数内的嵌套匿名字段投影和第二次 JSON 解码。Controller 构造时必须显式提供这四个值，不能靠 Go 字符串零值隐式补齐；后续消费者使用通过校验的指针值，不再重复判空。该调整不改变 JSON 字段、合法输出内容或 revision，当前也没有需迁移的生产调用方。不增加解码 DTO、反射校验框架或自定义 JSON 方法。

本次修正针对字段定义重复造成的维护成本，而不是仅将匿名结构体改名。以原有合法输出、必填字段缺失/null 反例和空字符串正例验证行为；生产端另验证四个字段为 nil 时拒绝发布。通过结构校验后，检查 revision 的长度、小写十六进制格式与重编码哈希；编解码共用 contentRevision，不对原始文本直接求哈希。失败返回 nil，不交付部分配置；成功结果属于本次调用方，没有全局缓存或运行视图交换。

解码测试覆盖 golden/空配置往返、缩进/对象成员顺序/等价转义不改变语义，示例中每种对象逐字段缺失/null，以及嵌套未知字段、同值重复字段、转义后重复键、类型错误、尾随 JSON、非法 UTF-8 与 revision 篡改。四种允许为空的必填字符串另有编解码往返正例，确保不把合法空值误当作缺失。字段和语义反例先补上剩余强类型内容的正确哈希，避免仅凭哈希不匹配让错误实现也通过；revision 缺失/null 单独验证。另检查严格错误的 FieldError 原因保留，以及两次解码结果和原始字节之间没有共享可变状态；所有权用例修改指针指向的字符串，而不只是重新赋一个指针。

此处没有文件加载、ConfigMap 写入或 Proxy 服务。ConfigMap 对完整对象的大小、所属 Namespace、权限与并发版本校验仍留在后续写入切片。原有 HTTP/xDS、API Server 与查询投影未修改。

本切片已核对状态、已跟踪差异及全部未跟踪文件，按 GO-FUNC-012/013/014 检查声明顺序与语义分段，并逐函数检查职责、命名、签名、错误保留及注释。`gofmt -l cmd internal pkg`、已跟踪与八个新增文件的空白检查、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1` 和 `go build ./...` 均通过。辅助命名与错误脚本通过；辅助注释脚本将没有单独包注释的 encode.go、decode.go、validation.go 报为缺失，人工确认包注释在 types.go 唯一定义，不复制注释来消除该误报。

未修改公开 API 类型、存储契约或生成源，所以本切片未再次生成或运行 API integration；迁移提交前的隔离 integration 已于本日复跑通过。没有运行 Manager/Envoy/Proxy 联调，也没有证明配置已写入 etcd；新代码尚无生产调用方。GoCanon MCP 来源与快照仍为 `/Users/lgc202/workspace/source/lgc202/gocanon`、`08260b7-dirty`，本契约不属于现有组件或 Spec/Status 资源生成器的生成范围。

## Controller 纯编译切片

内部编解码已提交为 `23f22057`。本切片在现有 gateway 包增加 [ai_compile.go](../../internal/controllermanager/controller/gateway/ai_compile.go)、[modelroute.go](../../internal/controllermanager/controller/gateway/modelroute.go) 与 [mcproute.go](../../internal/controllermanager/controller/gateway/mcproute.go)，领域输入仍用 resources。compileAI 将完整输入编译后调用 aidataplane.Encode，返回已校验且带 revision 的 JSON；不新增编译接口、协调器、发布任务或依赖。

沿用现有 selectGateways 与 compareMeta 的 Listener 裁决及稳定元数据顺序。AI 输入中正在删除的 Gateway 不再占用 Listener；正在删除的路由不编译，正在删除或缺失的目标保留空 backendName 匹配。协议绑定沿用官方的路由规则/ref 标识，同规则多 match、多个父 Listener 共用绑定，不合并不同规则的 override。非敏感网络结果仅包含有效使用者；端点按现有 Envoy 的 address、port、weight 顺序复制排序，不修改输入。Header 名统一小写，省略 Type 使用已确认的 Exact；模型值、显式 prefix 和 MCP 尾斜杠保持原值。

[纯编译测试](../../internal/controllermanager/controller/gateway/ai_compile_test.go) 从本稿已有领域示例生成共享 golden，核对输出字节与 revision，而不新增示例业务。另覆盖精确/catch-all 排序、OR 展开、父引用重叠去重、多规则 override、同名多 Namespace、共享端口、输入/端点排列不影响版本、同空间引用失败、目标缺失/删除时保留高优先级匹配、路由/Gateway 删除后的完整撤销、单后端约束、非法 payload 与路径错误原因保留。比较完整结果或目标顺序，不仅检查输出数量。

纯编译切片提交时，readResources、Reconcile、Watch、HTTP 发布和状态行为均未接入 AI 路径；入口只由测试调用，不能把 resources 中新增的 AI 字段视为已经完成读取。该切片不解决跨 Kind 网络入口裁决，不写入 ConfigMap，也不宣称配置已经保存到 etcd。后续写入边界和协调接线的实现见下文，跨组件激活仍需前文列出的运行契约。

本切片已按 GoCanon 的 Controller 规范草案及已确认编译契约，逐文件检查声明顺序、职责、签名、错误原因与注释，再独立复查语义空行。按用户 review 将资源快照变量命名为 snapshot，并沿用项目 Go 1.27 的 new(expr) 创建已初始化的值指针，保留 ptr.Deref。`gofmt -l cmd internal pkg`、已跟踪与四个未跟踪 Go 文件的空白检查、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1` 和 `go build ./...` 均通过。没有改动公开 API、存储或生成源，不重复生成和 API integration；未运行真实 Manager/Envoy/Proxy 联调。纯编译切片已通过用户 review 并获准本地提交。

## ConfigMap 写入边界切片

纯编译入口已提交为 `63bf1ba3`。`ai_config.go`（旧实现，已移除） 只保存 compileAI 成功返回的 JSON，使用原生 client.Client 写入和 client.Reader 读取；reader 后续由 Manager.GetAPIReader 注入。不新增 Publisher 接口、独立协调器、存储框架或客户端生命周期。

Namespace 显式提供，不设置缺省空间、不自动创建。空字节拒绝，撤销必须传完整空配置。写入前核对 Namespace 存在且未进入删除/Terminating；已有同名 ConfigMap 若无约定的 managed-by 标签，报告带对象身份的 Conflict，不接管。该标签不是服务身份或权限证明。ConfigMap 正在删除、所属 key 被 binaryData 占用、immutable 阻止修改时报告失败，不删除重建。

仅更新 data["config.json"]；保留其他 labels、annotations、ownerReferences、finalizers、data 和 binaryData，携带读取到的 resourceVersion。内容相同时不 Update，包括相同内容的 immutable 对象。完整对象的 data 与 binaryData 值字节总量不得超过原生 1 MiB 上限；API Server 仍作权威校验。Create 的 AlreadyExists、Update 的 Conflict 和读写根因均向调用方返回，由后续 Reconcile 重读并重算，不在写入器内部重试。Namespace 检查与 ConfigMap 写入不是事务，检查不消除并发删除窗口。

`写入边界测试`（旧实现，已移除） 使用标准 fake client 和 interceptor，核对创建/完整撤销、无关字段保留、原 resourceVersion、Context、独立读取依赖、幂等与 immutable、不接管、不自动创建 Namespace、不强删重建、完整对象 1 MiB 等于/超出边界及 API 错误原因保留。这些测试不替代真实 API Server 的授权、immutable、并发版本和存储验证。

写入边界提交为 `48d75133`。该切片没有装配到 Reconcile，也没有新增启动参数、Watch 或 AI status；提交时只有测试调用写入器，不能声称运行路径已经将配置保存到 etcd。协调接线属于下一节的独立切片。

本切片逐文件审查两份新增手写 Go 文件和文档差异，检查声明顺序、职责、Context 传播、错误原因、Namespace/所有权与字段保留；按用户 review 补齐成功返回与测试辅助函数的语义分隔，读取与错误检查保持相邻，子测试的 Fatal 使用本次子测试对象。`gofmt -l cmd internal pkg`、已跟踪及未跟踪文件的空白检查、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1` 和 `go build ./...` 均通过。不修改 API 类型、存储实现或生成源，未重复生成或运行 API integration，也未运行真实 Manager/Envoy/Proxy 联调；这些检查不证明授权、etcd 写入或加载完成。用户已完成 review 并授权本地提交。

## 共享协调器接线切片

[Controller](../../internal/controllermanager/controller/gateway/controller.go) 先读取两路共有的 Gateway、Backend，再独立读取 HTTPRoute 和 AIBackend/ModelRoute/MCPRoute。共有输入读取失败时两路均不发布；HTTPRoute 读取或 HTTP 发布/状态写回失败时，仍尝试 AI 编译与写入。AI 专属读取、编译或写入失败不阻断本轮 HTTP 发布。任何输入读取失败都不把部分结果当成完整空配置发布；旧配置保持到一次成功更新。各次 List 并非跨 Kind 事务，输入变化依靠后续事件重新收敛。

两路顺序执行、分别生效，最终通过 errors.Join 向工作队列保留两路错误原因，由框架限速重试；不回滚已经成功的一路，也不增加内部重试循环。Gateway 与 HTTPRoute 的状态仍只按原有 HTTP 接纳逻辑写回，HTTP 发布失败不写接纳状态，不增加 AI Ready/Programmed 状态。xDS readiness 也不代表 AI 配置已保存或 Proxy 已就绪。

[Options](../../internal/controllermanager/options.go) 新增必填 `--ai-config-namespace`，不提供默认空间、不自动创建；即使尚无 AI 资源，也必须指定该参数。它指向 Ingate API 中预先创建的 Namespace，不是 API Server 所在宿主集群的空间。[Manager](../../internal/controllermanager/manager.go) 通过 GetClient 注入缓存领域读取和原生写入，通过 GetAPIReader 注入配置对象的实时读取；按标准 Cache.ByObject 将 ConfigMap 的 List/Watch 限定为此 Namespace 下 `metadata.name=ingate-shared-data-plane-ai`，其他 Kind 的作用域不收窄。构造阶段不创建 Namespace 或 ConfigMap。

`事件过滤`（旧实现，已移除） 对六种领域资源保留创建、Spec Generation 更新、删除开始和最终删除，忽略 Status 与不影响代次的 metadata。displayName 属于 Spec，改名仍可能触发协调，但不进入编译结果，不改变内容 revision。ConfigMap 不使用 Generation 过滤，也不用 managed-by 标签作为 Cache 选择器，以免标签漂移后丢失事件。ConfigMap 只关心所属 payload、管理标签、binaryData 占用、immutable 和删除状态；无关字段或纯 resourceVersion 变化不入队。自身 payload 写入可能额外入队一次，后续幂等写入会跳过 Update。删除对象后按当前完整输入重建，未授权对象仍报告冲突而不接管。

所有事件映射到既有共享键，保留启动主动入队与单 Worker。上述错误隔离适用于 Reconcile 已开始执行后：Manager 构造可能通过发现查询 ConfigMap 作用域，Controller 启动必须等全部 Watch Cache 同步；某一资源的发现、List/Watch 权限或初始同步不可用仍可能阻止共享 Controller 启动，不能把两路错误隔离说成组件完全独立可用。当前只支持单 Manager 副本，未新增选主、服务身份或授权策略。

[协调测试](../../internal/controllermanager/controller/gateway/ai_reconcile_test.go) 核对完整模型/MCP 输入、HTTP 输出不变、重复写入、删除路由后的完整撤销、ConfigMap 删除后重建、六种资源读取失败、编译错误、HTTP 发布/状态失败、AI 写入失败及两路根因保留；`事件测试`（旧实现，已移除） 包含错误 Namespace、错误对象名及不应触发的更新。[Manager 测试](../../internal/controllermanager/manager_test.go) 验证标准 Cache 选择器能排除其他名称，不修改调用方缓存配置，也不收窄领域资源范围；构造测试使用标准 RESTMapper，不依赖本机 API Server。

本切片已通过用户 review 并获准本地提交；测试采用 fake client、interceptor 与标准对象/选择器。没有修改 API 类型、存储实现、生成源或依赖。尚未运行真实 Manager → API Server/etcd 的事件与存储链路，也未验证 RBAC、故障后的撤销时限或多组件激活，不接 AI 流量。编译存储接线确认后，下一步先明确 Proxy 消费与 Envoy 入口/出口的运行契约，再开始对应服务实现。

已逐文件审查当前新增和修改的手写文件，按 GO-FUNC-012 核对声明顺序，再检查读取完整性、发布失败隔离、根因保留、注释与测试反例，最后独立复查函数签名和语义空行。`gofmt -l cmd internal pkg`、已跟踪及三个未跟踪 Go 文件的空白检查、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1` 和 `go build ./...` 均通过。首次包级测试曾因构造参数尚未全部更新以及 RESTMapper 触发本机发现查询而失败，修正测试接线后重新验证通过；另一次包级复跑被沙箱拒绝回环端口监听，获准重跑的全量测试通过，失败不记作成功。没有改动 API 存储契约，未重跑 API integration 或生成命令；以上检查不替代实际事件、授权和存储验证。

## Proxy 配置消费：已确认语义与加载器切片

本节只确定配置消费，不扩展模型/MCP 接口、Envoy 入口/出口、Secret 或服务部署。用户已确认以下消费语义，加载器与 Factory 替换已提交为 `5bf59e98`；前文已确认的 payload 契约不变。

官方固定提交 `daa9f891a8afcb18576d4593ad18870d1d18abac` 的 [watcher.go](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/filterapi/watcher.go)读取分片文件：首次加载仅对校验和不一致、分片尚未出现这两种错误继续等待，其他初次加载错误返回；运行中加载失败会记录错误，并在下一轮重试，不调用成功替换路径。模型 [Server.LoadConfig](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/extproc/server.go#L80)先构造完整运行配置，成功后才替换，因此失败时旧配置仍保留。

不能因此声称官方完整实现了删除撤销：它的文件加载失败不主动清空运行配置；[MCP ProxyConfig.LoadConfig](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/mcpproxy/config.go#L253)还会在 MCPConfig 为 nil 时直接返回。我们不照搬这种 nil 分支，也不照搬源码中的无同步指针赋值。下表区别了「拒绝坏更新」和「确认对象已删除」。本轮核对的是此前取得的固定源码，不代表重新核验 main 或运行了官方组件。

| 输入或事件 | 已确认消费行为 | 依据与行为影响 |
| --- | --- | --- |
| 初次成功 List 未找到目标 ConfigMap | 没有可消费配置，等待后续创建 | Controller 与 Proxy 启动顺序不能决定是否永久失败；不把对象缺失编造成合法空 payload |
| 初次存在对象，但 key 缺失、内容非法或格式不支持 | 拒绝该对象，仍没有可消费配置；等待修正 | 复用严格 Decode，不保留半份结果；观察到对象不等于加载成功 |
| 相同对象的合法更新 | 完整解码和校验成功后，一次替换私有快照 | 沿用官方先构建再替换；不能边更新路由边更新后端 |
| 相同对象的非法更新或 key 缺失 | 不替换上一份有效快照；报告加载失败，等待有效更新 | 沿用官方保留上次有效值的取舍；这可能延迟撤销，不能宣称最新配置已生效 |
| 已确认的删除，或成功重新 List 确认对象不存在 | 清空可消费快照，等待重建 | Ingate 必要差异；确定的缺失不能继续解释为一次普通解析失败。Controller 重建对象前会有无配置窗口 |
| 同名对象被新 UID 的对象替代 | 先放弃旧对象快照，再验证新对象；新内容非法时保持无配置 | 使用标准 Kubernetes 对象实例身份，不跨实例保留旧配置；不向 payload 或数据库新增 UID 字段 |
| 合法的完整空配置 | 替换旧快照，保留零路由的有效配置 | 这是已经确认的显式撤销，不与未加载、JSON 空字节或对象缺失混为一谈 |
| List/Watch 断连、超时、Forbidden 等请求失败 | 不据此推断对象删除；由原生 SDK 重连/重读，保留之前有效快照 | 通信失败不是成功查询到「不存在」；持有旧配置不证明其仍新鲜，接流量前必须另行确定失联策略 |

managed-by 标签与内容哈希都不是身份认证。加载器不从标签猜测服务权限，也不用标签筛选 Watch 以掩盖同名对象变化。来源信任依赖明确的 Ingate 连接与访问权限；本切片不创建身份或凭据管理能力。

resourceVersion 只由原生 SDK 用于 List/Watch 连续性和事件处理，不转整数、排序或当作内容版本；UID 用于对象实例识别，revision 用于已校验内容识别。重复同内容事件可以复用快照，但不能忽略新 UID、删除或失效事件。旧实例的延迟删除通知不得清空新实例的已加载配置。停止加载器时清空本实例可消费状态，并等待受管处理结束。

配置加载成功只说明存在一份可解析的期望配置，不代表 Envoy 已安装同 revision 的出口、API 连接健康或 Proxy 可以接流量。[锁定版本的 SharedInformer 契约](https://github.com/kubernetes/client-go/blob/v0.37.0/tools/cache/shared_informer.go)也明确 HasSynced 只说明缓存收到过完整 List，甚至不代表单个处理器已经同步；不拿它替代 payload 校验、新鲜度或数据面就绪。失联后是否继续承接新请求以及截止时间，仍是接流量前必须确认的运行契约，本轮不设置猜测性的 TTL。

本代码切片采用已有 client-go 的标准 ConfigMap List/Watch 与 SharedInformer：限定 Namespace 和对象名、使用已有 aidataplane.Decode、完整快照替换与撤回、绑定运行 Context 的停止。消费者只读派生配置，不重新 Watch 六种领域资源，不写 API 对象，不导入 Controller 包，也不增加 ConfigReceiver 接口或自制 Watch 重试循环。快照由加载器私有持有，不向调用方暴露可变 map/slice；消费接口按实际调用设计，不先建立 Provider 或路由索引框架。

该切片不新增模型/MCP Handler、HTTP 监听、运行二进制或完整服务骨架；先做可独立审查的加载边界，不让未确定的流量协议决定配置消费。原生 List/Watch、严格解码和快照替换并不等于完成跨组件激活；组件装配与流量接入仍按后续已确认契约实施。

实现审查至少验证：首次缺失后创建、坏更新不会覆盖旧快照、成功空配置撤销、删除后清空与重建恢复、新 UID 的坏对象不沿用旧实例、错误 Namespace/名称不参与加载、断连不误清空、错误原因和取消传播。观察边界必须能区分读取失败与成功无对象。不得仅检查事件数或 HasSynced 为 true。

### 加载器实现与边界

[aiconfig.Loader](../../internal/aiproxy/pkg/aiconfig/loader.go) 位于 Proxy 内部，不依赖 Controller。`NewLoader(apiClient, namespace, logger)` 仅校验并保存原生客户端、显式空间与日志；`Run(ctx)` 同步运行一次，使用 [NewSharedInformerFactoryWithOptions](https://github.com/kubernetes/client-go/blob/v0.37.0/informers/factory.go) 获取 ConfigMap TypedInformer 并注册类型安全事件处理器，限定 Namespace、设置 `metadata.name` 精确选择器和零周期重同步，不设置标签选择器或自建重连循环。SDK 保留 Watch 请求中的 resourceVersion 等连续性参数；本包不对其排序或比较。

该 Factory 只管理目标 ConfigMap，先实例化 Informer 和注册处理器，再调用 `StartWithContext`。`WithTweakListOptions` 的条件作用于整个 Factory；不同筛选范围的资源不能直接复用当前固定对象名条件，应使用与其范围匹配的独立 Factory，不为尚未实现的资源预建监听。若在启动后才实例化其他 Informer，SDK 要求再次调用 Start，而不是自动开始监听。

`storage.go`（旧实现，已移除） 统一定义固定对象名与 data 键，Controller 写入、Manager Cache 和 Proxy 加载器引用同一契约。该文件不引入 Kubernetes 依赖，JSON 类型和内容哈希保持不变；managed-by 标签仍属于 Controller 的对象管理约定，没有搬成消费者鉴权条件。

单个 Informer 处理器串行消费事件，严格 Decode 创建独立配置，锁保护 UID 与完整快照的登记。相同实例的有效同 revision 可以复用旧快照，但新 UID 首先撤回旧值，非法的新实例不能借同名继承配置。SDK 在成功重新 List 发现缓存对象缺失时发出删除通知；List 请求失败不会走这条路径。类型安全删除参数仍可能只包含键而没有对象实例；缺少 UID 的通知记录错误，不根据名字清空其他实例，完整重新 List 的撤销路径已通过有缓存旧对象的反例验证。

`Run` 的 Context 传给原生 SDK 请求，Factory 异步启动后由 Run 等待取消；退出时先执行 `factory.Shutdown()` 等待 Informer 与事件处理器停止，再清空状态，不额外接管 SDK 内部请求的等待机制。Shutdown 只等待退出，不能代替 Context 取消；defer 的登记顺序保证先等待退出、后清空快照。正常取消原因通过 `%w` 返回，重复运行被拒绝，不用缓存同步替代配置有效或流量就绪。解码错误向事件处理边界返回并在该处记录一次；请求失败由 SDK 的 WatchErrorHandler 记录并继续 SDK 重试。本切片没有启动命令、HTTP 监听、运行组件装配、配置新鲜度探测或公开快照访问器；下一步按真实 Proxy 调用契约添加消费和生命周期接线，不把加载器的存在等同于 Proxy 已运行。

本轮重新查询 GoCanon 规则源 `/Users/lgc202/workspace/source/lgc202/gocanon`，revision `08260b72394a104ebe8286088eb23383cd6a6235`、版本 `08260b7-dirty`；完整读取开发入口和规范索引，沿用已读且来源未变的错误、防御、测试与 RUNTIME 草案，复核 GO-FUNC-012/013/014。Go 审查技能用于声明顺序、取消与等待、错误原因、反例断言及语义空行检查；用户确认的消费策略是本切片行为依据，不将草案称为稳定规则。

验证使用 Kubernetes fake client、原生 Informer 和 Go 标准 `testing/synctest`，不访问集群。测试覆盖首次缺失后创建、坏更新保留、有效空配置撤销、删除与重建、新 UID 的非法配置、旧 UID 的迟到删除、不同 Namespace/名称、失败的 Watch/List 与成功重读缺失、取消和错误根因。重读失败用例明确断言失败请求已执行，成功缺失用例也检查实际读取次数，不只比较事件数或缓存同步标志。首次 race 检查发现故障注入测试中的场景变量未同步，已改为锁保护和原子计数；后续检查结果按实际运行记录，不把首次失败计为通过。

没有改变 API 类型、存储实现、生成源或依赖，未执行真实 API Server/etcd/Manager/Envoy/Proxy 联调、RBAC 验证或模型请求。外部组件验证仍留待统一测试，本切片已通过用户代码 review 并获准本地提交，不推送或创建 PR。

已逐个审查全部新增与修改的手写文件，包括三个未跟踪 Go 文件；先检查声明顺序，再检查职责、作用域命名、签名、错误路径、实例身份与测试反例，最后独立复查换行和语义空行。`gofmt -l cmd internal pkg` 无输出；已跟踪及三个未跟踪文件的空白检查无诊断；`golangci-lint run ./...` 为 0 issues；`go vet ./...`、`go test ./... -count=1`、`go build ./...` 与 `go test -race ./internal/aiproxy/configloader -count=10` 均通过。新增本地文档链接已核对；这些结果不替代前述真实组件验证，也不表示整个仓库达到模板级或生产就绪。

## 同空间引用说明修正

2026-10-15，加载器提交后继续核对资源定义，修正内部与 v1alpha1 类型中六处过时的“集群级”引用说明。现有编译器已经使用源对象的 Namespace 解析 AIBackend、模型网络 Backend 和 MCP Backend；本轮只使注释与实现一致，不改变字段、校验、序列化格式或运行行为。已有编译测试同时覆盖其他 Namespace 的目标不可用，以及不同 Namespace 的同名资源分别解析。

使用 `scripts/generate-api.sh` 重新生成，OpenAPI 仅有三处 Description 变化，重复生成结果一致。首次生成因编译缓存读取失败中断，获准重试原脚本后成功。已逐个审查六份手写类型文件与文档，并独立复查声明布局和语义空行；`gofmt -l cmd internal pkg`、`git diff --check`、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1` 和 `go build ./...` 均通过。没有变更 API 存储契约，未重跑 integration 或真实组件联调；本轮修正已通过用户 review 并获准本地提交，不推送或创建 PR。

## AI Proxy 运行配置消费切片

2026-10-15 用户确认开始统一 AI Proxy，首步为 Chat Completions 普通响应与 SSE，以及同机 Envoy/Go 经 UDS 通信。按小步实施，本轮先接入运行配置和模型目标选择，不增加 HTTP Handler、监听端口或进程入口，不将上述范围一次展开成空服务。

参考固定提交 `daa9f891a8afcb18576d4593ad18870d1d18abac` 的 [filterapi.NewRuntimeConfig](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/filterapi/runtime.go) 和 [Server.LoadConfig](https://github.com/theagentrouter/agent-router/blob/daa9f891a8afcb18576d4593ad18870d1d18abac/internal/extproc/server.go)：独立构建请求使用的配置，成功后完整替换。此次重新读取此前取得的固定提交本地源码；远端 raw 文件获取失败，不声称核验了新的上游版本。只借鉴配置分层，没有移植源码、认证处理器、CEL 或 ExtProc 的 Processor 工厂；也不沿用其直接赋值运行配置的并发做法。

[aiconfig.Config](../../internal/aiproxy/pkg/aiconfig/config.go) 位于组件内部，与共享 JSON 契约 `internal/pkg/aidataplane` 和启动配置分开。包内 newConfig 复用严格 Decode，一次校验完整 payload 与 revision，再建立私有 Host 和协议后端查找表。查找键包含端口与 Host 的编译名称；现有契约只要求 Host 名在同端口集合内唯一，不能假设它在所有端口上全局唯一。原始配置、字段指针、切片与 map 不作为消费结果交付。

[Loader.Capture](../../internal/aiproxy/pkg/aiconfig/loader.go) 在读锁内捕获当前运行实例，尚未加载、确认删除、新 UID 的非法内容或停止后没有可供新请求使用的实例；合法空配置仍是有效实例。ResolveModel 只使用这份配置和已提取模型名，按编译顺序选择首个匹配；失效的首个命中返回 ErrModelBackendUnavailable，不继续尝试后续候选。不同规则的 schema、模型覆盖和网络引用保持各自绑定，返回值不含可变引用或网络端点。完整 Listener 绑定必须来自后续受保护的入口，不能把公网 Host 或客户端自报 Header 当作该身份；本轮尚未实现这一传输边界。

更新不修改已捕获实例；运行配置在状态锁外构建，单个 Informer 处理器串行更新，成功后在写锁内交换。Capture 的锁不跨越请求处理持有，不为每次请求复制完整 payload，也不重新查询领域资源。删除和停止使后续 Capture 失败，不取消此前持有旧实例的在途工作；真正的流取消、退出排空、旧视图期限和跨组件激活仍须随 HTTP/Envoy 切片确认。本轮不建立历史版本缓存或 Provider 接口，不承诺撤销即时生效。

测试覆盖完整配置保持、返回值与原始字节所有权、精确/空条件、编译顺序、失效首选不回退、端口/Listener/Namespace 隔离、不同规则绑定、严格错误原因、合法空配置，以及原生 Informer 首次读取后捕获、停止后撤回、坏更新保留、新 UID 撤回和并发捕获的一致性。原 Loader 用例仍比较完整运行内容，包括原 payload 的 MCP 与网络集合，不以只比较 revision 代替完整撤销验证。没有修改 API 类型、存储契约、JSON 字段或依赖，不重复生成或运行 API integration。

本轮按 GoCanon `08260b7-dirty` 工作流与用户确认的 GO-FUNC-012/013/014 审查，Go 审查技能用于只读边界、错误原因、反例和空行，technical-writing 用于区分本切片与完整 Proxy 能力。已逐文件审查全部新增与修改的手写代码，并独立复查声明顺序、函数签名与语义空行。`gofmt -l cmd internal pkg`、已跟踪与三个未跟踪文件的空白检查、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1`、`go build ./...` 与 `go test -race ./internal/aiproxy/... -count=10` 均通过。首轮 lint 发现一处 switch 前分隔遗漏，修正后重跑为 0 issues。尚未运行真实 API Server/etcd/Envoy/Proxy 联调、UDS 或模型请求，没有实现 Handler、SSE 或模型名 JSON 改写。本轮变更已通过用户 review 并获准本地提交，不推送或创建 PR。

## AI Proxy 配置包合并

2026-10-15 用户确认将 Proxy 内的 configloader 与 runtime 合并为 aiconfig，随后按 [0015](../decisions/0015-ai-proxy-package-layout.md) 迁移至 `internal/aiproxy/pkg/aiconfig`。两者共同完成动态 AI 配置的加载、发布与只读查询，没有独立消费者需要这条包边界；保留 Loader 和 Config 两个类型，分别放在 loader.go 与 config.go，不把监听逻辑和查询逻辑合成一个结构体。加载器构造函数命名为 NewLoader，配置构建函数 newConfig 收回包内，不保留旧包或兼容别名。

共享编译契约仍位于 `internal/pkg/aidataplane`，不引入 Kubernetes 依赖；aiconfig 不承担服务启动配置、HTTP 请求解析或上游转发。并包使 Loader 可以访问 Config 私有字段，因此只读约束适用于整个 aiconfig 包：配置只在发布前构建，更新时更换实例，不修改已交付请求的实例。本次只调整代码组织、标识符与文档引用，不改变 ConfigMap 筛选、UID 身份、错误保留、路由匹配或并发交接语义。此项合并已通过用户 review 并获准本地提交，不推送或创建 PR。

已逐文件审查手写代码和受影响引用，并独立复查声明顺序、错误检查相邻性及语义空行；原有不匹配、非法更新、对象替换、撤回和并发一致性断言保持不变。`gofmt -l cmd internal pkg`、`git diff --check` 及五个未跟踪 Go 文件的空白检查均无诊断；`golangci-lint run ./...` 为 0 issues，`go vet ./...`、`go test ./... -count=1`、`go build ./...` 和 `go test -race ./internal/aiproxy/... -count=10` 均通过。旧目录已为空并移除，未保留生产源码中的旧包引用。API 类型、存储契约与依赖未变，未重新生成或运行 API integration；未执行真实 API Server/Envoy/Proxy、UDS 或模型服务联调。下一步先 review 此项包合并，再继续独立的模型请求处理切片。

## 审查与验证边界

设计时的 GoCanon MCP 启动快照为 `336a025-dirty`。本次实施重新查询来源 `/Users/lgc202/workspace/source/lgc202/gocanon`，revision `08260b72394a104ebe8286088eb23383cd6a6235`，gitVersion `08260b7-dirty`；完整读取开发入口及规范索引，按 API Server、类型/声明/函数、错误、边界校验和测试规范审查，草案不等同于已接受规则。能力查询确认原生 ConfigMap 不适用业务 Spec/Status 资源生成器，因此复用仓库现有官方生成命令，而不是套业务模板。Go 审查技能辅助逐文件检查；technical-writing 用于区分确认契约、实现和验证边界。

设计阶段已解析示例的七个 YAML 文档与内层 JSON、复算 revision、核对内部引用和本地链接，并渲染检查 SVG。实现阶段逐个检查新增/修改的手写文件，先核对声明顺序，再核对职责、签名、错误与语义分段；转换和 DeepCopy 只核对生成结果及可重复性。隔离集成用例通过标准客户端、真实 HTTPS 和独立 etcd 验证 CRUD/Patch/DeleteCollection、名称/空间/标签筛选、Watch 不匹配反例、版本冲突和无条件更新、immutable、大小校验、Discovery/OpenAPI 与本地 NamespaceLifecycle 正反例。装配失败用例核对没有部分存储映射，并销毁已创建的 Namespace 存储。

`gofmt -l cmd internal pkg`、包含未跟踪文件的空白检查、`golangci-lint run ./...`、`go vet ./...`、`go test ./... -count=1`、`go build ./...`、`go test -tags=integration ./internal/apiserver -count=1` 均通过。额外的 `golangci-lint run --build-tags integration ./internal/apiserver` 发现原测试启动辅助函数遗漏响应 Body.Close 错误检查，补齐后重跑为 0 issues；常规检查及完整 integration 也已复跑通过。生成命令在受限缓存下失败，按原命令获准使用共享构建缓存后成功，重复生成的 31 个产物哈希一致。没有执行真实 Manager/Envoy/Proxy 联调、部署身份授权或自动 Namespace 删除联动；没有凭据或真实提供商验证。上述结果只覆盖本 API 切片，不证明 AI 编译、加载或转发已经可用，用户已完成 review 并授权本地提交。

## MCP 配置消费与入口激活（2026-10-16）

此前配置契约中的 MCPRoute、ResolveMCP、MCPTargetName 现已接入真实 Handler/Service、
组件装配和 Envoy 入口/出口。配置字段与格式不变：一次请求捕获一份配置，按端口、Host 完整标识、
精确路径取编译顺序首个候选；空 backendName 拒绝，不回退。热更新影响后续请求，已捕获请求
继续使用原路径及网络目标；确认删除仍由 Loader 撤回，坏更新沿既有规则保留有效配置。

首版为 2026-07-28 Streamable HTTP 的单 Backend 远程透明代理，具体协议、Origin 和验证范围见
[首版 MCP](ai-model-operations-and-deployment.md#首版-mcpstreamable-http2026-07-28)。
MCP 路径保留尾斜杠，精确入口优先普通 HTTP；所有非 POST 在该入口返回 405，不回退。
同 Host 同路径入口去重，但配置仍保留有序候选以表达失效首选拒绝。
同一 Listener/Host 实际启用模型路由时，MCP 与任一模型固定操作路径完全重叠会编译失败，
不会写入 ConfigMap 或发布该 AI 结果；已有有效结果保留。不含模型路由的入口不全局保留这些路径。

Envoy 对两类入口覆盖内部协议标记、端口和 Host，解决 Go 静态模型路径与动态 MCP 路径的分派；
这些字段只在受保护 UDS 上采信，转发时移除，不添加到公开产品协议或持久化 payload。
网络目标仍绑定内容，而非要求每请求整份 revision 相等；传播窗口中的旧目标只能失败，不能改投其他后端。

### TLS 可选字段与目标关联（2026-10-16）

格式仍为 v1，networkBackends 的可选 `tls` 对象包含必填 `sni` 和 `wellKnownCACertificates`。
没有 TLS 时编码器省略该字段，既有 golden 内容及明文目标名不变；显式 null 与省略同义。
空对象、缺身份、IP/通配符身份或非 System 信任根拒绝加载，不转成默认明文配置。
TLS 身份进入完整 revision 以及 ModelTargetName/MCPTargetName；切换 SNI 或主动删除 TLS
会改变出口目标名，因此旧 Go 配置不能借同一个目标名访问改变后的 TLS 目标。
这不冻结 DNS，也不增加版本协调或连接排空保证；无效配置仍沿现有 Loader 行为保留有效视图。
