# 00 先把代码地图看明白

先不要急着看具体函数。
先知道代码都放在哪里。

## 1. 最重要的目录

### `cmd/apiserver/`
这里放启动入口。

你可以把它理解成：
- 怎么接受命令行参数
- 怎么组织启动链
- 怎么把服务真正跑起来

### `internal/controlplane/apiserver/`
这里放 apiserver 本体实现。

包括：
- config
- server
- install
- registry
- strategy
- storage

### `pkg/apis/`
这里放资源类型定义。

不是对外 HTTP API。
而是：
- apiserver 认识的资源对象类型
- group/version/type 的源定义

### `pkg/generated/`
这里放生成代码。

### `proto/`
这里放内部 gRPC 契约源文件。

### `tools/hack/`
这里放工程脚本。

### `Makefile`
这里放统一开发入口。

## 2. 为什么这样分

因为这套目录在表达 3 层不同的东西：

1. **运行入口**
- `cmd/`

2. **业务/控制面实现**
- `internal/controlplane/apiserver/`

3. **资源和生成链**
- `pkg/apis/`
- `pkg/generated/`
- `proto/`

如果你不先把这个地图看懂，后面会一直把：
- 资源定义
- 运行入口
- 生成物
混在一起。
