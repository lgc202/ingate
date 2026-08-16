import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Activity, ArrowRight, Clock3, Gauge, ShieldCheck, TriangleAlert } from 'lucide-react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { getTrafficAnalysis, getTrafficAnalysisWorkspace } from '@/api/traffic';
import { useResource } from '@/api/useResource';
import { Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDuration } from '@/domain/requestRecord';
import type {
  TrafficAnalysisFilters,
  TrafficAnalysisWorkspace,
  TrafficBreakdownDimension,
  TrafficBreakdownItem,
  TrafficMetrics,
  TrafficTrendPoint,
} from '@/domain/traffic';
import { metricNumber } from '@/domain/traffic';

const presets = [
  { label: '最近 1 小时', hours: 1 },
  { label: '最近 6 小时', hours: 6 },
  { label: '最近 24 小时', hours: 24 },
  { label: '最近 7 天', hours: 7 * 24 },
];

const dimensions: Array<{ value: TrafficBreakdownDimension; label: string }> = [
  { value: 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY', label: '网关' },
  { value: 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE', label: '路由' },
  { value: 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE', label: '服务' },
];

type TrendMetric = 'requests' | 'latency';

export function TrafficAnalysisPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [initialFilters] = useState(() => filtersFromURL(searchParams));
  const [draft, setDraft] = useState<TrafficAnalysisFilters>(initialFilters);
  const [filters, setFilters] = useState<TrafficAnalysisFilters>(initialFilters);
  const [trendMetric, setTrendMetric] = useState<TrendMetric>('requests');
  const [notice, setNotice] = useState<string | null>(null);

  const loadAnalysis = useCallback(() => getTrafficAnalysis(filters), [filters]);
  const analysis = useResource(loadAnalysis);
  const workspace = useResource(getTrafficAnalysisWorkspace);
  const names = useMemo(() => resourceNames(workspace.data), [workspace.data]);

  const applyFilters = () => {
    if (!draft.startTime || !draft.endTime || new Date(draft.startTime) >= new Date(draft.endTime)) {
      setNotice('查询开始时间必须早于结束时间');
      return;
    }
    setFilters({ ...draft });
  };

  const applyPreset = (hours: number) => {
    const end = new Date();
    const start = new Date(end.getTime() - hours * 60 * 60 * 1000);
    const next = { ...draft, startTime: localDateTime(start), endTime: localDateTime(end) };
    setDraft(next);
    setFilters(next);
  };

  const selectDimension = (dimension: TrafficBreakdownDimension) => {
    setDraft((current) => ({ ...current, breakdownDimension: dimension }));
    setFilters((current) => ({ ...current, breakdownDimension: dimension }));
  };

  if (analysis.loading && !analysis.data) {
    return <PageFrame title="流量分析"><ResourceStatePanel title="正在加载流量分析" message="正在读取流量汇总与趋势" /></PageFrame>;
  }
  if (analysis.error || !analysis.data) {
    return <PageFrame title="流量分析"><ResourceStatePanel title="流量分析加载失败" message={analysis.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const summary = analysis.data.summary;
  const breakdownDimension = analysis.data.breakdownDimension;
  const elapsedSeconds = Math.max(1, (new Date(filters.endTime).getTime() - new Date(filters.startTime).getTime()) / 1000);

  return (
    <PageFrame title="流量分析">
      <Panel>
        <section className="traffic-filter-panel">
          <div className="traffic-presets" aria-label="快捷时间范围">
            {presets.map((preset) => (
              <button type="button" key={preset.label} className={matchesPreset(draft, preset.hours) ? 'is-active' : ''} onClick={() => applyPreset(preset.hours)}>{preset.label}</button>
            ))}
          </div>
          <div className="traffic-filter-grid">
            <Field label="开始时间"><input className="input" type="datetime-local" value={draft.startTime} onChange={(event) => setDraft({ ...draft, startTime: event.target.value })} /></Field>
            <Field label="结束时间"><input className="input" type="datetime-local" value={draft.endTime} onChange={(event) => setDraft({ ...draft, endTime: event.target.value })} /></Field>
            <Field label="网关"><ResourceSelect value={draft.gatewayID} placeholder="全部网关" options={workspace.data?.gateways} onChange={(gatewayID) => setDraft({ ...draft, gatewayID })} /></Field>
            <Field label="路由"><ResourceSelect value={draft.routeID} placeholder="全部路由" options={workspace.data?.routes} onChange={(routeID) => setDraft({ ...draft, routeID })} /></Field>
            <Field label="服务"><ResourceSelect value={draft.serviceID} placeholder="全部服务" options={workspace.data?.services} onChange={(serviceID) => setDraft({ ...draft, serviceID })} /></Field>
            <Button onClick={applyFilters}>查询</Button>
          </div>
        </section>
      </Panel>

      <section className="traffic-metric-grid">
        <MetricCard icon={<Activity />} label="请求总量" value={formatCount(summary.requestCount)} note={`平均 ${formatRate(metricNumber(summary.requestCount) / elapsedSeconds)}`} />
        <MetricCard icon={<ShieldCheck />} label="正常响应率" value={formatPercent(metricNumber(summary.nonErrorCount), metricNumber(summary.requestCount))} note={`2xx/3xx · 4xx ${formatCount(summary.clientErrorCount)} · 5xx ${formatCount(summary.serverErrorCount)}`} tone="success" />
        <MetricCard icon={<Gauge />} label="P95 总耗时" value={formatDuration(summary.p95Duration)} note={`P50 ${formatDuration(summary.p50Duration)} · P99 ${formatDuration(summary.p99Duration)}`} />
        <MetricCard icon={<TriangleAlert />} label="无响应请求" value={formatCount(summary.noResponseCount)} note={`${formatPercent(metricNumber(summary.noResponseCount), metricNumber(summary.requestCount))} 的请求未获得 HTTP 响应`} tone={metricNumber(summary.noResponseCount) > 0 ? 'warning' : 'success'} />
      </section>

      <section className="traffic-analysis-grid">
        <Panel
          title="流量趋势"
          subtitle={`${formatRange(filters)} · ${trendBucketLabel(filters)}汇总`}
          actions={<div className="traffic-chart-tabs"><button type="button" className={trendMetric === 'requests' ? 'is-active' : ''} onClick={() => setTrendMetric('requests')}>请求量</button><button type="button" className={trendMetric === 'latency' ? 'is-active' : ''} onClick={() => setTrendMetric('latency')}>P95 延迟</button></div>}
        >
          <TrafficTrendChart points={analysis.data.trend} metric={trendMetric} startTime={filters.startTime} endTime={filters.endTime} />
        </Panel>
        <Panel title="响应结果" subtitle="按最终 HTTP 结果分类">
          <ResponseDistribution metrics={summary} />
        </Panel>
      </section>

      <Panel
        title="资源排名"
        subtitle="按请求量从高到低排列"
        actions={<div className="traffic-dimension-tabs">{dimensions.map((dimension) => <button type="button" key={dimension.value} className={filters.breakdownDimension === dimension.value ? 'is-active' : ''} onClick={() => selectDimension(dimension.value)}>{dimension.label}</button>)}</div>}
      >
        {analysis.data.breakdown.length === 0 ? <EmptyState title="当前范围没有流量" message="调整时间或资源筛选后重新查询" /> : (
          <div className="table-scroll">
            <table className="table traffic-ranking-table">
              <thead><tr><th>{dimensionLabel(breakdownDimension)}</th><th>请求量</th><th>正常响应率</th><th>客户端错误</th><th>服务端错误</th><th>无响应</th><th>P95 总耗时</th><th /></tr></thead>
              <tbody>{analysis.data.breakdown.map((item) => (
                <tr key={item.resourceID} onClick={() => navigate(requestRecordURL(filters, breakdownDimension, item))}>
                  <td><strong>{resourceName(names, breakdownDimension, item.resourceID, Boolean(workspace.data))}</strong></td>
                  <td>{formatCount(item.metrics.requestCount)}</td>
                  <td>{formatPercent(metricNumber(item.metrics.nonErrorCount), metricNumber(item.metrics.requestCount))}</td>
                  <td>{formatCount(item.metrics.clientErrorCount)}</td>
                  <td>{formatCount(item.metrics.serverErrorCount)}</td>
                  <td>{formatCount(item.metrics.noResponseCount)}</td>
                  <td>{formatDuration(item.metrics.p95Duration)}</td>
                  <td><ArrowRight /></td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        )}
      </Panel>
      <Toast message={notice} tone="error" onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function MetricCard({ icon, label, value, note, tone = 'accent' }: { icon: ReactNode; label: string; value: string; note: string; tone?: 'accent' | 'success' | 'warning' }) {
  return <article className={`traffic-metric-card is-${tone}`}><span>{icon}</span><div><small>{label}</small><strong>{value}</strong><p>{note}</p></div></article>;
}

function TrafficTrendChart({ points, metric, startTime, endTime }: { points: TrafficTrendPoint[]; metric: TrendMetric; startTime: string; endTime: string }) {
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  if (points.length === 0) {
    return <div className="traffic-chart-empty"><Clock3 /><span>当前范围没有趋势数据</span></div>;
  }

  const samples = chartSamples(points, startTime, endTime);
  const values = samples.map((sample) => {
    if (metric === 'requests') return metricNumber(sample.metrics.requestCount);
    return metricNumber(sample.metrics.requestCount) > 0 && sample.metrics.p95Duration ? durationMilliseconds(sample.metrics.p95Duration) : null;
  });
  const observedValueCount = values.filter((value): value is number => value !== null).length;
  const maximum = niceChartMaximum(Math.max(...values.filter((value): value is number => value !== null), 1));
  const width = 960;
  const height = 230;
  const top = 12;
  const bottom = 10;
  const plotHeight = height - top - bottom;
  const x = (index: number) => samples.length === 1 ? width / 2 : (index / (samples.length - 1)) * width;
  const y = (value: number) => top + plotHeight - (value / maximum) * plotHeight;
  const paths = chartLinePaths(values, x, y, metric === 'latency');
  const area = metric === 'requests' && paths.length === 1 ? `${paths[0]} L ${width} ${top + plotHeight} L 0 ${top + plotHeight} Z` : '';
  const labels = chartRangeLabels(samples);
  const activeIndex = hoveredIndex !== null && hoveredIndex < samples.length ? hoveredIndex : null;
  const activeSample = activeIndex === null ? null : samples[activeIndex];
  const activeValue = activeIndex === null ? null : values[activeIndex];
  const activeLeft = activeIndex === null ? 0 : samples.length === 1 ? 50 : (activeIndex / (samples.length - 1)) * 100;
  return (
    <div className="traffic-trend-chart">
      <div className="traffic-chart-plot">
        <div className="traffic-chart-scale"><span>{metric === 'requests' ? formatCount(maximum) : formatMilliseconds(maximum)}</span><span>{metric === 'requests' ? formatCount(maximum / 2) : formatMilliseconds(maximum / 2)}</span><span>0</span></div>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={metric === 'requests' ? '请求量趋势' : 'P95 延迟趋势'}
          preserveAspectRatio="none"
          onPointerMove={(event) => {
            const bounds = event.currentTarget.getBoundingClientRect();
            const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width));
            setHoveredIndex(Math.round(ratio * (samples.length - 1)));
          }}
          onPointerLeave={() => setHoveredIndex(null)}
        >
          <defs><linearGradient id={`traffic-area-${metric}`} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#4057d5" stopOpacity="0.2" /><stop offset="100%" stopColor="#4057d5" stopOpacity="0.01" /></linearGradient></defs>
          {[0, 0.5, 1].map((ratio) => <line key={ratio} x1="0" x2={width} y1={top + plotHeight * ratio} y2={top + plotHeight * ratio} className="traffic-grid-line" />)}
          {area && <path d={area} fill={`url(#traffic-area-${metric})`} />}
          {paths.map((path, index) => <path key={index} d={path} className={`traffic-trend-line is-${metric}`} />)}
          {metric === 'latency' && observedValueCount === 1 ? values.map((value, index) => value === null ? null : <circle key={samples[index].startedAt} cx={x(index)} cy={y(value)} r="4" className="traffic-trend-dot" />) : null}
          <rect x="0" y="0" width={width} height={height} className="traffic-chart-hit-area" />
          {activeIndex !== null && <line x1={x(activeIndex)} x2={x(activeIndex)} y1={top} y2={top + plotHeight} className="traffic-hover-line" />}
          {activeIndex !== null && activeValue !== null && <circle cx={x(activeIndex)} cy={y(activeValue)} r="5" className="traffic-trend-dot is-active" />}
        </svg>
        {activeSample && <TrafficChartTooltip sample={activeSample} value={activeValue} metric={metric} left={activeLeft} />}
      </div>
      <div className="traffic-chart-labels">{labels.map((label) => <span key={label}>{label}</span>)}</div>
    </div>
  );
}

function TrafficChartTooltip({ sample, value, metric, left }: { sample: TrafficTrendPoint; value: number | null; metric: TrendMetric; left: number }) {
  const requests = metricNumber(sample.metrics.requestCount);
  return (
    <div className="traffic-chart-tooltip" style={{ left: `${Math.min(91, Math.max(9, left))}%` }}>
      <time>{formatTrendTime(sample.startedAt)}</time>
      <div><strong>{value === null ? '无请求' : metric === 'requests' ? formatCount(value) : formatMilliseconds(value)}</strong><span>{metric === 'requests' ? '请求量' : 'P95 总耗时'}</span></div>
      <dl>
        <div><dt>正常响应</dt><dd>{requests > 0 ? formatPercent(metricNumber(sample.metrics.nonErrorCount), requests) : '—'}</dd></div>
        <div><dt>P95 总耗时</dt><dd>{requests > 0 ? formatDuration(sample.metrics.p95Duration) : '—'}</dd></div>
      </dl>
    </div>
  );
}

function ResponseDistribution({ metrics }: { metrics: TrafficMetrics }) {
  const total = metricNumber(metrics.requestCount);
  const segments = [
    { key: 'success', label: '正常响应', value: metricNumber(metrics.nonErrorCount) },
    { key: 'client', label: '客户端错误', value: metricNumber(metrics.clientErrorCount) },
    { key: 'server', label: '服务端错误', value: metricNumber(metrics.serverErrorCount) },
    { key: 'missing', label: '无响应', value: metricNumber(metrics.noResponseCount) },
  ];
  return (
    <div className="traffic-distribution">
      <div className="traffic-distribution-total"><strong>{formatCount(total)}</strong><span>全部请求</span></div>
      <div className="traffic-distribution-bar">{segments.map((segment) => <i key={segment.key} className={`is-${segment.key}`} style={{ width: `${total > 0 ? (segment.value / total) * 100 : 0}%` }} />)}</div>
      <div className="traffic-distribution-list">{segments.map((segment) => <div key={segment.key}><span><i className={`is-${segment.key}`} />{segment.label}</span><strong>{formatPercent(segment.value, total)}</strong><small>{formatCount(segment.value)}</small></div>)}</div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="field"><span>{label}</span>{children}</label>;
}

function ResourceSelect({ value, placeholder, options, onChange }: { value?: string; placeholder: string; options?: Array<{ id: string; name: string }>; onChange: (value?: string) => void }) {
  return <select className="select" value={value ?? ''} onChange={(event) => onChange(event.target.value || undefined)}><option value="">{placeholder}</option>{options?.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}</select>;
}

interface ResourceNames {
  gateways: Map<string, string>;
  routes: Map<string, string>;
  services: Map<string, string>;
}

function resourceNames(workspace: TrafficAnalysisWorkspace | null): ResourceNames {
  return {
    gateways: new Map(workspace?.gateways.map(({ id, name }) => [id, name])),
    routes: new Map(workspace?.routes.map(({ id, name }) => [id, name])),
    services: new Map(workspace?.services.map(({ id, name }) => [id, name])),
  };
}

function resourceName(names: ResourceNames, dimension: TrafficBreakdownDimension, id: string, namesReady: boolean): string {
  if (!namesReady) return '名称加载中';
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY') return names.gateways.get(id) || '已删除的网关';
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE') return names.services.get(id) || '已删除的服务';
  return names.routes.get(id) || '已删除的路由';
}

function filtersFromURL(params: URLSearchParams): TrafficAnalysisFilters {
  const end = parseTime(params.get('endTime')) ?? new Date();
  const start = parseTime(params.get('startTime')) ?? new Date(end.getTime() - 60 * 60 * 1000);
  return {
    startTime: localDateTime(start),
    endTime: localDateTime(end),
    gatewayID: params.get('gatewayID') || undefined,
    routeID: params.get('routeID') || undefined,
    serviceID: params.get('serviceID') || undefined,
    breakdownDimension: trafficDimension(params.get('breakdownDimension')),
  };
}

function parseTime(value: string | null): Date | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function trafficDimension(value: string | null): TrafficBreakdownDimension {
  if (value === 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY' || value === 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE') return value;
  return 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE';
}

function requestRecordURL(filters: TrafficAnalysisFilters, dimension: TrafficBreakdownDimension, item: TrafficBreakdownItem): string {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
  });
  setQuery(query, 'gatewayID', filters.gatewayID);
  setQuery(query, 'routeID', filters.routeID);
  setQuery(query, 'serviceID', filters.serviceID);
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY') query.set('gatewayID', item.resourceID);
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE') query.set('routeID', item.resourceID);
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE') query.set('serviceID', item.resourceID);
  return `/requests?${query}`;
}

function setQuery(query: URLSearchParams, name: string, value?: string) {
  if (value) query.set(name, value);
}

function localDateTime(value: Date): string {
  const offset = value.getTimezoneOffset() * 60_000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
}

function formatCount(value: string | number | undefined): string {
  return new Intl.NumberFormat('zh-CN', { notation: metricNumber(value) >= 10_000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(metricNumber(value));
}

function formatRate(value: number): string {
  if (value >= 1) return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 1 }).format(value)} 次/秒`;
  if (value * 60 >= 1) return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 1 }).format(value * 60)} 次/分钟`;
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 1 }).format(value * 3600)} 次/小时`;
}

function matchesPreset(filters: TrafficAnalysisFilters, hours: number): boolean {
  const start = new Date(filters.startTime).getTime();
  const end = new Date(filters.endTime).getTime();
  return Number.isFinite(start) && Number.isFinite(end) && end - start === hours * 60 * 60 * 1000;
}

function formatPercent(value: number, total: number): string {
  if (total <= 0) return '0.00%';
  return `${((value / total) * 100).toFixed(2)}%`;
}

function durationMilliseconds(value?: string): number {
  if (!value) return 0;
  const seconds = Number(value.replace(/s$/, ''));
  return Number.isFinite(seconds) ? seconds * 1000 : 0;
}

function formatMilliseconds(value: number): string {
  if (value < 1) return `${Math.round(value * 1000)} μs`;
  if (value < 1000) return `${value.toFixed(value < 10 ? 2 : 1)} ms`;
  return `${(value / 1000).toFixed(2)} s`;
}

function formatTrendTime(value: string): string {
  const date = new Date(value);
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date);
}

function chartSamples(points: TrafficTrendPoint[], startTime: string, endTime: string): TrafficTrendPoint[] {
  const start = new Date(startTime).getTime();
  const end = new Date(endTime).getTime();
  const interval = trendBucketMilliseconds(end - start);
  const firstBucket = Math.floor(start / interval) * interval;
  const lastBucket = Math.floor((end - 1) / interval) * interval;
  const pointByTime = new Map(points.map((point) => [new Date(point.startedAt).getTime(), point]));
  const samples: TrafficTrendPoint[] = [];
  for (let startedAt = firstBucket; startedAt <= lastBucket; startedAt += interval) {
    samples.push(pointByTime.get(startedAt) ?? {
      startedAt: new Date(startedAt).toISOString(),
      metrics: {
        requestCount: 0,
        nonErrorCount: 0,
        clientErrorCount: 0,
        serverErrorCount: 0,
        noResponseCount: 0,
      },
    });
  }
  return samples;
}

function chartLinePaths(values: Array<number | null>, x: (index: number) => number, y: (value: number) => number, connectGaps: boolean): string[] {
  if (connectGaps) {
    const path = values.reduce((current, value, index) => (
      value === null ? current : `${current}${current ? ' L' : 'M'} ${x(index)} ${y(value)}`
    ), '');
    return path ? [path] : [];
  }

  const paths: string[] = [];
  let path = '';
  values.forEach((value, index) => {
    if (value === null) {
      if (path) paths.push(path);
      path = '';
      return;
    }
    path += `${path ? ' L' : 'M'} ${x(index)} ${y(value)}`;
  });
  if (path) paths.push(path);
  return paths;
}

function niceChartMaximum(value: number): number {
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  if (normalized <= 1) return magnitude;
  if (normalized <= 1.25) return 1.25 * magnitude;
  if (normalized <= 1.5) return 1.5 * magnitude;
  if (normalized <= 2) return 2 * magnitude;
  if (normalized <= 2.5) return 2.5 * magnitude;
  if (normalized <= 5) return 5 * magnitude;
  if (normalized <= 7.5) return 7.5 * magnitude;
  return 10 * magnitude;
}

function chartRangeLabels(samples: TrafficTrendPoint[]): string[] {
  if (samples.length === 1) return [formatTrendTime(samples[0].startedAt)];
  const middle = Math.floor((samples.length - 1) / 2);
  return [samples[0], samples[middle], samples[samples.length - 1]].map((sample) => formatTrendTime(sample.startedAt));
}

function trendBucketMilliseconds(range: number): number {
  if (range <= 2 * 60 * 60 * 1000) return 60 * 1000;
  if (range <= 24 * 60 * 60 * 1000) return 5 * 60 * 1000;
  if (range <= 7 * 24 * 60 * 60 * 1000) return 60 * 60 * 1000;
  return 24 * 60 * 60 * 1000;
}

function trendBucketLabel(filters: TrafficAnalysisFilters): string {
  const range = new Date(filters.endTime).getTime() - new Date(filters.startTime).getTime();
  const interval = trendBucketMilliseconds(range);
  if (interval === 60 * 1000) return '每分钟';
  if (interval === 5 * 60 * 1000) return '每 5 分钟';
  if (interval === 60 * 60 * 1000) return '每小时';
  return '每天';
}

function formatRange(filters: TrafficAnalysisFilters): string {
  return `${filters.startTime.replace('T', ' ')}—${filters.endTime.replace('T', ' ')}`;
}

function dimensionLabel(value: TrafficBreakdownDimension): string {
  return dimensions.find((item) => item.value === value)?.label ?? '资源';
}
