import { useState, type ReactNode } from 'react';
import { Plus, Route as RouteIcon } from 'lucide-react';
import { getPolicyWorkspace } from '@/api/policies';
import { deleteRoute, getRouteWorkspace, saveRoute } from '@/api/routes';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import {
  Badge,
  Button,
  Drawer,
  EmptyState,
  Modal,
  PageFrame,
  Panel,
  ResourceStatePanel,
  RowActions,
  SearchField,
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone } from '@/domain/common';
import type { PolicyWorkspace } from '@/domain/policy';
import type {
  HeaderMatch,
  HostRewriteMode,
  HttpMethod,
  RouteMutationPayload,
  RoutePathMatchType,
  RouteResource,
  RouteWorkspace,
  WeightedUpstream,
} from '@/domain/route';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';

const methods: HttpMethod[] = ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

interface RouteDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  gatewayIDs: string[];
  hostnames: string;
  pathType: RoutePathMatchType;
  path: string;
  methods: HttpMethod[];
  headers: HeaderMatch[];
  upstreams: WeightedUpstream[];
  hostRewriteMode: HostRewriteMode;
  customHostname: string;
  timeoutEnabled: boolean;
  timeoutMillis: number;
  retryEnabled: boolean;
  retryAttempts: number;
  perTryTimeoutMillis: number;
  requestHeaderModifier?: RouteResource['requestHeaderModifier'];
  responseHeaderModifier?: RouteResource['responseHeaderModifier'];
}

