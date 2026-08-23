import { useCallback, useState, type ReactNode } from 'react';
import { Globe, Layers3, Plus, Trash2 } from 'lucide-react';
import { Link, useSearchParams } from 'react-router-dom';
import { listCertificates } from '@/api/certificates';
import { deleteGateway, listGatewayPage, saveGateway } from '@/api/gateways';
import { getPolicyListWorkspace } from '@/api/policies';
import { listRoutes } from '@/api/routes';
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
import type { Gateway, GatewayListener, GatewayProtocol } from '@/domain/gateway';
import { gatewayProtocolLabel } from '@/domain/gateway';
import type { PolicyWorkspace } from '@/domain/policy';
import { policyTargetsResource } from '@/domain/policy';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';
import { ResourceTrafficSignal, useResourceTrafficOverview } from '@/features/traffic/ResourceTrafficSignal';
import { ResourceTrafficSummary } from '@/features/traffic/ResourceTrafficSummary';
import { buildGatewayPayload, createGatewayDraft, newListener, validateGatewayDraft, type GatewayDraft } from './form';

type GatewayEnabledFilter = 'all' | 'enabled' | 'disabled';
type GatewayStateFilter = 'all' | Exclude<ResourceState, 'Disabled'>;

interface GatewayFilters {
  query: string;
  enabled: GatewayEnabledFilter;
  state: GatewayStateFilter;
}

const emptyGatewayFilters = (): GatewayFilters => ({ query: '', enabled: 'all', state: 'all' });

