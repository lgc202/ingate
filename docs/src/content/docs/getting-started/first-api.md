---
title: 转发第一个 API
description: 创建 Service、Gateway 和 Route，并通过 Ingate 访问 httpbin
---

本页使用公开的 `httpbin.org` 演示一次完整 HTTP 转发。完成前请先[安装 Ingate](../installation/)并登录 Console。

## 1. 创建 HTTP Service

进入 **流量配置 → 服务**，选择 **创建服务**：

| 配置 | 值 |
| --- | --- |
| 名称 | `Httpbin 演示服务` |
| 服务类型 | `HTTP` |
| 地址 | `httpbin.org` |
| 端口 | `80` |
| 权重 | `1` |

保存 Service。新安装中尚未存在可发布的 Gateway 和 Route 时，Service 可能暂时显示“发布中”；完成下面三类资源后再统一确认状态。

## 2. 创建 HTTP Gateway

进入 **流量配置 → 网关**，选择 **创建网关**：

| 配置 | 值 |
| --- | --- |
| 名称 | `演示网关` |
| 协议 | `HTTP` |
| 端口 | `8080` |
| 域名 | 留空 |

域名留空表示该入口接收任意 Host。

## 3. 创建 API Route

进入 **流量配置 → 路由**，选择 **创建路由**：

| 配置 | 值 |
| --- | --- |
| 名称 | `Httpbin API` |
| 路由类型 | `API` |
| 访问方式 | `公开访问` |
| 生效网关 | `演示网关` |
| 路径匹配 | 前缀 `/` |
| 目标服务 | `Httpbin 演示服务`，权重 `100` |
| 转发主机名 | `使用服务地址` |

“使用服务地址”会把上游请求 Host 设置为 `httpbin.org`。公网虚拟主机通常依赖正确 Host，因此这里不要选择“保持请求主机”。

## 4. 发送请求

等待 Gateway、Route 和 Service 都显示“已生效”，然后执行：

```bash
curl -i http://127.0.0.1:8080/get
```

预期响应包含：

```text
HTTP/1.1 200 OK
content-type: application/json
```

如果任一资源显示“异常”，打开详情查看原因并先修正该资源。同一套 Ingate 以完整配置域为单位发布；一条无法编译的资源会阻止本次候选配置生效，但不会撤销上一版已经生效的配置。

## 5. 查看请求结果

进入 **观测分析 → 请求记录**。请求记录异步写入 ClickHouse，通常会有数秒延迟。打开记录详情可以看到：

- 命中的 Gateway 和 Route
- 最终 Service 与端点地址
- HTTP 响应状态
- 总耗时和首字节耗时

流量趋势和资源排行位于[流量分析](../../observability/traffic-analysis/)。
