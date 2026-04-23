# apiserver 学习入口

这组文档只服务一个目标：

**让你从小白状态开始，把当前 `ingate-apiserver` 跑通、验证清楚、再读懂源码。**

不要先去啃 Kubernetes 源码。

更好的顺序是：

1. 先知道当前实现到了什么程度
2. 先跑起来
3. 先验证资源、鉴权、kubectl、Table、证书这些行为
4. 再补 Kubernetes apiserver 开发必备细节
5. 最后按源码链路读代码

## 第一阶段：使用和验证

先读这些：

1. [00-current-state.md](./00-current-state.md)
2. [01-quickstart.md](./01-quickstart.md)
3. [02-verify-resources.md](./02-verify-resources.md)
4. [03-auth-kubectl-table.md](./03-auth-kubectl-table.md)
5. [04-certificates.md](./04-certificates.md)
6. [05-generated-vs-manual.md](./05-generated-vs-manual.md)

这一阶段的目标不是读源码。

这一阶段只回答：

- 服务怎么启动
- etcd 在哪里参与
- 哪些 API 能访问
- 哪些请求会被拒绝
- kubectl 为什么能连
- 哪些代码是手写的
- 哪些代码是生成的

## 第二阶段：Kubernetes 开发细节

再读：

- [k8s-details/README.md](./k8s-details/README.md)

这部分专门解释你提到的那些“不知道怎么来的东西”：

- `+k8s:deepcopy-gen=package` 这种注解是什么
- `+genclient` 是给谁看的
- `zz_generated.deepcopy.go` 为什么不能手写
- `clientset/informer/lister` 怎么生成
- OpenAPI 和 `managedFields` 为什么会扯到一起
- `Scheme`、GVK、Resource、REST path 怎么对应
- Makefile、脚本、工具链为什么要这样组织

## 第三阶段：源码阅读

最后读：

- [reading/README.md](./reading/README.md)

这部分按代码链路读：

- `main` 怎么进来
- options 怎么变成 config
- config 怎么创建 generic apiserver
- API group 怎么安装
- registry / strategy / storage 怎么协作
- authn/authz/admission/OpenAPI/Table/证书怎么接进去

## 当前不做什么

现在暂时不讲：

- audit
- 多租户
- 生产级证书体系
- controller-manager 完整实现
- admin-api 完整产品接口
- xDS 发布链路

这些会在后续组件阶段继续补。
