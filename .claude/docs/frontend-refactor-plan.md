# Ingate Console 前端极简与实际场景重构方案

## 一、 问题反思与核心纠偏原则

### 1. 彻底移除虚假监控与内部实现泄露
- **移除伪监控 Dashboard (`/dashboard`)**：当前后端未接入 TSDB/Prometheus 监控数据源，前端堆砌的假 QPS、假 TTFT 延迟与假图表脱离实际。
- **隐藏底层实现细节**：隐藏 Envoy xDS Candidate Snapshot、HCM Filter Chain 等内部细节，不泄露给普通用户。

### 2. 强类型资源与真实场景收敛
Ingate 数据面由 Controller 全量编译声明式资源，面向用户的核心视图收敛为 6 大强类型资源主线：
`网关` ➔ `路由` ➔ `服务` ➔ `策略` ➔ `证书` ➔ `配置状态`

---

## 二、 6 大核心视图的真实应用场景设计

### 1. 📦 上游服务 (Upstreams) —— “流量最终发往哪里”
- **AI 大模型服务**：
  - 核心输入：展示名称、厂商 (OpenAI/DeepSeek/Anthropic/Qwen/Gemini)、Base URL、API Key。
  - 隐式推断：https:// 开头自动开启 TLS 与 SNI 校验，通信协议固定为标准 HTTP/1.1，不暴露无意义的选择框。
- **应用微服务**：
  - 端点地址与端口、权重列表。
  - 高级配置（TLS / HTTP2 / gRPC）默认收纳折叠，仅在特殊微服务架构时展开。

### 2. 🔀 路由规则 (Routes) —— “请求如何匹配与分发”
- **应用路由**：匹配路径/Header ➔ 分发至应用 Upstream ➔ 悬浮显示挂载的限流/访问控制策略。
- **AI 路由**：监听 `POST /v1/chat/completions` ➔ 映射客户端公开模型别名（如 `deepseek-r1` ➔ DeepSeek Upstream, `gpt-4o` ➔ OpenAI Upstream）。

### 3. 🛡️ 治理策略 (Policies) —— “流量防护与配额控制”
强类型策略靶向绑定 (targetRefs)：
- **RateLimitPolicy (限流)**：请求速率限制，显示应用的目标 Gateway/Route。
- **AccessControlPolicy (ACL)**：IP 黑白名单。
- **TokenQuotaPolicy (AI 配额)**：Token 每日/每月预算池，绑定至 AI 路由。

### 4. 🚪 网关 (Gateways) —— “入口端口与证书绑定”
- 展示 HTTP 80 / HTTPS 443 监听器端口、匹配域名与 TLS 证书关联。

### 5. 🔐 证书 (Certificates) —— “TLS/HTTPS 证书管理”
- 导入/查看 PEM 格式证书链与 Key，显示签发域名与到期倒计时。

### 6. ⚙️ 配置状态 (Configuration Status) —— “配置是否生效”
- 表达“声明式资源 ➔ Envoy 数据面”的全量生效状态，展示资源同步版本与最新编译快照，消除底层黑话。

---

## 三、 执行计划

1. **路由与导航纠偏**：删除伪 Monitor Dashboard 导航与大屏，默认首页指向【网关】列表。
2. **清洗多余代码与废弃视图**：移除所有制造视觉噪音的假指标卡片与实现细节文案。
3. **精简 6 大核心页面**：按上述设计实现干净、高密度、符合真正运维逻辑的界面。
4. **编译与验证**：执行 `npm run build` 确保零错误。
