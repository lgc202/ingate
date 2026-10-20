# 文档图表维护

当前实现的两张图采用「Mermaid 源码 + 已渲染 SVG」：文档只嵌入 SVG，避免不同 Markdown
预览器重新布局、配色或裁切；需要修改关系时编辑 `.mmd`，再生成对应 SVG，不手工修改
渲染产物。其他已有图表保持原用途与历史范围，本次不统一改写。

| 源码 | 渲染产物 | 内容 |
| --- | --- | --- |
| [ai-current-config-flow.mmd](ai-current-config-flow.mmd) | [SVG](ai-current-config-flow.svg) | 已实现的配置下发与查询投影 |
| [ai-current-model-sequence.mmd](ai-current-model-sequence.mmd) | [SVG](ai-current-model-sequence.svg) | 已实现的模型请求及响应时序 |

两张图共用 [mermaid-config.json](mermaid-config.json)：中性浅色主题、白色画布、原生 SVG
文字，不使用 `foreignObject` HTML 标签。SVG 固定节点和连线位置；文档预览可等比缩放，
字体仍由阅读环境提供，字形不保证逐像素一致。

配置流转图在 `.mmd` 的 frontmatter 中指定 ELK 正交布局与 `rounded` 连线：主线段只走
水平或垂直方向，转向处保留小圆角，不使用斜向跨层连线。模型请求时序图继续使用
Mermaid 的时序布局；两图都需重新生成 SVG 后才会更新文档显示。

## 生成

本次实际使用 `@mermaid-js/mermaid-cli 11.12.0`、`mermaid 11.17.2` 和 Chrome `155.0.8059.82`，
字体为 Arial/PingFang SC。工具使用已有独立安装，不给 Go 项目增加 Node 依赖。
重新生成时应核对这两个包的实际版本；字体、浏览器或布局引擎版本变化后需要重新预览。

在仓库根目录，用已安装的上述版本运行：

```sh
mmdc -i docs/diagrams/ai-current-config-flow.mmd \
  -o docs/diagrams/ai-current-config-flow.svg \
  -c docs/diagrams/mermaid-config.json -b white -w 1200

mmdc -i docs/diagrams/ai-current-model-sequence.mmd \
  -o docs/diagrams/ai-current-model-sequence.svg \
  -c docs/diagrams/mermaid-config.json -b white -w 1200
```

若环境没有 Mermaid CLI 默认的浏览器，可使用其 `-p` 参数传入本机 Puppeteer 配置，
在配置中显式指定浏览器可执行路径；不要提交个人机器的绝对路径。生成后检查中文、
箭头、边缘节点与长标签是否完整，并同时审查 `.mmd`、渲染配置和 SVG 的变化。

本次在相同本机工具、浏览器和字体环境中重复生成，核对 SVG SHA-256 一致；并在
736 与 1100 像素容器中预览，检查文字边界和水平溢出。不同渲染器和字体环境仍需复核，
这些图形检查不替代 API Server、Envoy 或模型提供商的业务联调。

## 增量设计图

[控制面增量设计](../design/control-plane-incremental-design.md)使用以下两张手工坐标 SVG：

| SVG 源文件 | 内容 |
| --- | --- |
| [control-plane-incremental-architecture.svg](control-plane-incremental-architecture.svg) | 源缓存、增量计算、配置发布和两路 Delta |
| [control-plane-incremental-sequence.svg](control-plane-incremental-sequence.svg) | AI Proxy 首次订阅、应用、ACK、更新和重连 |

这两张 SVG 本身就是维护源，不由 Mermaid 生成，也没有对应 `.mmd`。
修改时直接编辑坐标与文本，保留水平/垂直连线，并在窄、宽容器中检查文字与箭头。
资源级两路 Delta 与 AI Proxy 查询视图现已接入；真实组件与部署验收仍未执行。当前配置流转图同步使用 Delta ADS。
