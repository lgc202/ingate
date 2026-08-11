import { AlertTriangle, ArrowRight, Bot, CircleCheck, Clock3, Route, Server, ShieldAlert, Zap } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Badge, Button, PageFrame, StatusDot } from '@/components/ui';
import { prototypeScenario } from '@/prototype/scenario';

export function OverviewPage() {
  const { summary, delivery, requests } = prototypeScenario;

  return (
    <PageFrame title="概览" subtitle="生产环境的 API、AI 流量和待处理问题" actions={<Link to="/routes"><Button variant="outline">管理路由</Button></Link>}>
      <section className="overview-status-bar">
        <div className="overview-status-primary"><span><CircleCheck /></span><div><small>环境状态</small><strong>核心流量运行正常</strong><p>配置版本 {delivery.version} 已同步至 {delivery.instances} 个网关实例</p></div></div>
        <div><small>今日请求</small><strong>142.7K</strong></div><div><small>成功率</small><strong>99.82%</strong></div><div><small>P95 延迟</small><strong>186 ms</strong></div>
        <Link to="/health">查看健康与发布 <ArrowRight /></Link>
      </section>

      <section className="overview-priority-grid">
        <Link to="/health" className="priority-card is-warning"><span><AlertTriangle /></span><div><small>服务告警</small><strong>Anthropic 公网延迟升高</strong><p>备用服务正常，当前未影响对外可用性</p></div><ArrowRight /></Link>
        <Link to="/callers" className="priority-card is-error"><span><ShieldAlert /></span><div><small>策略拒绝</small><strong>内部自动化额度已用尽</strong><p>AI 请求已停止，普通 API 权限不受影响</p></div><ArrowRight /></Link>
        <Link to="/health" className="priority-card"><span><Zap /></span><div><small>最近发布</small><strong>版本 {delivery.version} 已于 {delivery.updatedAt} 生效</strong><p>{delivery.resources} 项资源配置已同步</p></div><ArrowRight /></Link>
      </section>

      <section className="overview-main-grid">
        <div className="surface-card overview-resource-card">
          <header><div><span className="eyebrow">配置概况</span><h3>网关能力</h3></div><Link to="/gateways">查看配置</Link></header>
          <div className="resource-count-grid"><ResourceCount icon={Route} label="网关" value={summary.gateways} /><ResourceCount icon={Route} label="API 路由" value={summary.apiRoutes} /><ResourceCount icon={Bot} label="AI 路由" value={summary.aiRoutes} /><ResourceCount icon={Server} label="HTTP 服务" value={summary.httpServices} /><ResourceCount icon={Bot} label="大模型服务" value={summary.modelServices} /><ResourceCount icon={ShieldAlert} label="调用方" value={summary.callers} /></div>
          <div className="overview-flow"><span>普通 API</span><strong>网关</strong><ArrowRight /><strong>API 路由</strong><ArrowRight /><strong>HTTP 服务</strong></div>
          <div className="overview-flow"><span>AI 请求</span><strong>网关</strong><ArrowRight /><strong>AI 路由</strong><ArrowRight /><strong>大模型服务 / 实际模型</strong></div>
        </div>

        <div className="surface-card overview-request-card">
          <header><div><span className="eyebrow">最近请求</span><h3>需要关注的执行结果</h3></div><Link to="/requests">查看全部</Link></header>
          <div className="overview-request-list">
            {requests.slice(0, 4).map((request) => (
              <Link key={request.id} to={`/requests?query=${request.id}`}>
                <StatusDot status={request.status === 'success' ? 'healthy' : request.status === 'fallback' ? 'warning' : 'error'} />
                <div><strong>{request.id}</strong><span>{request.caller} · {request.kind === 'ai' ? request.model : `${request.method} ${request.path}`}</span></div>
                <div><Badge tone={request.status === 'success' ? 'success' : request.status === 'fallback' ? 'warning' : 'error'}>{request.status === 'success' ? '成功' : request.status === 'fallback' ? '主备切换' : request.status === 'blocked' ? '策略拒绝' : '失败'}</Badge><small><Clock3 /> {request.latency}</small></div>
              </Link>
            ))}
          </div>
        </div>
      </section>
    </PageFrame>
  );
}

function ResourceCount({ icon: Icon, label, value }: { icon: typeof Route; label: string; value: number }) {
  return <div><span><Icon /></span><p><small>{label}</small><strong>{value}</strong></p></div>;
}
