import { useCallback, useState } from 'react';
import { Plus, Route as RouteIcon } from 'lucide-react';
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
import type {
  RouteResource,
  RouteWorkspace,
} from '@/domain/route';
import { ResourceTrafficSignal, useResourceTrafficOverview } from '@/features/traffic/ResourceTrafficSignal';
import { createDraft, toPayload, validateDraft, type RouteDraft } from './form';
import { accessModeLabel, methodLabel, pathMatchLabel, routeServiceIDs } from './presentation';
import { RouteDetail } from './RouteDetail';
import { RouteEditor } from './RouteEditor';

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

function routeFilterSummary(filters: RouteFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.type !== 'all') conditions.push(`类型：${filters.type === 'AI' ? 'AI 路由' : 'API 路由'}`);
  if (filters.enabled !== 'all') conditions.push(`启用状态：${filters.enabled === 'enabled' ? '已启用' : '已停用'}`);
  if (filters.state !== 'all') conditions.push(`生效状态：${resourceStateLabel(filters.state)}`);
  return conditions.join(' · ') || '全部路由';
}
