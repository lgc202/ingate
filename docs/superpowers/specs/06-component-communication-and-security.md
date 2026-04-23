# Ingate v1 组件通信与安全边界

## 1. 目标

这份文档统一回答两个问题：

- 各组件如何通信
- 各组件之间如何建立安全边界与信任关系

## 2. 总体通信关系

```text
admin-api / console / cli
  -> ingate-apiserver
  -> etcd

controller-manager
  -> list/watch ingate-apiserver

service-discovery
  -> gRPC to controller-manager

xds-server
  -> list/watch ingate-apiserver
  <-> Envoy via ADS/xDS
  -> update status to ingate-apiserver
```

## 3. 各链路协议

### 3.1 `admin-api -> ingate-apiserver`

- 协议：`HTTPS REST/JSON`
- 用途：写入/读取声明式资源

### 3.2 `ingate-apiserver -> etcd`

- 协议：`etcd v3 gRPC`
- 用途：资源持久化、版本、watch 基础能力

### 3.3 `controller-manager -> ingate-apiserver`

- 协议：`list/watch + status update`
- 底层：`HTTPS + watch stream`
- 用途：监听资源变化、读取资源、回写状态

### 3.4 `service-discovery -> controller-manager`

- 协议：`gRPC`
- 用途：返回 endpoint 视图与解析结果

### 3.5 `xds-server -> ingate-apiserver`

- 协议：`list/watch + status update`
- 底层：`HTTPS + watch stream`
- 用途：消费 `ResolvedGateway`、回写 `Programmed`

### 3.6 `xds-server <-> Envoy`

- 协议：`ADS/xDS gRPC` 双向流
- 用途：下发 LDS/RDS/CDS/EDS/SDS，接收 ACK/NACK

### 3.7 `数据面/运维工具 -> xds-server`

- 协议：`gRPC`
- 用途：发现后端 endpoint、消费当前发布结果

## 4. 明确禁止的调用方向

禁止：

- `admin-api -> controller-manager`
- `admin-api -> xds-server`
- `controller-manager -> Envoy`
- `service-discovery -> xds-server`
- `controller-manager -> xds-server`
- `xds-server -> etcd`

原因：

- `apiserver` 必须是唯一资源真相源
- `controller-manager` 负责语义，不负责直接控制数据面
- `xds-server` 负责发布，不负责资源存储

## 5. 安全边界

### 5.1 外部入口

所有对外入口一律使用：

- `HTTPS`

包括：

- `admin-api`
- `ingate-apiserver`（如果对外开放）

### 5.2 内部组件通信

内部 gRPC 链路默认：

- `mTLS`

包括：

- `service-discovery -> controller-manager`
- `xds-server <-> Envoy`
- `数据面/运维工具 -> xds-server`

### 5.3 组件身份

每个组件应有独立服务身份，不复用：

- `apiserver`
- `controller-manager`
- `xds-server`
- `service-discovery`
- `admin-api`
- `gateway`

### 5.4 最小权限

- `controller-manager` 只拥有读资源与写状态的必要权限
- `xds-server` 不直接写 etcd
- `admin-api` 不直接驱动 controller/xds

## 6. 信任模型

`Ingate` v1 的信任边界应是：

1. 用户只信任 `admin-api` 和产品入口
2. 控制面内部只信任经过 mTLS 验证的组件身份
3. `ingate-apiserver` 是资源真相源
4. `etcd` 是底层持久化，不直接暴露给业务组件

## 7. 一句话结论

`Ingate` 的通信与安全边界应收敛为：

**所有配置都通过 `apiserver` 进入系统，`xds-server` 通过受控 watch/gRPC 面消费发布结果，所有内部组件默认使用 mTLS 和最小权限。**
