# 08 认证授权代码是怎么接进去的

这篇回答 5 个问题：
1. 认证和授权代码放在哪里
2. 启动时是怎么接进 `generic apiserver` 的
3. 为什么现在会同时有 admin 和 viewer 两类内置身份
4. 为什么授权现在改成了静态策略
5. 现在这套代码里哪些是手写的

## 1. 先看代码地图

这次 authn/authz 相关代码主要在这几个地方：
- `internal/controlplane/apiserver/auth/constants.go`
- `internal/controlplane/apiserver/auth/authenticator.go`
- `internal/controlplane/apiserver/auth/policy.go`
- `internal/controlplane/apiserver/auth/authorizer.go`
- `internal/controlplane/apiserver/options/auth.go`
- `internal/controlplane/apiserver/config.go`
- `tools/hack/verify-apiserver-auth.sh`
- `tools/hack/write-apiserver-kubeconfig.sh`
- `tools/hack/verify-apiserver-kubectl.sh`

你可以先这样理解：
- `options/auth.go`：命令行配置入口
- `authenticator.go`：把 HTTP 请求识别成哪个用户
- `policy.go`：定义静态授权策略模型
- `authorizer.go`：按策略判断这个用户能不能做这件事
- `config.go`：把前面这些东西真正接进 `generic apiserver`
- `verify-*`：证明这套链路真的工作，不是只写了代码

## 2. 为什么先有 `options/auth.go`

`options/auth.go` 负责把认证授权需要的启动参数集中起来。

它现在定义了：
- `AdminToken`
- `AdminUser`
- `AdminGroups`
- `ViewerToken`
- `ViewerUser`
- `ViewerGroups`
- `AnonymousPaths`
- `AuthorizationPolicyFile`

为什么不直接把这些值散写在 `config.go` 里？

因为那样会有三个问题：
1. 配置来源不清楚
2. 默认值和运行时值混在一起
3. 后面想替换成文件策略、环境注入时会越来越乱

所以这里继续沿用了前面 apiserver 的一贯做法：
- 先放进 `Options`
- 再在 `config.go` 里应用到运行时配置

## 3. 为什么要有 `constants.go`

`constants.go` 的作用不是“好看”，而是避免魔法值到处散落。

这里集中定义了：
- 默认管理员 token
- 默认管理员用户名
- 默认 viewer token
- 默认 viewer 用户名
- 默认 viewer 组
- 默认 `Authorization` 头前缀
- 默认匿名开放路径

为什么这么做？

因为认证授权是最容易出现“看起来能跑，但细节到处散”的地方。

比如如果：
- token 字符串到处写一遍
- `system:masters` 到处写一遍
- `ingate:viewers` 到处写一遍
- `/healthz`、`/openapi` 这些路径到处写一遍

那后面排查和修改都会很痛苦。

## 4. `authenticator.go` 在做什么

这里最核心的事情是：

**把一个 HTTP 请求识别成哪个用户。**

当前实现分两步：
1. 先试 bearer token
2. 如果不是 bearer token，再试匿名路径规则

对应代码里你会看到：
- `requestbearertoken.New(...)`
- `requestunion.New(...)`
- 自己写的 `prefixAnonymousAuthenticator`

### 为什么先试 bearer token

因为资源请求主要靠它。

如果请求头里有：

```text
Authorization: Bearer ingate-dev-admin-token
```

它会被映射成内置管理员用户。

如果请求头里有：

```text
Authorization: Bearer ingate-dev-viewer-token
```

它会被映射成内置查看者用户。

### 为什么要内置 viewer

因为只放管理员，你学到的只是：
- 带 token 能过
- 不带 token 不能过

但这不够说明“授权”真的存在。

viewer 的作用是让你能真实看到：
- 同样都通过了认证
- 但其中一个只能读
- 另一个能写

这才是更像正式控制面的边界。

### 为什么匿名部分要自己写 `prefixAnonymousAuthenticator`

因为我们当前需要的是：
- `/openapi`
- `/openapi/v2`
- `/openapi/v3`
- `/apis`
- `/apis/...`

这种“按路径前缀放行”的效果。

如果这里只做精确路径匹配，那：
- `/openapi` 能过
- 但 `/openapi/v2`、`/openapi/v3` 反而失败

这就是为什么这里要做自定义前缀匹配。

## 5. `policy.go` 在做什么

这是这次 authz 变化里最关键的一层。

它定义的是一份**静态授权策略模型**。

核心结构是：
- `Policy`
- `PolicyRule`

每条规则可以描述：
- 哪些用户
- 哪些用户组
- 哪些动词
- 哪些 API group
- 哪些资源
- 哪些资源名
- 哪些非资源路径

为什么要单独搞这一层，而不是继续在 `authorizer.go` 里写死？

因为继续写死会越来越快失控。

