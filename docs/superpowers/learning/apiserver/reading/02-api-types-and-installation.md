# 02 资源类型、Scheme、安装链

这篇解决一个核心问题：

**资源类型到底是怎么变成 API 路由的。**

## 1. 资源类型在哪里定义

在：
- `pkg/apis/gateway/v1alpha1/`
- `pkg/apis/policy/v1alpha1/`

这里定义的是：
- `Gateway`
- `Route`
- `Backend`
- `AuthPolicy`
- `TrafficPolicy`

为什么放这里？

因为这里表达的是：
- group
- version
- type

不是对外 HTTP handler。

## 2. `Scheme` 是什么

可以把 `Scheme` 理解成：

**整个 apiserver 的“类型登记簿”。**

如果一种资源没注册进 `Scheme`，系统就不知道：
- 请求体该反序列化成什么 Go 类型
- 返回体该按什么类型编码

所以 `Scheme` 不是小工具，而是资源系统的基础设施。

## 3. `install.go` 在干什么

它负责：
- 组装 API group info
- 调用 storage provider
- 把资源组安装进 generic apiserver

为什么还需要这一步？

因为“我定义了一个 Go struct”并不等于“它已经成为一个 API 资源”。

中间还要经过：
- 类型注册
- storage 提供
- group 安装

## 4. `RESTStorageProvider` 是什么

它可以理解成：

**某个资源组如何把自己的一批资源挂进 apiserver。**

例如：
- gateway 组 provider
- policy 组 provider

它负责告诉系统：
- 这组资源有哪些
- 每个资源对应哪个 storage 实现

## 5. 为什么要有这层 provider

因为资源组安装不是“写死一个 map 就完了”。

如果以后资源组变多、版本变多，这一层能把：
- 资源组安装
- 资源 storage 构建
分开。

这就是为什么代码看起来比小项目复杂一些，但长期更稳。
