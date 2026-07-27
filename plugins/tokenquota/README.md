# Ingate TokenQuota Plugin

`tokenquota` 是 Ingate 内置的 AI Token 配额数据面插件。用户只配置强类型的 `TokenQuotaPolicy`，Envoy Config Compiler 将策略编译为精确到 Gateway、Route 和 RouteRule 的私有执行配置；用户不直接安装插件，也不编辑插件 JSON 或 Redis 地址。

一个 Policy 定义一个跨全部 `targetRefs[]` 共享的预算池，并可选择所有命中请求共享、按网关看到的来源 IP 或按请求 Header 值划分主体。预算池身份由资源 UID 派生，删除后同名重建不会继承旧窗口计数；普通配置更新不会重置额度。IP 和 Header 原始值会先经过 SHA-256，再进入 Redis key，不保存明文主体值。

插件使用 Envoy bootstrap 中固定的 `ingate-system-redis`，按 Redis 服务端时间计算固定窗口。请求进入时原子查询当前窗口已用额度，达到上限后返回 OpenAI-compatible 错误响应；通过检查的请求不预扣额度，因此并发请求可能造成有限超额。当前语义是 best-effort 的后付费软额度，不是严格硬额度，也不应作为财务计费依据。

响应结束后，插件按归一化响应最后一个合法的 `usage.total_tokens` 原子记账。普通 JSON 响应读取最终 usage；SSE 响应增量解析事件，只保留最后一个合法值，并在完成标记下发前记账。没有合法 usage 时记录错误但不计费；流式连接在完成标记到达前中断时同样无法记账。长请求跨越窗口时，实际 Token 计入响应结束时所在的窗口。

主体划分存在以下约束：

- Header 模式必须使用可信认证层写入且客户端无法伪造的 Header；Header 缺失时，所有请求共用同一个未标识预算池
- 允许客户端自由修改或轮换 Header 值会绕过单主体额度，并产生高基数 Redis key
- IP 模式读取 Envoy 连接源地址；经过负载均衡或反向代理且未保留真实源地址时，识别到的可能是代理地址

TokenQuotaPolicy 保护流式请求时，AI Proxy 会为 OpenAI-compatible 上游内部注入 `stream_options.include_usage=true`。自定义 OpenAI-compatible 上游必须支持该参数，并在最终 SSE 事件返回 usage；不满足条件的上游不应启用流式 Token 配额。

`failurePolicy` 只控制请求前 Redis 检查失败时放行还是拒绝。响应后的记账失败会记录错误并继续返回上游响应，避免已经完成的模型请求因计费存储故障被改写。

代码边界：

```text
internal/app     # 装配并注册插件
internal/wasm    # Route 索引、Proxy-Wasm 生命周期和串行 Redis 调度
internal/policy  # 配额检查、主体 key 和最终裁决语义
internal/redis   # Redis 固定窗口检查与记账协议
internal/usage   # 普通响应和 SSE usage 提取
```

`pkg/plugin/tokenquota` 定义 Compiler 下发给插件的私有执行协议，`plugins/internal/redisabi` 隔离 Envoy 的 Redis hostcall ABI，`plugins/internal/redisresp` 提供共享 RESP2 编解码。

插件默认发布到 `/opt/ingate/plugins/tokenquota.wasm`，在仓库根目录运行：

```bash
make plugins-build
```
