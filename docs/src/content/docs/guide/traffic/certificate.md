---
title: 证书
description: 上传、复用和轮换 Gateway HTTPS 证书
---

Certificate 保存 Gateway HTTPS 入口使用的服务器证书和私钥。一个证书可以被多个 Gateway 入口复用。

## 创建证书

Console 提供两种互斥输入方式：

- 上传证书文件与私钥文件
- 手动粘贴 PEM 文本

两种方式最终都提交 PEM 字符串。系统不保存原始文件名或上传路径，因此文件名不会成为运行配置的一部分。

保存时会校验：

- 证书和私钥必须同时提供
- PEM 格式有效
- 证书与私钥相互匹配
- 能解析叶子证书的域名和有效期

证书尚未生效或已经过期不会阻止保存，但状态会明确提示问题。

## 安全边界

列表和详情可以返回证书摘要与公钥证书，但任何 Admin API 都不会返回私钥。更新名称时可以不提交密钥材料；替换证书时必须同时提交新的证书和私钥。

仍被 Gateway 引用的 Certificate 不能删除。轮换时应先完整替换密钥对，等待资源重新生效，再清理不再引用的证书。

## 声明式资源

```yaml
apiVersion: gateway.ingate.io/v1
kind: Certificate
metadata:
  name: 51d0a788-8104-49fa-97d5-1e2f29f592f9
spec:
  displayName: example-com
  certificatePEM: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  privateKeyPEM: |
    -----BEGIN PRIVATE KEY-----
    ...
    -----END PRIVATE KEY-----
```

当前只支持内联 PEM，不提供 ACME 自动签发、自动续期、外部 Secret 引用、PKCS#12 或客户端 mTLS。
