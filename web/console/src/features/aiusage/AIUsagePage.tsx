import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { ArrowDownToLine, ArrowUpFromLine, Bot, Sigma } from 'lucide-react';
import { getAIUsageAnalysis, getAIUsageWorkspace } from '@/api/aiUsage';
import { useResource } from '@/api/useResource';
import {
  Button,
  EmptyState,
  PageFrame,
  Panel,
  ResourceFilterField,
  ResourceListFilters,
  ResourceStatePanel,
  Toast,
} from '@/components/ui';
import type {
  AIUsageAnalysis,
  AIUsageBreakdownDimension,
  AIUsageBreakdownItem,
  AIUsageBreakdownOrder,
  AIUsageFilters,
  AIUsageResourceOption,
  AIUsageWorkspace,
} from '@/domain/aiUsage';
import { formatUsageCount, formatUsagePercent, usageNumber } from '@/domain/aiUsage';
import { localDateTime, roundUpToMinute } from '@/domain/timeRange';
import { AIUsageTrendChart, type AIUsageTrendMetric } from './AIUsageTrendChart';

const timePresets = [
  { value: 'today', label: '今天' },
  { value: '7d', label: '近 7 天' },
  { value: '15d', label: '近 15 天' },
  { value: '30d', label: '近 30 天' },
] as const;

const dimensions: Array<{ value: AIUsageBreakdownDimension; label: string }> = [
  { value: 'AI_USAGE_BREAKDOWN_DIMENSION_CALLER', label: '调用方' },
  { value: 'AI_USAGE_BREAKDOWN_DIMENSION_ROUTE', label: 'AI 路由' },
  { value: 'AI_USAGE_BREAKDOWN_DIMENSION_CLIENT_MODEL', label: '对外模型' },
  { value: 'AI_USAGE_BREAKDOWN_DIMENSION_SERVICE', label: '模型服务' },
  { value: 'AI_USAGE_BREAKDOWN_DIMENSION_ACTUAL_MODEL', label: '实际模型' },
];

const breakdownOrders: Array<{ value: AIUsageBreakdownOrder; label: string }> = [
  { value: 'AI_USAGE_BREAKDOWN_ORDER_CALL_COUNT', label: '模型调用' },
  { value: 'AI_USAGE_BREAKDOWN_ORDER_TOTAL_TOKENS', label: '总 Token' },
];

type TimePreset = typeof timePresets[number]['value'];

interface ResourceNames {
  callers: Map<string, string>;
  routes: Map<string, string>;
  services: Map<string, string>;
}

