import {
  AlertTriangle,
  CheckCircle2,
  ChevronRight,
  Clock3,
  Download,
  FileClock,
  Network,
  Server,
  ShieldCheck,
  Wrench,
} from 'lucide-react';
import { useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import type { AuditRecord, RequestRecord, TrafficType } from '../data';
import {
  Drawer,
  EmptyState,
  FilterTabs,
  Metric,
  PageHeader,
  SearchField,
  StatusBadge,
  TypeBadge,
} from '../components/ui';
import { usePrototype } from '../prototype-context';

type RequestFilter = 'ALL' | TrafficType | 'ERROR';

export function RequestPage() {
  const { requests } = usePrototype();
  const [params] = useSearchParams();
  const initialQuery = params.get('query') ?? '';
  const [query, setQuery] = useState(initialQuery);
  const [filter, setFilter] = useState<RequestFilter>('ALL');
  const [selected, setSelected] = useState<RequestRecord | null>(() => requests.find((item) => item.id === initialQuery) ?? null);
  const visible = useMemo(() => requests.filter((request) => {
    const matchesType = filter === 'ALL' || (filter === 'ERROR' ? request.result === '失败' || request.result === '策略拒绝' : request.type === filter);
    return matchesType && `${request.id}${request.caller}${request.route}${request.request}${request.target}`.toLowerCase().includes(query.toLowerCase());
  }), [filter, query, requests]);

  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="运行分析" title="请求记录" description="查找单次请求的路由、策略与服务执行结果" />
      <section className="metric-grid four"><Metric label="API 请求" value="98.3K" note="成功率 99.91%" /><Metric label="AI 请求" value="44.4K" note="19.7M Token" /><Metric label="MCP 工具调用" value="8.2K" note="成功率 99.90%" /><Metric label="异常与拒绝" value="257" note="服务异常 66 · 策略拒绝 191" tone="warning" /></section>
      <p className="privacy-note"><ShieldCheck />默认不记录请求正文</p>
      <section className="card table-card">
        <header className="table-toolbar"><SearchField value={query} onChange={setQuery} placeholder="搜索请求编号、路由、调用方或服务" /><FilterTabs value={filter} onChange={setFilter} options={[{ value: 'ALL', label: '全部', count: requests.length }, { value: 'API', label: 'API' }, { value: 'AI', label: 'AI' }, { value: 'MCP', label: 'MCP' }, { value: 'ERROR', label: '异常' }]} /></header>
        <div className="table-head request-columns"><span>时间 / 请求编号</span><span>调用方</span><span>路由 / 请求</span><span>结果</span><span>最终服务</span><span>耗时</span><span /></div>
        {visible.length ? visible.map((request) => <button key={request.id} className="table-row request-columns" type="button" onClick={() => setSelected(request)}><div><strong>{request.time}</strong><code>{request.id}</code></div><strong>{request.caller}</strong><div className="request-route"><span><TypeBadge type={request.type} /><strong>{request.route}</strong></span><small>{request.request}</small></div><StatusBadge state={request.result === '成功' ? 'healthy' : request.result === '主备切换' ? 'warning' : 'error'} label={`${request.code} · ${request.result}`} /><strong>{request.target}</strong><span>{request.latency}</span><ChevronRight /></button>) : <EmptyState title="没有匹配的请求" description="请调整搜索或筛选条件。" />}
      </section>
      {selected ? <RequestDetail request={selected} onClose={() => setSelected(null)} /> : null}
    </div>
  );
}