比如前一版那种写法，本质是：
- `if system:masters -> allow`
- `if anonymous public path -> allow`
- 其它 deny

一旦加上 viewer，再往后加 controller、admin-api、kubectl 等角色，很快就会变成一堆散乱分支。

静态策略模型的价值是：
- 规则变成数据，而不是硬编码流程
- 默认策略和自定义策略文件的边界清楚
- 后面扩展不会那么痛

## 6. `authorizer.go` 在做什么

认证只解决“你是谁”。

授权解决的是：

**你是谁之后，你能不能做这件事。**

当前授权逻辑不再是散写规则，而是：
1. 加载默认策略或外部策略文件
2. 遍历规则
3. 看用户是否匹配
4. 看请求属性是否匹配
5. 命中则允许，否则拒绝

默认策略现在至少包含 4 类规则：
- 匿名只允许访问公共非资源路径
- `system:masters` 允许所有资源请求
- `system:masters` 允许所有非资源请求
- `ingate:viewers` 只允许对 `gateway.ingate.io` 和 `policy.ingate.io` 做 `get/list/watch`

这比前一版更像真正的授权模型。

## 7. `config.go` 是怎么把它们接进去的

这一步是整个认证授权链路真正生效的关键。

在 `BuildGenericConfig(...)` 里，现在会做两件事：
1. 创建 request authenticator
2. 创建基于静态策略的 authorizer

然后把它们塞进 `generic apiserver`：
- `genericConfig.Config.Authentication.Authenticator = ...`
- `genericConfig.Config.Authorization.Authorizer = ...`

为什么这里才接？

因为：
- `options` 只是参数
- `auth/*.go` 只是能力实现
- 只有 `config.go` 才是把能力装到 apiserver 运行时里的地方

## 8. 为什么现在不直接上完整 Kubernetes RBAC / 委托链

可以做，但当前阶段不值得。

原因有三个：
1. 你现在的重点是把资源型 apiserver 学明白
2. 完整 RBAC / 委托链会引入更多集成前提
3. 那会把学习重点从资源主链拉到平台安全体系

所以当前实现选择的是：
- 保留 `generic apiserver` 的接线方式
- 但用一套更小、更容易看懂的 authenticator + static policy authorizer

这不是偷懒，而是当前阶段的合理收敛。

## 9. kubeconfig 和 kubectl 为什么也放在这组里

因为这两个东西可以直接证明 authz 已经不只是 curl 演示。

- `write-apiserver-kubeconfig.sh`
  - 负责生成一份同时包含 admin 和 viewer context 的 kubeconfig
- `verify-apiserver-kubectl.sh`
  - 负责用 admin context 创建资源
  - 再用 viewer context 做 `kubectl get`
  - 最后验证 viewer `kubectl create` 会被拒绝

这一步的价值很高，因为它说明：
- 你的 apiserver 已经不只是“能被 curl 调”
- 它已经足够像一个 Kubernetes 风格 apiserver，能让 `kubectl` 按发现、表格输出、权限边界正常工作

## 10. 这块里哪些是手写的，哪些不是

### 手写的
- `options/auth.go`
- `auth/constants.go`
- `auth/authenticator.go`
- `auth/policy.go`
- `auth/authorizer.go`
- `write-apiserver-kubeconfig.sh`
- `verify-apiserver-auth.sh`
- `verify-apiserver-kubectl.sh`

### 不是这块关心的生成代码
- `pkg/generated/*`
- `pkg/generated/openapi/*`
- `pkg/generated/proto/*`

为什么这里几乎全是手写的？

因为认证授权逻辑本来就不是靠生成器产出的。

生成器更适合：
- API 类型辅助代码
- clientset/informer/lister
- proto/openapi 输出

认证授权属于：
- 运行时策略
- 业务安全边界
- 启动配置装配

所以它天然是手写代码。

## 11. 读这块代码时最容易搞混的点

### 容易搞混 1：认证和授权是一回事
不是。
- 认证：识别用户
- 授权：判断权限

### 容易搞混 2：匿名就是没认证
不是。
在当前实现里，匿名也是一种明确身份。

### 容易搞混 3：viewer token 和 admin token 只是两个字符串
不是。
它们对应的是不同用户和不同用户组，最后命中的是不同授权规则。

### 容易搞混 4：为什么授权现在要从硬编码改成静态策略
因为角色一多，硬编码分支会越来越乱，而静态策略模型能把规则变成数据。

## 12. 这一层代码对后面有什么价值

它给后面的 controller 阶段带来的价值不只是“更安全”。

更重要的是：
- 资源面已经有明确边界
- 公共探活和 schema 发现也有明确边界
- 现在已经能真实区分只读和可写身份
- `kubectl` 已经能通过不同 context 体现这套边界

这意味着后面再加 admin-api、controller-manager 时，不会继续拿“匿名资源访问”或“所有 token 都是管理员”当前提。
