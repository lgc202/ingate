import { useCallback, useState, type ReactNode } from 'react';
import { Plus, Route as RouteIcon, Trash2 } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { listCallers } from '@/api/callers';
import { getPolicyListWorkspace } from '@/api/policies';
import { deleteRoute, getRouteOptions, listRoutePage, saveRoute } from '@/api/routes';
import { useCursorResource, useResource } from '@/api/useResource';
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
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone, type ResourceState } from '@/domain/common';
import type { Caller } from '@/domain/caller';
import type { PolicyWorkspace } from '@/domain/policy';
import { policyTargetsResource } from '@/domain/policy';
import type {
  AIModel,
  HeaderMatch,
  HostRewriteMode,
  HttpMethod,
  RouteMutationPayload,
  RoutePathMatchType,
  RouteAccessMode,
  RouteResource,
  RouteWorkspace,
  WeightedService,
} from '@/domain/route';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';
import { ResourceTrafficSignal, useResourceTrafficOverview } from '@/features/traffic/ResourceTrafficSignal';
import { ResourceTrafficSummary } from '@/features/traffic/ResourceTrafficSummary';

const methods: HttpMethod[] = ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

interface RouteDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  accessMode: RouteAccessMode;
  gatewayIDs: string[];
  hostnames: string;
  pathType: RoutePathMatchType;
  path: string;
  methods: HttpMethod[];
  headers: HeaderMatch[];
  services: WeightedService[];
  hostRewriteMode: HostRewriteMode;
  customHostname: string;
  timeoutEnabled: boolean;
  timeoutMillis: number;
  retryEnabled: boolean;
  retryAttempts: number;
  perTryTimeoutMillis: number;
  requestHeaderModifier?: RouteResource['requestHeaderModifier'];
  responseHeaderModifier?: RouteResource['responseHeaderModifier'];
  type: 'HTTP' | 'AI';
  aiModels: AIModel[];
}

type RouteTypeFilter = 'all' | RouteDraft['type'];
type RouteEnabledFilter = 'all' | 'enabled' | 'disabled';
type RouteStateFilter = 'all' | Exclude<ResourceState, 'Disabled'>;

interface RouteFilters {
  query: string;
  type: RouteTypeFilter;
  enabled: RouteEnabledFilter;
  state: RouteStateFilter;
}

const emptyRouteFilters = (): RouteFilters => ({ query: '', type: 'all', enabled: 'all', state: 'all' });

