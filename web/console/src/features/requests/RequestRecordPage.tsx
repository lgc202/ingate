import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { AlertTriangle, ArrowLeft, ArrowRight, CheckCircle2, ChevronRight, RefreshCw, Search, ShieldCheck } from 'lucide-react';
import { getRequestRecord, getRequestRecordWorkspace, listRequestRecords } from '@/api/requestRecords';
import { useResource } from '@/api/useResource';
import { Badge, Button, Drawer, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import type { RequestOutcome, RequestRecord, RequestRecordFilters } from '@/domain/requestRecord';
import {
  formatBytes,
  formatDuration,
  formatRequestTime,
  requestOutcomeLabel,
  requestOutcomeTone,
} from '@/domain/requestRecord';

const methodOptions = ['', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

export function RequestRecordPage() {
  const [draft, setDraft] = useState<RequestRecordFilters>(defaultFilters);
  const [filters, setFilters] = useState<RequestRecordFilters>(draft);
  const [pageTokens, setPageTokens] = useState(['']);
  const [pageIndex, setPageIndex] = useState(0);
  const [selected, setSelected] = useState<RequestRecord | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const pageToken = pageTokens[pageIndex] ?? '';
  const loadPage = useCallback(() => listRequestRecords(filters, pageToken), [filters, pageToken]);
  const records = useResource(loadPage);
  const workspace = useResource(getRequestRecordWorkspace);
  const names = useMemo(() => resourceNames(workspace.data), [workspace.data]);

  const applyFilters = () => {
    if (!draft.startTime || !draft.endTime || new Date(draft.startTime) >= new Date(draft.endTime)) {
      setNotice('查询开始时间必须早于结束时间');
      return;
    }
    setPageTokens(['']);
    setPageIndex(0);
    setFilters({ ...draft });
  };
  const openDetail = async (record: RequestRecord) => {
    setSelected(record);
    setDetailLoading(true);
    try {
      setSelected(await getRequestRecord(record.id, record.startedAt));
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '请求详情加载失败');
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
      eyebrow="观测分析"
      title="请求记录"
      subtitle="按单次请求查看匹配结果、响应状态和最终转发服务"
      actions={<Button variant="outline" onClick={() => void records.reload()}><RefreshCw className="h-3.5 w-3.5" />刷新</Button>}
    >
      <p className="request-privacy-note"><ShieldCheck />不持久化请求头、查询参数和正文，仅记录排障所需的请求元数据</p>

      <Panel
        title="请求明细"
        actions={<span className="text-xs text-slate-500">第 {pageIndex + 1} 页 · {records.data.records.length} 条</span>}
      >
        <div className="request-record-toolbar">
          <label className="request-record-search"><Search /><input placeholder="按请求 ID 精确查询" value={draft.requestID ?? ''} onChange={(event) => setDraft({ ...draft, requestID: event.target.value })} /></label>
          <div className="request-outcome-tabs">
            {outcomeOptions.map((option) => <button key={option.value} type="button" className={(draft.outcome ?? '') === option.value ? 'is-active' : ''} onClick={() => setDraft({ ...draft, outcome: option.value || undefined })}>{option.label}</button>)}
          </div>
          <Button size="sm" onClick={applyFilters}>查询</Button>
        </div>
        <details className="request-advanced-filters">
          <summary>更多筛选 <span>{formatFilterRange(draft)}</span></summary>
          <div className="request-record-filters">
            <Field label="开始时间"><input className="input" type="datetime-local" value={draft.startTime} onChange={(event) => setDraft({ ...draft, startTime: event.target.value })} /></Field>
            <Field label="结束时间"><input className="input" type="datetime-local" value={draft.endTime} onChange={(event) => setDraft({ ...draft, endTime: event.target.value })} /></Field>
            <Field label="方法"><select className="select" value={draft.method ?? ''} onChange={(event) => setDraft({ ...draft, method: event.target.value || undefined })}>{methodOptions.map((method) => <option key={method} value={method}>{method || '全部方法'}</option>)}</select></Field>
            <Field label="网关"><ResourceSelect value={draft.gatewayID} placeholder="全部网关" options={workspace.data?.gateways} onChange={(gatewayID) => setDraft({ ...draft, gatewayID })} /></Field>
            <Field label="路由"><ResourceSelect value={draft.routeID} placeholder="全部路由" options={workspace.data?.routes} onChange={(routeID) => setDraft({ ...draft, routeID })} /></Field>
            <Field label="服务"><ResourceSelect value={draft.serviceID} placeholder="全部服务" options={workspace.data?.services} onChange={(serviceID) => setDraft({ ...draft, serviceID })} /></Field>
            <Field label="Host"><input className="input font-mono" placeholder="精确匹配" value={draft.host ?? ''} onChange={(event) => setDraft({ ...draft, host: event.target.value })} /></Field>
            <Field label="路径前缀"><input className="input font-mono" placeholder="例如 /api/orders" value={draft.pathPrefix ?? ''} onChange={(event) => setDraft({ ...draft, pathPrefix: event.target.value })} /></Field>
          </div>
        </details>
        {records.data.records.length === 0 ? <EmptyState title="没有匹配的请求" message="调整时间范围或筛选条件后重新查询" /> : (
          <div className="table-scroll">
            <table className="table request-record-table">
              <thead><tr><th>时间 / 请求编号</th><th>实际请求</th><th>响应</th><th>匹配与转发</th><th>总耗时</th><th /></tr></thead>
              <tbody>
                {records.data.records.map((record) => (
                  <tr key={`${record.id}:${record.startedAt}`} className="request-record-row" onClick={() => void openDetail(record)}>
                    <td><div className="table-primary whitespace-nowrap">{formatRequestTime(record.startedAt)}</div><div className="table-secondary font-mono">{record.clientIP || '-'}</div></td>
                    <td><div className="request-record-target"><span className={`request-method request-method-${record.method.toLowerCase()}`}>{record.method || '-'}</span><code>{record.host}</code></div><strong className="request-path">{record.path || '/'}</strong><div className="table-secondary font-mono">{record.requestID || record.id}</div></td>
                    <td><div className="flex items-center gap-2"><strong>{record.statusCode || '-'}</strong><Badge tone={requestOutcomeTone(record.outcome)}>{requestOutcomeLabel(record.outcome)}</Badge></div><div className="table-secondary">{record.responseCodeDetails || '正常完成'}</div></td>
                    <td><div className="request-forwarding"><strong>{nameOrID(names.routes, record.routeID)}</strong><small>{nameOrID(names.gateways, record.gatewayID)} → {nameOrID(names.services, record.serviceID)}{record.upstreamAddress ? ` · ${record.upstreamAddress}` : ''}</small></div></td>
                    <td className="whitespace-nowrap">{formatDuration(record.duration)}</td>
                    <td><ChevronRight className="h-4 w-4 text-slate-400" /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="request-record-pagination">
          <Button variant="outline" size="sm" disabled={pageIndex === 0} onClick={() => setPageIndex((current) => current - 1)}><ArrowLeft className="h-3.5 w-3.5" />上一页</Button>
          <Button variant="outline" size="sm" disabled={!records.data.nextPageToken} onClick={nextPage}>下一页<ArrowRight className="h-3.5 w-3.5" /></Button>
        </div>
      </Panel>

      <Drawer title="请求详情" subtitle={selected?.requestID || selected?.id} isOpen={Boolean(selected)} onClose={() => setSelected(null)}>
        {selected ? <RequestDetail record={selected} names={names} loading={detailLoading} /> : null}
      </Drawer>
      <Toast message={notice} tone="error" onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function RequestDetail({ record, names, loading }: { record: RequestRecord; names: ResourceNames; loading: boolean }) {
  const failed = record.outcome === 'REQUEST_OUTCOME_CLIENT_ERROR' || record.outcome === 'REQUEST_OUTCOME_SERVER_ERROR';
  return (
    <div className="space-y-5">
      {loading ? <div className="rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-700">正在读取完整记录...</div> : null}
      <section className={`request-detail-hero ${failed ? 'is-error' : ''}`}>
        <span>{failed ? <AlertTriangle /> : <CheckCircle2 />}</span>
        <div><Badge tone={requestOutcomeTone(record.outcome)}>{record.statusCode || '-'} · {requestOutcomeLabel(record.outcome)}</Badge><strong>{requestVerdict(record)}</strong><code>{record.method || '-'} {record.host}{record.path}</code></div>
      </section>
      <DetailSection title="请求与响应">
        <DetailItem label="开始时间" value={formatRequestTime(record.startedAt)} />
        <DetailItem label="总耗时" value={formatDuration(record.duration)} />
        <DetailItem label="首字节时间" value={formatDuration(record.timeToFirstByte)} />
        <DetailItem label="客户端地址" value={record.clientIP || '-'} />
        <DetailItem label="请求大小" value={formatBytes(record.requestBytes)} />
        <DetailItem label="响应大小" value={formatBytes(record.responseBytes)} />
        <DetailItem label="协议" value={record.protocol || '-'} />
        <DetailItem label="响应说明" value={record.responseCodeDetails || '正常完成'} />
      </DetailSection>
      <DetailSection title="转发结果">
        <DetailItem label="网关" value={nameOrID(names.gateways, record.gatewayID)} code={record.gatewayID} />
        <DetailItem label="路由" value={nameOrID(names.routes, record.routeID)} code={record.routeID} />
        <DetailItem label="服务" value={nameOrID(names.services, record.serviceID)} code={record.serviceID} />
        <DetailItem label="最终服务地址" value={record.upstreamAddress || '-'} />
        <DetailItem label="转发尝试" value={record.upstreamAttempts ? `${record.upstreamAttempts} 次` : '-'} />
        <DetailItem label="网关实例" value={record.proxyInstanceID || '-'} />
      </DetailSection>
      <section className="request-processing-flow">
        <h3>处理链路</h3>
        <FlowStep number={1} title="接收请求" detail={`${record.method || '-'} ${record.host}${record.path} · 来源 ${record.clientIP || '-'}`} />
        <FlowStep number={2} title="匹配网关与路由" detail={`${nameOrID(names.gateways, record.gatewayID)} → ${nameOrID(names.routes, record.routeID)}`} />
        <FlowStep number={3} title="转发到服务" detail={`${nameOrID(names.services, record.serviceID)} · ${record.upstreamAddress || '未记录服务地址'} · ${record.upstreamAttempts || 0} 次尝试`} />
        <FlowStep number={4} title="返回响应" detail={`${record.statusCode || '-'} · ${requestOutcomeLabel(record.outcome)} · 总耗时 ${formatDuration(record.duration)}`} error={failed} />
      </section>
      <p className="request-privacy-note"><ShieldCheck />未持久化请求头、查询参数和正文</p>
    </div>
  );
}

function FlowStep({ number, title, detail, error = false }: { number: number; title: string; detail: string; error?: boolean }) {
  return <div className={`request-flow-step ${error ? 'is-error' : ''}`}><span>{number}</span><div><strong>{title}</strong><p>{detail}</p></div></div>;
}

function DetailSection({ title, children }: { title: string; children: ReactNode }) {
  return <section><h3 className="mb-2 text-sm font-semibold text-slate-900">{title}</h3><div className="request-detail-grid">{children}</div></section>;
}

function DetailItem({ label, value, code }: { label: string; value: string; code?: string }) {
  return <div><span>{label}</span><strong>{value}</strong>{code && code !== value ? <code>{code}</code> : null}</div>;
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

function resourceNames(workspace: Awaited<ReturnType<typeof getRequestRecordWorkspace>> | null): ResourceNames {
  return {
    gateways: new Map(workspace?.gateways.map(({ id, name }) => [id, name])),
    routes: new Map(workspace?.routes.map(({ id, name }) => [id, name])),
    services: new Map(workspace?.services.map(({ id, name }) => [id, name])),
  };
}

function nameOrID(names: Map<string, string>, id: string): string {
  return names.get(id) || id || '-';
}

function defaultFilters(): RequestRecordFilters {
  const end = new Date();
  const start = new Date(end.getTime() - 60 * 60 * 1000);
  return { startTime: localDateTime(start), endTime: localDateTime(end) };
}

function localDateTime(value: Date): string {
  const offset = value.getTimezoneOffset() * 60_000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
}

const outcomeOptions: Array<{ value: RequestOutcome | ''; label: string }> = [
  { value: '', label: '全部' },
  { value: 'REQUEST_OUTCOME_SUCCESS', label: '成功' },
  { value: 'REQUEST_OUTCOME_CLIENT_ERROR', label: '客户端错误' },
  { value: 'REQUEST_OUTCOME_SERVER_ERROR', label: '服务端错误' },
];

function requestVerdict(record: RequestRecord): string {
  if (record.outcome === 'REQUEST_OUTCOME_SUCCESS') return '请求已成功完成';
  if (record.outcome === 'REQUEST_OUTCOME_CLIENT_ERROR') return '请求被网关或服务拒绝';
  if (record.outcome === 'REQUEST_OUTCOME_SERVER_ERROR') return '目标服务没有成功响应';
  return '请求已完成，但没有可识别的 HTTP 结果';
}

function formatFilterRange(filters: RequestRecordFilters): string {
  return `${filters.startTime.replace('T', ' ')} 至 ${filters.endTime.replace('T', ' ')}`;
}