export function AIUsagePage() {
  const [initialFilters] = useState(defaultFilters);
  const [draft, setDraft] = useState<AIUsageFilters>(initialFilters);
  const [filters, setFilters] = useState<AIUsageFilters>(initialFilters);
  const [timePreset, setTimePreset] = useState<TimePreset | null>('today');
  const [trendMetric, setTrendMetric] = useState<AIUsageTrendMetric>('tokens');
  const [notice, setNotice] = useState<string | null>(null);

  const loadAnalysis = useCallback(() => getAIUsageAnalysis(filters), [filters]);
  const analysis = useResource(loadAnalysis);
  const workspace = useResource(getAIUsageWorkspace);
  const names = useMemo(() => resourceNames(workspace.data), [workspace.data]);

  const applyFilters = () => {
    if (!draft.startTime || !draft.endTime || new Date(draft.startTime) >= new Date(draft.endTime)) {
      setNotice('查询开始时间必须早于结束时间');
      return;
    }
    if (new Date(draft.endTime).getTime() - new Date(draft.startTime).getTime() > 90 * 24 * 60 * 60 * 1000) {
      setNotice('单次最多查询 90 天 AI 用量');
      return;
    }
    setFilters({ ...draft });
  };

  const applyTimePreset = (preset: TimePreset) => {
    const next = { ...draft, ...presetTimeRange(preset) };
    setTimePreset(preset);
    setDraft(next);
    setFilters(next);
  };

  const resetFilters = () => {
    const next = defaultFilters();
    setTimePreset('today');
    setDraft(next);
    setFilters(next);
  };

  const selectDimension = (dimension: AIUsageBreakdownDimension) => {
    setDraft((current) => ({ ...current, breakdownDimension: dimension }));
    setFilters((current) => ({ ...current, breakdownDimension: dimension }));
  };

  const selectOrder = (order: AIUsageBreakdownOrder) => {
    setDraft((current) => ({ ...current, breakdownOrder: order }));
    setFilters((current) => ({ ...current, breakdownOrder: order }));
  };

  if (analysis.loading && !analysis.data) {
    return <PageFrame title="AI 用量"><ResourceStatePanel title="正在加载 AI 用量" message="正在汇总模型调用与 Token 用量" /></PageFrame>;
  }
  if (analysis.error || !analysis.data) {
    return (
      <PageFrame title="AI 用量">
        <Panel title="AI 用量加载失败">
          <div className="ai-usage-error-state"><p>{analysis.error?.message ?? '请稍后重试'}</p><Button variant="outline" onClick={() => void analysis.reload()}>重新加载</Button></div>
        </Panel>
      </PageFrame>
    );
  }

  const summary = analysis.data.summary;
  const callCount = usageNumber(summary.callCount);
  const namesReady = Boolean(workspace.data);

  return (
    <PageFrame title="AI 用量">
      <Panel>
        <ResourceListFilters
          summary={formatRange(filters)}
          resultLabel={`${analysis.loading ? '更新中 · ' : ''}${formatUsageCount(summary.callCount)} 次模型调用`}
          onSearch={applyFilters}
          onReset={resetFilters}
        >
          <div className="resource-filter-presets" aria-label="快捷时间范围">
            <span>时间范围</span>
            {timePresets.map((preset) => <button type="button" key={preset.value} className={timePreset === preset.value ? 'is-active' : ''} onClick={() => applyTimePreset(preset.value)}>{preset.label}</button>)}
          </div>
          {workspace.error ? <div className="ai-usage-filter-warning">资源筛选项加载失败，仍可按时间范围查询</div> : null}
          <ResourceFilterField label="开始时间"><input className="input" type="datetime-local" value={draft.startTime} onChange={(event) => { setTimePreset(null); setDraft({ ...draft, startTime: event.target.value }); }} /></ResourceFilterField>
          <ResourceFilterField label="结束时间"><input className="input" type="datetime-local" value={draft.endTime} onChange={(event) => { setTimePreset(null); setDraft({ ...draft, endTime: event.target.value }); }} /></ResourceFilterField>
          <ResourceFilterField label="网关"><ResourceSelect value={draft.gatewayID} placeholder="全部网关" options={workspace.data?.gateways} loading={workspace.loading} onChange={(gatewayID) => setDraft({ ...draft, gatewayID })} /></ResourceFilterField>
          <ResourceFilterField label="调用方"><ResourceSelect value={draft.callerID} placeholder="全部调用方" options={workspace.data?.callers} loading={workspace.loading} onChange={(callerID) => setDraft({ ...draft, callerID })} /></ResourceFilterField>
          <ResourceFilterField label="AI 路由"><ResourceSelect value={draft.routeID} placeholder="全部 AI 路由" options={workspace.data?.routes} loading={workspace.loading} onChange={(routeID) => setDraft({ ...draft, routeID })} /></ResourceFilterField>
          <ResourceFilterField label="对外模型"><StringSelect value={draft.clientModel} placeholder="全部对外模型" options={workspace.data?.clientModels} loading={workspace.loading} onChange={(clientModel) => setDraft({ ...draft, clientModel })} /></ResourceFilterField>
          <ResourceFilterField label="模型服务"><ResourceSelect value={draft.serviceID} placeholder="全部模型服务" options={workspace.data?.services} loading={workspace.loading} onChange={(serviceID) => setDraft({ ...draft, serviceID })} /></ResourceFilterField>
          <ResourceFilterField label="实际模型"><StringSelect value={draft.actualModel} placeholder="全部实际模型" options={workspace.data?.actualModels} loading={workspace.loading} onChange={(actualModel) => setDraft({ ...draft, actualModel })} /></ResourceFilterField>
        </ResourceListFilters>
      </Panel>

      <section className="traffic-metric-grid">
        <UsageMetricCard icon={<Bot />} label="模型调用" value={formatUsageCount(summary.callCount)} note={`正常响应率 ${formatUsagePercent(usageNumber(summary.normalResponseCount), callCount)}`} />
        <UsageMetricCard icon={<ArrowDownToLine />} label="输入 Token" value={formatUsageCount(summary.inputTokens)} tone="input" />
        <UsageMetricCard icon={<ArrowUpFromLine />} label="输出 Token" value={formatUsageCount(summary.outputTokens)} tone="output" />
        <UsageMetricCard icon={<Sigma />} label="总 Token" value={formatUsageCount(summary.totalTokens)} note={`完整 Token 覆盖率 ${formatUsagePercent(usageNumber(summary.tokenReportedCallCount), callCount)}`} tone="total" />
      </section>

      <Panel
        title="用量趋势"
        subtitle={`${formatRange(filters)} · ${trendBucketLabel(filters)}汇总`}
        actions={<div className="traffic-chart-tabs"><button type="button" className={trendMetric === 'tokens' ? 'is-active' : ''} onClick={() => setTrendMetric('tokens')}>Token 用量</button><button type="button" className={trendMetric === 'calls' ? 'is-active' : ''} onClick={() => setTrendMetric('calls')}>调用次数</button></div>}
      >
        <AIUsageTrendChart points={analysis.data.trend} metric={trendMetric} startTime={filters.startTime} endTime={filters.endTime} />
      </Panel>

      <Panel
        title="用量排名"
        subtitle={`${orderLabel(filters.breakdownOrder)}从高到低排列`}
        actions={<div className="ai-usage-ranking-controls">
          <label><span>维度</span><select value={filters.breakdownDimension} onChange={(event) => selectDimension(event.target.value as AIUsageBreakdownDimension)}>{dimensions.map((dimension) => <option key={dimension.value} value={dimension.value}>{dimension.label}</option>)}</select></label>
          <label><span>排序</span><select value={filters.breakdownOrder} onChange={(event) => selectOrder(event.target.value as AIUsageBreakdownOrder)}>{breakdownOrders.map((order) => <option key={order.value} value={order.value}>{order.label}</option>)}</select></label>
        </div>}
      >
        <UsageRanking analysis={analysis.data} names={names} namesReady={namesReady} />
      </Panel>
      <Toast message={notice} tone="error" onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function UsageMetricCard({ icon, label, value, note, tone = 'calls' }: { icon: ReactNode; label: string; value: string; note?: string; tone?: 'calls' | 'input' | 'output' | 'total' }) {
  return <article className={`traffic-metric-card ai-usage-metric-card is-${tone}`}><span>{icon}</span><div><small>{label}</small><strong>{value}</strong>{note ? <p>{note}</p> : null}</div></article>;
}

function UsageRanking({ analysis, names, namesReady }: { analysis: AIUsageAnalysis; names: ResourceNames; namesReady: boolean }) {
  if (analysis.breakdown.length === 0) {
    return <EmptyState title="当前范围没有模型调用" message="调整时间或筛选条件后重新查询" />;
  }
  return (
    <div className="table-scroll">
      <table className="table ai-usage-ranking-table">
        <thead><tr><th>{dimensionLabel(analysis.breakdownDimension)}</th><th>模型调用</th><th>正常响应率</th><th>输入 Token</th><th>输出 Token</th><th>总 Token</th><th>Token 覆盖率</th></tr></thead>
        <tbody>{analysis.breakdown.map((item) => {
          const calls = usageNumber(item.metrics.callCount);
          return (
            <tr key={`${analysis.breakdownDimension}:${item.dimensionValue}`}>
              <td><strong>{breakdownName(names, analysis.breakdownDimension, item, namesReady)}</strong></td>
              <td>{formatUsageCount(item.metrics.callCount)}</td>
              <td>{formatUsagePercent(usageNumber(item.metrics.normalResponseCount), calls)}</td>
              <td>{formatUsageCount(item.metrics.inputTokens)}</td>
              <td>{formatUsageCount(item.metrics.outputTokens)}</td>
              <td><strong>{formatUsageCount(item.metrics.totalTokens)}</strong></td>
              <td>{formatUsagePercent(usageNumber(item.metrics.tokenReportedCallCount), calls)}</td>
            </tr>
          );
        })}</tbody>
      </table>
    </div>
  );
}

function ResourceSelect({ value, placeholder, options, loading, onChange }: { value?: string; placeholder: string; options?: AIUsageResourceOption[]; loading: boolean; onChange: (value?: string) => void }) {
  return <select className="select" value={value ?? ''} disabled={loading} onChange={(event) => onChange(event.target.value || undefined)}><option value="">{loading ? '正在加载' : placeholder}</option>{options?.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}</select>;
}

function StringSelect({ value, placeholder, options, loading, onChange }: { value?: string; placeholder: string; options?: string[]; loading: boolean; onChange: (value?: string) => void }) {
  return <select className="select" value={value ?? ''} disabled={loading} onChange={(event) => onChange(event.target.value || undefined)}><option value="">{loading ? '正在加载' : placeholder}</option>{options?.map((option) => <option key={option} value={option}>{option}</option>)}</select>;
}

function defaultFilters(): AIUsageFilters {
  return {
    ...presetTimeRange('today'),
    breakdownDimension: 'AI_USAGE_BREAKDOWN_DIMENSION_CALLER',
    breakdownOrder: 'AI_USAGE_BREAKDOWN_ORDER_CALL_COUNT',
  };
}

function presetTimeRange(preset: TimePreset) {
  const end = roundUpToMinute(new Date());
  const start = new Date(end);
  if (preset === 'today') {
    start.setHours(0, 0, 0, 0);
  } else {
    const days = preset === '7d' ? 7 : preset === '15d' ? 15 : 30;
    start.setDate(start.getDate() - days);
  }
  return { startTime: localDateTime(start), endTime: localDateTime(end) };
}

function resourceNames(workspace: AIUsageWorkspace | null): ResourceNames {
  return {
    callers: new Map(workspace?.callers.map(({ id, name }) => [id, name])),
    routes: new Map(workspace?.routes.map(({ id, name }) => [id, name])),
    services: new Map(workspace?.services.map(({ id, name }) => [id, name])),
  };
}

function breakdownName(names: ResourceNames, dimension: AIUsageBreakdownDimension, item: AIUsageBreakdownItem, namesReady: boolean): string {
  if (dimension === 'AI_USAGE_BREAKDOWN_DIMENSION_CLIENT_MODEL') return item.dimensionValue || '未记录对外模型';
  if (dimension === 'AI_USAGE_BREAKDOWN_DIMENSION_ACTUAL_MODEL') return item.dimensionValue || '未记录实际模型';
  if (dimension === 'AI_USAGE_BREAKDOWN_DIMENSION_CALLER' && !item.dimensionValue) return '公开调用';
  if (!namesReady) return '名称加载中';
  const values = dimension === 'AI_USAGE_BREAKDOWN_DIMENSION_CALLER'
    ? names.callers
    : dimension === 'AI_USAGE_BREAKDOWN_DIMENSION_ROUTE'
      ? names.routes
      : names.services;
  return values.get(item.dimensionValue) || '已删除资源';
}

function dimensionLabel(value: AIUsageBreakdownDimension): string {
  return dimensions.find((dimension) => dimension.value === value)?.label ?? '资源';
}

function orderLabel(value: AIUsageBreakdownOrder): string {
  return breakdownOrders.find((order) => order.value === value)?.label ?? '模型调用';
}

function formatRange(filters: Pick<AIUsageFilters, 'startTime' | 'endTime'>): string {
  const formatter = new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false });
  return `${formatter.format(new Date(filters.startTime))} 至 ${formatter.format(new Date(filters.endTime))}`;
}

function trendBucketLabel(filters: Pick<AIUsageFilters, 'startTime' | 'endTime'>): string {
  const range = new Date(filters.endTime).getTime() - new Date(filters.startTime).getTime();
  if (range <= 2 * 60 * 60 * 1000) return '每分钟';
  if (range <= 24 * 60 * 60 * 1000) return '每 5 分钟';
  if (range <= 7 * 24 * 60 * 60 * 1000) return '每小时';
  return '每天';
}
