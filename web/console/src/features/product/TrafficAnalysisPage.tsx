import { useMemo, useState } from 'react';
import { Activity, Bot, Gauge, Network, Server, Zap } from 'lucide-react';
import { Badge, PageFrame } from '@/components/ui';

type TrafficKind = 'all' | 'api' | 'ai';

interface TrafficRow {
  name: string;
  kind: Exclude<TrafficKind, 'all'>;
  service: string;
  requests: string;
  successRate: string;
  latency: string;
  traffic: string;
  state: 'healthy' | 'warning';
}

const rows: TrafficRow[] = [
  { name: '订单查询 API', kind: 'api', service: '订单服务', requests: '58.4K', successRate: '99.96%', latency: '86 ms', traffic: '8.4 GB', state: 'healthy' },
  { name: '客户资料 API', kind: 'api', service: '客户中心', requests: '33.8K', successRate: '99.91%', latency: '124 ms', traffic: '4.1 GB', state: 'healthy' },
  { name: '文件上传 API', kind: 'api', service: '文件服务', requests: '6.1K', successRate: '99.42%', latency: '680 ms', traffic: '38.6 GB', state: 'warning' },
  { name: '生产 AI 路由', kind: 'ai', service: '通义千问生产等 4 个服务', requests: '28.4K', successRate: '99.72%', latency: '1.8 s', traffic: '18.7M Token · ¥ 1,284', state: 'warning' },
  { name: '内部 AI 路由', kind: 'ai', service: '内部向量服务', requests: '16.0K', successRate: '99.99%', latency: '112 ms', traffic: '1.0M Token · ¥ 46', state: 'healthy' },
];

const metrics: Record<TrafficKind, Array<{ label: string; value: string; note: string }>> = {
  all: [
    { label: '今日请求', value: '142.7K', note: '较昨日 +6.4%' },
    { label: '成功率', value: '99.82%', note: '失败 257 次' },
    { label: 'P95 延迟', value: '186 ms', note: '全量请求' },
    { label: 'AI Token', value: '19.7M', note: '估算成本 ¥ 1,330' },
  ],
  api: [
    { label: 'API 请求', value: '98.3K', note: '占总请求 68.9%' },
    { label: '成功率', value: '99.89%', note: '失败 108 次' },
    { label: 'P95 延迟', value: '128 ms', note: 'HTTP 服务' },
    { label: '响应流量', value: '51.1 GB', note: '较昨日 +3.2%' },
  ],
  ai: [
    { label: 'AI 请求', value: '44.4K', note: '占总请求 31.1%' },
    { label: '成功率', value: '99.68%', note: '主备切换 16 次' },
    { label: 'P95 延迟', value: '1.8 s', note: '模型首 Token 612 ms' },
    { label: 'Token / 成本', value: '19.7M', note: '估算成本 ¥ 1,330' },
  ],
};

export function TrafficAnalysisPage() {
  const [kind, setKind] = useState<TrafficKind>('all');
  const visibleRows = useMemo(() => rows.filter((row) => kind === 'all' || row.kind === kind), [kind]);

  return (
    <PageFrame title="流量分析" subtitle="统一分析普通 API 和 AI 请求的流量、成功率、延迟与资源消耗">
      <nav className="resource-kind-tabs analysis-tabs">
        <FilterButton active={kind === 'all'} onClick={() => setKind('all')} icon={Network} label="全部流量" count="142.7K" />
        <FilterButton active={kind === 'api'} onClick={() => setKind('api')} icon={Server} label="普通 API" count="98.3K" />
        <FilterButton active={kind === 'ai'} onClick={() => setKind('ai')} icon={Bot} label="AI 请求" count="44.4K" />
      </nav>

      <section className="product-metrics four">
        {metrics[kind].map((metric, index) => <Metric key={metric.label} icon={[Activity, Gauge, Zap, Bot][index]} {...metric} />)}
      </section>

      <section className="cost-layout">
        <div className="surface-card cost-chart">
          <header><div><span className="eyebrow">过去 14 天</span><h3>请求量趋势</h3></div><Badge tone="success">高峰期成功率 99.76%</Badge></header>
          <div className="bar-chart">{[48, 63, 58, 72, 54, 77, 82, 68, 74, 88, 79, 91, 84, 96].map((height, index) => <i key={index}><b style={{ height: `${height}%` }} /><span>{index % 3 === 0 ? `${index + 1}日` : ''}</span></i>)}</div>
        </div>
        <div className="surface-card provider-cost traffic-composition">
          <header><span className="eyebrow">请求构成</span><h3>API 与 AI 流量</h3></header>
          <div className="cost-donut"><strong>142.7K</strong><span>今日请求</span></div>
          <div><p><i />普通 API <strong>68.9%</strong></p><p><i />AI 请求 <strong>31.1%</strong></p><p><i />策略拒绝 <strong>0.09%</strong></p></div>
        </div>
      </section>

      <section className="product-table-card">
        <header><div><h3>按路由查看</h3><p>AI 路由额外展示 Token 与估算成本，普通 API 展示响应流量</p></div><span className="table-period">过去 24 小时</span></header>
        <div className="product-table-head traffic-grid"><span>路由 / 类型</span><span>目标服务</span><span>请求</span><span>成功率</span><span>P95 延迟</span><span>资源用量</span><span>状态</span></div>
        {visibleRows.map((row) => <div className="product-table-row traffic-grid" key={row.name}><div className="name-cell"><span>{row.kind === 'api' ? <Server /> : <Bot />}</span><div><strong>{row.name}</strong><code>{row.kind === 'api' ? '普通 API' : 'AI 请求'}</code></div></div><span>{row.service}</span><strong>{row.requests}</strong><span>{row.successRate}</span><span>{row.latency}</span><span>{row.traffic}</span><Badge tone={row.state === 'healthy' ? 'success' : 'warning'}>{row.state === 'healthy' ? '正常' : '需关注'}</Badge></div>)}
      </section>
    </PageFrame>
  );
}

function FilterButton({ active, onClick, icon: Icon, label, count }: { active: boolean; onClick: () => void; icon: typeof Network; label: string; count: string }) {
  return <button type="button" className={active ? 'is-active' : ''} onClick={onClick}><Icon />{label}<span>{count}</span></button>;
}

function Metric({ icon: Icon, label, value, note }: { icon: typeof Activity; label: string; value: string; note: string }) {
  return <article><span><Icon /></span><div><small>{label}</small><strong>{value}</strong><p>{note}</p></div></article>;
}
