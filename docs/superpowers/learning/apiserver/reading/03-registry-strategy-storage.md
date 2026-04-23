# 03 registry、strategy、storage 各自负责什么

这一层是很多人第一次看 Kubernetes apiserver 时最容易懵的地方。

## 1. 先说最短结论

### `registry`
负责资源注册组织。

### `strategy`
负责资源语义规则。

例如：
- 创建前怎么清理字段
- 更新时哪些字段能改
- 怎么做 validation
- 怎么处理 generation

### `storage`
负责把资源真正接到底层 store 上。

例如：
- create/get/list/watch/delete
- `/status`

## 2. 为什么不能全塞到一个文件

因为这三件事不是一回事：

1. 资源“怎么挂到系统里”
2. 资源“语义规则是什么”
3. 资源“底层怎么存”

如果全混在一起，一开始看起来省事，但后面会越来越难维护。

## 3. `strategy` 为什么这么重要

很多人会以为最重要的是 storage。
其实不是。

真正决定资源行为味道的，常常是 `strategy`。

因为这里定义了：
- validation
- defaulting 入口
- generation 规则
- status/spec 的边界

这也是为什么我们前面要把 validation/defaulting 接进 `strategy`。

现在还多了一层：
- 普通更新时，`strategy` 会决定 `generation` 怎么变化
- `/status` 更新时，`strategy` 会决定哪些字段必须保留、状态结构怎么校验

## 4. `/status` 为什么单独做

因为声明式资源的关键原则是：
- 用户写 `spec`
- 系统写 `status`

如果不把 `/status` 单独做出来，后面 controller 会很难写，语义也会脏。

## 5. 现在 Ingate 的实现重点在哪里

当前你重点应该看：
- `internal/controlplane/apiserver/registry/gateway/...`
- `internal/controlplane/apiserver/registry/policy/...`

看每个资源目录里的：
- `strategy.go`
- `storage/storage.go`

你不需要先把每个细节背下来。
你先抓住分工。
