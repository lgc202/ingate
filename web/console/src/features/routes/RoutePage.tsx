import { useState, type ReactNode } from 'react';
import { Edit3, Plus, Route as RouteIcon, Trash2 } from 'lucide-react';
import { deleteRoute, getRouteWorkspace, saveRoute } from '@/api/routes';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Badge, Button, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import type { HeaderMatch, HttpMethod, RouteMutationPayload, RoutePathMatchType, RouteResource, WeightedUpstream } from '@/domain/route';

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
  timeoutEnabled: boolean;
  timeoutMillis: number;
  retryEnabled: boolean;
  retryAttempts: number;
  perTryTimeoutMillis: number;
  requestHeaderModifier?: RouteResource['requestHeaderModifier'];
  responseHeaderModifier?: RouteResource['responseHeaderModifier'];
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
    timeoutEnabled: Boolean(route?.timeout),
    timeoutMillis: route?.timeout?.requestMillis ?? 30000,
    retryEnabled: Boolean(route?.retry),
    retryAttempts: route?.retry?.attempts ?? 2,
    perTryTimeoutMillis: route?.retry?.perTryTimeoutMillis ?? 5000,
    requestHeaderModifier: route?.requestHeaderModifier,
    responseHeaderModifier: route?.responseHeaderModifier,
  };
}

