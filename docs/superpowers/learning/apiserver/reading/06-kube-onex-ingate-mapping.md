# 06 kube-apiserver、OneX、Ingate 怎么对应

这篇的目标不是做严格源码对照表。
而是帮你建立一个稳定直觉：

**Ingate 不是凭空造了一套 apiserver，它是在走一条非常明确的继承链。**

## 1. 最上游：kube-apiserver

它提供的是：
- API machinery
- generic apiserver
- registry / strategy / storage 这些模式
- secure serving
- list/watch 这些语义

## 2. 中间参考：OneX

OneX 做的事情，是把 kube-apiserver 这一套：
- 工程化
- 简化到更容易学
- 落到一个领域 apiserver 项目里

所以 OneX 对我们的价值非常大：
- 它比 kube-apiserver 更容易学
- 但又没有偏离正路

## 3. Ingate 当前在做什么

Ingate 不是抄 kube-apiserver 本体。
也不是抄 OneX 全仓。

Ingate 当前做的是：
- 启动链学 kube-apiserver / OneX
- 资源型控制面学 OneX
- 网关业务模型学 Higress
- 工程壳做适合自己项目规模的收敛

## 4. 为什么不直接照搬 Higress

因为 Higress 的强项不在：
- generic apiserver
- 资源型控制面
- generated client/informer/lister 这条链

它更强在：
- 网关业务
- 数据面
- 部署和外围工程能力

## 5. 你现在应该形成的判断

以后看到一段代码时，先问自己：

1. 这是 Kubernetes API machinery 的通用模式吗
2. 这是 OneX 风格的工程化包装吗
3. 这是 Ingate 自己的网关业务语义吗

如果你能这样分层看，代码就不会再显得那么乱。
