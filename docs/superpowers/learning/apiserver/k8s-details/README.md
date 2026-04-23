# K8s apiserver 开发细节

这组文档专门解释那些第一次看 Kubernetes 风格项目时最容易懵的东西。

它不按源码启动顺序讲。

它按“你开发时会遇到的疑问”讲。

## 推荐阅读顺序

1. [00-api-markers-and-comments.md](./00-api-markers-and-comments.md)
2. [01-code-generation-tools.md](./01-code-generation-tools.md)
3. [02-generated-files-explained.md](./02-generated-files-explained.md)
4. [03-scheme-gvk-resource.md](./03-scheme-gvk-resource.md)
5. [04-registry-strategy-storage.md](./04-registry-strategy-storage.md)
6. [05-openapi-and-managedfields.md](./05-openapi-and-managedfields.md)
7. [06-development-workflow.md](./06-development-workflow.md)

## 这组文档解决什么问题

你读完后应该能回答：

- `// +k8s:deepcopy-gen=package` 是什么
- `// +genclient` 是什么
- `// +groupName=...` 为什么放在 `doc.go`
- 哪些注解是给 `deepcopy-gen` 看
- 哪些注解是给 `client-gen` 看
- 哪些注解是给 `openapi-gen` 看
- 为什么会有 `zz_generated.*.go`
- 为什么生成代码不能手改
- 为什么改完 API 类型必须重新生成
- `apiVersion`、`kind`、GVK、Resource、REST path 是什么关系
- 为什么 Kubernetes 项目里总是看到 `Scheme`
- registry、strategy、storage 分别管什么
- OpenAPI 为什么会影响 `managedFields`

## 先记一个总原则

Kubernetes 风格 API 项目里，很多代码不是“业务代码”，而是“API machinery 需要的胶水”。

这些胶水代码通常由工具生成。

你手写的是语义源头：

- 资源类型
- 默认值
- 校验规则
- REST 行为
- 存储策略

工具生成的是重复胶水：

- DeepCopy
- clientset
- lister
- informer
- OpenAPI schema
- model name

这就是为什么目录看起来比普通 Go HTTP 服务复杂。
