# RouteEditor 路由表单抽屉化重构方案

## 一、 问题根因分析

在将 `RouteEditor` 从“整页编辑”调整为“右侧抽屉（Drawer）”后，出现严重排版重叠和布局错乱，根因如下：

1. **废弃的旧 CSS 样式与多列布局**：`RouteEditor.tsx` 依赖了大量的旧版全屏 CSS 类（如 `.route-workbench-grid`、`.route-form-nav` 侧边锚点菜单、`.route-editor-summary` 侧边浮动预览卡片），这些多列布局在宽度受限的 Drawer 容器内强行并排，导致严重重叠错位。
2. **信息结构冗余**：
   - 顶部重复渲染了 Header 面板（与 Drawer 自身的 `title/subtitle` 重复）；
   - 左侧包含了多余的锚点导航（在抽屉垂直滚动流中无实际必要）；
   - 右侧包含实时预览浮动面板（占据大量空间且与输入框叠加）；
   - 底部双重渲染保存/取消操作条。

---

## 二、 精简重构方案

将 `RouteEditor` 重构成干净、直接、完全适配抽屉容器（Drawer）的标准单列 Tailwind 表达格式（与 `GatewayPage`、`UpstreamPage` 表单样式保持一致）：

### 1. 结构瘦身与删除（“有些东西不是很有必要可以🙅”）
- **移除** 顶部重复 Header 栏（`route-workbench-header`）；
- **移除** 左侧侧边锚点菜单（`route-form-nav`）；
- **移除** 右侧侧边预览浮动面板（`route-editor-summary`）；
- **移除** 内嵌的底部操作条，统一使用 Drawer 的底部动作按钮。

### 2. 模块收敛（单列优雅垂直流）
将表单精简为 4 个清晰的卡片分块（Card Section）：
1. **基础信息**：路由名称、启用状态；
2. **匹配条件**：生效网关（多选）、规则名称、请求 Path 路径前缀、HTTP Methods（GET/POST 等）、域名 Hostnames；
3. **转发目标 & AI 模型路由**：
   - 支持“普通服务转发”与“模型服务代理”一键切换；
   - 普通转发：动态增加上游 Upstream 及其权重；
   - 模型代理：配置客户端模型别名 (model) 映射到目标模型 Upstream 及 upstreamModel；
4. **转发控制与治理**：超时、重试、Header 改写等高频配置。

---

## 三、 验证计划

1. 适配抽屉容器，确保全屏/任意宽度下无任何样式重叠与滚动溢出；
2. 运行 `npm run build` 确保 TypeScript 类型校验 0 Error；
3. 运行 `make verify` 确保前端构建与整体项目校验无误。
