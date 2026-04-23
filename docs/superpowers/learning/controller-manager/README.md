# controller-manager 学习入口

`ingate-controller-manager` 是资源收敛组件。

它的职责不是对外提供产品 API，也不是直接给数据面服务，而是把 `Gateway`、`Route`、`Backend`、`Certificate`、`AuthPolicy`、`TrafficPolicy` 这些资源收敛成更适合发布的控制面结果。

当前可以先把它理解成：

```text
apiserver 中的原始资源
  -> controller-manager 控制循环
  -> ResolvedGateway
```

## 它负责什么

- watch `apiserver` 里的资源对象
- 做依赖解析和状态收敛
- 产出 `ResolvedGateway`
- 回写 `Accepted` / `Resolved` 这类状态

它不负责：

- 面向前端的 HTTP JSON API
- 直接服务 Envoy 的 xDS 协议
- 真实数据流量转发

## 你应该先理解什么

如果你已经读过 `apiserver` 和 `admin-api`，下一步读它最合适。

重点不是“它能不能起进程”，而是：

- 为什么要有 `ResolvedGateway`
- 原始资源为什么不能直接拿去给数据面
- 哪些状态在这里收敛，哪些状态留给后续发布链路

## 当前文档

- [01-how-to-run.md](./01-how-to-run.md)

这篇文档目前偏运行和验证，覆盖：

- 本地 bring-up
- `ResolvedGateway` 验证
- `xds-server` 联调
- Envoy 联调

## 推荐阅读顺序

1. 先读 [01-how-to-run.md](./01-how-to-run.md)
2. 再配合这些规格文档一起看：
   - [03-control-plane.md](/Users/guangcaili/workplace/code/lgc202/ingate/docs/superpowers/specs/03-control-plane.md)
   - [04-delivery-and-xds.md](/Users/guangcaili/workplace/code/lgc202/ingate/docs/superpowers/specs/04-delivery-and-xds.md)

## 代码入口

- 命令入口：
  - [main.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/controller-manager/main.go)
- 启动和运行：
  - [run.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/controller-manager/app/run.go)
  - [server.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/controller-manager/app/server.go)
- 参数定义：
  - [options.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/controller-manager/app/options/options.go)

## 典型验证命令

```bash
make build-controller-manager
make verify-controller-manager
```

如果你想看 controller-manager 后续怎么进入发布链路，再读：

- [xds-server/README.md](/Users/guangcaili/workplace/code/lgc202/ingate/docs/superpowers/learning/xds-server/README.md)