function RequestDetail({ request, onClose }: { request: RequestRecord; onClose: () => void }) {
  const failed = request.result === '失败' || request.result === '策略拒绝';
  const usageLabel = request.type === 'AI' ? 'Token 用量' : request.type === 'MCP' ? '工具调用' : '响应流量';
  const facts = [
    ['调用方', request.caller],
    ['路由', request.route],
    ['请求', request.request],
    ['最终服务', request.target],
    ['总耗时', request.latency],
    [usageLabel, request.usage],
    ...(request.type === 'AI' ? [['估算成本', request.cost]] : []),
  ];

  return <Drawer title={`请求 ${request.id}`} description={`${request.time} · ${request.caller} · ${request.type}`} onClose={onClose} width="wide"><div className={`request-verdict ${failed ? 'is-error' : request.result === '主备切换' ? 'is-warning' : ''}`}><span>{failed ? <AlertTriangle /> : <CheckCircle2 />}</span><div><StatusBadge state={failed ? 'error' : request.result === '主备切换' ? 'warning' : 'healthy'} label={`${request.code} · ${request.result}`} /><h3>{request.result === '主备切换' ? '主服务失败后由备用服务成功响应' : request.result === '策略拒绝' ? '请求在到达服务之前被策略拒绝' : request.result === '失败' ? '目标服务没有成功响应' : '请求已成功完成'}</h3></div></div><div className="request-facts">{facts.map(([label, value]) => <div key={label}><span>{label}</span><strong>{value}</strong></div>)}</div><section className="execution-timeline"><header><span className="eyebrow">执行过程</span><h3>请求时间线</h3></header>{request.steps.map((step, index) => <div className="timeline-step" key={`${step.name}-${index}`}><span className={`timeline-dot state-${step.state}`}>{index + 1}</span><div><strong>{step.name}</strong><p>{step.detail}</p></div><small>{step.duration}</small></div>)}</section><p className="privacy-note"><ShieldCheck />未记录请求正文</p></Drawer>;
}

type AnalysisFilter = TrafficType;

const analysisViews = {
  API: {
    title: 'API 流量',
    metrics: [['API 请求', '98.3K', '+5.4%'], ['成功率', '99.91%', '5xx 62 次'], ['P95 延迟', '124 ms', 'P99 680 ms'], ['响应流量', '51.1 GB', '文件上传占 75%']],
    bars: [43, 52, 49, 64, 60, 69, 76, 72, 85, 81, 91, 83],
    unit: 'API 请求',
    composition: { title: '响应结果', total: '98.3K', label: 'API 请求', gradient: 'var(--teal) 0 99.4%, var(--amber) 99.4% 99.91%, var(--red) 99.91% 100%', segments: [['成功', '99.40%', 'api'], ['客户端错误', '0.51%', 'ai'], ['服务端错误', '0.09%', 'mcp']] },
    headers: ['路由', '类型', '目标服务', '请求', '成功率', 'P95 延迟', '响应流量'],
    rows: [['订单查询 API', 'API', '订单服务', '58.4K', '99.96%', '86 ms', '8.4 GB'], ['客户资料 API', 'API', '客户中心', '33.8K', '99.91%', '124 ms', '4.1 GB'], ['文件上传 API', 'API', '文件服务', '6.1K', '99.42%', '680 ms', '38.6 GB']],
  },
  AI: {
    title: 'AI 流量',
    metrics: [['AI 请求', '44.4K', '+9.1%'], ['首个 Token P95', '612 ms', 'Anthropic 2.8 s'], ['Token', '19.7M', '输入 14.2M · 输出 5.5M'], ['估算成本', '¥1,330', '缓存节省 ¥184']],
    bars: [28, 39, 35, 43, 49, 52, 58, 63, 69, 75, 82, 79],
    unit: 'Token',
    composition: { title: 'Token 构成', total: '19.7M', label: '本月 Token', gradient: 'var(--blue) 0 70.6%, var(--orange) 70.6% 98.5%, var(--teal) 98.5% 100%', segments: [['输入', '13.9M', 'api'], ['输出', '5.5M', 'ai'], ['缓存命中', '0.3M', 'mcp']] },
    headers: ['路由', '类型', '模型线路', '请求', '成功率', '首个 Token', 'Token'],
    rows: [['生产 AI 路由', 'AI', '3 个模型服务', '28.4K', '99.72%', '612 ms', '18.7M'], ['内部 AI 路由', 'AI', '通义千问生产', '16.0K', '99.99%', '112 ms', '1.0M']],
  },
  MCP: {
    title: 'MCP 流量',
    metrics: [['工具调用', '8.2K', '+12.4%'], ['成功率', '99.90%', '失败 8 次'], ['P95 耗时', '238 ms', 'P99 640 ms'], ['权限拒绝', '17', '主要为未授权工具']],
    bars: [20, 25, 28, 35, 31, 42, 45, 54, 61, 66, 74, 80],
    unit: '工具调用',
    composition: { title: '工具分布', total: '8.2K', label: '今日调用', gradient: 'var(--blue) 0 65.9%, var(--orange) 65.9% 100%', segments: [['web_search', '5.4K', 'api'], ['fetch_page', '2.8K', 'ai']] },
    headers: ['工具', '类型', 'MCP 服务', '调用', '成功率', 'P95 耗时', '权限拒绝'],
    rows: [['web_search', 'MCP', '搜索工具服务', '5.4K', '99.94%', '201 ms', '11'], ['fetch_page', 'MCP', '搜索工具服务', '2.8K', '99.86%', '298 ms', '6']],
  },
} as const;

