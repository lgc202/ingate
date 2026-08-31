import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { AlertTriangle, ArrowLeft, ArrowRight, CheckCircle2, ChevronRight } from 'lucide-react';
import { Link, useSearchParams } from 'react-router-dom';
import { getRequestRecord, getRequestRecordWorkspace, listRequestRecords } from '@/api/requestRecords';
import { useResource } from '@/api/useResource';
import {
  Badge,
  Button,
  Drawer,
  EmptyState,
  PageFrame,
  Panel,
  ResourceFilterField,
  ResourceListFilters,
  ResourceStatePanel,
  Toast,
} from '@/components/ui';
import type { RequestOutcome, RequestRecord, RequestRecordFilters, RequestRecordSummary, RequestRecordWorkspace } from '@/domain/requestRecord';
import {
  formatBytes,
  formatDuration,
  formatRequestTime,
  formatTokenCount,
} from '@/domain/requestRecord';
import { localDateTime, roundUpToMinute } from '@/domain/timeRange';
import { modelProtocolLabel } from '@/domain/service';

const methodOptions = ['', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];
const pageSizeOptions = [10, 20, 50];
const timePresets = [
  { value: 'hour', label: '近 1 小时' },
  { value: 'today', label: '今天' },
  { value: '7d', label: '近 7 天' },
  { value: '15d', label: '近 15 天' },
  { value: '30d', label: '近 30 天' },
] as const;

type TimePreset = typeof timePresets[number]['value'];

