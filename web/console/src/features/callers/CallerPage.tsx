import { useMemo, useState } from 'react';
import { Check, Copy, KeyRound, Plus, ShieldCheck, UserRound } from 'lucide-react';
import {
  createCaller,
  deleteCaller,
  disableAccessKey,
  getCallerWorkspace,
  issueAccessKey,
  updateCaller,
} from '@/api/callers';
import { useResource } from '@/api/useResource';
import { Badge, Button, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, RowActions, SearchField, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Caller, CallerRouteOption, IssuedAccessKey } from '@/domain/caller';

interface CallerDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  routeIDs: string[];
  accessKeyName: string;
  expiration: '90d' | 'none';
}

const emptyDraft = (): CallerDraft => ({
  name: '',
  enabled: true,
  routeIDs: [],
  accessKeyName: '默认密钥',
  expiration: '90d',
});

export function CallerPage() {
  const workspace = useResource(getCallerWorkspace);
  const [query, setQuery] = useState('');
  const [detailID, setDetailID] = useState<string | null>(null);
  const [draft, setDraft] = useState<CallerDraft | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Caller | null>(null);
  const [issueKeyFor, setIssueKeyFor] = useState<Caller | null>(null);
  const [issuedKey, setIssuedKey] = useState<IssuedAccessKey | null>(null);
  const [keyName, setKeyName] = useState('轮换密钥');
  const [keyExpiration, setKeyExpiration] = useState<'90d' | 'none'>('90d');
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  const detail = workspace.data?.callers.find((caller) => caller.id === detailID) ?? null;
  const visibleCallers = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!workspace.data) return [];
    return workspace.data.callers.filter((caller) => {
      const routeNames = caller.routeIDs.map((routeID) => workspace.data?.routes.find((route) => route.id === routeID)?.name ?? '').join(' ');
      return `${caller.name} ${routeNames}`.toLowerCase().includes(normalized);
    });
  }, [query, workspace.data]);

  if (workspace.loading && !workspace.data) {
    return <PageFrame title="调用方"><ResourceStatePanel title="正在加载调用方..." message="从管理 API 获取授权与密钥信息" /></PageFrame>;
  }
  if (workspace.error || !workspace.data) {
    return <PageFrame title="调用方"><ResourceStatePanel title="调用方加载失败" message={workspace.error?.message ?? '请稍后重试。'} /></PageFrame>;
  }

  const data = workspace.data;

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

  const issueKey = async () => {
    if (!issueKeyFor || !keyName.trim() || submitting) return;
    setSubmitting(true);
    try {
      const result = await issueAccessKey(issueKeyFor.id, issueKeyFor.version, keyName.trim(), expirationTime(keyExpiration));
      setIssuedKey(result);
      setIssueKeyFor(null);
      setKeyName('轮换密钥');
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
      <div className="space-y-6 mt-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
        <Panel>
          <div className="resource-list-toolbar">
            <SearchField value={query} onChange={setQuery} placeholder="搜索调用方或已授权路由" />
            <span>{visibleCallers.length} 个调用方</span>
          </div>
          {visibleCallers.length === 0 ? (
            <div className="p-5"><EmptyState title={data.callers.length === 0 ? '暂无调用方' : '没有匹配的调用方'} message={data.callers.length === 0 ? '创建调用方后即可为受保护路由签发访问密钥' : '请调整搜索条件'} /></div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead><tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50 font-medium">
                  <th className="py-2.5 px-3">调用方</th><th className="py-2.5 px-3">授权路由</th><th className="py-2.5 px-3">有效密钥</th><th className="py-2.5 px-3">状态</th><th className="py-2.5 px-3">更新时间</th><th className="py-2.5 px-3 text-right">操作</th>
                </tr></thead>
                <tbody className="divide-y divide-slate-100">
                  {visibleCallers.map((caller) => (
                    <tr key={caller.id} className="hover:bg-slate-50/80 transition-colors">
                      <td className="py-3 px-3"><div className="flex items-center gap-2"><UserRound className="h-4 w-4 text-blue-600" /><span className="font-semibold text-slate-900">{caller.name}</span></div></td>
                      <td className="py-3 px-3 text-slate-600">{routeSummary(caller, data.routes)}</td>
                      <td className="py-3 px-3 text-slate-700">{activeKeyCount(caller)} 把</td>
                      <td className="py-3 px-3"><Badge tone={caller.enabled ? 'success' : 'neutral'}>{caller.enabled ? '已启用' : '已停用'}</Badge></td>
                      <td className="py-3 px-3 text-slate-500">{formatDateTime(caller.updatedAt)}</td>
                      <td className="py-3 px-3 text-right"><RowActions onDetail={() => setDetailID(caller.id)} onEdit={() => setDraft(editDraft(caller))} onDelete={() => setDeleteCandidate(caller)} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
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
          </div>
        ) : null}
      </Drawer>

      <Drawer title={draft?.id ? '编辑调用方' : '创建调用方'} subtitle="为应用或服务授权受保护路由" isOpen={Boolean(draft)} onClose={() => setDraft(null)}>
        {draft ? <CallerEditor draft={draft} routes={data.routes} onChange={setDraft} onSave={() => void save()} submitting={submitting} /> : null}
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

function CallerEditor({ draft, routes, onChange, onSave, submitting }: { draft: CallerDraft; routes: CallerRouteOption[]; onChange: (draft: CallerDraft) => void; onSave: () => void; submitting: boolean }) {
  return (
    <div className="space-y-6">
      <section className="resource-detail-section space-y-4">
        <LabeledInput label="调用方名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} placeholder="例如：订单服务" />
        <label className="flex items-center gap-3 rounded-lg border border-slate-200 px-4 py-3 text-sm text-slate-700"><input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />启用调用方</label>
      </section>
      <section className="resource-detail-section">
        <h3 className="mb-3">可访问路由</h3>
        {routes.length === 0 ? <EmptyState title="暂无受保护路由" message="先将路由访问方式设置为调用方密钥" /> : <div className="grid grid-cols-1 md:grid-cols-2 gap-2">{routes.map((route) => <label key={route.id} className={`flex items-center gap-3 rounded-lg border px-4 py-3 text-sm cursor-pointer ${draft.routeIDs.includes(route.id) ? 'border-blue-300 bg-blue-50/70 text-blue-900' : 'border-slate-200 text-slate-700'}`}><input type="checkbox" checked={draft.routeIDs.includes(route.id)} onChange={() => onChange({ ...draft, routeIDs: toggleID(draft.routeIDs, route.id) })} />{route.name}</label>)}</div>}
      </section>
      {!draft.id ? <section className="resource-detail-section space-y-4"><h3>首个访问密钥</h3><LabeledInput label="密钥名称" value={draft.accessKeyName} onChange={(accessKeyName) => onChange({ ...draft, accessKeyName })} placeholder="例如：生产服务" /><ExpirationSelect value={draft.expiration} onChange={(expiration) => onChange({ ...draft, expiration })} /></section> : null}
      <div className="flex justify-end pt-2"><Button size="lg" disabled={!draft.name.trim() || (!draft.id && !draft.accessKeyName.trim()) || submitting} onClick={onSave}>{submitting ? '保存中...' : '保存调用方'}</Button></div>
    </div>
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
