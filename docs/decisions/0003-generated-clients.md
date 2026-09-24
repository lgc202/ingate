# 0003：生成供 API Server 调用方使用的类型化客户端

- 状态：待评审

## 背景

Ingate 已提供 Gateway、Route、Upstream 的 Generic API Server 存储和外部版本类型。后续 Admin API 需要调用这些资源的增删查改；由调用方手写资源路径、序列化和版本配置，会与现有的资源类型重复维护。

## 本轮实现

`scripts/generate-api.sh` 使用与 API Server 依赖同版本的 `client-gen`，从外部版本类型上的 `+genclient` 标记生成 `pkg/generated/clientset/versioned`。输出包含三种集群级资源的类型化客户端及 fake 客户端。生成前只清理该目录内带 `client-gen` 标记的 Go 文件，保留手写测试；重复执行生成脚本应得到相同结果。CI 重新生成并检查仓库中的生成文件是否同步。

Admin API 是这套客户端的预期调用方。API Server 负责存储，Controller Manager 已通过 controller-runtime 的 Client 和 Cache 读取资源，因此它们不依赖新生成的 clientset。本轮不生成 Informer、Lister 或 ApplyConfiguration；目前没有需要独立 List-Watch 缓存或 Server-Side Apply 的调用方。

fake 客户端测试覆盖 Gateway、Route、Upstream 的 Create 和 Get，确认生成的资源路径和集群级访问方式。fake 不执行 API Server 的字段校验，也不证明真实网络调用成功；Admin API 接入后仍需单独验证与 API Server 的调用链路。