export function RoutePage() {
  const { canWriteConfiguration } = useAuth();
  const resource = useResource(getRouteWorkspace);
  const [draft, setDraft] = useState<RouteDraft>(() => createDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<RouteResource | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  if (resource.loading && !resource.data) return <PageFrame title="路由"><ResourceStatePanel title="正在加载路由" message="正在读取当前路由配置" /></PageFrame>;
  if (resource.error || !resource.data) return <PageFrame title="路由"><ResourceStatePanel title="路由加载失败" message={resource.error?.message ?? '请稍后重试'} /></PageFrame>;

  const openEditor = (route?: RouteResource) => {
    setDraft(createDraft(route));
    setEditorOpen(true);
  };
  const save = async () => {
    const error = validateDraft(draft);
    if (error) {
      setNotice({ message: error, tone: 'error' });
      return;
    }
    setBusy(true);
    try {
      const saved = await saveRoute(toPayload(draft));
      await resource.reload();
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
      await resource.reload();
      setNotice({ message: `路由已删除：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除路由失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageFrame title="路由" subtitle="管理 Admin API 已支持的 HTTP 路由" actions={canWriteConfiguration ? <Button onClick={() => openEditor()}><Plus className="w-4 h-4" />创建路由</Button> : undefined}>
      <Panel>
        {resource.data.routes.length === 0 ? <EmptyState title="暂无路由" message="创建路由，将网关入口连接到服务" /> : <div className="overflow-x-auto"><table className="w-full text-left text-xs"><thead><tr className="border-b border-slate-200 text-slate-500"><th className="p-3">名称</th><th className="p-3">匹配</th><th className="p-3">网关</th><th className="p-3">目标服务</th><th className="p-3">状态</th><th className="p-3 text-right">操作</th></tr></thead><tbody className="divide-y divide-slate-100">{resource.data.routes.map((route) => <tr key={route.id}><td className="p-3"><div className="flex items-center gap-2"><RouteIcon className="w-4 h-4 text-blue-600" /><div><strong>{route.name}</strong><div className="font-mono text-[10px] text-slate-400">{route.id}</div></div></div></td><td className="p-3"><div className="font-mono">{route.match.path.type === 'ROUTE_PATH_MATCH_EXACT' ? '=' : '^'} {route.match.path.value}</div><div className="text-slate-400">{route.match.methods.length ? route.match.methods.join('、') : '所有方法'}</div></td><td className="p-3">{route.gatewayIDs.map((id) => resource.data!.gateways.find((gateway) => gateway.id === id)?.name ?? id).join('、')}</td><td className="p-3">{route.upstreams.map((target) => resource.data!.upstreams.find((upstream) => upstream.id === target.upstreamID)?.name ?? target.upstreamID).join('、')}</td><td className="p-3"><Badge tone={route.state === 'Ready' ? 'success' : route.state === 'Error' ? 'error' : 'neutral'}>{route.enabled ? route.state : 'Disabled'}</Badge></td><td className="p-3 text-right">{canWriteConfiguration ? <div className="inline-flex gap-1"><Button variant="ghost" size="sm" onClick={() => openEditor(route)}><Edit3 className="w-3.5 h-3.5" /></Button><Button variant="ghost" size="sm" onClick={() => setDeleteCandidate(route)}><Trash2 className="w-3.5 h-3.5 text-rose-600" /></Button></div> : '—'}</td></tr>)}</tbody></table></div>}
      </Panel>

      <Drawer title={draft.id ? `编辑路由：${draft.name}` : '创建路由'} subtitle="一条路由对应一组请求条件和转发目标" isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        <div className="space-y-5">
          <Field label="路由名称"><input className="input" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></Field>
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} />启用路由</label>
          <Field label="生效网关"><div className="grid grid-cols-2 gap-2">{resource.data.gateways.map((gateway) => <label key={gateway.id} className="flex items-center gap-2 text-xs border border-slate-200 rounded-lg p-2"><input type="checkbox" checked={draft.gatewayIDs.includes(gateway.id)} onChange={(event) => setDraft({ ...draft, gatewayIDs: event.target.checked ? [...draft.gatewayIDs, gateway.id] : draft.gatewayIDs.filter((id) => id !== gateway.id) })} />{gateway.name}</label>)}</div></Field>
          <div className="grid grid-cols-[150px_1fr] gap-3"><Field label="路径匹配"><select className="select" value={draft.pathType} onChange={(event) => setDraft({ ...draft, pathType: event.target.value as RoutePathMatchType })}><option value="ROUTE_PATH_MATCH_PREFIX">前缀</option><option value="ROUTE_PATH_MATCH_EXACT">精确</option></select></Field><Field label="请求路径"><input className="input font-mono" value={draft.path} onChange={(event) => setDraft({ ...draft, path: event.target.value })} /></Field></div>
          <Field label="请求方法（不选表示全部）"><div className="flex flex-wrap gap-2">{methods.map((method) => <label key={method} className="flex items-center gap-1.5 text-xs"><input type="checkbox" checked={draft.methods.includes(method)} onChange={(event) => setDraft({ ...draft, methods: event.target.checked ? [...draft.methods, method] : draft.methods.filter((item) => item !== method) })} />{method}</label>)}</div></Field>
          <Field label="域名（逗号分隔，留空继承网关）"><input className="input font-mono" value={draft.hostnames} onChange={(event) => setDraft({ ...draft, hostnames: event.target.value })} /></Field>
          <div className="space-y-2"><div className="flex justify-between"><strong className="text-xs">目标服务</strong><Button variant="soft" size="sm" onClick={() => setDraft({ ...draft, upstreams: [...draft.upstreams, { upstreamID: '', weight: 1 }] })}>添加目标</Button></div>{draft.upstreams.map((target, index) => <div key={index} className="grid grid-cols-[1fr_100px_36px] gap-2"><select className="select" value={target.upstreamID} onChange={(event) => setDraft({ ...draft, upstreams: draft.upstreams.map((item, current) => current === index ? { ...item, upstreamID: event.target.value } : item) })}><option value="">选择服务</option>{resource.data!.upstreams.map((upstream) => <option key={upstream.id} value={upstream.id}>{upstream.name} · {upstream.endpoint}</option>)}</select><input className="input" type="number" value={target.weight} onChange={(event) => setDraft({ ...draft, upstreams: draft.upstreams.map((item, current) => current === index ? { ...item, weight: Number(event.target.value) } : item) })} /><Button variant="ghost" size="sm" onClick={() => setDraft({ ...draft, upstreams: draft.upstreams.filter((_, current) => current !== index) })}>×</Button></div>)}</div>
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.timeoutEnabled} onChange={(event) => setDraft({ ...draft, timeoutEnabled: event.target.checked })} />配置请求超时</label>{draft.timeoutEnabled ? <Field label="请求超时（毫秒）"><input className="input" type="number" value={draft.timeoutMillis} onChange={(event) => setDraft({ ...draft, timeoutMillis: Number(event.target.value) })} /></Field> : null}
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.retryEnabled} onChange={(event) => setDraft({ ...draft, retryEnabled: event.target.checked })} />配置失败重试</label>{draft.retryEnabled ? <div className="grid grid-cols-2 gap-3"><Field label="重试次数"><input className="input" type="number" value={draft.retryAttempts} onChange={(event) => setDraft({ ...draft, retryAttempts: Number(event.target.value) })} /></Field><Field label="单次超时（毫秒）"><input className="input" type="number" value={draft.perTryTimeoutMillis} onChange={(event) => setDraft({ ...draft, perTryTimeoutMillis: Number(event.target.value) })} /></Field></div> : null}
          <div className="flex justify-end gap-2 pt-3 border-t border-slate-200"><Button variant="ghost" onClick={() => setEditorOpen(false)}>取消</Button><Button disabled={busy} onClick={save}>{busy ? '保存中...' : '保存路由'}</Button></div>
        </div>
      </Drawer>
      <Modal title="删除路由" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}><div className="p-6 space-y-5"><p className="text-sm">确定删除路由“{deleteCandidate?.name}”吗？</p><div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div></div></Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function validateDraft(draft: RouteDraft): string | undefined {
  if (!draft.name.trim()) return '请输入路由名称';
  if (draft.gatewayIDs.length === 0) return '至少选择一个网关';
  if (!draft.path.startsWith('/')) return '请求路径必须以 / 开头';
  if (draft.upstreams.length === 0 || draft.upstreams.some((item) => !item.upstreamID || item.weight < 1 || item.weight > 1000)) return '至少配置一个有效的目标服务';
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
    requestHeaderModifier: draft.requestHeaderModifier,
    responseHeaderModifier: draft.responseHeaderModifier,
    timeout: draft.timeoutEnabled ? { requestMillis: draft.timeoutMillis } : undefined,
    retry: draft.retryEnabled ? { attempts: draft.retryAttempts, perTryTimeoutMillis: draft.perTryTimeoutMillis } : undefined,
  };
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
