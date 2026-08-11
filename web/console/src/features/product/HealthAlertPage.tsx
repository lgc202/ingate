import { Bot, CheckCircle2, ChevronRight, CircleAlert, Clock3, HeartPulse, Network, Server, TriangleAlert } from 'lucide-react';
import { Badge, PageFrame, StatusDot } from '@/components/ui';
import { prototypeScenario } from '@/prototype/scenario';

const components = [
  { name: '生产网关', type: '网关', detail: '3 个实例 · 9 条路由', state: 'healthy' as const, availability: '99.99%', latency: '4 ms', icon: Network },
  { name: '通义千问生产', type: '大模型服务', detail: 'qwen-max', state: 'healthy' as const, availability: '99.98%', latency: '620 ms', icon: Bot },
  { name: 'Anthropic 公网', type: '大模型服务', detail: 'claude-sonnet-4', state: 'warning' as const, availability: '99.72%', latency: '2.8 s', icon: Bot },
  { name: '订单服务', type: 'HTTP 服务', detail: '2 个端点', state: 'healthy' as const, availability: '99.96%', latency: '86 ms', icon: Server },
];

export function HealthAlertPage() {
  const { delivery } = prototypeScenario;
  return (
    <PageFrame title="健康与发布" subtitle="统一查看网关实例、服务线路、告警和配置发布结果">
      <section className="health-banner"><span><HeartPulse /></span><div><small>生产环境总体状态</small><h2>运行正常，1 条线路需要关注</h2><p>所有网关实例和配置发布正常。Anthropic 公网连续 8 分钟延迟偏高，备用线路可用。</p></div><Badge tone="success"><StatusDot status="healthy" /> 核心流量正常</Badge></section>
      <section className="product-metrics four"><HealthMetric label="网关实例" value="3 / 3" note="全部在线" /><HealthMetric label="服务端点" value="11 / 12" note="1 个降级" warning /><HealthMetric label="配置状态" value="已同步" note="版本 142" /><HealthMetric label="活跃告警" value="2" note="0 紧急 · 2 警告" warning /></section>
      <div className="health-layout">
        <section className="product-table-card"><header><div><h3>关键组件</h3><p>最近 24 小时可用性和 P95 延迟</p></div></header>{components.map((component) => <button type="button" className="health-component" key={component.name}><span><component.icon /></span><div><strong>{component.name}</strong><small>{component.type} · {component.detail}</small></div><div><small>可用率</small><strong>{component.availability}</strong></div><div><small>P95 延迟</small><strong>{component.latency}</strong></div><Badge tone={component.state === 'healthy' ? 'success' : 'warning'}><StatusDot status={component.state} /> {component.state === 'healthy' ? '正常' : '需关注'}</Badge><ChevronRight /></button>)}</section>
        <aside className="active-alerts surface-card"><header><span className="eyebrow">当前告警</span><h3>需要关注</h3></header><Alert icon={TriangleAlert} title="Anthropic 公网延迟升高" detail="P95 已达 2.8 秒，备用线路正常" time="持续 8 分钟" /><Alert icon={CircleAlert} title="内部自动化额度已用尽" detail="后续 AI 请求已按策略拒绝" time="持续 20 分钟" /><div className="resolved-alert"><CheckCircle2 /><div><strong>今天已自动恢复 3 项</strong><p>最近恢复：通义千问灾备连接</p></div></div></aside>
      </div>
      <section className="delivery-card surface-card">
        <header><div><span className="eyebrow">配置发布</span><h3>当前生效版本</h3></div><Badge tone="success"><StatusDot status="healthy" />全部实例已同步</Badge></header>
        <div className="delivery-current"><div><small>版本</small><strong>{delivery.version}</strong></div><div><small>生效时间</small><strong>今天 {delivery.updatedAt}</strong></div><div><small>网关实例</small><strong>{delivery.instances}</strong></div><div><small>资源数量</small><strong>{delivery.resources}</strong></div></div>
        <div className="delivery-history"><div><span><CheckCircle2 /></span><p><strong>版本 142</strong><small>更新生产 AI 路由和 Claude Sonnet 模型线路</small></p><time>14:31:08</time></div><div><span><CheckCircle2 /></span><p><strong>版本 141</strong><small>签发 api.example.com 网关证书</small></p><time>11:04:09</time></div><div><span><CheckCircle2 /></span><p><strong>版本 140</strong><small>更新内部自动化调用方额度</small></p><time>09:42:31</time></div></div>
      </section>
    </PageFrame>
  );
}

function HealthMetric({ label, value, note, warning = false }: { label: string; value: string; note: string; warning?: boolean }) { return <article className={warning ? 'is-warning' : ''}><small>{label}</small><strong>{value}</strong><p>{note}</p></article>; }
function Alert({ icon: Icon, title, detail, time }: { icon: typeof TriangleAlert; title: string; detail: string; time: string }) { return <article className="active-alert"><span><Icon /></span><div><strong>{title}</strong><p>{detail}</p><small><Clock3 /> {time}</small></div></article>; }
