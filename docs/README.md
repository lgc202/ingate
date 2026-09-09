# Ingate 文档目录

发布站点的正文位于 `src/content/docs`。目录按读者要完成的事情划分：

- `getting-started`：第一次安装和调用
- `concepts`：产品模型与系统边界
- `guide`：Console 和产品 API 的使用方法
- `operations`：部署、监控和故障处置
- `development`：当前实现的架构、协议和工程取舍
- `reference`：字段、接口与当前能力边界

`docs/design` 保存尚未全部落地或仍可能调整的设计材料，不作为已实现能力的依据。产品和运维文档只描述当前代码能够提供的行为；实现发生变化时，应同步修改对应的开发者文档。

本地构建站点：

```bash
cd docs
npm install
npm run build
```
