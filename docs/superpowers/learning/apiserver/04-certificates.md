# 04 证书和 HTTPS

当前 `ingate-apiserver` 默认是 HTTPS 服务。

所以它启动时必须有：

- server certificate
- private key

## 1. 默认证书从哪里来

当前默认会自动准备自签名证书。

证书通常出现在：

```text
_output/certificates/apiserver.crt
_output/certificates/apiserver.key
```

它不是手写在仓库里的。

它是在 secure serving 配置完成阶段生成或复用的。

代码链路在：

```text
cmd/apiserver/main.go
-> cmd/apiserver/app/server.go
-> cmd/apiserver/app/options/options.go
-> internal/controlplane/apiserver/options/options.go
-> SecureServing.MaybeDefaultWithSelfSignedCerts(...)
```

## 2. 为什么 curl 要用 `-k`

本地自签名证书不被系统信任。

所以开发验证时会用：

```bash
curl -k https://127.0.0.1:18443/healthz
```

`-k` 的意思是：

**跳过客户端对服务端证书链的信任检查。**

这只适合本地开发。

## 3. 为什么 apiserver 不直接用 HTTP

因为 Kubernetes apiserver 默认就是 secure serving。

即使是本地开发，也应该尽早接近真实形态。

这样后续接 kubectl、client-go、认证授权时，不会再大改启动模型。

## 4. 如果以后使用自己的证书怎么办

原则上不改业务代码。

应该通过启动参数传入：

- cert file
- key file
- bind address
- secure port

也就是说，证书属于部署配置，不属于资源业务逻辑。

## 5. 当前阶段的边界

当前做的是开发态可用证书。

还不是生产证书体系。

生产环境后续要考虑：

- 证书签发来源
- CA 信任链
- 证书轮换
- 证书过期监控
- 组件间 mTLS
