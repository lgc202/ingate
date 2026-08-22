# Transformer

Transformer 是 Ingate 维护的首款标准 Proxy-Wasm 插件，用于按顺序修改请求和响应 Header。

配置结构参考 [Higress Transformer](https://github.com/alibaba/higress/tree/main/plugins/wasm-go/extensions/transformer)，当前只实现控制台已经开放的 `remove`、`rename`、`replace`、`add` 和 `append` 操作。插件使用 [Proxy-Wasm Go SDK](https://github.com/proxy-wasm/proxy-wasm-go-sdk) 和官方 Envoy 支持的标准 ABI，不依赖 Higress Envoy 扩展。

```bash
make wasm-plugins
```

产物位于 `_output/plugins/transformer.wasm`。

## 发布

插件的名称、许可证、兼容版本和 OCI 仓库保存在同目录的 `plugin.json`，它们不是每次发布都要修改的版本记录。

发布新版本时，在 GitHub Actions 中运行 `Plugin release`，选择 `transformer` 并输入语义版本。工作流会构建并推送不可变 OCI 制品，再把版本和制品摘要追加到官方插件目录；无需修改目录 JSON 或创建插件专用 Git Tag。
