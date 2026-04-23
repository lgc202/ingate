# 11 静态策略授权和 kubeconfig 是怎么接进去的

这篇讲两件事：

1. 为什么现在的授权从“硬编码放行”升级成了静态策略
2. 为什么 `kubectl` 能直接连 `ingate-apiserver`

## 1. 为什么 authz 要继续收

前一版授权只有一个很小的规则：

- `system:masters` 放行
- 公共非资源路径匿名放行
- 其它全部拒绝

它能工作，但不够像一个正式控制面。

现在我们把它推进成：

- 内置默认静态策略
- 可选外部策略文件
- admin / viewer 两种内置身份

## 2. 关键代码在哪里

- [auth constants](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/auth/constants.go)
- [auth options](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/options/auth.go)
- [auth authenticator](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/auth/authenticator.go)
- [auth authorizer](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/auth/authorizer.go)
- [auth policy](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/auth/policy.go)
- [config](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/config.go)
- [write kubeconfig script](/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/write-apiserver-kubeconfig.sh)
- [verify kubectl script](/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/verify-apiserver-kubectl.sh)

## 3. 静态策略长什么样

现在 authorizer 不再只看几条硬编码 if/else。
它会加载一个 `Policy`：

- `Users`
- `Groups`
- `Verbs`
- `APIGroups`
- `Resources`
- `ResourceNames`
- `NonResourceURLs`

这套模型故意没有直接做成完整 RBAC API 对象。
原因是：

1. 当前阶段先要一个好学、好控的最小授权系统
2. 先把授权边界讲清楚，比直接引入完整 RBAC 更重要
3. 后面如果需要，可以再演进

## 4. 默认策略做了什么

默认策略里最关键的三类规则是：

1. 匿名用户可以访问公共非资源路径
2. `system:masters` 拥有完整访问权限
3. `ingate:viewers` 只能对 `gateway.ingate.io` / `policy.ingate.io` 做只读访问

这就是为什么我们新增了 `viewer` token 和 `viewer` context。

## 5. 为什么 authenticator 也要改

如果只有一个 admin token，authz 再复杂也演示不出效果。

所以 `authenticator.go` 现在不只注册：
- admin token

还注册：
- viewer token

这样才能真正验证：

- admin create 成功
- viewer get 成功
- viewer create 失败

## 6. 为什么要有 kubeconfig 生成脚本

你当然可以继续手写：

- server
- token
- context

但这会出现两个问题：

1. 文档很容易漂
2. 每次验证都在重复搭环境

所以我们把 kubeconfig 写成正式脚本：

- [write-apiserver-kubeconfig.sh](/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/write-apiserver-kubeconfig.sh)

这样它就变成工程的一部分，而不是文档里的临时步骤。

## 7. 为什么 `kubectl get` 能证明很多事

当 `kubectl get gateways` 真能跑通时，实际证明的不只是“HTTP 通了”。

它同时证明：

1. kubeconfig 可用
2. TLS 入口可用
3. authn/authz 可用
4. discovery 可用
5. 资源 API 路径符合 Kubernetes 习惯
6. 服务端 Table 输出可用

所以 `verify-apiserver-kubectl` 是一个价值很高的端到端验证。

## 8. 这块和“魔法字符串”有什么关系

如果把：

- viewer token
- viewer group
- context name
- cluster name
- public path pattern

这些字符串散在各处，后面这条链会非常难维护。

所以这里有两个收口点：

1. Go 侧常量与默认策略
2. 脚本侧固定变量与统一输出文件

这样你查问题时，至少知道该去哪里找，而不是在整个仓库里 grep 一圈。

## 9. 你读完后应该能回答

1. 为什么现在 authz 不再只是一个硬编码 authorizer
2. 为什么要新增 viewer 身份
3. 为什么 kubeconfig 生成脚本属于正式工程能力
4. 为什么 `kubectl get` 是很有价值的集成验证
