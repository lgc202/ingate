# Ingate RateLimit Plugin

`ratelimit` 是 Ingate 内置治理插件，消费 xDS 下发的限流治理配置。

控制台用户不直接安装或配置这个插件。用户创建 `RateLimitPolicy`、`PolicyBinding` 和 `RedisStore` 后，控制面会自动生成：

- Listener 级 Wasm filter 配置：插件基础配置、RedisStore 和 route/rule 命中的限流 binding

插件入口直接使用 proxy-wasm Go SDK，不依赖 Higress `wasm-go/pkg/wrapper`。Redis-backed global limit 通过 `ingate-dataplane` 执行 Redis 访问，插件只负责匹配、生成 key、调用数据面服务和执行 failOpen / failClose 决策。

## Code Organization

```text
internal/app        # 装配并注册插件
internal/wasm       # Proxy-Wasm 生命周期和请求适配
internal/policy     # 限流策略判断、key 生成、本地计数和 global check
internal/dataplane  # 调用 ingate-dataplane
```

`pkg/plugin/ratelimit` 定义 xDS 下发给插件的可执行配置。`wasm` 只处理 Proxy-Wasm SDK 动作，`policy` 承载限流领域逻辑，`dataplane` 只封装外部数据面调用。新增内置插件时优先沿用这个边界。

默认发布路径：

```text
/opt/ingate/plugins/ratelimit.wasm
```

## Build

```bash
make ratelimit-plugin-build
```

插件随根 Go module 构建，使用 Go 标准工具链按 `GOOS=wasip1 GOARCH=wasm -buildmode=c-shared` 生成 Wasm 产物。这个构建方式会导出 Envoy 识别 Proxy-Wasm 插件所需的 ABI 入口。

普通 Go 单元测试不需要启动 Envoy：

```bash
cd plugins/ratelimit
go test ./...
```