export function GatewayPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterDraft, setFilterDraft] = useState<GatewayFilters>(emptyGatewayFilters);
  const [filters, setFilters] = useState<GatewayFilters>(emptyGatewayFilters);
  const [pageSize, setPageSize] = useState(10);
  const [draft, setDraft] = useState<GatewayDraft>(() => createGatewayDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Gateway | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const loadPage = useCallback((cursor: string) => listGatewayPage({
    limit: pageSize,
    cursor,
    query: filters.query.trim() || undefined,
    enabled: filters.enabled === 'all' ? undefined : filters.enabled === 'enabled',
    state: filters.state === 'all' ? undefined : filters.state.toUpperCase(),
  }), [filters, pageSize]);
  const gateways = useCursorResource(loadPage, {
    autoRefreshWhen: (data) => data.items.some((gateway) => gateway.enabled && gateway.state === 'Pending'),
  });
  const list = gateways.data?.items ?? [];
  const detail = list.find((gateway) => gateway.id === searchParams.get('detail')) ?? null;
  const trafficOverview = useResourceTrafficOverview('gateway', list.map((gateway) => gateway.id));
  const certificates = useResource(listCertificates, { enabled: editorOpen });
  const policies = useResource(getPolicyListWorkspace, { enabled: Boolean(detail) });
  const routes = useResource(listRoutes, { enabled: Boolean(detail) });

  if (gateways.loading && !gateways.data) {
    return <PageFrame title="网关"><ResourceStatePanel title="正在加载网关" message="正在读取当前网关配置" /></PageFrame>;
  }
  if (gateways.error || !gateways.data) {
    return <PageFrame title="网关"><ResourceStatePanel title="网关加载失败" message={gateways.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const certificateList = certificates.data?.certificates ?? [];
  const referencingRoutes = (gatewayID: string) => routes.data?.routes.filter((route) => route.gatewayIDs.includes(gatewayID)) ?? [];
  const referencingPolicies = (gatewayID: string) => policies.data?.policies.filter((policy) => policyTargetsResource(policy, 'Gateway', gatewayID)) ?? [];
  const setDetail = (gateway?: Gateway) => {
    const next = new URLSearchParams(searchParams);
    if (gateway) next.set('detail', gateway.id);
    else next.delete('detail');
    setSearchParams(next);
  };

  const openEditor = (gateway?: Gateway) => {
    setDraft(createGatewayDraft(gateway));
    setEditorOpen(true);
  };
  const save = async () => {
    const error = validateGatewayDraft(draft);
    if (error) {
      setNotice({ message: error, tone: 'error' });
      return;
    }
    setBusy(true);
    try {
      const saved = await saveGateway(buildGatewayPayload(draft));
      await gateways.reload();
      setEditorOpen(false);
      setNotice({ message: `网关已保存：${saved.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存网关失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };
  const toggleGateway = async (gateway: Gateway) => {
    if (busy) return;
    setBusy(true);
    try {
      await saveGateway(buildGatewayPayload({ ...createGatewayDraft(gateway), enabled: !gateway.enabled }));
      await gateways.reload();
      setNotice({ message: `网关已${gateway.enabled ? '停用' : '启用'}：${gateway.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新网关启用状态失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };
  const remove = async () => {
    if (!deleteCandidate) return;
    setBusy(true);
    try {
      await deleteGateway(deleteCandidate.id, deleteCandidate.version);
      await gateways.reload();
      setDeleteCandidate(null);
      setNotice({ message: `网关已删除：${deleteCandidate.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除网关失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageFrame
      title="网关"
      actions={<Button onClick={() => openEditor()}><Plus className="w-4 h-4" />创建网关</Button>}
    >
      <Panel>
        <ResourceListFilters
          summary={gatewayFilterSummary(filters)}
          resultLabel={`本页 ${list.length} 个网关`}
          onSearch={() => { gateways.reset(); setFilters({ ...filterDraft }); }}
          onReset={() => {
            const next = emptyGatewayFilters();
            setFilterDraft(next);
            setFilters(next);
            gateways.reset();
          }}
        >
          <ResourceFilterField label="关键词">
            <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索网关、域名或监听入口" />
          </ResourceFilterField>
          <ResourceFilterField label="启用状态">
            <select className="select" value={filterDraft.enabled} onChange={(event) => setFilterDraft((current) => ({ ...current, enabled: event.target.value as GatewayEnabledFilter }))}>
              <option value="all">全部启用状态</option>
              <option value="enabled">已启用</option>
              <option value="disabled">已停用</option>
            </select>
          </ResourceFilterField>
          <ResourceFilterField label="生效状态">
            <select className="select" value={filterDraft.state} onChange={(event) => setFilterDraft((current) => ({ ...current, state: event.target.value as GatewayStateFilter }))}>
              <option value="all">全部生效状态</option>
              <option value="Ready">已生效</option>
              <option value="Pending">待生效</option>
              <option value="Error">生效失败</option>
            </select>
          </ResourceFilterField>
        </ResourceListFilters>
        {list.length === 0 ? <div className="p-5"><EmptyState title={filters.query || filters.enabled !== 'all' || filters.state !== 'all' ? '没有匹配的网关' : '暂无网关'} message={filters.query || filters.enabled !== 'all' || filters.state !== 'all' ? '请调整搜索条件' : '创建网关后即可接收客户端流量'} /></div> : (
          <div className="table-scroll resource-table-scroll">
            <table className="table resource-table resource-table-has-toggle resource-gateway-table">
              <thead><tr><th>名称</th><th>监听入口</th><th>最近 1 小时</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
              <tbody>
                {list.map((gateway) => (
                  <tr key={gateway.id}>
                    <td><div className="resource-table-name"><Layers3 className="text-blue-600" /><strong>{gateway.name}</strong></div></td>
                    <td><div className="flex flex-wrap gap-1.5">{gateway.listeners.map((listener) => <Badge key={listener.name} tone="neutral">{gatewayProtocolLabel(listener.protocol)} · {listener.port} · {listener.hostname || '全部域名'}</Badge>)}</div></td>
                    <td><ResourceTrafficSignal resourceID={gateway.id} overview={trafficOverview} /></td>
                    <td>
                      <div className="resource-state-badges">
                        <Badge tone={gateway.enabled ? 'accent' : 'neutral'}>{gateway.enabled ? '已启用' : '已停用'}</Badge>
                        {gateway.enabled ? <Badge tone={resourceStateTone(gateway.state)}>{resourceStateLabel(gateway.state)}</Badge> : null}
                      </div>
                    </td>
                    <td className="resource-table-time">{formatDateTime(gateway.updatedAt || gateway.createdAt)}</td>
                    <td>
                      <RowActions
                        onDetail={() => setDetail(gateway)}
                        onEdit={() => openEditor(gateway)}
                        onToggle={() => void toggleGateway(gateway)}
                        toggleLabel={gateway.enabled ? '停用' : '启用'}
                        toggleDisabled={busy}
                        onDelete={() => setDeleteCandidate(gateway)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {list.length > 0 ? <ResourcePagination page={gateways.page} pageSize={pageSize} itemCount={list.length} hasNext={gateways.hasNext} onPageChange={(nextPage) => nextPage > gateways.page ? gateways.next() : gateways.previous()} onPageSizeChange={(size) => { gateways.reset(); setPageSize(size); }} /> : null}
      </Panel>

      <Drawer title="网关详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail()}>
        {detail ? <GatewayDetail gateway={detail} routes={referencingRoutes(detail.id)} policies={policies.data} onPoliciesChanged={policies.reload} /> : null}
      </Drawer>

      <Drawer title={draft.id ? `编辑网关：${draft.name}` : '创建网关'} isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        {certificates.loading && !certificates.data ? (
          <ResourceStatePanel title="正在加载证书" message="正在读取 HTTPS 监听入口可使用的证书" />
        ) : certificates.error ? (
          <ResourceStatePanel title="证书加载失败" message={certificates.error.message} />
        ) : <div className="space-y-5">
          <Field label="网关名称"><input className="input" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></Field>
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} />启用网关</label>
          <div className="space-y-3">
            <div className="flex items-center justify-between"><strong className="text-xs">监听入口</strong><Button variant="soft" size="sm" onClick={() => setDraft({ ...draft, listeners: [...draft.listeners, newListener(draft.listeners.length)] })}>添加入口</Button></div>
            {draft.listeners.map((listener, index) => (
              <ListenerEditor
                key={index}
                listener={listener}
                certificates={certificateList.map((certificate) => ({ id: certificate.id, name: certificate.name }))}
                onChange={(next) => setDraft({ ...draft, listeners: draft.listeners.map((item, current) => current === index ? next : item) })}
                onRemove={() => setDraft({ ...draft, listeners: draft.listeners.filter((_, current) => current !== index) })}
              />
            ))}
          </div>
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-3"><Button variant="ghost" onClick={() => setEditorOpen(false)}>取消</Button><Button disabled={busy} onClick={save}>{busy ? '保存中...' : '保存网关'}</Button></div>
        </div>}
      </Drawer>

      <Modal title="删除网关" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}><div className="space-y-5 p-6"><p className="text-sm">确定删除网关“{deleteCandidate?.name}”吗？</p><div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div></div></Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function GatewayDetail({ gateway, routes, policies, onPoliciesChanged }: { gateway: Gateway; routes: Array<{ id: string; name: string; ai?: unknown }>; policies: PolicyWorkspace | null; onPoliciesChanged: () => Promise<void> }) {
  const state = gateway.enabled ? gateway.state : 'Disabled';
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{gateway.name}</h3><p>{gateway.enabled ? '网关已启用' : '网关已停用'}</p></div>
        <Badge tone={resourceStateTone(state)}>{resourceStateLabel(state)}</Badge>
      </section>
      <ResourceTrafficSummary kind="gateway" resourceID={gateway.id} />
      <section className="resource-detail-section">
        <h3>基本信息</h3>
        <div className="resource-detail-grid">
          <div><span>启用状态</span><strong>{gateway.enabled ? '已启用' : '已停用'}</strong></div>
          <div><span>生效状态</span><strong>{gateway.message || resourceStateLabel(state)}</strong></div>
          <div><span>更新时间</span><strong>{formatDateTime(gateway.updatedAt || gateway.createdAt)}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(gateway.createdAt)}</strong></div>
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>监听入口</h3>
        <div className="resource-detail-list">
          {gateway.listeners.map((listener) => (
            <article key={listener.name}>
              <div><strong>{listener.name}</strong><small>{listener.hostname || '全部域名'}</small></div>
              <Badge tone={listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' ? 'accent' : 'neutral'}>{gatewayProtocolLabel(listener.protocol)} · {listener.port}</Badge>
            </article>
          ))}
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>关联路由</h3>
        {routes.length > 0 ? <div className="resource-detail-list">{routes.map((route) => <article key={route.id}><div><strong>{route.name}</strong><small>{route.ai ? 'AI 路由' : 'API 路由'}</small></div><Badge tone="neutral">路由</Badge></article>)}</div> : <p className="text-xs text-slate-500">当前没有路由使用此网关</p>}
      </section>
      {policies ? (
        <section className="resource-detail-section">
          <h3>应用策略</h3>
          <GovernancePolicyPanel targetKind="Gateway" targetID={gateway.id} targetName={gateway.name} workspace={policies} onChanged={onPoliciesChanged} />
        </section>
      ) : null}
    </div>
  );
}

function ListenerEditor({ listener, certificates, onChange, onRemove }: { listener: GatewayListener; certificates: Array<{ id: string; name: string }>; onChange: (listener: GatewayListener) => void; onRemove: () => void }) {
  const setProtocol = (protocol: GatewayProtocol) => onChange({
    ...listener,
    protocol,
    certificateID: protocol === 'GATEWAY_PROTOCOL_HTTPS' ? listener.certificateID : '',
  });
  return (
    <div className="space-y-3 rounded-xl border border-slate-200 bg-slate-50/60 p-4">
      <div className="grid grid-cols-[1fr_120px_100px] gap-3"><Field label="名称"><input className="input" value={listener.name} onChange={(event) => onChange({ ...listener, name: event.target.value })} /></Field><Field label="协议"><select className="select" value={listener.protocol} onChange={(event) => setProtocol(event.target.value as GatewayProtocol)}><option value="GATEWAY_PROTOCOL_HTTP">HTTP</option><option value="GATEWAY_PROTOCOL_HTTPS">HTTPS</option></select></Field><Field label="端口"><input className="input" type="number" min="1" max="65535" placeholder={listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' ? '443' : '80'} value={listener.port || ''} onChange={(event) => onChange({ ...listener, port: Number(event.target.value) })} /></Field></div>
      <Field label="域名（留空表示全部域名）"><div className="relative"><Globe className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" /><input className="input input-leading-icon font-mono" placeholder="api.example.com 或 *.example.com" value={listener.hostname} onChange={(event) => onChange({ ...listener, hostname: event.target.value })} /></div></Field>
      {listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' ? certificates.length > 0 ? <Field label="TLS 证书"><select className="select" value={listener.certificateID} onChange={(event) => onChange({ ...listener, certificateID: event.target.value })}><option value="">选择证书</option>{certificates.map((certificate) => <option key={certificate.id} value={certificate.id}>{certificate.name}</option>)}</select></Field> : <p className="text-xs text-amber-700">请先<Link className="mx-1 font-semibold underline" to="/certificates">创建证书</Link>再配置 HTTPS</p> : null}
      <div className="flex justify-end"><Button variant="ghost" size="sm" onClick={onRemove}><Trash2 className="h-3.5 w-3.5 text-rose-600" />删除入口</Button></div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}

function gatewayFilterSummary(filters: GatewayFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.enabled !== 'all') conditions.push(`启用状态：${filters.enabled === 'enabled' ? '已启用' : '已停用'}`);
  if (filters.state !== 'all') conditions.push(`生效状态：${resourceStateLabel(filters.state)}`);
  return conditions.join(' · ') || '全部网关';
}
