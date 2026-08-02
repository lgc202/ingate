# Ingate Console 场景化企业级前端重构方案

## 1. 重构背景与目标

Ingate 是一套面向 API 网关、AI 网关及未来 Agent 网关的声明式 Envoy 控制面。

之前的页面侧重于**数据结构的增删改查（CRUD 表单罗列）**，缺少真实网关运维和部署时的**实际业务应用场景**。

本次重构旨在打造 **场景驱动、高信息密度、零冗余废话、现代化视觉质感** 的企业级 API 与 AI 控制台：
1. **场景化导向 (Scenario-Driven)**：区分大模型 AI 服务与普通 API 后端，提供主流 AI 厂商（OpenAI、DeepSeek、Anthropic、通义千问、Gemini 等）的一键模版接入。
2. **流量与模型拓扑可视化**：清晰呈现 `客户端请求 model 别名` ➔ `Ingate ai-proxy 转化/治理` ➔ `上游 Endpoint` 的多厂商路由映射图。
3. **策略预设模板 (Policy Presets)**：提供场景化治理模板（如：API 限流防刷、内部 IP 白名单、AI Token 配额限制）。
4. **控制台总览 Hub (Overview Hub)**：首页集中呈现 Envoy xDS 指纹、端口监听器状态、接入厂商与 Token 预算感知。

---

## 2. 三步走重构规划

### 第一步：重构「上游服务 (Upstreams & AI Models Hub)」
- **模式分组**：将页面清晰划分为 **【AI 大模型服务】** 与 **【应用后端 / 微服务】** 两个场景标签页。
- **厂商预设快捷模版**：提供 OpenAI、DeepSeek、Anthropic、Qwen、Gemini 等快捷卡片，自动填充 Base URL 与默认协议，用户只需配置或修改 API Key。
- **模型目录管理**：可视化展示每个 AI Upstream 下维护的公开模型列表（如 `deepseek-chat`, `deepseek-reasoner`, `gpt-4o`）。

### 第二步：重构「路由管理 (Routes & Traffic Flow)」
- **AI 模型路由拓扑视图**：直观展示客户端调用 `POST /v1/chat/completions` 时，请求体中的公开 `model` 别名如何跨厂商转发到不同 Upstream。
- **流量拓扑卡片**：以 `入口 Gateway` ➔ `匹配条件 (Path/Header/Model)` ➔ `挂载治理策略` ➔ `目标 Cluster` 的节点流向图替代纯文本表格。

### 第三步：打造「控制台首页总览 (Overview Hub)」与全量编译验证
- 建立首页 Overview 概览指标面板（已接入 AI 厂商、活跃网关监听器、xDS 候选指纹、Token 额度监控）。
- 执行 `npm run build` 和 TypeScript 全量校验，确保 0 Error。
