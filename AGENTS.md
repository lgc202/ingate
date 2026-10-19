# 代码评审入口

本仓库采用 GoCanon 的 Go 规范。每次请用户评审代码或提交前，按以下顺序检查当前变更；用户未确认前不要提交。

## GoCanon 接入

本次接入核对的服务与规则版本为 `880007d44098493410c4ce8e507cd383ff1a660c`，规则源工作区 clean。它是核对记录，不代表运行中的服务永远使用该版本；每次任务先调用 `gocanon_list_capabilities`，记录实际构建版本、规则来源、revision 和 dirty，不从历史对话猜测当前版本或本机路径。

通过 MCP 完整读取以下入口，再按索引选择本次适用的规范及已接受 ADR：

- `gocanon://rules/AGENTS.md`
- `gocanon://rules/skills/gocanon-development/SKILL.md`
- `gocanon://rules/docs/standards/README.md`

工具参数以实际 schema 为准，每次调用显式传当前项目的绝对 `project`，不使用服务进程的工作目录。资源是服务启动快照；本地文件变化不表示运行中的快照已经更新。需要读取模板以外的实现时，使用能力查询确认的规则源；来源不可访问或客户端没有发现所需工具时，报告具体缺口，不编造规则或手造生成结果。

对照组件结构使用 `gocanon_get_component_example`：先获取文件索引，再指定 `file` 读取完整参考源码。基础包先用 `gocanon_plan_package` 核对已有实现和冲突，通过 `gocanon_get_package_example` 获取接入代码。已有包不会被追加工具覆盖；按当前产品契约适配，不因某项生成不适用而重写全部基础设施。只有确需验证实际生成或运行结果时，才在隔离目录创建参考工程。

本项目没有 `gocanon.json`；`gocanon_check_project` 返回 `not_managed` 表示未执行受管理文件检查，不是通过，也不要求补一份清单。手写入口、业务与基础包由本项目维护；protobuf 和 Kubernetes 官方产物通过现有生成命令更新，沿用 CI 的重生成差异检查。不要将规则自检样本、示例数据库或空业务复制到本产品。

## 代码评审

1. 同时查看 `git status --short`、`git diff --name-only` 和 `git ls-files --others --exclude-standard`。逐个审查新增与修改的手写文件；单独使用 `git diff` 会漏掉未跟踪文件。生成文件只核对生成结果与可重复性，不按手写代码逐行审查。
2. 先按 GoCanon 的 GO-FUNC-012 检查类型、方法和辅助函数的声明顺序，再逐函数检查职责、命名、参数与返回值、错误路径、注释是否仍符合实现。赋值与对应的错误检查保持相邻；只在语义阶段变化处用空行分隔，不机械地要求每个 `return` 前空行。这对应 GO-FUNC-013、GO-FUNC-014。
   功能审查结束后，再做一遍独立的换行与空行复查：逐函数区分输入准备/解码、校验、依赖调用、结果转换和状态登记，测试区分输入准备、被测操作和结果断言。不得因 lint 通过跳过这一步；同阶段简单语句不逐行分隔，独立错误检查块遵守统一块后分隔。
3. 检查测试断言能否在目标行为失效时真正失败。涉及筛选、鉴权或冲突检测时，不只验证成功输入，还要验证至少一个不匹配或冲突输入。
4. 检查本次实现是否使包注释、决策文档和示例过时。运行 `gofmt -l cmd internal pkg`、`git diff --check`、`golangci-lint run ./...`、`go vet ./...`、`go test ./...` 和 `go build ./...`；`git diff --check` 不覆盖未跟踪文件。变更 API Server 存储契约时再运行 `go test -tags=integration ./internal/apiserver`，变更 API 类型时重新生成并核对生成文件。

`.golangci.yml` 采用 GoCanon 已接受的固定策略，见 [0016](docs/decisions/0016-gocanon-template-alignment.md)：错误赋值与检查相邻、块后分隔、超过两行的块在 return 前分隔、块首尾无多余空行；短块不强制 return 前空行。不启用 wsl 的全部默认规则。Context 字段、接收者命名、Context 首参和 Admin API 业务层的登记存储导入由成熟 lint 检查；生成代码按标准标记排除。规则样本和自检脚本属于 GoCanon，不复制到本产品。
自动检查不能判断命名、注释和空行的语义；不要编写猜测这些语义的仓库专用扫描器来代替人工审查。
评审结论须区分已通过的检查与尚未执行的真实组件验证；不能仅凭格式、静态检查和单元测试通过就宣称代码达到模板级或生产就绪。
