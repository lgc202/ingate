import { useState, type ReactNode } from 'react';
import { Edit3, Plus, Server, Trash2 } from 'lucide-react';
import { deleteUpstream, listUpstreams, saveUpstream } from '@/api/upstreams';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Badge, Button, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Upstream } from '@/domain/upstream';
import { upstreamLoadBalancingLabel, upstreamLoadBalancingOptions } from '@/domain/upstream';
import { buildUpstreamPayload, createUpstreamDraft, validateUpstreamDraft, type UpstreamDraft } from './form';

export function UpstreamPage() {
  const { canWriteConfiguration } = useAuth();
  const resource = useResource(listUpstreams);
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
      subtitle="管理 Admin API 已支持的 HTTP 上游服务"
      actions={canWriteConfiguration ? <Button onClick={() => openEditor()}><Plus className="w-4 h-4" />创建服务</Button> : undefined}
    >
      <Panel>
        {resource.data.upstreams.length === 0 ? <EmptyState title="暂无服务" message="创建服务后即可在路由中选择转发目标" /> : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead><tr className="border-b border-slate-200 text-slate-500"><th className="p-3">名称</th><th className="p-3">地址</th><th className="p-3">连接</th><th className="p-3">负载均衡</th><th className="p-3">状态</th><th className="p-3">更新时间</th><th className="p-3 text-right">操作</th></tr></thead>
              <tbody className="divide-y divide-slate-100">
                {resource.data.upstreams.map((item) => (
                  <tr key={item.id}>
                    <td className="p-3"><div className="flex items-center gap-2"><Server className="w-4 h-4 text-blue-600" /><div><strong>{item.name}</strong><div className="font-mono text-[10px] text-slate-400">{item.id}</div></div></div></td>
                    <td className="p-3 font-mono text-[11px]">{item.endpoints.map((endpoint) => `${endpoint.address}:${endpoint.port}`).join('、')}</td>
                    <td className="p-3">{item.tls ? `HTTPS · ${item.tls.serverName}` : 'HTTP'}</td>
                    <td className="p-3">{upstreamLoadBalancingLabel(item.loadBalancing)}</td>
                    <td className="p-3"><Badge tone={item.state === 'Ready' ? 'success' : item.state === 'Error' ? 'error' : 'neutral'}>{item.state}</Badge></td>
                    <td className="p-3 text-slate-500">{formatDateTime(item.updatedAt || item.createdAt)}</td>
                    <td className="p-3 text-right">{canWriteConfiguration ? <div className="inline-flex gap-1"><Button variant="ghost" size="sm" onClick={() => openEditor(item)}><Edit3 className="w-3.5 h-3.5" /></Button><Button variant="ghost" size="sm" onClick={() => setDeleteCandidate(item)}><Trash2 className="w-3.5 h-3.5 text-rose-600" /></Button></div> : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Drawer title={draft.id ? `编辑服务：${draft.name}` : '创建服务'} subtitle="配置上游地址、HTTPS 和健康检查" isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        <div className="space-y-5">
          <Field label="服务名称"><input className="input" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></Field>
          <Field label="负载均衡"><select className="select" value={draft.loadBalancing} onChange={(event) => setDraft({ ...draft, loadBalancing: event.target.value as UpstreamDraft['loadBalancing'] })}>{upstreamLoadBalancingOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></Field>
          <div className="space-y-2"><div className="flex justify-between"><strong className="text-xs">服务地址</strong><Button variant="soft" size="sm" onClick={() => setDraft({ ...draft, endpoints: [...draft.endpoints, { address: '', port: 8080, weight: 1 }] })}>添加地址</Button></div>{draft.endpoints.map((endpoint, index) => <div key={index} className="grid grid-cols-[1fr_90px_90px_36px] gap-2"><input className="input font-mono" placeholder="service.example.com" value={endpoint.address} onChange={(event) => setDraft({ ...draft, endpoints: draft.endpoints.map((item, current) => current === index ? { ...item, address: event.target.value } : item) })} /><input className="input" type="number" min="1" max="65535" value={endpoint.port} onChange={(event) => setDraft({ ...draft, endpoints: draft.endpoints.map((item, current) => current === index ? { ...item, port: Number(event.target.value) } : item) })} /><input className="input" type="number" min="1" max="10000" value={endpoint.weight} onChange={(event) => setDraft({ ...draft, endpoints: draft.endpoints.map((item, current) => current === index ? { ...item, weight: Number(event.target.value) } : item) })} /><Button variant="ghost" size="sm" onClick={() => setDraft({ ...draft, endpoints: draft.endpoints.filter((_, current) => current !== index) })}>×</Button></div>)}</div>
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

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
