# Envoy 数据面学习入口

这里的 Envoy 不是控制面组件，而是 Ingate 当前 demo 和发布链路里的数据面代理。

它的职责是：

- 接收真实请求
- 连接 `xds-server`
- 拉取 LDS / RDS / CDS / EDS
- 按 Host/Path 匹配规则转发到 backend endpoint

## 在当前项目里的位置

当前项目不是自己实现一个代理，而是：

- 控制面负责生成和发布配置
- Envoy 负责真正执行流量转发

所以完整链路是：

```text
Gateway / Route / Backend
  -> apiserver
  -> controller-manager
  -> ResolvedGateway
  -> xds-server
  -> Envoy
  -> upstream backend
```

## 当前 compose 里的工作方式

compose 版 Envoy bootstrap 在：

- [envoy.yaml](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/envoy/envoy.yaml)

它当前做了三件关键事：

1. 用 `xds_cluster` 指向 `xds-server:19090`
2. 通过 ADS 拉 LDS 和 CDS
3. 打开 admin 端口 `9901`

默认宿主机映射：

- proxy：
  - `127.0.0.1:10080`
- admin：
  - `127.0.0.1:19901`

## 当前默认路由行为

当前 demo 默认要求：

- `Host = api.example.com`
- `PathPrefix = /orders`

所以：

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

会命中。

但：

```bash
curl http://127.0.0.1:10080/orders
```

通常会返回 `404`，因为它的 Host 是 `127.0.0.1:10080`，不匹配当前路由里的 hostname。

## 你通常怎么观测它

主要看 Envoy admin：

```bash
curl http://127.0.0.1:19901/ready
curl http://127.0.0.1:19901/clusters
curl http://127.0.0.1:19901/config_dump
```

这三个接口分别回答：

- ready：
  - Envoy 自己是否 ready
- clusters：
  - 当前有哪些 upstream cluster，以及连接/请求统计如何
- config_dump：
  - 当前实际加载了哪些 listener、route、cluster、endpoint

## 最常见的两类问题

### 1. 404 Not Found

通常是路由没命中。

优先检查：

- Host 是否匹配
- Path 是否匹配

### 2. upstream connect error / timeout

通常说明：

- Envoy 自己起来了
- 路由也可能命中了
- 但最后一跳 upstream 连接不通，或者启动初期还没稳定

优先检查：

- `curl http://127.0.0.1:19901/clusters`
- `./_output/darwin_arm64/ingatectl xds resolve --backend <name>`

## 推荐联调顺序

1. 看 `admin-api` 资源
2. 看 `ingatectl` 的 xDS 解析结果
3. 看 Envoy admin
4. 再发真实流量请求

也就是：

```text
admin-api -> xds-server -> Envoy admin -> real request
```

## 当前验证入口

自动化验证：

```bash
make verify-envoy
make verify-compose
```

手动验证：

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

如果你想看部署层面的排障手册，而不是服务原理，读：

- [deploy/compose/operations.md](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/operations.md)
