# Ingate Managed Rate Limit Plugin

`rate-limit` 是 Ingate 内置治理插件，消费 xDS 下发的 managed rate-limit 配置。

控制台用户不直接安装或配置这个插件。用户创建 `RateLimitPolicy`、`PolicyBinding` 和 `RedisStore` 后，控制面会自动生成：

- Listener 级 Wasm filter 配置：插件基础配置、RedisStore 和 route/rule 命中的限流 binding

插件入口直接使用 proxy-wasm Go SDK，不依赖 Higress `wasm-go/pkg/wrapper`。Redis-backed global limit 需要 Ingate 自己的数据面 Redis 执行器或明确验证过的标准 host 能力；当前插件不会继承 Higress wrapper 的 Redis client。

默认发布路径：

```text
/opt/ingate/plugins/rate-limit.wasm
```

## Build

```bash
make managed-rate-limit-plugin-build
```

本地构建需要安装 TinyGo。普通 Go 单元测试不依赖 TinyGo：

```bash
cd plugins/managed/rate-limit
go test ./...
```

macOS 可以通过 TinyGo 官方 Homebrew tap 安装：

```bash
brew tap tinygo-org/tools
brew install tinygo
```

当前使用 `-gc=leaking` 生成 Wasm。`custom` GC 需要再单独验证 TinyGo 版本和 SDK 依赖的兼容性，不作为首版运行链路的阻塞项。
