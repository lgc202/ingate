# IP Restriction

`iprestriction` 是 Ingate 内置的客户端 IP 访问限制 Wasm 插件。控制面根据 `IPRestrictionPolicy` 生成按 Route 索引的允许列表或拒绝列表，插件使用 Envoy 提供的下游连接地址完成判断。

插件不读取 `X-Forwarded-For` 等客户端可伪造的请求头。需要经过代理保留真实客户端 IP 时，应先在 Gateway 层建立明确的可信代理配置。
