# Ingate RateLimit Plugin

`ratelimit` 是 Ingate 内置限流插件。用户只配置强类型的 `RateLimitPolicy` 和 `PolicyBinding`，Envoy Config Compiler 会生成插件可直接执行的 route/rule 策略索引。

Global 限流固定使用 Envoy bootstrap 中的 `ingate-system-redis`。插件通过 Ingate 自己维护的最小 Redis ABI adapter 调用 Redis，不依赖 Higress 的产品模型、wrapper 或高层 SDK，也不需要独立数据面代理进程。

代码边界：

```text
internal/app      # 装配并注册插件
internal/runtime  # 加载策略索引并准备限流检查
internal/wasm     # Proxy-Wasm 生命周期与请求控制
internal/policy   # 策略匹配、key 和裁决语义
internal/redis    # RESP 编解码与 Redis 限流算法
```

`pkg/plugin/ratelimit` 定义 Compiler 下发给插件的内部执行协议，`plugins/internal/redisabi` 隔离 Higress Envoy 提供的 Redis hostcall ABI。

插件默认发布到 `/opt/ingate/plugins/ratelimit.wasm`，在仓库根目录运行：

```bash
make ratelimit-plugin-build
```
