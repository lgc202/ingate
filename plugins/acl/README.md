# Ingate ACL Plugin

`acl` 是 Ingate 内置访问控制插件，消费 xDS 下发的 ACL 可执行配置。

控制台用户不直接安装或配置这个插件。`AccessControlPolicy` 通过自身的 `targetRefs[]` 声明生效的 Gateway 或 Route，Envoy Config Compiler 将其编译成 route 策略索引并注入 Envoy Wasm filter。

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
