import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://lgc202.github.io',
  base: '/ingate',
  integrations: [
    starlight({
      title: 'Ingate',
      description: '基于 Envoy 的声明式 API 与 AI 网关',
      favicon: '/ingate.svg',
      defaultLocale: 'root',
      locales: {
        root: {
          label: '简体中文',
          lang: 'zh-CN',
        },
      },
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/lgc202/ingate',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/lgc202/ingate/edit/main/docs/',
      },
      lastUpdated: true,
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: '开始',
          items: [
            { label: '认识 Ingate', slug: 'getting-started/introduction' },
            { label: '安装', slug: 'getting-started/installation' },
            { label: '转发第一个 API', slug: 'getting-started/first-api' },
            { label: '发布第一个模型', slug: 'getting-started/first-ai-route' },
          ],
        },
        {
          label: '概念与架构',
          items: [
            { label: '系统架构', slug: 'concepts/architecture' },
            { label: '资源关系', slug: 'concepts/resources' },
          ],
        },
        {
          label: '使用指南',
          items: [
            {
              label: '流量管理',
              items: [
                { label: '网关入口', slug: 'guide/traffic/gateway' },
                { label: '路由', slug: 'guide/traffic/route' },
                { label: '服务', slug: 'guide/traffic/service' },
                { label: '证书', slug: 'guide/traffic/certificate' },
              ],
            },
            {
              label: '访问治理',
              items: [
                { label: '调用方与访问密钥', slug: 'guide/governance/caller' },
                { label: 'IP 访问限制', slug: 'guide/governance/ip-restriction' },
                { label: '请求限流', slug: 'guide/governance/rate-limit' },
                { label: 'Token 额度', slug: 'guide/governance/token-quota' },
                { label: '请求响应转换', slug: 'guide/governance/header-transformation' },
                { label: '模拟响应', slug: 'guide/governance/mock-response' },
              ],
            },
            { label: '插件生命周期', slug: 'guide/plugins/overview' },
            {
              label: '观测分析',
              items: [
                { label: '请求记录', slug: 'guide/analytics/request-records' },
                { label: '流量分析', slug: 'guide/analytics/traffic-analysis' },
                { label: 'AI 用量', slug: 'guide/analytics/ai-usage' },
              ],
            },
            { label: '运维助手', slug: 'guide/assistant' },
          ],
        },
        {
          label: '运维',
          items: [
            { label: '配置与维护', slug: 'operations/overview' },
            {
              label: 'ALS',
              items: [
                { label: '运行模型', slug: 'operations/als' },
                { label: '配置', slug: 'operations/als/configuration' },
                { label: 'SLO 与告警处置', slug: 'operations/als/monitoring' },
                { label: '恢复与迁移', slug: 'operations/als/recovery' },
              ],
            },
          ],
        },
        {
          label: '开发者文档',
          items: [
            { label: '阅读入口', slug: 'development/overview' },
            {
              label: 'ALS 设计',
              items: [
                { label: '设计总览', slug: 'development/architecture/als' },
                { label: 'Kafka 可靠写入', slug: 'development/architecture/als/kafka' },
                { label: 'WAL 与故障恢复', slug: 'development/architecture/als/wal' },
                { label: '记录 ID 与幂等', slug: 'development/architecture/als/idempotency' },
                { label: '并发与状态迁移', slug: 'development/architecture/als/concurrency' },
                { label: '可观测性与验证', slug: 'development/architecture/als/observability' },
              ],
            },
          ],
        },
        {
          label: '参考',
          items: [
            { label: '声明式 API', slug: 'reference/declarative-api' },
            { label: '当前边界', slug: 'reference/current-scope' },
          ],
        },
      ],
    }),
  ],
});
