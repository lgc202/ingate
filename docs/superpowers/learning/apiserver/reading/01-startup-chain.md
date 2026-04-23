# 01 启动链：main -> command -> options -> config -> server

这是整个 apiserver 最核心的一条线。

## 1. 先看 `main`

文件：
- `cmd/apiserver/main.go`

它只做两件事：
1. 支持 `--version`
2. 调 `app.NewAPIServerCommand()`

为什么 `main` 要这么薄？

因为长期维护项目里：
- `main` 不应该塞业务逻辑
- 它只是入口

这也是 `kube-apiserver` / `OneX` 的常见模式。

## 2. 再看 `app/server.go`

它负责把启动过程分成几段：
- 创建 command
- 绑定参数
- `Complete`
- `Validate`
- `Run`

为什么不直接 `flag.Parse()`？

因为一旦参数多起来，`flag.Parse()` 风格很快会变乱。

`command/options/config/run` 这套写法更适合：
- 参数越来越多
- 校验越来越多
- 组件越来越复杂

## 3. `options` 是什么

`options` 可以理解成：

**启动这个 apiserver 所需要的一整组配置。**

比如：
- bind address
- secure port
- etcd servers
- 推荐选项

为什么要单独有 `options`？

因为“启动参数”本身就是一个稳定概念。
它不应该散在各处。

## 4. `config` 是什么

`config` 的职责是：

**把 Ingate 自己的配置，灌进 generic apiserver 的标准配置对象里。**

这一步很重要，因为它是“我们自己的系统”和“K8s 通用 apiserver machinery”真正接起来的地方。

## 5. `server` 是什么

`server` 的职责是：
- 创建 generic apiserver 实例
- 安装 API group
- 最后真正 `Run`

所以这条链可以背成：

```text
main
-> command
-> options
-> config
-> server
-> generic apiserver
```

如果你把这条线看懂了，后面很多代码就不会再显得神秘。


## 额外补充：认证和授权现在挂在哪

当前 Ingate 的最小认证授权也是在启动链里接进去的，不是在业务 handler 里临时判断。

挂点在：
- `internal/controlplane/apiserver/options/*`
- `internal/controlplane/apiserver/config.go`

也就是：
- `options` 决定默认 token、匿名路径这些启动参数
- `config.go` 把 authenticator 和 authorizer 接进 `generic apiserver`