export function RoutePage() {
  const { canWriteConfiguration } = useAuth();
  const workspace = useResource(getRouteWorkspace);
  const policies = useResource(getPolicyWorkspace);
  const [query, setQuery] = useState('');
  const [detail, setDetail] = useState<RouteResource | null>(null);
  const [draft, setDraft] = useState<RouteDraft>(() => createDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<RouteResource | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  if (workspace.loading && !workspace.data) {
    return <PageFrame title="路由"><ResourceStatePanel title="正在加载路由" message="正在读取当前路由配置" /></PageFrame>;
  }
  if (workspace.error || !workspace.data) {
    return <PageFrame title="路由"><ResourceStatePanel title="路由加载失败" message={workspace.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const data = workspace.data;
  const visibleRoutes = filterRoutes(data, query);

  const openEditor = (route?: RouteResource) => {
    setDraft(createDraft(route));
    setEditorOpen(true);
  };

  const save = async () => {
    const validationError = validateDraft(draft);
    if (validationError) {
      setNotice({ message: validationError, tone: 'error' });
      return;
    }

    setBusy(true);
    try {
      const saved = await saveRoute(toPayload(draft));
      await workspace.reload();
      setEditorOpen(false);
      setNotice({ message: `路由已保存：${saved.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存路由失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!deleteCandidate) return;

    setBusy(true);
    try {
      await deleteRoute(deleteCandidate.id, deleteCandidate.version);
      await workspace.reload();
      setNotice({ message: `路由已删除：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除路由失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageFrame
      eyebrow="流量配置"
      title="路由"
      subtitle="定义外部请求的匹配条件和目标服务"
      actions={canWriteConfiguration ? <Button onClick={() => openEditor()}><Plus className="h-4 w-4" />创建路由</Button> : undefined}
    >
      <Panel>
        <div className="resource-list-toolbar">
          <SearchField value={query} onChange={setQuery} placeholder="搜索路由、域名、路径或服务" />
          <span>{visibleRoutes.length} 条路由</span>
        </div>
        {visibleRoutes.length === 0 ? (
          <div className="p-5">
            <EmptyState title={data.routes.length === 0 ? '暂无路由' : '没有匹配的路由'} message={data.routes.length === 0 ? '创建路由，将网关入口连接到服务' : '请调整搜索条件'} />
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead><tr className="border-b border-slate-200 text-slate-500"><th className="p-3">名称</th><th className="p-3">请求匹配</th><th className="p-3">网关</th><th className="p-3">目标服务</th><th className="p-3">状态</th><th className="p-3 text-right">操作</th></tr></thead>
              <tbody className="divide-y divide-slate-100">
                {visibleRoutes.map((route) => (
                  <tr key={route.id}>
                    <td className="p-3"><div className="flex items-center gap-2"><RouteIcon className="h-4 w-4 shrink-0 text-blue-600" /><strong>{route.name}</strong></div></td>
                    <td className="p-3"><div className="table-primary font-mono">{pathMatchLabel(route)} {route.match.path.value}</div><div className="table-secondary">{methodLabel(route)}</div></td>
                    <td className="p-3">{resourceNames(route.gatewayIDs, data.gateways)}</td>
                    <td className="p-3">{resourceNames(route.upstreams.map((target) => target.upstreamID), data.upstreams)}</td>
                    <td className="p-3"><Badge tone={resourceStateTone(route.enabled ? route.state : 'Disabled')}>{resourceStateLabel(route.enabled ? route.state : 'Disabled')}</Badge></td>
                    <td className="p-3 text-right"><RowActions onDetail={() => setDetail(route)} onEdit={canWriteConfiguration ? () => openEditor(route) : undefined} onDelete={canWriteConfiguration ? () => setDeleteCandidate(route) : undefined} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Drawer title="路由详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail ? <RouteDetail route={detail} workspace={data} policies={policies.data} onPoliciesChanged={policies.reload} /> : null}
      </Drawer>

      <Drawer title={draft.id ? `编辑路由：${draft.name}` : '创建路由'} subtitle="一条路由对应一组请求条件和转发目标" isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        <RouteEditor draft={draft} workspace={data} busy={busy} onChange={setDraft} onCancel={() => setEditorOpen(false)} onSave={save} />
      </Drawer>

      <Modal title="删除路由" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}>
        <div className="space-y-5"><p className="text-sm">确定删除路由“{deleteCandidate?.name}”吗？</p><div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div></div>
      </Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function RouteDetail({ route, workspace, policies, onPoliciesChanged }: { route: RouteResource; workspace: RouteWorkspace; policies: PolicyWorkspace | null; onPoliciesChanged: () => Promise<void> }) {
  const state = route.enabled ? route.state : 'Disabled';
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero"><div><h3>{route.name}</h3></div><Badge tone={resourceStateTone(state)}>{resourceStateLabel(state)}</Badge></section>
      <DetailSection title="请求匹配">
        <DetailItem label="域名" value={route.hostnames.length > 0 ? route.hostnames.join('、') : '继承网关域名'} />
        <DetailItem label="路径" value={`${pathMatchLabel(route)} ${route.match.path.value}`} code />
        <DetailItem label="请求方法" value={methodLabel(route)} />
        <DetailItem label="请求头条件" value={route.match.headers.length > 0 ? route.match.headers.map((header) => `${header.name}: ${header.value}`).join('、') : '无'} />
      </DetailSection>
      <DetailSection title="转发设置">
        <DetailItem label="生效网关" value={resourceNames(route.gatewayIDs, workspace.gateways)} />
        <DetailItem label="目标服务" value={route.upstreams.map((target) => `${resourceName(target.upstreamID, workspace.upstreams)} · 权重 ${target.weight}`).join('、')} />
        <DetailItem label="转发主机名" value={hostRewriteLabel(route)} code={route.hostRewrite.mode === 'HOST_REWRITE_MODE_CUSTOM'} />
        <DetailItem label="请求超时" value={route.timeout ? `${route.timeout.requestMillis} 毫秒` : '使用系统默认值'} />
        <DetailItem label="失败重试" value={route.retry ? `${route.retry.attempts} 次 · 单次 ${route.retry.perTryTimeoutMillis} 毫秒` : '未配置'} />
      </DetailSection>
      <DetailSection title="资源信息">
        <DetailItem label="配置状态" value={route.message || resourceStateLabel(state)} />
        <DetailItem label="更新时间" value={formatDateTime(route.updatedAt || route.createdAt)} />
        <DetailItem label="创建时间" value={formatDateTime(route.createdAt)} />
        <DetailItem label="配置版本" value={String(route.version)} />
      </DetailSection>
      {policies ? <section className="resource-detail-section"><h3>流量策略</h3><GovernancePolicyPanel targetKind="Route" targetID={route.id} targetName={route.name} workspace={policies} onChanged={onPoliciesChanged} /></section> : null}
    </div>
  );
}

function RouteEditor({ draft, workspace, busy, onChange, onCancel, onSave }: { draft: RouteDraft; workspace: RouteWorkspace; busy: boolean; onChange: (draft: RouteDraft) => void; onCancel: () => void; onSave: () => void }) {
  return (
    <div className="space-y-5">
      <Field label="路由名称"><input className="input" value={draft.name} onChange={(event) => onChange({ ...draft, name: event.target.value })} /></Field>
      <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />启用路由</label>
      <Field label="生效网关"><div className="grid grid-cols-2 gap-2">{workspace.gateways.map((gateway) => <label key={gateway.id} className="flex items-center gap-2 rounded-lg border border-slate-200 p-3 text-xs"><input type="checkbox" checked={draft.gatewayIDs.includes(gateway.id)} onChange={(event) => onChange({ ...draft, gatewayIDs: event.target.checked ? [...draft.gatewayIDs, gateway.id] : draft.gatewayIDs.filter((id) => id !== gateway.id) })} />{gateway.name}</label>)}</div></Field>
      <div className="grid grid-cols-[150px_1fr] gap-3"><Field label="路径匹配"><select className="select" value={draft.pathType} onChange={(event) => onChange({ ...draft, pathType: event.target.value as RoutePathMatchType })}><option value="ROUTE_PATH_MATCH_PREFIX">前缀</option><option value="ROUTE_PATH_MATCH_EXACT">精确</option></select></Field><Field label="请求路径"><input className="input font-mono" value={draft.path} onChange={(event) => onChange({ ...draft, path: event.target.value })} /></Field></div>
      <Field label="请求方法（不选表示全部）"><div className="flex flex-wrap gap-3">{methods.map((method) => <label key={method} className="flex items-center gap-1.5 text-xs"><input type="checkbox" checked={draft.methods.includes(method)} onChange={(event) => onChange({ ...draft, methods: event.target.checked ? [...draft.methods, method] : draft.methods.filter((item) => item !== method) })} />{method}</label>)}</div></Field>
      <Field label="域名（逗号分隔，留空继承网关）"><input className="input font-mono" value={draft.hostnames} onChange={(event) => onChange({ ...draft, hostnames: event.target.value })} /></Field>
      <div className="space-y-2">
        <div className="flex justify-between"><strong className="text-xs">目标服务</strong><Button variant="soft" size="sm" onClick={() => onChange({ ...draft, upstreams: [...draft.upstreams, { upstreamID: '', weight: 1 }] })}>添加目标</Button></div>
        {draft.upstreams.map((target, index) => <div key={index} className="grid grid-cols-[1fr_100px_36px] gap-2"><select className="select" value={target.upstreamID} onChange={(event) => onChange({ ...draft, upstreams: replaceAt(draft.upstreams, index, { ...target, upstreamID: event.target.value }) })}><option value="">选择服务</option>{workspace.upstreams.map((upstream) => <option key={upstream.id} value={upstream.id}>{upstream.name} · {upstream.endpoint}</option>)}</select><input className="input" type="number" min="1" max="1000" aria-label="服务权重" value={target.weight} onChange={(event) => onChange({ ...draft, upstreams: replaceAt(draft.upstreams, index, { ...target, weight: Number(event.target.value) }) })} /><Button variant="ghost" size="sm" aria-label="删除目标服务" onClick={() => onChange({ ...draft, upstreams: draft.upstreams.filter((_, current) => current !== index) })}>×</Button></div>)}
      </div>
      <details className="rounded-xl border border-slate-200 bg-slate-50/60 p-4">
        <summary className="cursor-pointer text-sm font-semibold text-slate-800">高级转发设置</summary>
        <div className="mt-4 space-y-4">
          <Field label="转发主机名">
            <select className="select" value={draft.hostRewriteMode} onChange={(event) => onChange({ ...draft, hostRewriteMode: event.target.value as HostRewriteMode })}>
              <option value="HOST_REWRITE_MODE_SERVICE_ADDRESS">使用服务地址（推荐）</option>
              <option value="HOST_REWRITE_MODE_PRESERVE">保持请求主机</option>
              <option value="HOST_REWRITE_MODE_CUSTOM">自定义主机名</option>
            </select>
          </Field>
          {draft.hostRewriteMode === 'HOST_REWRITE_MODE_CUSTOM' ? <Field label="自定义主机名"><input className="input font-mono" value={draft.customHostname} onChange={(event) => onChange({ ...draft, customHostname: event.target.value })} placeholder="例如 www.baidu.com" /></Field> : null}
          <p className="text-xs leading-5 text-slate-500">目标服务依赖固定 Host 时使用服务地址或自定义主机名；内部服务需要接收原始域名时选择保持请求主机。</p>
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.timeoutEnabled} onChange={(event) => onChange({ ...draft, timeoutEnabled: event.target.checked })} />配置请求超时</label>
          {draft.timeoutEnabled ? <Field label="请求超时（毫秒）"><input className="input" type="number" value={draft.timeoutMillis} onChange={(event) => onChange({ ...draft, timeoutMillis: Number(event.target.value) })} /></Field> : null}
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.retryEnabled} onChange={(event) => onChange({ ...draft, retryEnabled: event.target.checked })} />配置失败重试</label>
          {draft.retryEnabled ? <div className="grid grid-cols-2 gap-3"><Field label="重试次数"><input className="input" type="number" value={draft.retryAttempts} onChange={(event) => onChange({ ...draft, retryAttempts: Number(event.target.value) })} /></Field><Field label="单次超时（毫秒）"><input className="input" type="number" value={draft.perTryTimeoutMillis} onChange={(event) => onChange({ ...draft, perTryTimeoutMillis: Number(event.target.value) })} /></Field></div> : null}
        </div>
      </details>
      <div className="flex justify-end gap-2 border-t border-slate-200 pt-3"><Button variant="ghost" onClick={onCancel}>取消</Button><Button disabled={busy} onClick={onSave}>{busy ? '保存中...' : '保存路由'}</Button></div>
    </div>
  );
}

function DetailSection({ title, children }: { title: string; children: ReactNode }) {
  return <section className="resource-detail-section"><h3>{title}</h3><div className="resource-detail-grid">{children}</div></section>;
}

function DetailItem({ label, value, code = false }: { label: string; value: string; code?: boolean }) {
  return <div><span>{label}</span>{code ? <code>{value}</code> : <strong>{value}</strong>}</div>;
}

function createDraft(route?: RouteResource): RouteDraft {
  return {
    id: route?.id,
    version: route?.version,
    name: route?.name ?? '',
    enabled: route?.enabled ?? true,
    gatewayIDs: route?.gatewayIDs ?? [],
    hostnames: route?.hostnames.join(', ') ?? '',
    pathType: route?.match.path.type ?? 'ROUTE_PATH_MATCH_PREFIX',
    path: route?.match.path.value ?? '/',
    methods: route?.match.methods ?? [],
    headers: route?.match.headers.map((header) => ({ ...header })) ?? [],
    upstreams: route?.upstreams.map((upstream) => ({ ...upstream })) ?? [],
    hostRewriteMode: route?.hostRewrite.mode ?? 'HOST_REWRITE_MODE_SERVICE_ADDRESS',
    customHostname: route?.hostRewrite.hostname ?? '',
    timeoutEnabled: Boolean(route?.timeout),
    timeoutMillis: route?.timeout?.requestMillis ?? 30000,
    retryEnabled: Boolean(route?.retry),
    retryAttempts: route?.retry?.attempts ?? 2,
    perTryTimeoutMillis: route?.retry?.perTryTimeoutMillis ?? 5000,
    requestHeaderModifier: route?.requestHeaderModifier,
    responseHeaderModifier: route?.responseHeaderModifier,
  };
}

function filterRoutes(workspace: RouteWorkspace, query: string): RouteResource[] {
  const normalizedQuery = query.trim().toLowerCase();
  return workspace.routes.filter((route) => {
    const gatewayNames = resourceNames(route.gatewayIDs, workspace.gateways);
    const upstreamNames = resourceNames(route.upstreams.map((target) => target.upstreamID), workspace.upstreams);
    return `${route.name} ${route.hostnames.join(' ')} ${route.match.path.value} ${gatewayNames} ${upstreamNames}`.toLowerCase().includes(normalizedQuery);
  });
}

function resourceNames(ids: string[], options: Array<{ id: string; name: string }>): string {
  return ids.map((id) => resourceName(id, options)).join('、') || '—';
}

function resourceName(id: string, options: Array<{ id: string; name: string }>): string {
  return options.find((option) => option.id === id)?.name ?? id;
}

function methodLabel(route: RouteResource): string {
  return route.match.methods.length > 0 ? route.match.methods.join('、') : '所有方法';
}

function pathMatchLabel(route: RouteResource): string {
  return route.match.path.type === 'ROUTE_PATH_MATCH_EXACT' ? '精确' : '前缀';
}

function replaceAt<T>(items: T[], index: number, value: T): T[] {
  return items.map((item, current) => current === index ? value : item);
}

function validateDraft(draft: RouteDraft): string | undefined {
  if (!draft.name.trim()) return '请输入路由名称';
  if (draft.gatewayIDs.length === 0) return '至少选择一个网关';
  if (!draft.path.startsWith('/')) return '请求路径必须以 / 开头';
  if (draft.upstreams.length === 0 || draft.upstreams.some((item) => !item.upstreamID || item.weight < 1 || item.weight > 1000)) return '至少配置一个有效的目标服务';
  if (draft.hostRewriteMode === 'HOST_REWRITE_MODE_CUSTOM' && !validHostname(draft.customHostname)) return '请输入有效的自定义主机名';
  if (draft.timeoutEnabled && (draft.timeoutMillis < 100 || draft.timeoutMillis > 300000)) return '请求超时范围应为 100 到 300000 毫秒';
  if (draft.retryEnabled && (draft.retryAttempts < 1 || draft.retryAttempts > 5 || draft.perTryTimeoutMillis < 100 || draft.perTryTimeoutMillis > 60000)) return '重试配置不正确';
  return undefined;
}

function toPayload(draft: RouteDraft): RouteMutationPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    enabled: draft.enabled,
    gatewayIDs: draft.gatewayIDs,
    hostnames: draft.hostnames.split(/[,，\s]+/).map((value) => value.trim().toLowerCase()).filter(Boolean),
    match: { path: { type: draft.pathType, value: draft.path.trim() }, methods: draft.methods, headers: draft.headers },
    upstreams: draft.upstreams,
    hostRewrite: {
      mode: draft.hostRewriteMode,
      hostname: draft.hostRewriteMode === 'HOST_REWRITE_MODE_CUSTOM' ? draft.customHostname.trim().toLowerCase() : undefined,
    },
    requestHeaderModifier: draft.requestHeaderModifier,
    responseHeaderModifier: draft.responseHeaderModifier,
    timeout: draft.timeoutEnabled ? { requestMillis: draft.timeoutMillis } : undefined,
    retry: draft.retryEnabled ? { attempts: draft.retryAttempts, perTryTimeoutMillis: draft.perTryTimeoutMillis } : undefined,
  };
}

function hostRewriteLabel(route: RouteResource): string {
  switch (route.hostRewrite.mode) {
    case 'HOST_REWRITE_MODE_SERVICE_ADDRESS': return '使用服务地址';
    case 'HOST_REWRITE_MODE_CUSTOM': return route.hostRewrite.hostname || '未填写';
    default: return '保持请求主机';
  }
}

function validHostname(value: string): boolean {
  const hostname = value.trim();
  return hostname.length > 0 && hostname.length <= 253 && hostname.split('.').every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label));
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
