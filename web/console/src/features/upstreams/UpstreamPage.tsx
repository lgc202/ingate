import { useState, type ReactNode } from 'react';
import { Plus, Server, Trash2 } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { deleteUpstream, listUpstreams, saveUpstream } from '@/api/upstreams';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Badge, Button, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, RowActions, SearchField, Toast } from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone } from '@/domain/common';
import type { Upstream, UpstreamEndpoint } from '@/domain/upstream';
import { upstreamLoadBalancingLabel, upstreamLoadBalancingOptions } from '@/domain/upstream';
import { ResourceTrafficSummary } from '@/features/traffic/ResourceTrafficSummary';
import { buildUpstreamPayload, createUpstreamDraft, validateUpstreamDraft, type UpstreamDraft } from './form';

export function UpstreamPage() {
  const { canWriteConfiguration } = useAuth();
  const resource = useResource(listUpstreams);
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState('');
  const [draft, setDraft] = useState<UpstreamDraft>(() => createUpstreamDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Upstream | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  if (resource.loading && !resource.data) {
    return <PageFrame title="服务"><ResourceStatePanel title="正在加载服务" message="正在读取当前服务配置" /></PageFrame>;
  }
  if (resource.error || !resource.data) {
    return <PageFrame title="服务"><ResourceStatePanel title="服务加载失败" message={resource.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const openEditor = (upstream?: Upstream) => {
    setDraft(createUpstreamDraft(upstream));
    setEditorOpen(true);
  };

  const normalizedQuery = query.trim().toLowerCase();
  const detail = resource.data.upstreams.find((upstream) => upstream.id === searchParams.get('detail')) ?? null;
  const visibleUpstreams = resource.data.upstreams.filter((upstream) => (
    `${upstream.name} ${upstream.endpoints.map((endpoint) => `${endpoint.address}:${endpoint.port}`).join(' ')}`
      .toLowerCase()
      .includes(normalizedQuery)
  ));
  const setDetail = (upstream?: Upstream) => {
    const next = new URLSearchParams(searchParams);
    if (upstream) next.set('detail', upstream.id);
    else next.delete('detail');
    setSearchParams(next);
  };
  const updateEndpoint = (index: number, endpoint: UpstreamEndpoint) => {
    setDraft({
      ...draft,
      endpoints: draft.endpoints.map((item, current) => current === index ? endpoint : item),
    });
  };
  const save = async () => {
    const errors = validateUpstreamDraft(draft);
    if (errors.length > 0) {
      setNotice({ message: errors[0], tone: 'error' });
      return;
    }
    setBusy(true);
    try {
      const saved = await saveUpstream(buildUpstreamPayload(draft));
      await resource.reload();
      setEditorOpen(false);
      setNotice({ message: `服务已保存：${saved.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存服务失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };
  const remove = async () => {
    if (!deleteCandidate) return;
    setBusy(true);
    try {
      await deleteUpstream(deleteCandidate.id, deleteCandidate.version);
      await resource.reload();
      setNotice({ message: `服务已删除：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除服务失败', tone: 'error' });
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageFrame
      title="服务"
      actions={canWriteConfiguration ? <Button onClick={() => openEditor()}><Plus className="w-4 h-4" />创建服务</Button> : undefined}
    >
      <Panel>
        <div className="resource-list-toolbar">
          <SearchField value={query} onChange={setQuery} placeholder="搜索服务、地址或端口" />
          <span>{visibleUpstreams.length} 个服务</span>
        </div>
        {visibleUpstreams.length === 0 ? <div className="p-5"><EmptyState title={resource.data.upstreams.length === 0 ? '暂无服务' : '没有匹配的服务'} message={resource.data.upstreams.length === 0 ? '创建服务后即可在路由中选择转发目标' : '请调整搜索条件'} /></div> : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead><tr className="border-b border-slate-200 text-slate-500"><th className="p-3">名称</th><th className="p-3">地址</th><th className="p-3">连接</th><th className="p-3">负载均衡</th><th className="p-3">状态</th><th className="p-3">更新时间</th><th className="p-3 text-right">操作</th></tr></thead>
              <tbody className="divide-y divide-slate-100">
                {visibleUpstreams.map((item) => (
                  <tr key={item.id}>
                    <td className="p-3"><div className="flex items-center gap-2"><Server className="w-4 h-4 text-blue-600" /><strong>{item.name}</strong></div></td>
                    <td className="p-3 font-mono text-[11px]">{item.endpoints.map((endpoint) => `${endpoint.address}:${endpoint.port}`).join('、')}</td>
                    <td className="p-3">{item.tls ? `HTTPS · ${item.tls.serverName}` : 'HTTP'}</td>
                    <td className="p-3">{upstreamLoadBalancingLabel(item.loadBalancing)}</td>
                    <td className="p-3"><Badge tone={resourceStateTone(item.state)}>{resourceStateLabel(item.state)}</Badge></td>
                    <td className="p-3 text-slate-500">{formatDateTime(item.updatedAt || item.createdAt)}</td>
                    <td className="p-3 text-right"><RowActions onDetail={() => setDetail(item)} onEdit={canWriteConfiguration ? () => openEditor(item) : undefined} onDelete={canWriteConfiguration ? () => setDeleteCandidate(item) : undefined} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Drawer title="服务详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail()}>
        {detail ? <UpstreamDetail upstream={detail} /> : null}
      </Drawer>

      <Drawer title={draft.id ? `编辑服务：${draft.name}` : '创建服务'} isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        <div className="space-y-5">
          <Field label="服务名称"><input className="input" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></Field>
          <Field label="负载均衡"><select className="select" value={draft.loadBalancing} onChange={(event) => setDraft({ ...draft, loadBalancing: event.target.value as UpstreamDraft['loadBalancing'] })}>{upstreamLoadBalancingOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></Field>
          <div className="space-y-3">
            <div className="flex items-start justify-between gap-4">
              <strong className="text-xs">服务地址</strong>
              <Button variant="soft" size="sm" onClick={() => setDraft({ ...draft, endpoints: [...draft.endpoints, { address: '', port: 8080, weight: 1 }] })}>添加地址</Button>
            </div>
            <div className="overflow-x-auto">
              <div className="grid min-w-[480px] gap-2">
                <div className="grid grid-cols-[minmax(0,1fr)_90px_90px_36px] gap-2 px-1 text-[11px] font-medium text-slate-500" aria-hidden="true">
                  <span>地址</span><span>端口</span><span>权重</span><span />
                </div>
                {draft.endpoints.map((endpoint, index) => (
                  <div key={index} className="grid grid-cols-[minmax(0,1fr)_90px_90px_36px] gap-2">
                    <input className="input font-mono" aria-label={`地址 ${index + 1}`} placeholder="service.example.com" value={endpoint.address} onChange={(event) => updateEndpoint(index, { ...endpoint, address: event.target.value })} />
                    <input className="input" aria-label={`端口 ${index + 1}`} type="number" min="1" max="65535" value={endpoint.port} onChange={(event) => updateEndpoint(index, { ...endpoint, port: Number(event.target.value) })} />
                    <input className="input" aria-label={`权重 ${index + 1}`} type="number" min="1" max="1000" value={endpoint.weight} onChange={(event) => updateEndpoint(index, { ...endpoint, weight: Number(event.target.value) })} />
                    <Button variant="ghost" size="sm" aria-label={`删除地址 ${index + 1}`} onClick={() => setDraft({ ...draft, endpoints: draft.endpoints.filter((_, current) => current !== index) })}><Trash2 className="h-3.5 w-3.5 text-rose-600" /></Button>
                  </div>
                ))}
              </div>
            </div>
          </div>
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.httpsEnabled} onChange={(event) => setDraft({ ...draft, httpsEnabled: event.target.checked })} />使用 HTTPS</label>
          {draft.httpsEnabled ? <Field label="证书服务名称"><input className="input" value={draft.serverName} onChange={(event) => setDraft({ ...draft, serverName: event.target.value })} /></Field> : null}
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={draft.healthCheckEnabled} onChange={(event) => setDraft({ ...draft, healthCheckEnabled: event.target.checked })} />启用主动健康检查</label>
          {draft.healthCheckEnabled ? <div className="grid grid-cols-3 gap-3"><Field label="检查路径"><input className="input" value={draft.healthCheckPath} onChange={(event) => setDraft({ ...draft, healthCheckPath: event.target.value })} /></Field><Field label="间隔（秒）"><input className="input" type="number" value={draft.healthCheckInterval} onChange={(event) => setDraft({ ...draft, healthCheckInterval: Number(event.target.value) })} /></Field><Field label="超时（秒）"><input className="input" type="number" value={draft.healthCheckTimeout} onChange={(event) => setDraft({ ...draft, healthCheckTimeout: Number(event.target.value) })} /></Field></div> : null}
          <div className="flex justify-end gap-2 pt-3 border-t border-slate-200"><Button variant="ghost" onClick={() => setEditorOpen(false)}>取消</Button><Button disabled={busy} onClick={save}>{busy ? '保存中...' : '保存服务'}</Button></div>
        </div>
      </Drawer>

      <Modal title="删除服务" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}><div className="p-6 space-y-5"><p className="text-sm">确定删除服务“{deleteCandidate?.name}”吗？</p><div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div></div></Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function UpstreamDetail({ upstream }: { upstream: Upstream }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{upstream.name}</h3></div>
        <Badge tone={resourceStateTone(upstream.state)}>{resourceStateLabel(upstream.state)}</Badge>
      </section>
      <ResourceTrafficSummary kind="service" resourceID={upstream.id} />
      <section className="resource-detail-section">
        <h3>连接设置</h3>
        <div className="resource-detail-grid">
          <div><span>协议</span><strong>{upstream.tls ? 'HTTPS' : 'HTTP'}</strong></div>
          <div><span>负载均衡</span><strong>{upstreamLoadBalancingLabel(upstream.loadBalancing)}</strong></div>
          <div><span>TLS 服务名称</span><strong>{upstream.tls?.serverName || '—'}</strong></div>
          <div><span>主动健康检查</span><strong>{upstream.healthCheck ? `${upstream.healthCheck.path} · 每 ${upstream.healthCheck.intervalSeconds} 秒` : '未启用'}</strong></div>
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>服务地址</h3>
        <div className="resource-detail-list">
          {upstream.endpoints.map((endpoint) => <article key={`${endpoint.address}:${endpoint.port}`}><div><strong>{endpoint.address}:{endpoint.port}</strong><small>转发权重 {endpoint.weight}</small></div><Badge tone="neutral">{upstream.tls ? 'HTTPS' : 'HTTP'}</Badge></article>)}
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>资源信息</h3>
        <div className="resource-detail-grid">
          <div><span>配置状态</span><strong>{upstream.message || resourceStateLabel(upstream.state)}</strong></div>
          <div><span>更新时间</span><strong>{formatDateTime(upstream.updatedAt || upstream.createdAt)}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(upstream.createdAt)}</strong></div>
          <div><span>配置版本</span><strong>{upstream.version}</strong></div>
        </div>
      </section>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
