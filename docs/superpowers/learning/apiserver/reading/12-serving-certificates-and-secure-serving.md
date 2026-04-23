# 12 apiserver 证书和 secure serving 代码链路怎么读

这篇只讲一条链：

**`ingate-apiserver` 的 HTTPS 证书是怎么从代码里一路准备出来的。**

目标是让你读完后能回答：
- 为什么会有 `apiserver.crt`
- 它不是在哪个 shell 脚本里手搓出来的
- 为什么启动时自动就有证书
- 如果以后要改成自己的证书，应该改哪一层

## 1. 先看最短链路

你可以先把这条链记成：

```text
main
-> app command
-> options.NewOptions()
-> options.Complete()
-> SecureServing.MaybeDefaultWithSelfSignedCerts(...)
-> apiserver 用证书启动 HTTPS
```

这条链是关键。

证书生成不在：
- `Makefile`
- `build.sh`
- `run-apiserver.sh`

而是在 **apiserver 自己的 options 完成阶段**。

## 2. 从哪里开始看

先看这里：

- [cmd/apiserver/main.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/apiserver/main.go)
- [cmd/apiserver/app/server.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/apiserver/app/server.go)
- [cmd/apiserver/app/options/options.go](/Users/guangcaili/workplace/code/lgc202/ingate/cmd/apiserver/app/options/options.go)

这里本身不生成证书。
它只负责：
- 把 command 跑起来
- 创建并完成一份 server options

也就是说，`main` 只是入口，不是证书逻辑所在地。

## 3. 真正的证书配置起点在哪里

看这里：

- [internal/controlplane/apiserver/options/options.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/options/options.go)

先看 `NewOptions()`。

里面最关键的是这几行：
- `genericoptions.NewSecureServingOptions().WithLoopback()`
- `secureServing.BindAddress = 127.0.0.1`
- `secureServing.BindPort = 18443`
- `secureServing.ServerCert.CertDirectory = "_output/certificates"`
- `secureServing.ServerCert.PairName = "apiserver"`

这几行的意思分别是：
- 要启用 secure serving
- 默认监听本地 `127.0.0.1:18443`
- 证书目录是 `_output/certificates`
- 文件名前缀是 `apiserver`

所以你后面看到的文件名自然就是：
- `_output/certificates/apiserver.crt`
- `_output/certificates/apiserver.key`

## 4. 为什么不是在 shell 脚本里生成证书

这是一个很容易误判的点。

因为你平时可能会觉得：
- 构建产物目录在 `_output`
- 运行脚本也在 `tools/hack`
- 那证书是不是也应该由脚本生成

但当前实现没有这么做。

原因是：

**证书属于 apiserver secure serving 配置的一部分，不是构建产物。**

也就是说：
- 二进制构建：属于 `build.sh`
- 证书准备：属于 apiserver 启动配置

这个分层更对。

否则你会把：
- 构建阶段
- 启动阶段
- 安全配置阶段

全搅在一起。

## 5. 真正生成证书的是哪行代码

继续看同一个文件里的 `Complete()`：

- [options.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/options/options.go)

里面关键步骤是：

1. `DefaultAdvertiseAddress(...)`
2. 取 `AdvertiseAddress`
3. 调 `MaybeDefaultWithSelfSignedCerts(...)`

这里最重要的是第三步。

### `MaybeDefaultWithSelfSignedCerts(...)` 在做什么

你可以先把它理解成：

**如果没有显式提供证书文件，就帮你生成一套默认自签名证书。**

所以它不是“每次都强制重新生成”。
它更像：
- 有就用
- 没有就自动补一套

这个行为非常适合当前阶段：
- 本地开发省心
- 不需要你先学完整 PKI
- 但 HTTPS 是真的

## 6. 为什么这里还要先算 `AdvertiseAddress`

因为证书不是只要“有一个文件”就行。

证书里还要带上它声称自己服务于哪些地址，也就是：
- 主机名
- IP
- SAN

所以在生成证书前，代码必须先知道：
- 这个服务准备以什么地址对外宣告自己

这就是为什么会先走：
- `DefaultAdvertiseAddress(...)`
- 再调用 `MaybeDefaultWithSelfSignedCerts(...)`

## 7. 现在证书里会放哪些名字

还是看 `Complete()` 这一段。

当前传进去的有三类：

1. `advertiseAddress.String()`
2. `AlternateDNS`
3. `127.0.0.1`

其中 `AlternateDNS` 在 `NewOptions()` 里定义的是：
- `localhost`
- `ingate.local`

所以你现在可以这样理解：

当前自签名证书至少围绕这些值生成：
- `127.0.0.1`
- `localhost`
- `ingate.local`
- advertise address

这就是为什么本地验证时：
- `curl -k https://127.0.0.1:18443/healthz`
- `kubectl` 指向 `https://127.0.0.1:18443`

都能成立。

## 8. `run-apiserver.sh` 在这里扮演什么角色

看这里：

- [run-apiserver.sh](/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/run-apiserver.sh)

这个脚本其实很薄。
它只做：
- 找到二进制
- 检查 etcd 地址
- 执行 apiserver

它**不会生成证书**。

所以你要把角色分清楚：
- `run-apiserver.sh`：负责启动进程
- `options.Complete()`：负责把 secure serving 配好
- `MaybeDefaultWithSelfSignedCerts(...)`：负责在需要时准备证书

## 9. 为什么 `_output/certificates` 不等于“构建产物目录”

虽然它也在 `_output/` 下面，但语义不一样。

- `_output/<os>_<arch>/...`：更像编译产物
- `_output/certificates/...`：更像运行时生成的本地开发资产

为什么仍然放在 `_output` 下面？

因为它们都属于：
- 本地生成
- 仓库不提交
- 开发环境可清理

但你脑子里一定要分开：
- 二进制不是证书
- 证书不是源码
- 证书也不是 Makefile 预制文件

## 10. 如果以后要换成自己的证书，应该改哪里

先说原则：

**不要去改 `MaybeDefaultWithSelfSignedCerts()` 的内部逻辑。**

应该改的是：
- secure serving 选项
- cert/key 的来源

也就是说，正确方向是：

1. 给 apiserver 增加或使用现成的 secure serving 参数
2. 显式传入：
   - cert file
   - key file
3. 让 secure serving 直接读取你提供的证书
4. 只有在你没提供时，才让 `MaybeDefaultWithSelfSignedCerts()` 兜底

这才是更像正式工程的做法。

## 11. 为什么当前阶段不急着先做“自定义证书全流程”

因为现在你在学的是：
- 自定义 generic apiserver
- secure serving 是怎么接进去的
- HTTPS 为什么是真的

如果现在就把重点切到：
- 内部 CA
- cert-manager
- Secret 挂载
- 证书轮转

学习主线会被打散。

所以当前阶段最合理的是：
- 先理解自动自签名这条线
- 先把 secure serving 学明白
- 后面进入更真实部署时，再切到自定义证书方案

## 12. 读完这一篇，你应该能回答

1. 为什么 `apiserver.crt` 不是在脚本里手工生成的
2. 为什么启动 apiserver 时它会自动出现
3. 为什么证书目录和文件名会是 `_output/certificates/apiserver.*`
4. 为什么当前自签名证书足够支撑本地开发
5. 如果以后要换成自己的证书，应该改 secure serving 输入，而不是去改构建脚本