export function RoutePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterDraft, setFilterDraft] = useState<RouteFilters>(emptyRouteFilters);
  const [filters, setFilters] = useState<RouteFilters>(emptyRouteFilters);
  const [pageSize, setPageSize] = useState(10);
  const [draft, setDraft] = useState<RouteDraft>(() => createDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<RouteResource | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const loadPage = useCallback((cursor: string) => listRoutePage({
    limit: pageSize,
    cursor,
    query: filters.query.trim() || undefined,
    type: filters.type === 'all' ? undefined : `ROUTE_TYPE_${filters.type === 'HTTP' ? 'API' : 'AI'}`,
    enabled: filters.enabled === 'all' ? undefined : filters.enabled === 'enabled',
    state: filters.state === 'all' ? undefined : filters.state.toUpperCase(),
  }), [filters, pageSize]);
  const routes = useCursorResource(loadPage, {
    autoRefreshWhen: (data) => data.items.some((route) => route.enabled && route.state === 'Pending'),
  });
  const currentRoutes = routes.data?.items ?? [];
  const detail = currentRoutes.find((route) => route.id === searchParams.get('detail')) ?? null;
  const options = useResource(getRouteOptions, { enabled: Boolean(detail || editorOpen) });
  const policies = useResource(getPolicyListWorkspace, { enabled: Boolean(detail) });
  const callers = useResource(listCallers, { enabled: Boolean(detail) });
  const trafficOverview = useResourceTrafficOverview('route', currentRoutes.map((route) => route.id));

  if (routes.loading && !routes.data) {
    return <PageFrame title="路由"><ResourceStatePanel title="正在加载路由" message="正在读取当前路由配置" /></PageFrame>;
  }
  if (routes.error || !routes.data) {
    return <PageFrame title="路由"><ResourceStatePanel title="路由加载失败" message={routes.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const data: RouteWorkspace = {
    routes: currentRoutes,
    gateways: options.data?.gateways ?? [],
    services: options.data?.services ?? [],
  };
  const referencingCallers = (routeID: string) => callers.data?.filter((caller) => caller.routeIDs.includes(routeID)) ?? [];
  const referencingPolicies = (routeID: string) => policies.data?.policies.filter((policy) => policyTargetsResource(policy, 'Route', routeID)) ?? [];
  const setDetail = (route?: RouteResource) => {
    const next = new URLSearchParams(searchParams);
    if (route) next.set('detail', route.id);
    else next.delete('detail');
    setSearchParams(next);
  };

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
      await routes.reload();
      setEditorOpen(false);
      setNotice({ message: `路由已保存：${saved.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存路由失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  const toggleRoute = async (route: RouteResource) => {
    if (busy) return;
    setBusy(true);
    try {
      await saveRoute(toPayload({ ...createDraft(route), enabled: !route.enabled }));
      await routes.reload();
      setNotice({ message: `路由已${route.enabled ? '停用' : '启用'}：${route.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新路由启用状态失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!deleteCandidate) return;

    setBusy(true);
    try {
      await deleteRoute(deleteCandidate.id, deleteCandidate.version);
      await routes.reload();
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
      title="路由"
      actions={<Button onClick={() => openEditor()}><Plus className="h-4 w-4" />创建路由</Button>}
    >
      <Panel>
        <ResourceListFilters
          summary={routeFilterSummary(filters)}
          resultLabel={`本页 ${currentRoutes.length} 条路由`}
          onSearch={() => { routes.reset(); setFilters({ ...filterDraft }); }}
          onReset={() => {
            const next = emptyRouteFilters();
            setFilterDraft(next);
            setFilters(next);
            routes.reset();
          }}
        >
          <ResourceFilterField label="关键词">
            <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索路由、域名或路径" />
          </ResourceFilterField>
          <ResourceFilterField label="路由类型">
            <select className="select" value={filterDraft.type} onChange={(event) => setFilterDraft((current) => ({ ...current, type: event.target.value as RouteTypeFilter }))}>
              <option value="all">全部类型</option>
              <option value="HTTP">API 路由</option>
              <option value="AI">AI 路由</option>
            </select>
          </ResourceFilterField>
          <ResourceFilterField label="启用状态">
            <select className="select" value={filterDraft.enabled} onChange={(event) => setFilterDraft((current) => ({ ...current, enabled: event.target.value as RouteEnabledFilter }))}>
              <option value="all">全部启用状态</option>
              <option value="enabled">已启用</option>
              <option value="disabled">已停用</option>
            </select>
          </ResourceFilterField>
          <ResourceFilterField label="生效状态">
            <select className="select" value={filterDraft.state} onChange={(event) => setFilterDraft((current) => ({ ...current, state: event.target.value as RouteStateFilter }))}>
              <option value="all">全部生效状态</option>
              <option value="Ready">已生效</option>
              <option value="Pending">待生效</option>
              <option value="Error">生效失败</option>
            </select>
          </ResourceFilterField>
        </ResourceListFilters>
        {currentRoutes.length === 0 ? (
          <div className="p-5">
            <EmptyState title={filters.query || filters.type !== 'all' || filters.enabled !== 'all' || filters.state !== 'all' ? '没有匹配的路由' : '暂无路由'} message={filters.query || filters.type !== 'all' || filters.enabled !== 'all' || filters.state !== 'all' ? '请调整搜索条件' : '创建路由，将网关入口连接到服务'} />
          </div>
        ) : (
          <div className="table-scroll resource-table-scroll">
            <table className="table resource-table resource-table-has-toggle resource-route-table">
              <thead><tr><th>路由</th><th>请求匹配</th><th>转发关系</th><th>最近 1 小时</th><th>状态</th><th>操作</th></tr></thead>
              <tbody>
                {currentRoutes.map((route) => (
                  <tr key={route.id}>
                    <td><div className="resource-table-name"><RouteIcon className="text-blue-600" /><strong>{route.name}</strong></div><div className="table-secondary mt-1">{route.ai ? 'AI 路由' : 'API 路由'} · {accessModeLabel(route.accessMode)}</div></td>
                    <td><div className="table-primary font-mono">{pathMatchLabel(route)} {route.match.path.value}</div><div className="table-secondary">{route.ai ? `${route.ai.models.length} 个客户端模型` : methodLabel(route)}</div></td>
                    <td><div className="table-primary">{route.gatewayIDs.length} 个网关</div><div className="table-secondary">{new Set(routeServiceIDs(route)).size} 个目标服务</div></td>
                    <td><ResourceTrafficSignal resourceID={route.id} overview={trafficOverview} /></td>
                    <td>
                      <div className="resource-state-badges">
                        <Badge tone={route.enabled ? 'accent' : 'neutral'}>{route.enabled ? '已启用' : '已停用'}</Badge>
                        {route.enabled ? <Badge tone={resourceStateTone(route.state)}>{resourceStateLabel(route.state)}</Badge> : null}
                      </div>
                      <div className="table-secondary mt-1">{formatDateTime(route.updatedAt || route.createdAt)}</div>
                    </td>
                    <td>
                      <RowActions
                        onDetail={() => setDetail(route)}
                        onEdit={() => openEditor(route)}
                        onToggle={() => void toggleRoute(route)}
                        toggleLabel={route.enabled ? '停用' : '启用'}
                        toggleDisabled={busy}
                        onDelete={() => setDeleteCandidate(route)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {currentRoutes.length > 0 ? <ResourcePagination page={routes.page} pageSize={pageSize} itemCount={currentRoutes.length} hasNext={routes.hasNext} onPageChange={(nextPage) => nextPage > routes.page ? routes.next() : routes.previous()} onPageSizeChange={(size) => { routes.reset(); setPageSize(size); }} /> : null}
      </Panel>

      <Drawer title="路由详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail()}>
        {detail && options.loading && !options.data ? <ResourceStatePanel title="正在加载关联资源" message="正在读取网关和目标服务" /> : null}
        {detail && options.error ? <ResourceStatePanel title="关联资源加载失败" message={options.error.message} /> : null}
        {detail && options.data ? <RouteDetail route={detail} workspace={data} callers={referencingCallers(detail.id)} policies={policies.data} onPoliciesChanged={policies.reload} /> : null}
      </Drawer>

      <Drawer title={draft.id ? `编辑路由：${draft.name}` : '创建路由'} isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        {options.loading && !options.data ? <ResourceStatePanel title="正在加载关联资源" message="正在读取可选网关和目标服务" /> : null}
        {options.error ? <ResourceStatePanel title="关联资源加载失败" message={options.error.message} /> : null}
        {options.data ? <RouteEditor draft={draft} workspace={data} busy={busy} onChange={setDraft} onCancel={() => setEditorOpen(false)} onSave={save} /> : null}
      </Drawer>

      <Modal title="删除路由" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}>
        <div className="space-y-5">
          <p className="text-sm">确定删除路由“{deleteCandidate?.name}”吗？</p>
          <div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div>
        </div>
      </Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function RouteDetail({ route, workspace, callers, policies, onPoliciesChanged }: { route: RouteResource; workspace: RouteWorkspace; callers: Caller[]; policies: PolicyWorkspace | null; onPoliciesChanged: () => Promise<void> }) {
  const state = route.enabled ? route.state : 'Disabled';
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero"><div><h3>{route.name}</h3><p>{route.enabled ? '路由已启用' : '路由已停用'}</p></div><Badge tone={resourceStateTone(state)}>{resourceStateLabel(state)}</Badge></section>
      <ResourceTrafficSummary kind="route" resourceID={route.id} />
      <RouteCallExample route={route} workspace={workspace} />
      <DetailSection title="请求匹配">
        <DetailItem label="路由类型" value={route.ai ? 'AI 路由' : 'API 路由'} />
        <DetailItem label="访问方式" value={accessModeLabel(route.accessMode)} />
        <DetailItem label="域名" value={route.hostnames.length > 0 ? route.hostnames.join('、') : '继承网关域名'} />
        <DetailItem label="路径" value={`${pathMatchLabel(route)} ${route.match.path.value}`} code />
        <DetailItem label="请求方法" value={methodLabel(route)} />
        <DetailItem label="请求头条件" value={route.match.headers.length > 0 ? route.match.headers.map((header) => `${header.name}: ${header.value}`).join('、') : '无'} />
      </DetailSection>
      {route.ai ? <AIModelDetail route={route} workspace={workspace} /> : null}
      <DetailSection title="转发设置">
        <DetailItem label="生效网关" value={resourceNames(route.gatewayIDs, workspace.gateways)} />
        <DetailItem label="目标服务" value={route.ai ? resourceNames(routeServiceIDs(route), workspace.services) : route.services.map((target) => `${resourceName(target.serviceID, workspace.services)} · 权重 ${target.weight}`).join('、')} />
        <DetailItem label="转发主机名" value={hostRewriteLabel(route)} code={route.hostRewrite.mode === 'HOST_REWRITE_MODE_CUSTOM'} />
        <DetailItem label="请求超时" value={route.timeout ? `${route.timeout.requestMillis} 毫秒` : '使用系统默认值'} />
        <DetailItem label="失败重试" value={route.retry ? `${route.retry.attempts} 次 · 单次 ${route.retry.perTryTimeoutMillis} 毫秒` : '未配置'} />
      </DetailSection>
      <DetailSection title="资源信息">
        <DetailItem label="启用状态" value={route.enabled ? '已启用' : '已停用'} />
        <DetailItem label="生效状态" value={route.message || resourceStateLabel(state)} />
        <DetailItem label="更新时间" value={formatDateTime(route.updatedAt || route.createdAt)} />
        <DetailItem label="创建时间" value={formatDateTime(route.createdAt)} />
      </DetailSection>
      {route.accessMode === 'ROUTE_ACCESS_MODE_CALLER' ? (
        <section className="resource-detail-section">
          <h3>授权调用方</h3>
          {callers.length > 0 ? <div className="resource-detail-list">{callers.map((caller) => <article key={caller.id}><div><strong>{caller.name}</strong><small>{caller.enabled ? '已启用' : '已停用'}</small></div><Badge tone="neutral">调用方</Badge></article>)}</div> : <p className="text-xs text-slate-500">当前没有调用方获准访问此路由</p>}
        </section>
      ) : null}
      {policies ? <section className="resource-detail-section"><h3>应用策略</h3><GovernancePolicyPanel targetKind="Route" targetID={route.id} targetName={route.name} workspace={policies} onChanged={onPoliciesChanged} /></section> : null}
    </div>
  );
}

function RouteCallExample({ route, workspace }: { route: RouteResource; workspace: RouteWorkspace }) {
  const gateway = workspace.gateways.find((item) => route.gatewayIDs.includes(item.id));
  const routeHostname = route.hostnames[0];
  const listener = gateway?.listeners.find((item) => !routeHostname || !item.hostname || item.hostname === routeHostname)
    ?? gateway?.listeners[0];
  if (!listener) return null;
  const scheme = listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' ? 'https' : 'http';
  const hostname = routeHostname || listener.hostname;
  const address = `${scheme}://<网关地址>:${listener.port}${route.match.path.value}`;
  const command = [`curl '${address}'`];
  if (route.ai) command.push("-H 'Content-Type: application/json'");
  if (hostname) command.push(`-H 'Host: ${hostname}'`);
  if (route.accessMode === 'ROUTE_ACCESS_MODE_CALLER') command.push("-H 'Authorization: Bearer <访问密钥>'");
  if (route.ai) {
    command.push(`-d '${JSON.stringify({
      model: route.ai.models[0]?.name || '<客户端模型名>',
      messages: [{ role: 'user', content: '你好' }],
    })}'`);
  }
  return <section className="resource-detail-section"><h3>调用示例</h3><pre className="route-call-example"><code>{command.join(' \\\n  ')}</code></pre></section>;
}

function AIModelDetail({ route, workspace }: { route: RouteResource; workspace: RouteWorkspace }) {
  return (
    <section className="resource-detail-section">
      <h3>模型发布</h3>
      <div className="resource-detail-list">
        {route.ai?.models.map((model) => (
          <article key={model.name}>
            <div><strong>{model.name}</strong><small>{model.targets.map((target) => `${resourceName(target.serviceID, workspace.services)} / ${target.model} / 权重 ${target.weight}`).join('、')}</small></div>
            <Badge tone="purple">客户端模型</Badge>
          </article>
        ))}
      </div>
    </section>
  );
}

function RouteEditor({ draft, workspace, busy, onChange, onCancel, onSave }: { draft: RouteDraft; workspace: RouteWorkspace; busy: boolean; onChange: (draft: RouteDraft) => void; onCancel: () => void; onSave: () => void }) {
  const httpServices = workspace.services.filter((service) => service.type === 'HTTP');
  const modelServices = workspace.services.filter((service) => service.type === 'MODEL');

  return (
    <div className="space-y-5">
      <Field label="路由名称"><input className="input" value={draft.name} onChange={(event) => onChange({ ...draft, name: event.target.value })} /></Field>
      {!draft.id ? (
        <Field label="路由类型">
          <select className="select" value={draft.type} onChange={(event) => onChange(routeDraftWithType(draft, event.target.value as RouteDraft['type']))}>
            <option value="HTTP">API 路由</option>
            <option value="AI">AI 路由</option>
          </select>
        </Field>
      ) : <Field label="路由类型"><Badge tone={draft.type === 'AI' ? 'purple' : 'neutral'}>{draft.type === 'AI' ? 'AI 路由' : 'API 路由'}</Badge></Field>}
      <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />启用路由</label>
      <Field label="访问方式">
        <select className="select" value={draft.accessMode} onChange={(event) => onChange({ ...draft, accessMode: event.target.value as RouteAccessMode })}>
          <option value="ROUTE_ACCESS_MODE_PUBLIC">公开访问</option>
          <option value="ROUTE_ACCESS_MODE_CALLER">调用方密钥</option>
        </select>
      </Field>
      <GatewaySelectionEditor draft={draft} gateways={workspace.gateways} onChange={onChange} />
      <div className="grid grid-cols-[150px_1fr] gap-3"><Field label="路径匹配"><select className="select" disabled={draft.type === 'AI'} value={draft.pathType} onChange={(event) => onChange({ ...draft, pathType: event.target.value as RoutePathMatchType })}><option value="ROUTE_PATH_MATCH_TYPE_PREFIX">前缀</option><option value="ROUTE_PATH_MATCH_TYPE_EXACT">精确</option></select></Field><Field label="请求路径"><input className="input font-mono" value={draft.path} onChange={(event) => onChange({ ...draft, path: event.target.value })} /></Field></div>
      {draft.type === 'HTTP' ? <Field label="请求方法（不选表示全部）"><div className="flex flex-wrap gap-3">{methods.map((method) => <label key={method} className="flex items-center gap-1.5 text-xs"><input type="checkbox" checked={draft.methods.includes(method)} onChange={(event) => onChange({ ...draft, methods: event.target.checked ? [...draft.methods, method] : draft.methods.filter((item) => item !== method) })} />{method}</label>)}</div></Field> : <Field label="请求方法"><input className="input font-mono" value="POST" disabled /></Field>}
      <Field label="域名（逗号分隔，留空继承网关）"><input className="input font-mono" value={draft.hostnames} onChange={(event) => onChange({ ...draft, hostnames: event.target.value })} /></Field>
      {draft.type === 'HTTP' ? <HTTPForwardingEditor draft={draft} services={httpServices} onChange={onChange} /> : <AIForwardingEditor draft={draft} services={modelServices} onChange={onChange} />}
      <details className="rounded-xl border border-slate-200 bg-slate-50/60 p-4">
        <summary className="cursor-pointer text-sm font-semibold text-slate-800">高级转发设置</summary>
        <div className="mt-4 space-y-4">
          <Field label="转发主机名">
            <select className="select" value={draft.hostRewriteMode} onChange={(event) => onChange({ ...draft, hostRewriteMode: event.target.value as HostRewriteMode })}>
              <option value="HOST_REWRITE_MODE_SERVICE_HOST">使用服务端点主机名（推荐）</option>
              <option value="HOST_REWRITE_MODE_PRESERVE">保持请求主机</option>
              <option value="HOST_REWRITE_MODE_CUSTOM">自定义主机名</option>
            </select>
          </Field>
          {draft.hostRewriteMode === 'HOST_REWRITE_MODE_CUSTOM' ? <Field label="自定义主机名"><input className="input font-mono" value={draft.customHostname} onChange={(event) => onChange({ ...draft, customHostname: event.target.value })} placeholder="例如 www.baidu.com" /></Field> : null}
          <p className="text-xs leading-5 text-slate-500">目标服务依赖固定 Host 时使用服务端点主机名或自定义主机名；内部服务需要接收原始域名时选择保持请求主机。</p>
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

function routeDraftWithType(draft: RouteDraft, type: RouteDraft['type']): RouteDraft {
  if (type === 'AI') {
    return {
      ...draft,
      type,
      pathType: 'ROUTE_PATH_MATCH_TYPE_EXACT',
      path: '/v1/chat/completions',
      methods: ['POST'],
      services: [],
      aiModels: [{ name: '', targets: [{ serviceID: '', model: '', weight: 1 }] }],
    };
  }

  return {
    ...draft,
    type,
    pathType: 'ROUTE_PATH_MATCH_TYPE_PREFIX',
    path: '/',
    methods: [],
    services: [{ serviceID: '', weight: 1 }],
    aiModels: [],
  };
}

function GatewaySelectionEditor({ draft, gateways, onChange }: { draft: RouteDraft; gateways: RouteWorkspace['gateways']; onChange: (draft: RouteDraft) => void }) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between"><strong className="text-xs">生效网关</strong><Button variant="soft" size="sm" onClick={() => onChange({ ...draft, gatewayIDs: [...draft.gatewayIDs, ''] })}>添加网关</Button></div>
      <div className="grid gap-2">
        {draft.gatewayIDs.map((gatewayID, index) => (
          <div key={index} className="grid grid-cols-[minmax(0,1fr)_36px] gap-2">
            <select className="select" aria-label={`生效网关 ${index + 1}`} value={gatewayID} onChange={(event) => onChange({ ...draft, gatewayIDs: replaceAt(draft.gatewayIDs, index, event.target.value) })}>
              <option value="">选择网关</option>
              {gateways.map((gateway) => <option key={gateway.id} value={gateway.id} disabled={gateway.id !== gatewayID && draft.gatewayIDs.includes(gateway.id)}>{gateway.name}</option>)}
            </select>
            <Button variant="ghost" size="sm" aria-label={`删除生效网关 ${index + 1}`} onClick={() => onChange({ ...draft, gatewayIDs: draft.gatewayIDs.filter((_, current) => current !== index) })}><Trash2 className="h-3.5 w-3.5 text-rose-600" /></Button>
          </div>
        ))}
      </div>
    </div>
  );
}

function HTTPForwardingEditor({ draft, services, onChange }: { draft: RouteDraft; services: RouteWorkspace['services']; onChange: (draft: RouteDraft) => void }) {
  return (
    <div className="space-y-2">
      <div className="flex justify-between"><strong className="text-xs">目标服务</strong><Button variant="soft" size="sm" onClick={() => onChange({ ...draft, services: [...draft.services, { serviceID: '', weight: 1 }] })}>添加目标</Button></div>
      <div className="grid grid-cols-[minmax(0,1fr)_100px_36px] gap-2 px-1 text-[11px] font-medium text-slate-500" aria-hidden="true"><span>服务</span><span>权重</span><span /></div>
      {draft.services.map((target, index) => <div key={index} className="grid grid-cols-[minmax(0,1fr)_100px_36px] gap-2"><select className="select" value={target.serviceID} onChange={(event) => onChange({ ...draft, services: replaceAt(draft.services, index, { ...target, serviceID: event.target.value }) })}><option value="">选择 HTTP 服务</option>{services.map((service) => <option key={service.id} value={service.id}>{service.name} · {service.endpoint}</option>)}</select><input className="input" type="number" min="1" max="1000" aria-label="服务权重" value={target.weight} onChange={(event) => onChange({ ...draft, services: replaceAt(draft.services, index, { ...target, weight: Number(event.target.value) }) })} /><Button variant="ghost" size="sm" aria-label="删除目标服务" onClick={() => onChange({ ...draft, services: draft.services.filter((_, current) => current !== index) })}><Trash2 className="h-3.5 w-3.5 text-rose-600" /></Button></div>)}
    </div>
  );
}

function AIForwardingEditor({ draft, services, onChange }: { draft: RouteDraft; services: RouteWorkspace['services']; onChange: (draft: RouteDraft) => void }) {
  const updateModel = (index: number, model: AIModel) => onChange({ ...draft, aiModels: replaceAt(draft.aiModels, index, model) });
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between"><strong className="text-xs">发布模型</strong><Button variant="soft" size="sm" onClick={() => onChange({ ...draft, aiModels: [...draft.aiModels, { name: '', targets: [{ serviceID: '', model: '', weight: 1 }] }] })}>添加模型</Button></div>
      {services.length === 0 ? <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">请先在服务页面创建模型服务</div> : null}
      {draft.aiModels.map((model, modelIndex) => (
        <section key={modelIndex} className="rounded-xl border border-slate-200 bg-slate-50/50 p-4 space-y-3">
          <div className="grid grid-cols-[1fr_36px] gap-2"><Field label="客户端模型名"><input className="input font-mono" value={model.name} onChange={(event) => updateModel(modelIndex, { ...model, name: event.target.value })} placeholder="例如 qwen-max" /></Field><Button className="self-end" variant="ghost" size="sm" aria-label="删除客户端模型" onClick={() => onChange({ ...draft, aiModels: draft.aiModels.filter((_, index) => index !== modelIndex) })}><Trash2 className="h-3.5 w-3.5 text-rose-600" /></Button></div>
          <div className="flex items-center justify-between"><strong className="text-[11px] text-slate-600">模型线路</strong><Button variant="ghost" size="sm" onClick={() => updateModel(modelIndex, { ...model, targets: [...model.targets, { serviceID: '', model: '', weight: 1 }] })}>添加线路</Button></div>
          <div className="grid gap-2">
            <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_80px_36px] gap-2 px-1 text-[11px] font-medium text-slate-500" aria-hidden="true"><span>模型服务</span><span>真实模型名</span><span>权重</span><span /></div>
            {model.targets.map((target, targetIndex) => <div key={targetIndex} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_80px_36px] gap-2"><select className="select" aria-label="模型服务" value={target.serviceID} onChange={(event) => updateModel(modelIndex, { ...model, targets: replaceAt(model.targets, targetIndex, { ...target, serviceID: event.target.value }) })}><option value="">选择模型服务</option>{services.map((service) => <option key={service.id} value={service.id}>{service.name}</option>)}</select><input className="input font-mono" aria-label="真实模型名" value={target.model} onChange={(event) => updateModel(modelIndex, { ...model, targets: replaceAt(model.targets, targetIndex, { ...target, model: event.target.value }) })} placeholder="例如 qwen-max" /><input className="input" type="number" min="1" max="1000" aria-label="线路权重" value={target.weight} onChange={(event) => updateModel(modelIndex, { ...model, targets: replaceAt(model.targets, targetIndex, { ...target, weight: Number(event.target.value) }) })} /><Button variant="ghost" size="sm" aria-label="删除模型线路" onClick={() => updateModel(modelIndex, { ...model, targets: model.targets.filter((_, index) => index !== targetIndex) })}><Trash2 className="h-3.5 w-3.5 text-rose-600" /></Button></div>)}
          </div>
        </section>
      ))}
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
    accessMode: route?.accessMode ?? 'ROUTE_ACCESS_MODE_PUBLIC',
    gatewayIDs: route ? route.gatewayIDs : [''],
    hostnames: route?.hostnames.join(', ') ?? '',
    pathType: route?.match.path.type ?? 'ROUTE_PATH_MATCH_TYPE_PREFIX',
    path: route?.match.path.value ?? '/',
    methods: route?.match.methods ?? [],
    headers: route?.match.headers.map((header) => ({ ...header })) ?? [],
    services: route ? route.services.map((service) => ({ ...service })) : [{ serviceID: '', weight: 1 }],
    hostRewriteMode: route?.hostRewrite.mode ?? 'HOST_REWRITE_MODE_SERVICE_HOST',
    customHostname: route?.hostRewrite.hostname ?? '',
    timeoutEnabled: Boolean(route?.timeout),
    timeoutMillis: route?.timeout?.requestMillis ?? 30000,
    retryEnabled: Boolean(route?.retry),
    retryAttempts: route?.retry?.attempts ?? 2,
    perTryTimeoutMillis: route?.retry?.perTryTimeoutMillis ?? 5000,
    requestHeaderModifier: route?.requestHeaderModifier,
    responseHeaderModifier: route?.responseHeaderModifier,
    type: route?.ai ? 'AI' : 'HTTP',
    aiModels: route?.ai?.models.map((model) => ({ ...model, targets: model.targets.map((target) => ({ ...target })) })) ?? [],
  };
}

function routeFilterSummary(filters: RouteFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.type !== 'all') conditions.push(`类型：${filters.type === 'AI' ? 'AI 路由' : 'API 路由'}`);
  if (filters.enabled !== 'all') conditions.push(`启用状态：${filters.enabled === 'enabled' ? '已启用' : '已停用'}`);
  if (filters.state !== 'all') conditions.push(`生效状态：${resourceStateLabel(filters.state)}`);
  return conditions.join(' · ') || '全部路由';
}

function resourceNames(ids: string[], options: Array<{ id: string; name: string }>): string {
  return ids.map((id) => resourceName(id, options)).join('、') || '—';
}

function resourceName(id: string, options: Array<{ id: string; name: string }>): string {
  return options.find((option) => option.id === id)?.name ?? id;
}

function routeServiceIDs(route: RouteResource): string[] {
  if (!route.ai) return route.services.map((target) => target.serviceID);
  return [...new Set(route.ai.models.flatMap((model) => model.targets.map((target) => target.serviceID)))];
}

function methodLabel(route: RouteResource): string {
  return route.match.methods.length > 0 ? route.match.methods.join('、') : '所有方法';
}

function pathMatchLabel(route: RouteResource): string {
  return route.match.path.type === 'ROUTE_PATH_MATCH_TYPE_EXACT' ? '精确' : '前缀';
}

function replaceAt<T>(items: T[], index: number, value: T): T[] {
  return items.map((item, current) => current === index ? value : item);
}

function validateDraft(draft: RouteDraft): string | undefined {
  if (!draft.name.trim()) return '请输入路由名称';
  if (draft.gatewayIDs.length === 0 || draft.gatewayIDs.some((id) => !id)) return '至少选择一个有效的网关';
  if (new Set(draft.gatewayIDs).size !== draft.gatewayIDs.length) return '生效网关不能重复';
  if (!draft.path.startsWith('/')) return '请求路径必须以 / 开头';
  if (draft.type === 'HTTP' && (draft.services.length === 0 || draft.services.some((item) => !item.serviceID || item.weight < 1 || item.weight > 1000))) return '至少配置一个有效的目标服务';
  if (draft.type === 'AI') {
    if (draft.aiModels.length === 0) return '至少发布一个客户端模型';
    const names = draft.aiModels.map((model) => model.name.trim());
    if (names.some((name) => !name) || new Set(names).size !== names.length) return '客户端模型名不能为空或重复';
    if (draft.aiModels.some((model) => model.targets.length === 0 || model.targets.some((target) => !target.serviceID || !target.model.trim() || target.weight < 1 || target.weight > 1000))) return '每个客户端模型至少需要一条有效的模型线路';
  }
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
    accessMode: draft.accessMode,
    gatewayIDs: draft.gatewayIDs,
    hostnames: draft.hostnames.split(/[,，\s]+/).map((value) => value.trim().toLowerCase()).filter(Boolean),
    match: { path: { type: draft.pathType, value: draft.path.trim() }, methods: draft.type === 'AI' ? ['POST'] : draft.methods, headers: draft.headers },
    services: draft.type === 'HTTP' ? draft.services : [],
    ai: draft.type === 'AI' ? { models: draft.aiModels.map((model) => ({ name: model.name.trim(), targets: model.targets.map((target) => ({ ...target, model: target.model.trim() })) })) } : undefined,
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
    case 'HOST_REWRITE_MODE_SERVICE_HOST': return '使用服务端点主机名';
    case 'HOST_REWRITE_MODE_CUSTOM': return route.hostRewrite.hostname || '未填写';
    default: return '保持请求主机';
  }
}

function accessModeLabel(mode: RouteAccessMode): string {
  return mode === 'ROUTE_ACCESS_MODE_PUBLIC' ? '公开访问' : '调用方密钥';
}

function validHostname(value: string): boolean {
  const hostname = value.trim();
  return hostname.length > 0 && hostname.length <= 253 && hostname.split('.').every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label));
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
