import { useMemo, useState } from 'react';
import { Check, Copy, KeyRound, Plus, ShieldCheck, UserRound } from 'lucide-react';
import { Link } from 'react-router-dom';
import {
  createCaller,
  deleteCaller,
  disableAccessKey,
  getCallerWorkspace,
  issueAccessKey,
  updateCaller,
} from '@/api/callers';
import { getPolicyWorkspace } from '@/api/policies';
import { useResource } from '@/api/useResource';
import {
  Badge,
  Button,
  Drawer,
  EmptyState,
  Modal,
  PageFrame,
  Panel,
  ResourceFilterField,
  ResourceListFilters,
  ResourcePagination,
  ResourceStatePanel,
  RowActions,
  SearchField,
  SelectPopover,
  Toast,
} from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Caller, CallerRouteOption, IssuedAccessKey } from '@/domain/caller';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';

interface CallerDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  routeIDs: string[];
  accessKeyName: string;
  expiration: '90d' | 'none';
}

type CallerStateFilter = 'all' | 'enabled' | 'disabled';

interface CallerFilters {
  query: string;
  state: CallerStateFilter;
}

const emptyDraft = (): CallerDraft => ({
  name: '',
  enabled: true,
  routeIDs: [],
  accessKeyName: '',
  expiration: '90d',
});
const emptyCallerFilters = (): CallerFilters => ({ query: '', state: 'all' });

