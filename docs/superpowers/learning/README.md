# Learning

这里放 Ingate 的学习文档。

当前已经有两个学习入口：

- [apiserver/README.md](./apiserver/README.md)
- [admin-api/README.md](./admin-api/README.md)
- [controller-manager/README.md](./controller-manager/README.md)
- [xds-server/README.md](./xds-server/README.md)
- [envoy/README.md](./envoy/README.md)

当前建议顺序：

1. 先把 `ingate-apiserver` 跑起来
2. 再用脚本和手工命令验证它真的能工作
3. 再补 Kubernetes apiserver 开发必备概念
4. 再读 apiserver 源码
5. 然后进入 `admin-api`，理解产品 API 如何调用 apiserver
6. 再进入 `controller-manager`，理解控制循环如何收敛资源
7. 再进入 `xds-server`，理解发布链路和 discovery
8. 最后看 `envoy`，理解真实流量如何命中并转发

为什么这样安排？

因为你现在不是只在学一个 Go 服务。

你实际在学的是一套 Kubernetes 风格控制面开发方式：

- 资源对象怎么定义
- `apiVersion/kind` 为什么重要
- `spec/status` 为什么分开
- `Scheme`、GVK、Resource 是什么关系
- `registry/strategy/storage` 各自负责什么
- `deepcopy-gen`、`defaulter-gen`、`client-gen`、`openapi-gen` 为什么存在
- 生成代码哪些能改，哪些不能改
- Makefile 和 `tools/hack` 为什么要做成工程入口

所以文档不会只告诉你“执行什么命令”。

它会明确解释：

- 这个命令在证明什么
- 这个文件为什么存在
- 这个注解给谁看
- 这个生成物怎么来的
- 这个设计和 kube-apiserver / OneX 的关系是什么

历史学习入口已经合并为当前这一个 apiserver 主目录。
