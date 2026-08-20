# Certificate 资源

Certificate 保存 Gateway HTTPS Listener 使用的服务器证书和私钥。证书是可复用资源，多个 Gateway Listener 可以引用同一个 Certificate。

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

`metadata.name` 是不可变资源 ID，资源引用始终使用该 ID。`spec.displayName` 是用户可编辑且同类资源内唯一的展示名称。

Certificate 不保存上传方式、原始文件名或来源。文件上传和手动粘贴只是控制台的两种输入方式，Admin API 最终都接收 PEM 字符串。API Server 会去除首尾空白，并统一保留一个结尾换行。

证书和私钥必须同时存在、使用有效 PEM 格式且相互匹配。系统解析叶子证书得到 DNS 名称和有效期，但不会因为证书当前尚未生效或已经过期而拒绝保存；运行状态由 Controller 负责表达。

## Admin API

列表只返回证书摘要，不返回 PEM：

```json
{
  "id": "51d0a788-8104-49fa-97d5-1e2f29f592f9",
  "name": "example-com",
  "dnsNames": ["example.com", "*.example.com"],
  "notBefore": "2026-08-01T00:00:00Z",
  "notAfter": "2027-08-01T00:00:00Z",
  "state": "READY",
  "message": "配置已生效",
  "version": 2,
  "createdAt": "2026-08-10T10:00:00Z",
  "updatedAt": "2026-08-10T10:15:00Z"
}
```

详情、创建和更新响应会额外返回 `certificatePEM`，以便用户查看公钥证书。任何接口都不返回 `privateKeyPEM`。

更新时 `certificatePEM` 和 `privateKeyPEM` 都是可选字段：

- 两个字段都省略，只更新展示名称并保留当前证书
- 两个字段同时提供，完整替换证书和私钥
- 只提供其中一个，或者提交空字符串，返回请求错误
- Certificate 不支持清空密钥材料；不再使用时应删除资源

查询、创建和更新接口直接返回 Certificate，删除成功返回空对象。列表使用不透明的 `limit/cursor` 游标分页。更新和删除必须提交映射 `metadata.generation` 的 `version`；Controller 更新 status 不改变版本。

仍被 Gateway Listener 引用的 Certificate 不能删除。Admin API 在删除前提供即时引用校验，Controller status 负责声明式 API 并发写入后的最终裁决。

## MVP 边界

当前只支持内联 PEM 证书，不支持 ACME 自动签发、证书自动续期、外部 Secret 引用、SDS、mTLS 客户端证书、PKCS#12/JKS、证书链编辑和私钥导出。