export function AnalysisPage() {
  const [filter, setFilter] = useState<AnalysisFilter>('API');
  const view = analysisViews[filter];
  return <div className="page-stack page-enter"><PageHeader eyebrow="运行分析" title="流量分析" /><FilterTabs value={filter} onChange={setFilter} options={[{ value: 'API', label: 'API' }, { value: 'AI', label: 'AI' }, { value: 'MCP', label: 'MCP' }]} /><section className="metric-grid four">{view.metrics.map(([label, value, note]) => <Metric key={label} label={label} value={value} note={note} />)}</section><section className="analysis-layout"><article className="card chart-card"><header className="card-header"><div><span className="eyebrow">过去 12 小时</span><h3>{view.title}趋势</h3></div><span>{view.unit}</span></header><div className="bar-chart">{view.bars.map((height, index) => <i key={index}><b style={{ height: `${height}%` }} /><span>{index % 2 === 0 ? `${index + 3}:00` : ''}</span></i>)}</div></article><article className="card composition-card"><header><span className="eyebrow">构成</span><h3>{view.composition.title}</h3></header><div className="donut" style={{ background: `radial-gradient(circle,#fffef9 0 52%,transparent 53%), conic-gradient(${view.composition.gradient})` }}><div><strong>{view.composition.total}</strong><span>{view.composition.label}</span></div></div>{view.composition.segments.map(([label, value, tone]) => <p key={label}><i className={tone} />{label}<strong>{value}</strong></p>)}</article></section><section className="card ranking-card"><header className="card-header"><div><span className="eyebrow">明细排名</span><h3>{view.title}</h3></div><Link to={`/requests?query=${encodeURIComponent(view.rows[0][0])}`}>查看请求 <ChevronRight /></Link></header><div className="ranking-row ranking-head">{view.headers.map((header) => <span key={header}>{header}</span>)}</div>{view.rows.map((row) => <Link className="ranking-row" to={`/requests?query=${encodeURIComponent(row[0])}`} key={row[0]}>{row.map((cell, index) => index === 1 ? <TypeBadge key={`${row[0]}-${cell}`} type={cell as TrafficType} /> : <span key={`${row[0]}-${cell}-${index}`}>{cell}</span>)}</Link>)}</section></div>;
}