export function RequestRecordPage() {
  const [searchParams] = useSearchParams();
  const [initialFilters] = useState(() => requestFiltersFromURL(searchParams));
  const [draft, setDraft] = useState<RequestRecordFilters>(initialFilters);
  const [filters, setFilters] = useState<RequestRecordFilters>(initialFilters);
  const [timePreset, setTimePreset] = useState<TimePreset | null>(() => matchingTimePreset(initialFilters));
  const [pageTokens, setPageTokens] = useState(['']);
  const [pageIndex, setPageIndex] = useState(0);
  const [pageSize, setPageSize] = useState(10);
  const [selected, setSelected] = useState<RequestRecordSummary | null>(null);
  const [detail, setDetail] = useState<RequestRecord | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const pageToken = pageTokens[pageIndex] ?? '';
  const loadPage = useCallback(() => listRequestRecords(filters, pageToken, pageSize), [filters, pageSize, pageToken]);
  const records = useResource(loadPage);
  const workspace = useResource(getRequestRecordWorkspace, { enabled: Boolean(records.data) });
  const names = useMemo(() => resourceNames(workspace.data), [workspace.data]);
  const namesReady = Boolean(workspace.data);

  const applyFilters = () => {
    if (!draft.startTime || !draft.endTime || new Date(draft.startTime) >= new Date(draft.endTime)) {
      setNotice('查询开始时间必须早于结束时间');
      return;
    }
    setPageTokens(['']);
    setPageIndex(0);
    setFilters({ ...draft });
  };
  const applyTimePreset = (preset: TimePreset) => {
    const end = roundUpToMinute(new Date());
    const start = presetStartTime(preset, end);
    const next = { ...draft, startTime: localDateTime(start), endTime: localDateTime(end) };
    setTimePreset(preset);
    setDraft(next);
    setPageTokens(['']);
    setPageIndex(0);
    setFilters(next);
  };
  const resetFilters = () => {
    const next = requestFiltersFromURL(new URLSearchParams());
    setDraft(next);
    setFilters(next);
    setTimePreset(matchingTimePreset(next));
    setPageTokens(['']);
    setPageIndex(0);
  };
  const openDetail = async (record: RequestRecordSummary) => {
    setSelected(record);
    setDetail(null);
    setDetailError(null);
    setDetailLoading(true);
    try {
      setDetail(await getRequestRecord(record.id, record.startedAt));
    } catch (error) {
      setDetailError(error instanceof Error ? error.message : '请求详情加载失败');
    } finally {
      setDetailLoading(false);
    }
  };
  const nextPage = () => {
    const token = records.data?.nextPageToken;
    if (!token) return;
    setPageTokens((current) => [...current.slice(0, pageIndex + 1), token]);
    setPageIndex((current) => current + 1);
  };

  if (records.loading && !records.data) {
    return <PageFrame title="请求记录"><ResourceStatePanel title="正在加载请求记录" message="正在读取网关最近处理的请求元数据" /></PageFrame>;
  }
  if (records.error || !records.data) {
    return <PageFrame title="请求记录"><ResourceStatePanel title="请求记录加载失败" message={records.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  return (
    <PageFrame
      title="请求记录"
    >
      <Panel>
        <ResourceListFilters
          summary={formatFilterRange(filters)}
          resultLabel={`${records.data.records.length} 条记录`}
          onSearch={applyFilters}
          onReset={resetFilters}
        >
          <div className="resource-filter-presets" aria-label="快捷时间范围">
            <span>时间范围</span>
            {timePresets.map((preset) => <button type="button" key={preset.value} className={timePreset === preset.value ? 'is-active' : ''} onClick={() => applyTimePreset(preset.value)}>{preset.label}</button>)}
          </div>
          <ResourceFilterField label="结果"><select className="select" value={draft.outcome ?? ''} onChange={(event) => setDraft({ ...draft, outcome: (event.target.value || undefined) as RequestOutcome | undefined })}>{outcomeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></ResourceFilterField>
          <ResourceFilterField label="开始时间"><input className="input" type="datetime-local" value={draft.startTime} onChange={(event) => { setTimePreset(null); setDraft({ ...draft, startTime: event.target.value }); }} /></ResourceFilterField>
          <ResourceFilterField label="结束时间"><input className="input" type="datetime-local" value={draft.endTime} onChange={(event) => { setTimePreset(null); setDraft({ ...draft, endTime: event.target.value }); }} /></ResourceFilterField>
          <ResourceFilterField label="方法"><select className="select" value={draft.method ?? ''} onChange={(event) => setDraft({ ...draft, method: event.target.value || undefined })}>{methodOptions.map((method) => <option key={method} value={method}>{method || '全部方法'}</option>)}</select></ResourceFilterField>
          <ResourceFilterField label="网关"><ResourceSelect value={draft.gatewayID} placeholder="全部网关" options={workspace.data?.gateways} onChange={(gatewayID) => setDraft({ ...draft, gatewayID })} /></ResourceFilterField>
          <ResourceFilterField label="路由"><ResourceSelect value={draft.routeID} placeholder="全部路由" options={workspace.data?.routes} onChange={(routeID) => setDraft({ ...draft, routeID })} /></ResourceFilterField>
          <ResourceFilterField label="服务"><ResourceSelect value={draft.serviceID} placeholder="全部服务" options={workspace.data?.services} onChange={(serviceID) => setDraft({ ...draft, serviceID })} /></ResourceFilterField>
          <ResourceFilterField label="调用方"><ResourceSelect value={draft.callerID} placeholder="全部调用方" options={workspace.data?.callers} onChange={(callerID) => setDraft({ ...draft, callerID })} /></ResourceFilterField>
          <ResourceFilterField label="Host"><input className="input font-mono" placeholder="精确匹配" value={draft.host ?? ''} onChange={(event) => setDraft({ ...draft, host: event.target.value })} /></ResourceFilterField>
          <ResourceFilterField label="路径前缀"><input className="input font-mono" placeholder="例如 /api/orders" value={draft.pathPrefix ?? ''} onChange={(event) => setDraft({ ...draft, pathPrefix: event.target.value })} /></ResourceFilterField>
        </ResourceListFilters>
        {records.data.records.length === 0 ? <EmptyState title="没有匹配的请求" message="调整时间范围或筛选条件后重新查询" /> : (
          <div className="table-scroll request-record-table-scroll">
            <table className="table request-record-table">
              <thead><tr><th>时间</th><th>请求</th><th>响应</th><th>网关</th><th>路由</th><th>目标服务</th><th>调用方</th><th>总耗时</th><th /></tr></thead>
              <tbody>
                {records.data.records.map((record) => (
                  <tr key={`${record.id}:${record.startedAt}`} className="request-record-row" onClick={() => void openDetail(record)}>
                    <td><time className="request-record-time" dateTime={record.startedAt}>{formatRequestTime(record.startedAt)}</time></td>
                    <td><div className="request-record-target"><span className={`request-method request-method-${record.method.toLowerCase()}`}>{record.method || '-'}</span><code>{record.host}</code></div><strong className="request-path">{record.path || '/'}</strong></td>
                    <td><div className="request-response"><Badge tone={responseTone(record)}>{responseStatus(record)}</Badge>{rejectionLabel(record) ? <span>{rejectionLabel(record)}</span> : null}</div></td>
                    <td><strong className="request-resource-name">{resourceName(names.gateways, record.gatewayID, namesReady ? '已删除的网关' : '名称加载中')}</strong></td>
                    <td><strong className="request-resource-name">{resourceName(names.routes, record.routeID, namesReady ? '已删除的路由' : '名称加载中')}</strong></td>
                    <td><strong className="request-resource-name">{resourceName(names.services, record.serviceID, namesReady ? '已删除的服务' : '名称加载中')}</strong>{record.aiModelCall ? <span className="request-model-summary">{modelMapping(record.aiModelCall)} · {modelTokenSummary(record.aiModelCall)}</span> : null}</td>
                    <td><strong className="request-resource-name">{callerLabel(record, names, namesReady)}</strong></td>
                    <td className="whitespace-nowrap">{formatDuration(record.duration)}</td>
                    <td><ChevronRight className="h-4 w-4 text-slate-400" /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="request-record-pagination">
          <span className="request-page-status">第 {pageIndex + 1} 页 · 本页 {records.data.records.length} 条</span>
          <label className="request-page-size">
            <span>每页</span>
            <select
              value={pageSize}
              onChange={(event) => {
                setPageTokens(['']);
                setPageIndex(0);
                setPageSize(Number(event.target.value));
              }}
            >
              {pageSizeOptions.map((option) => <option key={option} value={option}>{option}</option>)}
            </select>
            <span>条</span>
          </label>
          <div className="request-page-actions">
            <Button variant="outline" size="sm" disabled={pageIndex === 0} onClick={() => setPageIndex((current) => current - 1)}><ArrowLeft className="h-3.5 w-3.5" />上一页</Button>
            <Button variant="outline" size="sm" disabled={!records.data.nextPageToken} onClick={nextPage}>下一页<ArrowRight className="h-3.5 w-3.5" /></Button>
          </div>
        </div>
      </Panel>

      <Drawer title="请求详情" subtitle={selected ? `${selected.method} ${selected.host}${selected.path}` : undefined} isOpen={Boolean(selected)} onClose={() => setSelected(null)}>
        {detailLoading ? <DetailLoading /> : null}
        {!detailLoading && detailError && selected ? <DetailError message={detailError} onRetry={() => void openDetail(selected)} /> : null}
        {!detailLoading && detail ? <RequestDetail record={detail} names={names} namesReady={namesReady} /> : null}
      </Drawer>
      <Toast message={notice} tone="error" onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function RequestDetail({ record, names, namesReady }: { record: RequestRecord; names: ResourceNames; namesReady: boolean }) {
  const failed = record.statusCode === 0 || record.outcome === 'REQUEST_OUTCOME_CLIENT_ERROR' || record.outcome === 'REQUEST_OUTCOME_SERVER_ERROR';
  return (
    <div className="space-y-5">
      <section className={`request-detail-hero ${failed ? 'is-error' : ''}`}>
        <span>{failed ? <AlertTriangle /> : <CheckCircle2 />}</span>
        <div><Badge tone={responseTone(record)}>{record.statusCode ? `${record.statusCode} · ${responseLabel(record)}` : '无响应'}</Badge>{failed ? <strong>{requestVerdict(record)}</strong> : null}{rejectionGuidance(record) ? <p>{rejectionGuidance(record)}</p> : null}<code>{record.method || '-'} {record.host}{record.path}</code></div>
      </section>
      <DetailSection title="请求与响应">
        <DetailItem label="开始时间" value={formatRequestTime(record.startedAt)} />
        <DetailItem label="总耗时" value={formatDuration(record.duration)} />
        <DetailItem label="首字节时间" value={formatDuration(record.timeToFirstByte)} />
        <DetailItem label="客户端地址" value={record.clientIP || '-'} />
        <DetailItem label="请求大小" value={formatBytes(record.requestBytes)} />
        <DetailItem label="响应大小" value={formatBytes(record.responseBytes)} />
      </DetailSection>
      {record.aiModelCall ? <DetailSection title="模型调用" layout="model">
        <DetailItem label="客户端模型" value={record.aiModelCall.clientModel || '-'} />
        <DetailItem label="实际模型" value={actualModel(record.aiModelCall)} />
        <DetailItem label="接口协议" value={modelProtocolLabel(record.aiModelCall.protocol) || '-'} />
        <DetailItem label="输入 Token" value={formatTokenCount(record.aiModelCall.inputTokens)} />
        <DetailItem label="输出 Token" value={formatTokenCount(record.aiModelCall.outputTokens)} />
        <DetailItem label="总 Token" value={formatTokenCount(record.aiModelCall.totalTokens)} />
        <DetailItem label="结束原因" value={finishReasonLabel(record.aiModelCall.finishReason)} wide />
      </DetailSection> : null}
      {record.callerID ? <DetailSection title="访问身份">
        <ResourceDetailItem label="调用方" id={record.callerID} names={names.callers} path="/callers" deletedLabel={namesReady ? '已删除的调用方' : '名称加载中'} />
        <DetailItem label="访问密钥" value={names.accessKeys.get(record.accessKeyID) || (namesReady ? '已停用或删除的密钥' : '名称加载中')} />
      </DetailSection> : callerLabel(record, names, namesReady) === '未识别调用方' ? <DetailSection title="访问身份">
        <DetailItem label="调用方" value="未识别调用方" />
      </DetailSection> : null}
      <DetailSection title="转发结果" layout="forwarding">
        <ResourceDetailItem label="网关" id={record.gatewayID} names={names.gateways} path="/gateways" deletedLabel={namesReady ? '已删除的网关' : '名称加载中'} />
        <ResourceDetailItem label="路由" id={record.routeID} names={names.routes} path="/routes" deletedLabel={namesReady ? '已删除的路由' : '名称加载中'} />
        {record.serviceID ? <ResourceDetailItem label="服务" id={record.serviceID} names={names.services} path="/services" deletedLabel={namesReady ? '已删除的服务' : '名称加载中'} /> : <DetailItem label="服务" value="未转发" />}
        <DetailItem label="最终服务地址" value={record.serviceAddress || '-'} />
        <DetailItem label="转发尝试" value={record.serviceAttempts ? `${record.serviceAttempts} 次` : '-'} />
      </DetailSection>
    </div>
  );
}

function DetailLoading() {
  return (
    <div className="grid gap-3" role="status">
      <div className="h-24 animate-pulse rounded-xl bg-slate-100" />
      <div className="grid grid-cols-3 gap-2">
        <div className="h-20 animate-pulse rounded-lg bg-slate-100" />
        <div className="h-20 animate-pulse rounded-lg bg-slate-100" />
        <div className="h-20 animate-pulse rounded-lg bg-slate-100" />
      </div>
      <p className="text-xs text-slate-500">正在读取完整请求记录</p>
    </div>
  );
}

function DetailError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="rounded-xl border border-rose-200 bg-rose-50 p-5">
      <div className="flex items-start gap-3">
        <AlertTriangle className="h-5 w-5 shrink-0 text-rose-600" />
        <div className="grid gap-3">
          <div><strong className="text-sm text-rose-900">请求详情加载失败</strong><p className="mt-1 text-xs text-rose-700">{message}</p></div>
          <Button variant="outline" size="sm" onClick={onRetry}>重新加载</Button>
        </div>
      </div>
    </div>
  );
}

function DetailSection({ title, layout, children }: { title: string; layout?: 'forwarding' | 'model'; children: ReactNode }) {
  const className = layout ? `request-detail-grid request-${layout}-grid` : 'request-detail-grid';
  return <section><h3 className="mb-2 text-sm font-semibold text-slate-900">{title}</h3><div className={className}>{children}</div></section>;
}

function DetailItem({ label, value, wide = false }: { label: string; value: string; wide?: boolean }) {
  return <div className={wide ? 'is-wide' : undefined}><span>{label}</span><strong>{value}</strong></div>;
}

function ResourceDetailItem({ label, id, names, path, deletedLabel }: { label: string; id: string; names: Map<string, string>; path: string; deletedLabel: string }) {
  const name = names.get(id);
  return <div><span>{label}</span>{name ? <Link className="request-resource-link" to={`${path}?detail=${encodeURIComponent(id)}`}>{name}<ArrowRight /></Link> : <strong>{deletedLabel}</strong>}</div>;
}

function ResourceSelect({ value, placeholder, options, onChange }: { value?: string; placeholder: string; options?: Array<{ id: string; name: string }>; onChange: (value?: string) => void }) {
  return <select className="select" value={value ?? ''} onChange={(event) => onChange(event.target.value || undefined)}><option value="">{placeholder}</option>{options?.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}</select>;
}

interface ResourceNames {
  gateways: Map<string, string>;
  routes: Map<string, string>;
  routeAccessModes: Map<string, RequestRecordWorkspace['routes'][number]['accessMode']>;
  services: Map<string, string>;
  callers: Map<string, string>;
  accessKeys: Map<string, string>;
}

function resourceNames(workspace: Awaited<ReturnType<typeof getRequestRecordWorkspace>> | null): ResourceNames {
  return {
    gateways: new Map(workspace?.gateways.map(({ id, name }) => [id, name])),
    routes: new Map(workspace?.routes.map(({ id, name }) => [id, name])),
    routeAccessModes: new Map(workspace?.routes.map(({ id, accessMode }) => [id, accessMode])),
    services: new Map(workspace?.services.map(({ id, name }) => [id, name])),
    callers: new Map(workspace?.callers.map(({ id, name }) => [id, name])),
    accessKeys: new Map(workspace?.callers.flatMap((caller) => caller.accessKeys.map(({ id, name }) => [id, name] as const))),
  };
}

function callerLabel(record: RequestRecord | RequestRecordSummary, names: ResourceNames, namesReady: boolean): string {
  if (record.callerID) return resourceName(names.callers, record.callerID, namesReady ? '已删除的调用方' : '名称加载中');
  if (!namesReady) return '名称加载中';
  return names.routeAccessModes.get(record.routeID) === 'ROUTE_ACCESS_MODE_PUBLIC' ? '公开访问' : '未识别调用方';
}

function resourceName(names: Map<string, string>, id: string, deletedLabel: string): string {
  if (!id) return '—';
  return names.get(id) || deletedLabel;
}

function requestFiltersFromURL(params: URLSearchParams): RequestRecordFilters {
  const end = parseFilterTime(params.get('endTime')) ?? roundUpToMinute(new Date());
  const start = parseFilterTime(params.get('startTime')) ?? new Date(end.getTime() - 60 * 60 * 1000);
  return {
    startTime: localDateTime(start),
    endTime: localDateTime(end),
    gatewayID: params.get('gatewayID') || undefined,
    routeID: params.get('routeID') || undefined,
    serviceID: params.get('serviceID') || undefined,
    callerID: params.get('callerID') || undefined,
    outcome: requestOutcome(params.get('outcome')),
  };
}

function parseFilterTime(value: string | null): Date | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function presetStartTime(preset: TimePreset, end: Date): Date {
  if (preset === 'today') return new Date(end.getFullYear(), end.getMonth(), end.getDate());
  const days = preset === '7d' ? 7 : preset === '15d' ? 15 : preset === '30d' ? 30 : 0;
  return new Date(end.getTime() - (days > 0 ? days * 24 * 60 * 60 * 1000 : 60 * 60 * 1000));
}

function matchingTimePreset(filters: RequestRecordFilters): TimePreset | null {
  const start = new Date(filters.startTime);
  const end = new Date(filters.endTime);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return null;

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  if (start.getTime() === today.getTime()) return 'today';

  const range = end.getTime() - start.getTime();
  if (range === 60 * 60 * 1000) return 'hour';
  if (range === 7 * 24 * 60 * 60 * 1000) return '7d';
  if (range === 15 * 24 * 60 * 60 * 1000) return '15d';
  if (range === 30 * 24 * 60 * 60 * 1000) return '30d';
  return null;
}

const outcomeOptions: Array<{ value: RequestOutcome | ''; label: string }> = [
  { value: '', label: '全部结果' },
  { value: 'REQUEST_OUTCOME_SUCCESS', label: '正常响应（2xx/3xx）' },
  { value: 'REQUEST_OUTCOME_CLIENT_ERROR', label: '客户端错误（4xx）' },
  { value: 'REQUEST_OUTCOME_SERVER_ERROR', label: '服务端错误（5xx）' },
  { value: 'REQUEST_OUTCOME_NO_RESPONSE', label: '无响应' },
];

function requestOutcome(value: string | null): RequestOutcome | undefined {
  if (
    value === 'REQUEST_OUTCOME_SUCCESS' ||
    value === 'REQUEST_OUTCOME_CLIENT_ERROR' ||
    value === 'REQUEST_OUTCOME_SERVER_ERROR' ||
    value === 'REQUEST_OUTCOME_NO_RESPONSE'
  ) return value;
  return undefined;
}

function responseStatus(record: RequestRecord | RequestRecordSummary): string {
  return record.statusCode ? String(record.statusCode) : '无响应';
}

function responseLabel(record: RequestRecord | RequestRecordSummary): string {
  if (record.statusCode >= 200 && record.statusCode < 300) return '成功';
  if (record.statusCode >= 300 && record.statusCode < 400) return '重定向';
  if (record.statusCode >= 400 && record.statusCode < 500) return '客户端错误';
  if (record.statusCode >= 500) return '服务端错误';
  return '无响应';
}

function responseTone(record: RequestRecord | RequestRecordSummary): 'success' | 'warning' | 'error' | 'neutral' {
  if (record.statusCode >= 200 && record.statusCode < 300) return 'success';
  if (record.statusCode >= 300 && record.statusCode < 400) return 'warning';
  if (record.statusCode >= 400) return 'error';
  return 'warning';
}

function requestVerdict(record: RequestRecord): string {
  const rejection = rejectionLabel(record);
  if (rejection) return rejection;
  if (record.outcome === 'REQUEST_OUTCOME_SUCCESS') return '请求已成功完成';
  if (record.outcome === 'REQUEST_OUTCOME_CLIENT_ERROR') return '请求被网关或服务拒绝';
  if (record.outcome === 'REQUEST_OUTCOME_SERVER_ERROR') return '目标服务没有成功响应';
  return '请求未获得 HTTP 响应';
}

function rejectionLabel(record: RequestRecord | RequestRecordSummary): string | null {
  if (record.rejectionReason === 'REQUEST_REJECTION_REASON_TOKEN_QUOTA_EXCEEDED') return 'Token 额度已用尽';
  return null;
}

function rejectionGuidance(record: RequestRecord): string | null {
  if (record.rejectionReason === 'REQUEST_REJECTION_REASON_TOKEN_QUOTA_EXCEEDED') {
    return '当前周期的 Token 额度已达到上限，可在调用方详情查看用量和重置时间。';
  }
  return null;
}

function formatFilterRange(filters: RequestRecordFilters): string {
  const start = filters.startTime.replace('T', ' ');
  const end = filters.endTime.replace('T', ' ');
  if (start.slice(0, 10) === end.slice(0, 10)) {
    return `${start}—${end.slice(11)}`;
  }
  return `${start}—${end}`;
}

function actualModel(call: NonNullable<RequestRecord['aiModelCall']>): string {
  if (call.responseModel && call.responseModel !== call.targetModel) {
    return `${call.responseModel}（配置 ${call.targetModel}）`;
  }
  return call.responseModel || call.targetModel || '-';
}

function modelMapping(call: NonNullable<RequestRecord['aiModelCall']>): string {
  const actual = call.responseModel || call.targetModel;
  if (!call.clientModel) return actual || '模型调用';
  if (!actual || actual === call.clientModel) return call.clientModel;
  return `${call.clientModel} → ${actual}`;
}

function modelTokenSummary(call: NonNullable<RequestRecord['aiModelCall']>): string {
  return call.totalTokens === undefined ? 'Token 未返回' : `${formatTokenCount(call.totalTokens)} Token`;
}

function finishReasonLabel(reason: string): string {
  switch (reason) {
    case 'stop':
    case 'end_turn':
      return '正常结束';
    case 'length':
    case 'max_tokens':
      return '达到最大输出长度';
    case 'tool_calls':
    case 'tool_use':
      return '请求调用工具';
    default:
      return reason || '-';
  }
}
