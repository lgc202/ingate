# 参与贡献

感谢你参与 Ingate。提交代码前，请先通过 Issue 说明可复现的问题或真实使用场景；小范围修复可以直接提交 Pull Request。

## 开发环境

仓库当前使用 Go 1.26、Node.js 24 和 npm 11。Docker Compose 仅用于本地联调。

```bash
make check-tools
make tools
```

所有项目级 Go 工具都会安装到 `.tools/bin`，不会修改全局 `GOPATH/bin`。

## 开发流程

1. 保持改动聚焦，一个 Pull Request 只解决一个明确问题。
2. 修改 Proto、Wire 装配或其他生成源后执行 `make generate`。
3. 提交前执行 `make verify`。
4. 涉及 Go 依赖或安全边界时额外执行 `make vuln`。

`make verify` 会执行 Go、Proto、GitHub Actions 静态检查，验证生成代码，并编译后端与两个前端项目。实际流量行为仍需通过组件联调验证。

## 代码与协议

- 声明式资源、Admin API 和前端交互应保持各自清晰的协议边界。
- 不向外部 API 暴露 Envoy、xDS、插件 ABI 等内部实现细节。
- Go 标识符和日志使用英文；代码注释使用中文，并说明领域约束或不明显的原因。
- 不提交密钥、访问令牌、请求正文、运行数据和 `_output` 等本地产物。

提交信息建议使用 `type: summary` 形式，例如 `fix: reject duplicate gateway names`。