export function HealthPage() {
  const { gateways, routes, services, certificates, policies, currentVersion, releaseHistory } = usePrototype();
  const serviceWarnings = services.filter((service) => service.state === 'warning' || service.state === 'error' || service.state === 'unverified');
  const visibleServices = services;

  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="运行分析" title="健康与发布" description="代理实例、服务连接和配置生效状态" />
      <section className="health-hero"><span><AlertTriangle /></span><div><small>生产环境</small><h2>{serviceWarnings.length} 项服务性能下降</h2><p>网关实例和配置发布均正常</p></div><StatusBadge state="warning" label="需要关注" /></section>
      <section className="metric-grid four"><Metric label="代理实例" value="5 / 5" note="全部在线" tone="good" /><Metric label="健康服务" value={`${services.length - serviceWarnings.length} / ${services.length}`} note={`${serviceWarnings.length} 个性能下降`} /><Metric label="配置状态" value={releaseHistory[0]?.state ?? '已生效'} note={`当前版本 ${currentVersion}`} tone={releaseHistory[0]?.state === '发布中' ? 'warning' : 'good'} /><Metric label="运行告警" value={String(serviceWarnings.length)} note={`0 紧急 · ${serviceWarnings.length} 警告`} tone="warning" /></section>
      <section className="health-layout">
        <article className="card component-card">
          <header className="card-header"><div><span className="eyebrow">关键组件</span><h3>当前状态</h3></div></header>
          {gateways.map((gateway) => <div className="component-row" key={gateway.id}><span><Network /></span><div><strong>{gateway.name}</strong><small>逻辑网关 · {gateway.listeners.length} 个监听入口</small></div><strong>99.99%</strong><span>P95 4 ms</span><StatusBadge state={gateway.state} /></div>)}
          {visibleServices.map((service) => {
            const detail = service.type === 'HTTP' ? `${service.endpoints.length} 个端点` : service.capabilities[0] ?? '尚未发现能力';
            return <div className="component-row" key={service.id}><span>{service.type === 'MCP' ? <Wrench /> : <Server />}</span><div><strong>{service.name}</strong><small>{service.type === 'MODEL' ? '模型服务' : service.type === 'MCP' ? 'MCP 服务' : 'HTTP 服务'} · {detail}</small></div><strong>{service.successRate}</strong><span>{service.latency}</span><StatusBadge state={service.state} /></div>;
          })}
        </article>
        <aside className="card alert-card"><header><span className="eyebrow">运行告警</span><h3>需要处理</h3></header><div><AlertTriangle /><p><strong>Anthropic 公网响应变慢</strong><span>首个 Token P95 2.8 秒，备用线路正常</span><small><Clock3 />持续 8 分钟</small></p></div><div><AlertTriangle /><p><strong>文件服务错误率升高</strong><span>成功率 99.42%，正在观察</span><small><Clock3 />持续 11 分钟</small></p></div><footer><CheckCircle2 /><span><strong>今天已恢复 3 项</strong><small>最近恢复：搜索工具服务连接</small></span></footer></aside>
      </section>
      <section className="card release-card"><header className="card-header"><div><span className="eyebrow">配置发布</span><h3>版本记录</h3></div><StatusBadge state={releaseHistory[0]?.state === '发布中' ? 'pending' : 'healthy'} label={releaseHistory[0]?.state === '发布中' ? '正在同步' : '全部实例已同步'} /></header><div className="release-summary"><div><span>当前版本</span><strong>v{currentVersion}</strong></div><div><span>最近变更</span><strong>{releaseHistory[0]?.time ?? '—'}</strong></div><div><span>代理实例</span><strong>5 / 5</strong></div><div><span>声明资源</span><strong>{gateways.length + routes.length + services.length + certificates.length + policies.length}</strong></div></div><div className="release-history">{releaseHistory.map((release) => <div key={release.version}><span>{release.state === '发布中' ? <Clock3 /> : <CheckCircle2 />}</span><p><strong>版本 {release.version}</strong><small>{release.summary} · {release.resources}</small></p><time>{release.time}</time><StatusBadge state={release.state === '发布中' ? 'pending' : 'healthy'} label={release.state} /></div>)}</div></section>
    </div>
  );
}

export function AuditPage() {
  const { auditRecords } = usePrototype();
  const [query, setQuery] = useState('');
  const [resourceType, setResourceType] = useState<'ALL' | AuditRecord['resourceType']>('ALL');
  const visible = auditRecords.filter((record) => (resourceType === 'ALL' || record.resourceType === resourceType) && `${record.actor}${record.action}${record.resource}${record.detail}`.includes(query));
  const exportRecords = () => {
    const rows = [['时间', '操作者', '动作', '资源', '详情'], ...visible.map((record) => [record.time, record.actor, record.action, record.resource, record.detail])];
    const csv = rows.map((row) => row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(',')).join('\n');
    const url = URL.createObjectURL(new Blob([`\ufeff${csv}`], { type: 'text/csv;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'ingate-audit-2026-08-12.csv';
    link.click();
    URL.revokeObjectURL(url);
  };

  return <div className="page-stack page-enter"><PageHeader eyebrow="系统管理" title="审计日志" description="配置、权限与凭据操作" actions={<button className="button button-secondary" type="button" onClick={exportRecords} disabled={!visible.length}><Download />导出日志</button>} /><section className="card audit-card"><header className="table-toolbar"><SearchField value={query} onChange={setQuery} placeholder="搜索操作者、动作或资源" /><FilterTabs value={resourceType} onChange={setResourceType} options={[{ value: 'ALL', label: '全部', count: auditRecords.length }, { value: '网关', label: '网关' }, { value: '路由', label: '路由' }, { value: '服务', label: '服务' }, { value: '调用方', label: '调用方' }, { value: '流量策略', label: '策略' }]} /></header><div className="audit-date"><span>最近 180 天 · 当前显示 {visible.length} 项</span><i /></div>{visible.length ? visible.map((record, index) => <article className="audit-row" key={record.id}><span><FileClock /></span><div><div><strong>{record.action}</strong><StatusBadge state={record.result === '成功' ? 'healthy' : 'error'} label={record.result} /></div><p>{record.detail}</p><small>{record.resourceType} · {record.resource}</small></div><div><strong>{record.actor}</strong><time>{record.time}</time></div>{index < visible.length - 1 ? <i /> : null}</article>) : <EmptyState title="没有匹配的审计记录" description="请调整搜索或资源类型。" />}</section></div>;
}
