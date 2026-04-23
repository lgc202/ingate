# xds-server 学习入口

`ingate-xds-server` 是发布组件。

它消费 `ResolvedGateway`，维护发布缓存，并把结果通过 discovery/configsync/ADS 暴露给数据面和运维工具。

当前可以把它理解成：

```text
ResolvedGateway
  -> xds-server
  -> EffectiveConfig / discovery result / ADS resources
```

## 它负责什么

- watch `ResolvedGateway`
- 把 `ResolvedGateway` 翻译成运行态配置
- 维护已发布的 runtime cache
- 提供 discovery gRPC
- 提供 configsync 视图
- 提供 ADS/xDS 资源
- 回写 `Programmed`

它不负责：

- 原始资源 CRUD
- 前端/API 层
- 真正执行转发

## 为什么它重要

这是控制面和数据面之间最关键的桥接点。

在这之前：

- 你看到的是资源对象和控制循环结果

在这之后：

- Envoy 才能拿到 listener / route / cluster / endpoint

所以它回答的问题是：

- 这个 gateway 现在有没有被发布
- 发布成了什么
- backend endpoint 最终被解析成什么
- Envoy 理论上应该拿到哪些 xDS 资源

## 推荐先读什么

- [04-delivery-and-xds.md](/Users/guangcaili/workplace/code/lgc202/ingate/docs/superpowers/specs/04-delivery-and-xds.md)
- [controller-manager/01-how-to-run.md](/Users/guangcaili/workplace/code/lgc202/ingate/docs/superpowers/learning/controller-manager/01-how-to-run.md)
- [deploy/compose/ingatectl.md](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/ingatectl.md)

## 代码入口

- 命令入口：
  - [main.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/xds-server/main.go)
- 启动入口：
  - [server.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/xds-server/app/server.go)
- 参数定义：
  - [options.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/xds-server/app/options/options.go)

## 当前实现分层

当前实现主要分成这些目录：

- watch：
  - [resolvedgateway.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/watch/resolvedgateway.go)
- translate：
  - [resolvedgateway.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/translate/resolvedgateway.go)
- cache：
  - [cache.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/cache/cache.go)
- config：
  - [config.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/config/config.go)
- publish / configsync：
  - [server.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/publish/server.go)
- ads：
  - [service.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/ads/service.go)
  - [resources.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/xds/ads/resources.go)

## 运维视角怎么用它

它通常不是你直接用浏览器访问的服务，而是通过这些方式观测：

- `ingatectl xds list`
- `ingatectl xds summary`
- `ingatectl xds resolve`
- `ingatectl xds check`
- `ingatectl xds ads`

当前 compose 默认对外暴露：

```text
127.0.0.1:19090
```

这也是 `ingatectl` 默认连接的地址。

## 典型验证命令

```bash
make build-xds-server
make verify-xds-server
```

如果你想继续顺着数据面往下看，下一步读：

- [envoy/README.md](/Users/guangcaili/workplace/code/lgc202/ingate/docs/superpowers/learning/envoy/README.md)
