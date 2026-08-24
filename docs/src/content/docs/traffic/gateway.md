---
title: 网关入口
description: 配置 HTTP、HTTPS、监听端口、域名和证书
---

Gateway 定义流量从哪里进入 Ingate。它只负责监听协议、端口、域名和 TLS；请求匹配与转发目标由 Route 管理。

## 创建入口

在 Console 中进入 **流量管理 → 网关**，选择 **创建网关**：

1. 填写一个便于识别的名称
2. 添加 HTTP 或 HTTPS 监听入口
3. 设置数据面实际监听的端口
4. 按需填写域名；留空表示接受任意 Host
5. HTTPS 入口选择已经创建的证书

同一 Gateway 可以包含多个监听入口。一套 Ingate 也可以创建多个 Gateway，它们最终发布到同一组配置完全相同的 Envoy 实例。

## 域名与端口冲突

同协议、同端口的入口只有在域名范围互不重叠时才能共存：

- 留空域名会与同端口的任意其他域名冲突
- `*.example.com` 会与 `api.example.com` 冲突
- `api.example.com` 与 `admin.example.com` 可以共享端口
- 同一端口不能同时声明 HTTP 和 HTTPS

保存前 Admin API 会检查当前资源；Controller 仍会对并发写入后的完整资源集合做最终裁决。

## HTTPS

HTTPS 入口必须引用 Certificate。多个 HTTPS 入口可以在同一端口上按域名选择不同证书，Envoy 使用 SNI 选择对应配置。

证书的有效期不会阻止资源保存。过期、尚未生效或域名不匹配会通过资源状态反馈，不会静默替换用户配置。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: Gateway
metadata:
  name: 5cb83268-6e5c-42af-a4d0-3f40fd449b66
spec:
  displayName: public-gateway
  enabled: true
  listeners:
    - name: http
      protocol: HTTP
      port: 8080
      hostname: "*.example.com"
    - name: https
      protocol: HTTPS
      port: 8443
      hostname: "*.example.com"
      certificateRef: 51d0a788-8104-49fa-97d5-1e2f29f592f9
```

`metadata.name` 是不可变资源 ID，`spec.displayName` 是用户可编辑的名称。停用 Gateway 后，它会在停用配置成功发布时退出数据面监听。

## 当前边界

当前支持 HTTP、HTTPS、域名与通配域名、端口和证书引用；暂不提供自定义监听 IP、mTLS、TLS 加密套件或单个入口的高级 Envoy FilterChain 参数。