export function CallerPage() {
  const workspace = useResource(getCallerWorkspace);
  const policies = useResource(getPolicyWorkspace);
  const [filterDraft, setFilterDraft] = useState<CallerFilters>(emptyCallerFilters);
  const [filters, setFilters] = useState<CallerFilters>(emptyCallerFilters);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [detailID, setDetailID] = useState<string | null>(null);
  const [draft, setDraft] = useState<CallerDraft | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Caller | null>(null);
  const [issueKeyFor, setIssueKeyFor] = useState<Caller | null>(null);
  const [issuedKey, setIssuedKey] = useState<IssuedAccessKey | null>(null);
  const [keyName, setKeyName] = useState('');
  const [keyExpiration, setKeyExpiration] = useState<'90d' | 'none'>('90d');
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  const detail = workspace.data?.callers.find((caller) => caller.id === detailID) ?? null;
  const visibleCallers = useMemo(() => {
    const normalized = filters.query.trim().toLowerCase();
    if (!workspace.data) return [];
    return workspace.data.callers.filter((caller) => {
      const routeNames = caller.routeIDs.map((routeID) => workspace.data?.routes.find((route) => route.id === routeID)?.name ?? '').join(' ');
      const matchesState = filters.state === 'all'
        || (filters.state === 'enabled' && caller.enabled)
        || (filters.state === 'disabled' && !caller.enabled);
      return matchesState && `${caller.name} ${routeNames}`.toLowerCase().includes(normalized);
    });
  }, [filters, workspace.data]);

  if (workspace.loading && !workspace.data) {
    return <PageFrame title="调用方"><ResourceStatePanel title="正在加载调用方..." message="从管理 API 获取授权与密钥信息" /></PageFrame>;
  }
  if (workspace.error || !workspace.data) {
    return <PageFrame title="调用方"><ResourceStatePanel title="调用方加载失败" message={workspace.error?.message ?? '请稍后重试。'} /></PageFrame>;
  }

  const data = workspace.data;
  const pageCount = Math.max(1, Math.ceil(visibleCallers.length / pageSize));
  const currentPage = Math.min(page, pageCount);
  const pagedCallers = visibleCallers.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  const save = async () => {
    if (!draft || !draft.name.trim() || submitting) return;
    if (!draft.id && !draft.accessKeyName.trim()) return;
    setSubmitting(true);
    try {
      if (draft.id) {
        await updateCaller({
          id: draft.id,
          version: draft.version,
          name: draft.name.trim(),
          enabled: draft.enabled,
          routeIDs: draft.routeIDs,
        });
        setNotice({ message: `调用方已更新：${draft.name.trim()}`, tone: 'success' });
      } else {
        const result = await createCaller({
          name: draft.name.trim(),
          enabled: draft.enabled,
          routeIDs: draft.routeIDs,
          accessKeyName: draft.accessKeyName.trim(),
          accessKeyExpiresAt: expirationTime(draft.expiration),
        });
        setIssuedKey(result.issuedAccessKey);
        setNotice({ message: `调用方已创建：${draft.name.trim()}`, tone: 'success' });
      }
      await workspace.reload();
      setDraft(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存调用方失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteCandidate || submitting) return;
    setSubmitting(true);
    try {
      await deleteCaller(deleteCandidate.id, deleteCandidate.version);
      await workspace.reload();
      setDeleteCandidate(null);
      setNotice({ message: `调用方已删除：${deleteCandidate.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除调用方失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const toggleCaller = async (caller: Caller) => {
    if (submitting) return;
    setSubmitting(true);
    try {
      await updateCaller({
        id: caller.id,
        version: caller.version,
        name: caller.name,
        enabled: !caller.enabled,
        routeIDs: caller.routeIDs,
      });
      await workspace.reload();
      setNotice({ message: `调用方已${caller.enabled ? '停用' : '启用'}：${caller.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新调用方启用状态失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const issueKey = async () => {
    if (!issueKeyFor || !keyName.trim() || submitting) return;
    setSubmitting(true);
    try {
      const result = await issueAccessKey(issueKeyFor.id, issueKeyFor.version, keyName.trim(), expirationTime(keyExpiration));
      setIssuedKey(result);
      setIssueKeyFor(null);
      setKeyName('');
      await workspace.reload();
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '签发访问密钥失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const disableKey = async (caller: Caller, keyID: string) => {
    try {
      await disableAccessKey(caller.id, keyID, caller.version);
      await workspace.reload();
      setNotice({ message: '访问密钥已停用', tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '停用访问密钥失败', tone: 'error' });
    }
  };

  return (
    <PageFrame
      title="调用方"
      actions={<Button onClick={() => setDraft(emptyDraft())}><Plus className="h-4 w-4" />创建调用方</Button>}
    >
      <div className="space-y-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
        <Panel>
          <ResourceListFilters
            summary={callerFilterSummary(filters)}
            resultLabel={`${visibleCallers.length} 个调用方`}
            onSearch={() => { setPage(1); setFilters({ ...filterDraft }); }}
            onReset={() => {
              const next = emptyCallerFilters();
              setFilterDraft(next);
              setFilters(next);
              setPage(1);
            }}
          >
            <ResourceFilterField label="关键词">
              <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索调用方或已授权路由" />
            </ResourceFilterField>
            <ResourceFilterField label="启用状态">
              <select className="select" value={filterDraft.state} onChange={(event) => setFilterDraft((current) => ({ ...current, state: event.target.value as CallerStateFilter }))}>
                <option value="all">全部启用状态</option>
                <option value="enabled">已启用</option>
                <option value="disabled">已停用</option>
              </select>
            </ResourceFilterField>
          </ResourceListFilters>
          {visibleCallers.length === 0 ? (
            <div className="p-5"><EmptyState title={data.callers.length === 0 ? '暂无调用方' : '没有匹配的调用方'} message={data.callers.length === 0 ? '创建调用方后即可为受保护路由签发访问密钥' : '请调整搜索条件'} /></div>
          ) : (
            <div className="table-scroll resource-table-scroll">
              <table className="table resource-table resource-table-has-toggle resource-caller-table">
                <thead><tr>
                  <th>调用方</th><th>授权路由</th><th>有效密钥</th><th>状态</th><th>更新时间</th><th>操作</th>
                </tr></thead>
                <tbody>
                  {pagedCallers.map((caller) => (
                    <tr key={caller.id}>
                      <td><div className="resource-table-name"><UserRound className="text-blue-600" /><strong>{caller.name}</strong></div></td>
                      <td>{routeSummary(caller, data.routes)}</td>
                      <td>{activeKeyCount(caller)} 个</td>
                      <td><Badge tone={caller.enabled ? 'success' : 'neutral'}>{caller.enabled ? '已启用' : '已停用'}</Badge></td>
                      <td className="resource-table-time">{formatDateTime(caller.updatedAt || caller.createdAt)}</td>
                      <td>
                        <RowActions
                          onDetail={() => setDetailID(caller.id)}
                          onEdit={() => setDraft(editDraft(caller))}
                          onToggle={() => void toggleCaller(caller)}
                          toggleLabel={caller.enabled ? '停用' : '启用'}
                          toggleDisabled={submitting}
                          onDelete={() => setDeleteCandidate(caller)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {visibleCallers.length > 0 ? <ResourcePagination page={currentPage} pageSize={pageSize} total={visibleCallers.length} onPageChange={setPage} onPageSizeChange={(size) => { setPage(1); setPageSize(size); }} /> : null}
        </Panel>
      </div>

      <Drawer title="调用方详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetailID(null)}>
        {detail ? (
          <div className="space-y-6">
            <section className="resource-detail-hero">
              <div><h3>{detail.name}</h3><p>{routeSummary(detail, data.routes)}</p></div>
              <Badge tone={detail.enabled ? 'success' : 'neutral'}>{detail.enabled ? '已启用' : '已停用'}</Badge>
            </section>
            <section className="resource-detail-section">
              <h3>基本信息</h3>
              <div className="resource-detail-grid">
                <div><span>启用状态</span><strong>{detail.enabled ? '已启用' : '已停用'}</strong></div>
                <div><span>授权路由</span><strong>{detail.routeIDs.length} 条</strong></div>
                <div><span>更新时间</span><strong>{formatDateTime(detail.updatedAt || detail.createdAt)}</strong></div>
              </div>
            </section>
            <section className="resource-detail-section">
              <div className="flex items-center justify-between gap-3 mb-3"><h3>访问密钥</h3><Button size="sm" variant="outline" onClick={() => setIssueKeyFor(detail)}><KeyRound className="h-3.5 w-3.5" />签发密钥</Button></div>
              <div className="divide-y divide-slate-200 rounded-lg border border-slate-200">
                {detail.accessKeys.map((key) => (
                  <div key={key.id} className="flex items-center justify-between gap-4 px-4 py-3">
                    <div><div className="font-medium text-slate-900">{key.name}</div><div className="mt-1 text-xs text-slate-500">创建于 {formatDateTime(key.createdAt)}{key.expiresAt ? ` · 到期于 ${formatDateTime(key.expiresAt)}` : ' · 长期有效'}</div></div>
                    <div className="flex items-center gap-3"><Badge tone={keyStatus(key).tone}>{keyStatus(key).label}</Badge>{key.enabled ? <button type="button" className="link-button danger" onClick={() => void disableKey(detail, key.id)}>停用</button> : null}</div>
                  </div>
                ))}
              </div>
            </section>
            <section className="resource-detail-section">
              <h3 className="mb-3">Token 额度</h3>
              {policies.data ? (
                <GovernancePolicyPanel
                  targetKind="Caller"
                  targetID={detail.id}
                  targetName={detail.name}
                  workspace={policies.data}
                  onChanged={policies.reload}
                />
              ) : <span className="text-xs text-slate-500">额度策略加载中...</span>}
            </section>
            <section className="resource-detail-section">
              <h3 className="mb-3">排查请求</h3>
              <p className="mb-3 text-xs text-slate-500">查看归属于该调用方的请求状态、路由匹配和最终转发服务。</p>
              <Link className="inline-flex rounded-lg bg-blue-50 px-3 py-2 text-xs font-medium text-blue-700 hover:bg-blue-100" to={`/requests?callerID=${encodeURIComponent(detail.id)}`}>查看请求记录</Link>
            </section>
          </div>
        ) : null}
      </Drawer>

      <Drawer title={draft?.id ? '编辑调用方' : '创建调用方'} subtitle="为应用或服务授权受保护路由" isOpen={Boolean(draft)} onClose={() => setDraft(null)}>
        {draft ? <CallerEditor draft={draft} routes={data.routes} onChange={setDraft} onCancel={() => setDraft(null)} onSave={() => void save()} submitting={submitting} /> : null}
      </Drawer>

      <Modal title="签发访问密钥" isOpen={Boolean(issueKeyFor)} onClose={() => setIssueKeyFor(null)}>
        <div className="space-y-4">
          <LabeledInput label="密钥名称" value={keyName} onChange={setKeyName} placeholder="例如：生产服务" />
          <ExpirationSelect value={keyExpiration} onChange={setKeyExpiration} />
          <div className="flex justify-end gap-3 pt-2"><Button variant="ghost" onClick={() => setIssueKeyFor(null)}>取消</Button><Button disabled={!keyName.trim() || submitting} onClick={() => void issueKey()}>{submitting ? '签发中...' : '签发密钥'}</Button></div>
        </div>
      </Modal>

      <Modal title="保存访问密钥" isOpen={Boolean(issuedKey)} onClose={() => setIssuedKey(null)}>
        {issuedKey ? <IssuedKeyPanel issued={issuedKey} onClose={() => setIssuedKey(null)} /> : null}
      </Modal>

      <Modal title="确认删除调用方" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}>
        <div className="space-y-5"><p className="text-sm text-slate-600">删除后，该调用方的全部密钥会立即失效。历史请求仍保留原有归属。</p><div className="flex justify-end gap-3"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={submitting} onClick={() => void confirmDelete()}>确认删除</Button></div></div>
      </Modal>
    </PageFrame>
  );
}

function callerFilterSummary(filters: CallerFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.state !== 'all') conditions.push(`启用状态：${filters.state === 'enabled' ? '已启用' : '已停用'}`);
  return conditions.join(' · ') || '全部调用方';
}

function CallerEditor({ draft, routes, onChange, onCancel, onSave, submitting }: { draft: CallerDraft; routes: CallerRouteOption[]; onChange: (draft: CallerDraft) => void; onCancel: () => void; onSave: () => void; submitting: boolean }) {
  return (
    <div className="space-y-6">
      <section className="resource-detail-section space-y-4">
        <LabeledInput label="调用方名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} placeholder="例如：订单服务" />
        <label className="flex items-center gap-3 rounded-lg border border-slate-200 px-4 py-3 text-sm text-slate-700"><input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />启用调用方</label>
      </section>
      <section className="resource-detail-section">
        <CallerRouteSelect
          routes={routes}
          value={draft.routeIDs}
          onChange={(routeIDs) => onChange({ ...draft, routeIDs })}
        />
      </section>
      {!draft.id ? <section className="resource-detail-section space-y-4"><h3>首个访问密钥</h3><LabeledInput label="密钥名称" value={draft.accessKeyName} onChange={(accessKeyName) => onChange({ ...draft, accessKeyName })} placeholder="例如：生产服务" /><ExpirationSelect value={draft.expiration} onChange={(expiration) => onChange({ ...draft, expiration })} /></section> : null}
      <div className="flex justify-end gap-2 border-t border-slate-200 pt-3"><Button variant="ghost" onClick={onCancel}>取消</Button><Button size="lg" disabled={!draft.name.trim() || (!draft.id && !draft.accessKeyName.trim()) || submitting} onClick={onSave}>{submitting ? '保存中...' : '保存调用方'}</Button></div>
    </div>
  );
}

function CallerRouteSelect({
  routes,
  value,
  onChange,
}: {
  routes: CallerRouteOption[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const [query, setQuery] = useState('');
  const selected = new Set(value);
  const selectedRoutes = routes.filter((route) => selected.has(route.id));
  const normalizedQuery = query.trim().toLowerCase();
  const visibleRoutes = routes.filter((route) => route.name.toLowerCase().includes(normalizedQuery));
  const summary = selectedRoutes.length === 0
    ? '未授权任何路由'
    : selectedRoutes.length === 1
      ? selectedRoutes[0].name
      : `已选择 ${selectedRoutes.length} 条路由`;

  return (
    <SelectPopover
      label="可访问路由"
      summary={summary}
      emptyMessage="暂无受保护路由，请先将路由访问方式设置为调用方密钥"
      hasOptions={routes.length > 0}
    >
      <div className="resource-select-search">
        <SearchField value={query} onChange={setQuery} placeholder="搜索路由" />
      </div>
      {visibleRoutes.length > 0 ? visibleRoutes.map((route) => {
        const checked = selected.has(route.id);
        return (
          <button
            key={route.id}
            className={`resource-select-option${checked ? ' selected' : ''}`}
            type="button"
            role="option"
            aria-selected={checked}
            aria-pressed={checked}
            onClick={() => onChange(toggleID(value, route.id))}
          >
            <span className="multi-check">{checked ? '✓' : ''}</span>
            <strong>{route.name}</strong>
            <small>{checked ? '已授权' : '未授权'}</small>
          </button>
        );
      }) : <div className="resource-select-empty">没有匹配的路由</div>}
    </SelectPopover>
  );
}

function IssuedKeyPanel({ issued, onClose }: { issued: IssuedAccessKey; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => { await navigator.clipboard.writeText(issued.secret); setCopied(true); };
  return <div className="space-y-5"><div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">完整密钥只显示这一次。关闭后无法再次查看，请立即保存到密钥管理系统。</div><div><div className="mb-2 text-xs font-semibold text-slate-600">{issued.accessKey.name}</div><div className="flex items-center gap-2 rounded-lg border border-slate-300 bg-slate-950 px-4 py-3"><code className="min-w-0 flex-1 break-all text-xs text-emerald-300">{issued.secret}</code><Button size="sm" variant="outline" onClick={() => void copy()}>{copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}{copied ? '已复制' : '复制'}</Button></div></div><div className="flex justify-end"><Button onClick={onClose}>我已保存</Button></div></div>;
}

function LabeledInput({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return <label className="block"><span className="mb-1.5 block text-xs font-semibold text-slate-700">{label}</span><input className="w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500/20" value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} /></label>;
}

function ExpirationSelect({ value, onChange }: { value: '90d' | 'none'; onChange: (value: '90d' | 'none') => void }) {
  return <label className="block"><span className="mb-1.5 block text-xs font-semibold text-slate-700">有效期</span><select className="select w-full" value={value} onChange={(event) => onChange(event.target.value as '90d' | 'none')}><option value="90d">90 天</option><option value="none">长期有效</option></select></label>;
}

function editDraft(caller: Caller): CallerDraft { return { id: caller.id, version: caller.version, name: caller.name, enabled: caller.enabled, routeIDs: caller.routeIDs, accessKeyName: '', expiration: '90d' }; }
function toggleID(values: string[], id: string) { return values.includes(id) ? values.filter((value) => value !== id) : [...values, id]; }
function expirationTime(value: '90d' | 'none') { return value === 'none' ? undefined : new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString(); }
function activeKeyCount(caller: Caller) { return caller.accessKeys.filter((key) => keyStatus(key).label === '有效').length; }
function routeSummary(caller: Caller, routes: CallerRouteOption[]) { const names = caller.routeIDs.map((id) => routes.find((route) => route.id === id)?.name).filter(Boolean); return names.length === 0 ? '未授权路由' : names.length <= 2 ? names.join('、') : `${names.slice(0, 2).join('、')} 等 ${names.length} 条`; }
function keyStatus(key: Caller['accessKeys'][number]): { label: string; tone: 'success' | 'warning' | 'neutral' } { if (!key.enabled) return { label: '已停用', tone: 'neutral' }; if (key.expiresAt && new Date(key.expiresAt).getTime() <= Date.now()) return { label: '已过期', tone: 'warning' }; return { label: '有效', tone: 'success' }; }
