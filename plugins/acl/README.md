# Ingate ACL Plugin

`acl` 是 Ingate 内置访问控制插件，消费 xDS 下发的 ACL 可执行配置。

控制台用户不直接安装或配置这个插件。`AccessControlPolicy` 通过自身的 `targetRefs[]` 声明生效的 Gateway 或 Route，Envoy Config Compiler 将其编译成 route 策略索引并注入 Envoy Wasm filter。

## Code Organization

```text
internal/app     # 装配并注册插件
internal/wasm    # Proxy-Wasm 生命周期、请求属性读取和拒绝响应
internal/policy  # Route 策略索引、ACL 规则匹配和 allow / deny 决策
```

`pkg/plugin/acl` 定义 xDS 下发给插件的可执行配置。`policy` 根据 Gateway 和 Route 建立策略索引并完成访问判断，`wasm` 只负责 Proxy-Wasm 生命周期和请求响应适配。

默认发布路径：

```text
/opt/ingate/plugins/acl.wasm
```
