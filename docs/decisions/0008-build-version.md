# 0008：二进制构建版本与开发命令入口

- 状态：已接受（用户已确认提交）
- 日期：2026-10-07

三个组件原先没有 Ingate 自身的构建版本入口。API Server 的 Kubernetes EffectiveVersion 和原生 `/version` 属于兼容性契约，不用它们表示 Ingate 发布版本，也不在本轮修改。

依据 GoCanon 已接受的 ADR 0046、0047，增加共享 `internal/pkg/version`。该包提供 Info、Get、String 和 Text，不读取配置、执行 Git、注册 Flag、打印或退出进程。构建通过链接器 `-X` 注入版本、提交、工作区状态和构建时间；Go 版本、编译器和平台来自标准库。未注入时为 devel/unknown，不把运行时间当作构建时间。

三个二进制的 `--version` 均输出同一组多行构建信息后结束。Admin API 使用独立 FlagSet，保持帮助成功、参数解析失败退出码 2、运行失败退出码 1；版本分支先于配置加载和数据库迁移。两个 Cobra 入口使用内建版本标志，先于参数校验和 Run。Kratos App 的 Version 只保存版本号。

根 Makefile 默认只显示帮助，实际命令按职责放入 `scripts/make-rules/`。build 默认发现 cmd 入口，支持 BINS 子集和入口名称目标，输出到被忽略的 bin/；版本元数据延迟到构建时求值。VERSION 和 BUILD_DATE 可覆盖；Git 不可用时使用开发默认值。Git tree state 包含未跟踪文件，dirty 不代表相应改动都已编入二进制。版本标识不保证是 SemVer。

build、test、vet、lint、fmt-check、verify 不生成或修改源码、依赖和锁文件。generate 显式复用现有 Kubernetes 脚本和 Buf 配置；工具版本与现有 CI 一致。verify 不启动 MySQL、Redis 或 etcd。默认帮助不调用 Go、Git、date 或生成工具；dry-run 构建可以读取版本元数据，不承诺不调用这些工具。

验证覆盖默认版本与文本格式、三个版本入口绕过配置和运行、Admin API 的参数退出码，以及实际链接注入后的二进制输出。默认 make 与 make help 通过临时工具探针确认不调用 Go、gofmt、Git 或 date；fmt-check 在格式工具失败时返回失败，BINS 子集 dry-run 只选择目标入口。make -j4 verify 已通过，前后全部跟踪及未跟踪源码的内容摘要一致。版本输出不是组件可用性检查；本轮不增加版本 HTTP API、版本比较、自动发布、镜像或部署流程。
