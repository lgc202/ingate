import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://lgc202.github.io',
  base: '/ingate',
  integrations: [
    starlight({
      title: 'Ingate',
      description: '基于 Envoy 的声明式 API 与 AI 网关',
      favicon: '/favicon.svg',
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
          label: '流量管理',
          items: [
            { label: '网关入口', slug: 'traffic/gateway' },
            { label: '路由', slug: 'traffic/route' },
            { label: '服务', slug: 'traffic/service' },
            { label: '证书', slug: 'traffic/certificate' },
          ],
        },
        {
          label: '访问治理',
          items: [
            { label: '调用方与访问密钥', slug: 'governance/caller' },
            { label: 'IP 访问限制', slug: 'governance/ip-restriction' },
            { label: '请求限流', slug: 'governance/rate-limit' },
            { label: 'Token 额度', slug: 'governance/token-quota' },
          ],
        },
        {
          label: '插件',
          items: [{ label: '插件生命周期', slug: 'plugins/overview' }],
        },
        {
          label: '观测分析',
          items: [
            { label: '请求记录', slug: 'observability/request-records' },
            { label: '流量分析', slug: 'observability/traffic-analysis' },
            { label: 'AI 用量', slug: 'observability/ai-usage' },
          ],
        },
        {
          label: '运维',
          items: [{ label: '配置与维护', slug: 'operations/overview' }],
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
