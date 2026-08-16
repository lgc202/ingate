import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Activity, ArrowRight, Clock3, Gauge, ShieldCheck, TriangleAlert } from 'lucide-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { getTrafficAnalysis, getTrafficAnalysisWorkspace } from '@/api/traffic';
import { useResource } from '@/api/useResource';
import { Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDuration } from '@/domain/requestRecord';
import type { RequestOutcome } from '@/domain/requestRecord';
import { localDateTime, recentTimeRange, roundUpToMinute } from '@/domain/timeRange';
import type {
  TrafficAnalysisFilters,
  TrafficAnalysisWorkspace,
  TrafficBreakdownDimension,
  TrafficBreakdownItem,
  TrafficBreakdownOrder,
  TrafficMetrics,
  TrafficTrendPoint,
} from '@/domain/traffic';
import { formatTrafficCount, formatTrafficPercent, metricNumber } from '@/domain/traffic';

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

const breakdownOrders: Array<{ value: TrafficBreakdownOrder; label: string }> = [
  { value: 'TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT', label: '请求量' },
  { value: 'TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE', label: '服务端错误率' },
  { value: 'TRAFFIC_BREAKDOWN_ORDER_P95_DURATION', label: 'P95 总耗时' },
];

type TrendMetric = 'requests' | 'latency';
type LatencyPercentile = 'p50' | 'p95' | 'p99';

