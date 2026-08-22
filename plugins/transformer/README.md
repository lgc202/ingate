# Transformer

Transformer 是 Ingate 维护的首款标准 Proxy-Wasm 插件，用于按顺序修改请求和响应 Header。

配置结构参考 [Higress Transformer](https://github.com/alibaba/higress/tree/main/plugins/wasm-go/extensions/transformer)，当前只实现控制台已经开放的 `remove`、`rename`、`replace`、`add` 和 `append` 操作。插件使用 [Proxy-Wasm Go SDK](https://github.com/proxy-wasm/proxy-wasm-go-sdk) 和官方 Envoy 支持的标准 ABI，不依赖 Higress Envoy 扩展。

```bash
make wasm-plugins
```

产物位于 `_output/plugins/transformer.wasm`。
