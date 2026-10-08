# 0009：普通服务日志初始化与组件日志边界

- 状态：已接受（用户已确认提交）
- 日期：2026-10-07

Admin API 原先直接使用默认 slog，格式和输出缺少配置入口。Controller Manager 的 controller-runtime 已使用 klog，连接池关闭日志却另走 slog。依据 GoCanon 已接受的 ADR 0045，统一初始化责任，不统一三个组件的日志调用 API。

`internal/pkg/log` 接收普通 Go Config，返回原生 `*slog.Logger`、关闭函数和 error；不依赖组件配置、封装日志方法或设置全局 Logger。支持 debug/info/warn/error、JSON/text、stdout/stderr/file 组合和调用位置；输出不能重复。多输出复用 Go 1.27 的 slog.NewMultiHandler，一个目标失败后仍尝试其他目标。

Admin API 的可选 Log protobuf 经严格加载和校验，再转换为普通 Config。在创建客户端前构造 Logger，附加稳定 service 字段，将其传给 Kratos。组件退出时的独立清理错误使用该实例；成功装配后，Kratos 设置默认 Logger，既有请求未知错误仍只由原有 ToTransportError 边界记录，不新增请求日志中间件或原始请求参数记录。默认日志从旧默认文本/stderr 变为 JSON/stdout，省略 Log 配置也采用新默认值。

API Server 和 Controller Manager 保留 Kubernetes cli.Run 的日志 Flag、初始化和 FlushLogs。Manager 连接池关闭日志改用 klog.FromContext(ctx)，不引入 slog 桥接或第二个全局日志后端。启动失败在尚未取得框架日志时仍由既有命令入口输出一次。

文件输出完整沿用 ADR 0045，复用已锁定的 lumberjack v2.2.1，并将其从间接依赖改为直接依赖，不引入新模块或升级版本。选择 file 时必须提供路径；大小为零沿用上游 100 MiB 默认值，备份数量或保留天数为零时不按该维度清理，二者均为零时保留全部备份。可以启用 gzip 压缩。相对路径以进程工作目录为基准，多进程必须使用不同路径。

初始化拒绝目录及其他非普通文件，避免 lumberjack 的失败后轮转行为移动目录；它不提前打开文件或确认运行期可写性。Run 在获得关闭函数后立即安排 defer，覆盖后续构造失败、运行失败和正常停止。关闭错误与原运行错误通过 errors.Join 保留；连接池清理日志先于日志文件关闭。仅使用标准流时关闭函数为空操作，不关闭 stdout/stderr。

关闭函数沿用 lumberjack.Close，只关闭当前日志文件，不等待后台压缩和备份清理，也不终止其后台维护 goroutine。组件按单进程启动一次的契约使用，退出时进程回收后台任务；不将 Close 描述为全部后台资源已结束。首次写入失败、权限变化、磁盘满或采集器故障都可能丢失日志；slog 便捷方法不向调用方返回写入错误，普通运行日志不是可靠审计。

测试覆盖默认值、级别过滤、两种格式、输出选择和组合、调用位置、非法或重复配置、单目标写入失败、共享包不改变全局 Logger、protobuf 转换，以及 Kratos 的 Logger 和版本装配。真实临时文件检查首次写入、追加、大小轮转、数量及天数清理、零值保留和 gzip 内容；标准流关闭责任单独检查。关闭函数的成功、单独失败和与运行同时失败均验证原因保留；临时 Go overlay 注入关闭错误，确认 Run 的启动失败路径执行关闭并保留两个原因，不修改仓库实现。装配测试不启动端口或连接真实数据库，不证明外部日志采集、服务联合启动或生产部署安全。无动态级别、热更新、远程导出、审计和请求级观测扩展。

默认示例仍只选择 stdout。需要文件落盘时显式增加配置：

```yaml
log:
  level: info
  format: json
  outputs: [stdout, file]
  file:
    path: logs/admin-api.log
    max_size_mb: 100
    max_backups: 7
    max_age_days: 7
    compress: true
```
