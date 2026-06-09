# Ingate ACL Plugin

`acl` 是 Ingate 内置访问控制插件，消费 xDS 下发的 ACL 可执行配置。

控制台用户不直接安装或配置这个插件。控制面中的 `AccessControlPolicy` 和 `PolicyBinding` 会被编译成插件运行时配置，由 xDS target 注入到 Envoy Wasm filter。

## Code Organization

```text
internal/app      # 装配并注册插件
internal/runtime  # 编译插件配置，保存 route index 和 header plan
internal/wasm     # Proxy-Wasm 生命周期适配和 action 执行
internal/policy   # ACL 规则匹配和 allow / deny 决策
```

`pkg/plugin/acl` 定义 xDS 下发给插件的可执行配置。`wasm` 只处理 Proxy-Wasm SDK 动作，`runtime` 承载配置编译后的执行计划，`policy` 承载 ACL 领域逻辑。

默认发布路径：

```text
/opt/ingate/plugins/acl.wasm
```