interface LatencySeries {
  key: LatencyPercentile;
  label: string;
  values: Array<number | null>;
}

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
    const next = { ...draft, ...recentTimeRange(hours) };
    setDraft(next);
    setFilters(next);
  };

  const selectDimension = (dimension: TrafficBreakdownDimension) => {
    setDraft((current) => ({ ...current, breakdownDimension: dimension }));
    setFilters((current) => ({ ...current, breakdownDimension: dimension }));
  };

  const selectBreakdownOrder = (breakdownOrder: TrafficBreakdownOrder) => {
    setDraft((current) => ({ ...current, breakdownOrder }));
    setFilters((current) => ({ ...current, breakdownOrder }));
  };

  if (analysis.loading && !analysis.data) {
    return <PageFrame title="流量分析"><ResourceStatePanel title="正在加载流量分析" message="正在读取流量汇总与趋势" /></PageFrame>;
  }
  if (analysis.error || !analysis.data) {
    return <PageFrame title="流量分析"><ResourceStatePanel title="流量分析加载失败" message={analysis.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const summary = analysis.data.summary;
  const breakdownDimension = analysis.data.breakdownDimension;
  const requestCount = metricNumber(summary.requestCount);
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
        <MetricCard icon={<Activity />} label="请求总量" value={formatTrafficCount(summary.requestCount)} note={`平均 ${formatRate(requestCount / elapsedSeconds)}`} />
        <MetricCard icon={<ShieldCheck />} label="正常响应率" value={formatTrafficPercent(metricNumber(summary.nonErrorCount), requestCount)} note={`2xx/3xx · 客户端错误 ${formatTrafficCount(summary.clientErrorCount)} · 服务端错误 ${formatTrafficCount(summary.serverErrorCount)}`} tone="success" />
        <MetricCard icon={<Gauge />} label="平均总耗时" value={requestCount > 0 ? formatDuration(summary.averageDuration) : '—'} note={requestCount > 0 ? `P95 ${formatDuration(summary.p95Duration)} · 基于 ${formatTrafficCount(requestCount)} 次请求` : '当前范围没有请求'} />
        <MetricCard icon={<TriangleAlert />} label="无响应请求" value={formatTrafficCount(summary.noResponseCount)} note={requestCount > 0 ? `${formatTrafficPercent(metricNumber(summary.noResponseCount), requestCount)} 的请求未获得 HTTP 响应` : '当前范围没有请求'} tone={metricNumber(summary.noResponseCount) > 0 ? 'warning' : 'success'} />
      </section>

      <section className="traffic-analysis-grid">
        <Panel
          title="流量趋势"
          subtitle={`${formatRange(filters)} · ${trendBucketLabel(filters)}汇总`}
          actions={<div className="traffic-chart-tabs"><button type="button" className={trendMetric === 'requests' ? 'is-active' : ''} onClick={() => setTrendMetric('requests')}>请求量</button><button type="button" className={trendMetric === 'latency' ? 'is-active' : ''} onClick={() => setTrendMetric('latency')}>耗时分位</button></div>}
        >
          <TrafficTrendChart points={analysis.data.trend} metric={trendMetric} startTime={filters.startTime} endTime={filters.endTime} />
        </Panel>
        <Panel title="响应结果" subtitle="按最终 HTTP 结果分类">
          <ResponseDistribution metrics={summary} filters={filters} />
        </Panel>
      </section>

      <Panel
        title="资源排名"
        subtitle={`${breakdownOrderLabel(filters.breakdownOrder)}从高到低排列`}
        actions={<div className="traffic-ranking-controls"><label><span>排序</span><select value={filters.breakdownOrder} onChange={(event) => selectBreakdownOrder(event.target.value as TrafficBreakdownOrder)}>{breakdownOrders.map((order) => <option key={order.value} value={order.value}>{order.label}</option>)}</select></label><div className="traffic-dimension-tabs">{dimensions.map((dimension) => <button type="button" key={dimension.value} className={filters.breakdownDimension === dimension.value ? 'is-active' : ''} onClick={() => selectDimension(dimension.value)}>{dimension.label}</button>)}</div></div>}
      >
        {analysis.data.breakdown.length === 0 ? <EmptyState title="当前范围没有流量" message="调整时间或资源筛选后重新查询" /> : (
          <div className="table-scroll">
            <table className="table traffic-ranking-table">
              <thead><tr><th>{dimensionLabel(breakdownDimension)}</th><th>请求量</th><th>正常响应率</th><th>客户端错误</th><th>服务端错误率</th><th>无响应</th><th>P95 总耗时</th><th /></tr></thead>
              <tbody>{analysis.data.breakdown.map((item) => (
                <tr key={item.resourceID} onClick={() => navigate(requestRecordURL(filters, breakdownDimension, item))}>
                  <td><strong>{resourceName(names, breakdownDimension, item.resourceID, Boolean(workspace.data))}</strong></td>
                  <td>{formatTrafficCount(item.metrics.requestCount)}</td>
                  <td>{formatTrafficPercent(metricNumber(item.metrics.nonErrorCount), metricNumber(item.metrics.requestCount))}</td>
                  <td>{formatTrafficCount(item.metrics.clientErrorCount)}</td>
                  <td><strong>{formatTrafficPercent(metricNumber(item.metrics.serverErrorCount), metricNumber(item.metrics.requestCount))}</strong><small>{formatTrafficCount(item.metrics.serverErrorCount)} 次</small></td>
                  <td>{formatTrafficCount(item.metrics.noResponseCount)}</td>
                  <td><strong>{formatDuration(item.metrics.p95Duration)}</strong><small>{formatTrafficCount(item.metrics.requestCount)} 次请求</small></td>
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
  const requestValues = samples.map((sample) => metricNumber(sample.metrics.requestCount));
  const latencySeries: LatencySeries[] = [
    { key: 'p50', label: 'P50', values: latencyValues(samples, 'p50Duration') },
    { key: 'p95', label: 'P95', values: latencyValues(samples, 'p95Duration') },
    { key: 'p99', label: 'P99', values: latencyValues(samples, 'p99Duration') },
  ];
  const chartValues = metric === 'requests' ? requestValues : latencySeries.flatMap((series) => series.values);
  const maximum = niceChartMaximum(Math.max(...chartValues.filter((value): value is number => value !== null), 1));
  const width = 960;
  const height = 230;
  const top = 12;
  const bottom = 10;
  const plotHeight = height - top - bottom;
  const x = (index: number) => samples.length === 1 ? width / 2 : (index / (samples.length - 1)) * width;
  const y = (value: number) => top + plotHeight - (value / maximum) * plotHeight;
  const requestPaths = chartLinePaths(requestValues, x, y, 0);
  const requestArea = requestPaths.length === 1 ? `${requestPaths[0]} L ${width} ${top + plotHeight} L 0 ${top + plotHeight} Z` : '';
  const showLatencyPoints = samples.filter((sample) => metricNumber(sample.metrics.requestCount) > 0).length <= 24;
  const labels = chartRangeLabels(samples);
  const activeIndex = hoveredIndex !== null && hoveredIndex < samples.length ? hoveredIndex : null;
  const activeSample = activeIndex === null ? null : samples[activeIndex];
  const activeRequestValue = activeIndex === null ? null : requestValues[activeIndex];
  const activeP95Value = activeIndex === null ? null : latencySeries[1].values[activeIndex];
  const activeP99Value = activeIndex === null ? null : latencySeries[2].values[activeIndex];
  const activeLeft = activeIndex === null ? 0 : samples.length === 1 ? 50 : (activeIndex / (samples.length - 1)) * 100;
  return (
    <div className={`traffic-trend-chart is-${metric}`}>
      <div className="traffic-chart-plot">
        <div className="traffic-chart-scale"><span>{metric === 'requests' ? formatTrafficCount(maximum) : formatMilliseconds(maximum)}</span><span>{metric === 'requests' ? formatTrafficCount(maximum / 2) : formatMilliseconds(maximum / 2)}</span><span>0</span></div>
        {metric === 'latency' && <div className="traffic-latency-legend">{latencySeries.map((series) => <span key={series.key}><i className={`is-${series.key}`} />{series.label}</span>)}</div>}
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={metric === 'requests' ? '请求量趋势' : '耗时分位趋势'}
          preserveAspectRatio="none"
          onPointerMove={(event) => {
            const bounds = event.currentTarget.getBoundingClientRect();
            const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width));
            setHoveredIndex(Math.round(ratio * (samples.length - 1)));
          }}
          onPointerLeave={() => setHoveredIndex(null)}
        >
          <defs><linearGradient id="traffic-area-requests" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#4057d5" stopOpacity="0.2" /><stop offset="100%" stopColor="#4057d5" stopOpacity="0.01" /></linearGradient></defs>
          {[0, 0.5, 1].map((ratio) => <line key={ratio} x1="0" x2={width} y1={top + plotHeight * ratio} y2={top + plotHeight * ratio} className="traffic-grid-line" />)}
          {metric === 'requests' && requestArea && <path d={requestArea} fill="url(#traffic-area-requests)" />}
          {metric === 'requests' ? requestPaths.map((path, index) => <path key={index} d={path} className="traffic-trend-line is-requests" />) : latencySeries.flatMap((series) => (
            chartLinePaths(series.values, x, y, 1).map((path, index) => <path key={`${series.key}-${index}`} d={path} className={`traffic-trend-line is-${series.key}`} />)
          ))}
          {metric === 'latency' && showLatencyPoints ? <>
            {latencySeries[2].values.map((value, index) => value === null ? null : <circle key={samples[index].startedAt} cx={x(index)} cy={y(value)} r="4.5" className="traffic-trend-dot is-p99" />)}
            {latencySeries[1].values.map((value, index) => value === null ? null : <circle key={samples[index].startedAt} cx={x(index)} cy={y(value)} r="2.5" className="traffic-trend-dot is-p95" />)}
          </> : null}
          <rect x="0" y="0" width={width} height={height} className="traffic-chart-hit-area" />
          {activeIndex !== null && <line x1={x(activeIndex)} x2={x(activeIndex)} y1={top} y2={top + plotHeight} className="traffic-hover-line" />}
          {metric === 'requests' && activeIndex !== null && activeRequestValue !== null && <circle cx={x(activeIndex)} cy={y(activeRequestValue)} r="5" className="traffic-trend-dot is-active" />}
          {metric === 'latency' && activeIndex !== null && activeP99Value !== null && <circle cx={x(activeIndex)} cy={y(activeP99Value)} r="7" className="traffic-trend-dot is-active is-p99" />}
          {metric === 'latency' && activeIndex !== null && activeP95Value !== null && <circle cx={x(activeIndex)} cy={y(activeP95Value)} r="4.5" className="traffic-trend-dot is-active is-p95" />}
        </svg>
        {activeSample && <TrafficChartTooltip sample={activeSample} metric={metric} left={activeLeft} />}
      </div>
      <div className="traffic-chart-labels">{labels.map((label) => <span key={label}>{label}</span>)}</div>
    </div>
  );
}

function TrafficChartTooltip({ sample, metric, left }: { sample: TrafficTrendPoint; metric: TrendMetric; left: number }) {
  const requests = metricNumber(sample.metrics.requestCount);
  return (
    <div className={`traffic-chart-tooltip is-${metric}`} style={{ left: `${Math.min(91, Math.max(9, left))}%` }}>
      <time>{formatTrendTime(sample.startedAt)}</time>
      {metric === 'requests' ? <>
        <div><strong>{formatTrafficCount(requests)}</strong><span>请求量</span></div>
        <dl>
          <div><dt>正常响应</dt><dd>{formatTrafficPercent(metricNumber(sample.metrics.nonErrorCount), requests)}</dd></div>
          <div><dt>P95 总耗时</dt><dd>{requests > 0 ? formatDuration(sample.metrics.p95Duration) : '—'}</dd></div>
        </dl>
      </> : <>
        <div className="traffic-percentile-heading"><strong>{requests > 0 ? '耗时分位' : '无请求'}</strong><span>{formatTrafficCount(requests)} 次请求</span></div>
        <dl className="traffic-percentile-values">
          <div><dt><i className="is-p50" />P50</dt><dd>{requests > 0 ? formatDuration(sample.metrics.p50Duration) : '—'}</dd></div>
          <div><dt><i className="is-p95" />P95</dt><dd>{requests > 0 ? formatDuration(sample.metrics.p95Duration) : '—'}</dd></div>
          <div><dt><i className="is-p99" />P99</dt><dd>{requests > 0 ? formatDuration(sample.metrics.p99Duration) : '—'}</dd></div>
        </dl>
      </>}
    </div>
  );
}

function ResponseDistribution({ metrics, filters }: { metrics: TrafficMetrics; filters: TrafficAnalysisFilters }) {
  const total = metricNumber(metrics.requestCount);
  const segments: Array<{ key: string; label: string; value: number; outcome: RequestOutcome }> = [
    { key: 'success', label: '正常响应', value: metricNumber(metrics.nonErrorCount), outcome: 'REQUEST_OUTCOME_SUCCESS' },
    { key: 'client', label: '客户端错误', value: metricNumber(metrics.clientErrorCount), outcome: 'REQUEST_OUTCOME_CLIENT_ERROR' },
    { key: 'server', label: '服务端错误', value: metricNumber(metrics.serverErrorCount), outcome: 'REQUEST_OUTCOME_SERVER_ERROR' },
    { key: 'missing', label: '无响应', value: metricNumber(metrics.noResponseCount), outcome: 'REQUEST_OUTCOME_NO_RESPONSE' },
  ];
  return (
    <div className="traffic-distribution">
      <div className="traffic-distribution-total"><strong>{formatTrafficCount(total)}</strong><span>全部请求</span></div>
      <div className="traffic-distribution-bar">{segments.map((segment) => <i key={segment.key} className={`is-${segment.key}`} style={{ width: `${total > 0 ? (segment.value / total) * 100 : 0}%` }} />)}</div>
      <div className="traffic-distribution-list">{segments.map((segment) => <Link key={segment.key} to={requestResultURL(filters, segment.outcome)}><span><i className={`is-${segment.key}`} />{segment.label}</span><strong>{formatTrafficPercent(segment.value, total)}</strong><small>{formatTrafficCount(segment.value)}</small><ArrowRight /></Link>)}</div>
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
  const end = parseTime(params.get('endTime')) ?? roundUpToMinute(new Date());
  const start = parseTime(params.get('startTime')) ?? new Date(end.getTime() - 60 * 60 * 1000);
  return {
    startTime: localDateTime(start),
    endTime: localDateTime(end),
    gatewayID: params.get('gatewayID') || undefined,
    routeID: params.get('routeID') || undefined,
    serviceID: params.get('serviceID') || undefined,
    breakdownDimension: trafficDimension(params.get('breakdownDimension')),
    breakdownOrder: trafficBreakdownOrder(params.get('breakdownOrder')),
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

function trafficBreakdownOrder(value: string | null): TrafficBreakdownOrder {
  if (value === 'TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE' || value === 'TRAFFIC_BREAKDOWN_ORDER_P95_DURATION') return value;
  return 'TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT';
}

function requestRecordURL(filters: TrafficAnalysisFilters, dimension: TrafficBreakdownDimension, item: TrafficBreakdownItem): string {
  const query = requestRecordQuery(filters);
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY') query.set('gatewayID', item.resourceID);
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE') query.set('routeID', item.resourceID);
  if (dimension === 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE') query.set('serviceID', item.resourceID);
  return `/requests?${query}`;
}

function requestResultURL(filters: TrafficAnalysisFilters, outcome: RequestOutcome): string {
  const query = requestRecordQuery(filters);
  query.set('outcome', outcome);
  return `/requests?${query}`;
}

function requestRecordQuery(filters: TrafficAnalysisFilters): URLSearchParams {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
  });
  setQuery(query, 'gatewayID', filters.gatewayID);
  setQuery(query, 'routeID', filters.routeID);
  setQuery(query, 'serviceID', filters.serviceID);
  return query;
}

function setQuery(query: URLSearchParams, name: string, value?: string) {
  if (value) query.set(name, value);
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

function latencyValues(samples: TrafficTrendPoint[], field: 'p50Duration' | 'p95Duration' | 'p99Duration'): Array<number | null> {
  return samples.map((sample) => {
    const value = sample.metrics[field];
    return metricNumber(sample.metrics.requestCount) > 0 && value ? durationMilliseconds(value) : null;
  });
}

function chartLinePaths(values: Array<number | null>, x: (index: number) => number, y: (value: number) => number, maxMissingBuckets: number): string[] {
  const paths: string[] = [];
  let path = '';
  let previousIndex: number | null = null;
  values.forEach((value, index) => {
    if (value === null) return;
    if (previousIndex !== null && index - previousIndex > maxMissingBuckets + 1) {
      if (path) paths.push(path);
      path = '';
    }
    path += `${path ? ' L' : 'M'} ${x(index)} ${y(value)}`;
    previousIndex = index;
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

function breakdownOrderLabel(value: TrafficBreakdownOrder): string {
  return breakdownOrders.find((item) => item.value === value)?.label ?? '请求量';
}
