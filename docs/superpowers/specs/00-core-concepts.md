# Ingate 核心概念

这份文档不讨论实现细节，只解释这套架构里最常出现的概念。

目的只有一个：

**先把词汇表统一，再看后面的设计文档。**

## 1. 什么是 `ingate-apiserver`

你可以把它理解成：

**Ingate 的资源入口和资源真相源。**

它负责：

- 接收声明式资源
- 保存资源
- 提供查询
- 提供 `list/watch`
- 保存 `status`

它不负责：

- 直接下发 Envoy 配置
- 直接处理请求流量
- 给最终用户提供友好的产品界面

一句话：

`ingate-apiserver` 负责回答：

**“系统当前期望是什么？”**

## 2. 什么是 `controller-manager`

很多人第一次看到这个词，会以为它是“管理 controller 的小工具”。

在这套架构里，你应该把它理解成：

**运行控制循环的核心宿主。**

它内部会跑多类 controller，例如：

- `gateway controller`
- `route controller`
- `backend controller`
- `authpolicy controller`
- `trafficpolicy controller`

这些 controller 负责：

- 监听资源变化
- 校验配置
- 解析引用
- 合并策略
- 构建最终有效配置
- 回写状态

一句话：

`ingate-controller-manager` 负责回答：

**“怎样把用户声明的资源，收敛成真正生效的网关配置？”**

## 3. 什么是 `spec`

`spec` 表示：

**用户希望系统最终变成什么样。**

例如：

- 这个网关监听哪个端口
- 这条路由匹配哪个路径
- 这个后端指向哪个上游
- 这个策略想启用什么认证

你可以把 `spec` 理解成：

**期望状态。**

## 4. 什么是 `status`

`status` 表示：

**系统当前实际观察到的结果。**

例如：

- 这条路由是否已被接受
- 引用的 backend 是否存在
- 配置是否已发布给 Envoy
- Envoy 是否 ACK
- 是否有错误

你可以把 `status` 理解成：

**实际状态。**

所以：

- `spec` 是“我想要什么”
- `status` 是“系统现在做到了什么”

## 5. 什么是 `list/watch`

这不是普通的“查一下数据库”。

你可以这样理解：

- `list`：先拿一份当前完整资源列表
- `watch`：之后持续接收变化事件

controller 依赖它，是因为 controller 不是只运行一次，而是：

**一直盯着资源变化，并持续收敛。**

如果只有普通查询，没有 `watch`，控制循环就会退化成：

- 不断轮询
- 自己猜哪些东西变了

这不是我们想要的声明式控制面体验。

## 6. 什么是 `IR`

`IR` 是 **中间表示**。

你可以把它理解成：

**控制器把各种资源解释、归并之后，得到的一份“最终有效配置草图”。**

它不是：

- 原始资源对象
- 也不是 Envoy 的原生配置

为什么需要它？

因为：

- `Route`
- `Backend`
- `AuthPolicy`
- `TrafficPolicy`

这些资源会互相影响。

如果没有 `IR`，代码很快就会变成：

- 一边处理资源语义
- 一边拼 Envoy 配置

后面会很乱。

一句话：

`IR` 负责把：

**“用户声明了什么”**

变成：

**“系统最终认定应该生效什么”**

## 7. 什么是 `xDS`

`xDS` 是 Envoy 使用的动态配置协议。

你可以把它理解成：

**控制面把配置发给 Envoy 的方式。**

所以：

- `xDS` 是下发协议
- 不是存储
- 也不是用户接口

一句话：

`xDS` 负责回答：

**“如何把配置可靠地送到 Envoy？”**

## 8. 什么是 `Envoy`

`Envoy` 是数据面。

它负责：

- 接请求
- 匹配路由
- 做认证
- 做限流
- 转发到后端
- 输出日志和指标

它不负责：

- 管理资源
- 决定资源语义
- 当配置数据库

一句话：

`Envoy` 负责回答：

**“请求来了之后具体怎么执行？”**

## 9. 什么是“控制面”和“数据面”

在 `Ingate` 里：

- `ingate-apiserver`
- `ingate-controller-manager`
- `ingate-xds-server`

这些属于：

**控制面**

它们负责：

- 接收配置
- 理解配置
- 发布配置

而 `Envoy` 属于：

**数据面**

它负责：

- 真正处理流量

## 10. 一条最简单的完整链路

你可以把整个系统先记成这条链路：

```text
用户提交资源
  -> ingate-apiserver
  -> ingate-controller-manager
  -> IR
  -> ingate-xds-server
  -> Envoy
```

再加上结果回写：

```text
Envoy
  -> ingate-xds-server
  -> ingate-apiserver(status)
```

只要先记住这两条链路，后面大多数概念都会好理解很多。

## 11. 阅读建议

如果你现在对 K8s 术语还不熟，建议按这个顺序看：

1. 当前文档
2. `01-overview.md`
3. `02-resource-model.md`
4. `03-control-plane.md`
5. `04-delivery-and-xds.md`

后面的文档可以等主链路建立之后再看。
