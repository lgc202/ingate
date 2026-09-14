# 参与贡献

感谢你参与 Ingate。提交代码前，请先通过 Issue 说明可复现的问题或真实使用场景；小范围修复可以直接提交 Pull Request。

## 开发环境

仓库当前使用 Go 1.27、Node.js 24 和 npm 11。开发环境中的 Docker Compose 用于本地联调；正式发布另行提供固定版本的 Compose 安装包。

```bash
make check-tools
make tools
```

所有项目级 Go 工具都会安装到 `.tools/bin`，不会修改全局 `GOPATH/bin`。

## 开发流程

1. 保持改动聚焦，一个 Pull Request 只解决一个明确问题。
2. 修改 Proto、Wire 装配或其他生成源后执行 `make generate`。
3. 本地按改动范围执行必要的格式化、生成校验、测试或编译；`make verify` 只在 CI 中运行，本地开发和 Agent 不执行。
4. 涉及 Go 依赖或安全边界时额外执行 `make vuln`。
5. 通过功能分支创建 Pull Request，检查通过后合并；不要直接推送 `main`。

`make verify` 会执行 Go、Proto、GitHub Actions 静态检查，验证生成代码，并编译后端与两个前端项目。实际流量行为仍需通过组件联调验证。

`make docker-up` 会保留本地数据卷。当前项目不兼容开发阶段的旧表结构；需要按当前代码重新初始化所有本地数据时，执行 `make docker-reset`。该命令会删除当前 Ingate Compose 项目的全部开发数据卷。

## 代码与协议

具体编码与审查规则见 [Ingate 开发约定](AGENTS.md)，包括输入契约、抽象边界、命名、声明顺序、注释和重构验证要求。自动检查通过后仍需人工审查可读性，并说明实际验证范围和未覆盖项。

- 声明式资源、Admin API 和前端交互应保持各自清晰的协议边界。
- 不向外部 API 暴露 Envoy、xDS、插件 ABI 等内部实现细节。
- Go 标识符和日志使用英文；代码注释使用中文，并说明领域约束或不明显的原因。
- 不提交密钥、访问令牌、请求正文、运行数据和 `_output` 等本地产物。

提交信息建议使用 `type: summary` 形式，例如 `fix: reject duplicate gateway names`。
