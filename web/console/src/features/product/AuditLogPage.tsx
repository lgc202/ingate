import { AppWindow, CheckCircle2, FileClock, Filter, KeyRound, Route, Search, Server, ShieldCheck } from 'lucide-react';
import { Badge, Button, PageFrame } from '@/components/ui';

const events = [
  { time: '14:28:42', actor: '林工程师', action: '更新 AI 路由', resource: '生产 AI 路由', detail: '将 claude-sonnet 备用服务调整为 Bedrock 灾备', icon: Route, result: '成功' },
  { time: '13:56:17', actor: '王管理员', action: '签发访问密钥', resource: '客服助手', detail: '签发“客服生产环境”访问密钥，有效期 90 天', icon: KeyRound, result: '成功' },
  { time: '11:04:09', actor: '系统', action: '发布配置', resource: '生产环境', detail: '14 项资源变更已应用到全部网关实例', icon: CheckCircle2, result: '成功' },
  { time: '09:42:31', actor: '赵开发', action: '创建模型服务', resource: '通义千问灾备', detail: '接入百炼国际站，提供 qwen-max 实际模型', icon: Server, result: '成功' },
];

export function AuditLogPage() {
  return (
    <PageFrame title="审计日志" subtitle="记录管理面配置、权限与凭据操作，不记录业务请求正文" actions={<Button variant="outline"><FileClock className="h-4 w-4" />导出日志</Button>}>
      <section className="audit-filter"><label className="product-search"><Search /><input placeholder="搜索操作者、动作或资源" /></label><Button variant="outline"><Filter />筛选</Button><span>保留最近 180 天 · 今天 46 项变更</span></section>
      <section className="audit-timeline">
        <div className="audit-date"><span>今天 · 2026 年 8 月 11 日</span><i /></div>
        {events.map((event) => <article key={`${event.time}-${event.action}`}><span className="audit-icon"><event.icon /></span><div className="audit-main"><div><strong>{event.action}</strong><Badge tone="success">{event.result}</Badge></div><p>{event.detail}</p><small>{event.resource}</small></div><div className="audit-actor"><strong>{event.actor}</strong><span>{event.time}</span></div></article>)}
        <div className="audit-date"><span>昨天 · 2026 年 8 月 10 日</span><i /></div>
        <article><span className="audit-icon"><ShieldCheck /></span><div className="audit-main"><div><strong>更新 IP 访问限制</strong><Badge tone="success">成功</Badge></div><p>新增办公网出口地址 203.0.113.0/24</p><small>管理面访问限制</small></div><div className="audit-actor"><strong>王管理员</strong><span>18:12:06</span></div></article>
      </section>
    </PageFrame>
  );
}
