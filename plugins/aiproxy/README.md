# AI Proxy 插件

AI Proxy 是 Ingate 内置的数据面插件。第一版只处理 OpenAI-compatible 的
`POST /v1/chat/completions` 请求。一条模型 Route 固定转发到一个 Upstream，
`models[]` 只把请求体中的客户端模型别名映射为实际的 `upstreamModel`；插件完成
别名匹配和模型名改写，并注入该 Route 使用的上游 API Key。

Upstream 选择由 Envoy 的静态 Route 配置完成，插件不参与 cluster 选择，也不使用
内部选路 Header。第一版不支持根据请求体 `model` 跨多个 Provider 或 Upstream
动态选路，这样请求体缓冲和模型改写不会影响 Envoy 的路由生命周期。

插件只加工请求，普通响应与 SSE 流式响应均由 Envoy 原样转发。运行时配置由
Envoy Config Compiler 生成，用户不会直接编辑插件私有 JSON。

第一版单次请求体上限为 1 MiB，只面向常规文本对话 JSON，不支持在请求体中
携带大图、大文件等大体积多模态输入。该限制避免为同一 Listener 上的普通路由
扩大共享内存边界。

插件默认发布到 `/opt/ingate/plugins/ai-proxy.wasm`。
