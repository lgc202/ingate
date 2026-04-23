# Ingate 设计文档索引

这组文档描述 `Ingate` 的 v1 架构设计。

当前设计基线是：

- 参考 `OneX` 的控制面与工程组织方式
- 参考 `Higress` 的网关业务模型与 Envoy/xDS 主链路
- 目标是企业级、可独立部署、可横向扩展、可逐步接近 K8s 风格控制面

## 阅读顺序

1. [Ingate 核心概念](./00-core-concepts.md)
2. [Ingate v1 总览](./01-overview.md)
3. [Ingate v1 资源模型](./02-resource-model.md)
4. [Ingate v1 控制面核心](./03-control-plane.md)
5. [Ingate v1 发布链路与 xDS](./04-delivery-and-xds.md)
6. [Ingate v1 管理接口与 API 分层](./05-api-and-management.md)
7. [Ingate v1 组件通信与安全边界](./06-component-communication-and-security.md)
8. [Ingate v1 代码架构与生成链路](./07-code-architecture-and-codegen.md)
9. [Ingate v1 可用性、恢复与测试](./08-reliability-and-testing.md)
10. [Ingate v1 Console 设计](./09-console-design.md)

## 每篇文档解决什么问题

### 00. 核心概念

解释这套架构里最常出现的概念：

- `apiserver`
- `controller-manager`
- `spec/status`
- `list/watch`
- `IR`
- `xDS`
- `Envoy`

### 01. 总览

回答：

- `Ingate` 到底是什么
- 顶层组件有哪些
- 主链路怎么走

### 02. 资源模型

回答：

- `Gateway / Route / Backend / AuthPolicy / TrafficPolicy` 怎么定义
- 哪些是 v1 范围
- 哪些能力留到后续扩展

### 03. 控制面核心

回答：

- `controller-manager` 内部怎么分层
- 为什么必须有 `IR`
- 策略如何合并
- 统一状态模型是什么

### 04. 发布链路与 xDS

回答：

- `xds-server` 内部怎么拆
- `controller-manager -> xds-server` 的发布契约是什么
- `EffectiveConfig` 传什么，不传什么

### 05. 管理接口与 API 分层

回答：

- `ingate-apiserver` 和 `admin-api` 如何分工
- 资源接口和产品接口为什么必须分层
- `admin-api` 的工作流边界是什么

### 06. 组件通信与安全边界

回答：

- 各组件如何通信
- 哪些调用方向被禁止
- 组件之间如何建立信任关系

### 07. 代码架构与生成链路

回答：

- 目录结构怎么组织
- `pkg/apis`、`proto`、`pkg/generated` 怎么分工
- codegen 与 Make 主线怎么收口

### 08. 可用性、恢复与测试

回答：

- 重启后如何恢复
- 各组件如何做 HA 和横向扩展
- 测试如何分层覆盖关键风险

### 09. Console 设计

回答：

- 企业级前端控制台第一版怎么设计
- 为什么先做静态高保真原型
- 左侧导航、顶部栏、资源列表、详情抽屉和拓扑视图怎么组织
- `Console` 后续如何通过 `admin-api` 接入真实数据
