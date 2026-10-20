# 0015：AI Proxy 应用分层与能力包布局

- 状态：已接受，当前布局已实现。
- 日期：2026-10-15；2026-10-20 按 Delta ADS 和完整请求链整理说明。

## 背景与决定

AI Proxy 是独立运行组件。服务启动、入站协议和完整用例属于应用层，
配置消费、协议解析与适配属于组件私有能力；不能把一个请求拆成多个隐式 Context 交接阶段，
也不为凑齐普通 CRUD 分层创建 Repository。

当前目录如下。这里只列实际职责，不要求后续提供商复制同样数量的包：

```text
internal/
├── aiproxy/
│   ├── app.go               # Run、依赖装配与关闭顺序
│   ├── config.go            # 启动配置读取、转换与校验
│   ├── conf/                # protobuf 启动契约及生成代码
│   ├── server/              # 路由注册、可信 UDS 入口、错误编码
│   ├── handler/             # 有界读取、协议错误与响应流写回
│   ├── errno/               # 本组件公开错误，不与管理端混用
│   ├── service/
│   │   ├── modelproxy/      # 两个模型创建操作的完整编排
│   │   └── mcpproxy/        # 单后端 MCP POST 请求编排
│   └── pkg/
│       ├── aiconfig/        # Delta ADS 订阅、批次应用、不可变查询视图
│       ├── apischema/openai/ # 当前需要读取或返回的协议字段
│       ├── endpointspec/    # 按操作解析请求，不选择后端
│       ├── translator/      # OpenAI 同协议模型名覆盖
│       ├── mcpprotocol/     # MCP JSON-RPC 信封与镜像 Header
│       ├── envoyclient/     # 固定 Envoy UDS 的原生 HTTP Client
│       └── configtest/      # 跨包测试使用的受控 ADS 资源源
└── pkg/aidataplane/          # 控制面中间表示、目标绑定与校验
api/aidataplane/v1/           # Controller 与 Proxy 共享的 protobuf 分发契约
```

aiconfig 的 Loader 和 Config 放在同一个包，因为它们共同拥有配置构建与只读交接的不变量。
启动 conf 与动态配置不是一回事：前者启动时读取，后者由 ADS 更新；不把两者合成全局配置对象。
configtest 只供测试组装，生产调用链不导入它。

## 调用方向

```text
Server → Handler → modelproxy.Service / mcpproxy.Service
                       ├─ aiconfig：捕获一次配置并查询
                       ├─ endpointspec / mcpprotocol：解析校验
                       ├─ translator：模型名适配
                       └─ 原生 http.Client → Envoy UDS
```

Service 完成从解析、路由到 Client.Do 的整个用例，返回原生 HTTP Response。
Handler 不持有转发器或选中后端，也不使用 Context 隐式交付目标；
Context 只沿入口传递当前请求的可信 Listener/Host 与取消状态。

能力包不依赖 conf、Server、Handler、Service 或 Controller。
Controller 依赖共享编译表示和分发协议，不导入 Proxy 的 aiconfig。
pkg 子目录表示组件所有权，不自动形成 Go 编译期访问限制，依赖方向仍需审查。

## 生命周期与错误

组合根构造原生 HTTP Client、Service、Handler 和 Kratos Server。
Listener、Loader、Logger 由成功构造链保证有效，不增加逐层 nil 检查或虚假失败测试。
外部路径、正文大小、启动字段与 ADS 批次在各自真实输入边界校验。

Run 拥有日志、发现连接与入口监听器，Kratos 拥有 HTTP 生命周期；
HTTP 有界排空后取消并等待 Loader，最后清理出站连接池与其他资源。
Handler 接管响应 Body 并关闭，不另建流框架。

Service 返回稳定用例错误并保留原因，不打印同一错误。
Handler 映射当前协议并消费需要记录的失败，Server 编码安全的公开结果。
上游响应不经过本地错误编码，流开始后失败必须中断，不能追加 JSON。

模型错误使用 OpenAI error envelope；MCP 使用 JSON-RPC 2.0。
这些协议不能为统一 errno 而合并为同一种响应格式。

## 采用理由与边界

原有 ReverseProxy 与请求准备接口已删除。完整编排放入 Service 后，调用路径明确，
不需要 Handler 同时持有用例和转发器，也不需要额外目标 Context。
原生 HTTP Client 仍只连接固定 UDS，不因删除 ReverseProxy 而允许任意上游请求。

增加其他提供商时按实际 schema 和转换需求扩展协议能力，不能只扩大枚举后假定请求已适配；
不预建 Provider 注册表、空包、插件机制或转换矩阵。
详见[运行契约](../design/ai-model-operations-and-deployment.md)。

本页描述布局与所有权。具体协议、配置恢复和测试范围分别见
[编译资源契约](../design/ai-compiled-config-contract.md)、
[增量设计](../design/control-plane-incremental-design.md)和[审查记录](../refactoring-review.md)。
这些决定不代表真实 Envoy 或生产部署已经验收。
