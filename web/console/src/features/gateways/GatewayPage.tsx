import { useState, type ReactNode } from 'react';
import { Edit3, Globe, Layers3, Plus, Trash2 } from 'lucide-react';
import { Link } from 'react-router-dom';
import { listCertificates } from '@/api/certificates';
import { deleteGateway, listGateways, saveGateway } from '@/api/gateways';
import { getPolicyWorkspace } from '@/api/policies';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Badge, Button, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Gateway, GatewayListener, GatewayProtocol } from '@/domain/gateway';
import { gatewayProtocolLabel } from '@/domain/gateway';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';
import { buildGatewayPayload, createGatewayDraft, newListener, validateGatewayDraft, type GatewayDraft } from './form';

export function GatewayPage() {
  const { canWriteConfiguration } = useAuth();
  const gateways = useResource(listGateways);
  const certificates = useResource(listCertificates);
  const policies = useResource(getPolicyWorkspace);
  const [selectedID, setSelectedID] = useState('');
  const [draft, setDraft] = useState<GatewayDraft>(() => createGatewayDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Gateway | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  if (gateways.loading && !gateways.data) {
    return <PageFrame title="网关"><ResourceStatePanel title="正在加载网关" message="正在读取当前网关配置" /></PageFrame>;
  }
  if (gateways.error || !gateways.data) {
    return <PageFrame title="网关"><ResourceStatePanel title="网关加载失败" message={gateways.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const list = gateways.data.gateways;
  const selected = list.find((gateway) => gateway.id === selectedID) ?? list[0];
  const certificateList = certificates.data?.certificates ?? [];

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
      setSelectedID(saved.id);
      setEditorOpen(false);
      setNotice({ message: `网关已保存：${saved.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存网关失败', tone: 'error' });
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
      subtitle="管理客户端访问入口、监听端口和 TLS 证书"
      actions={canWriteConfiguration ? <Button onClick={() => openEditor()}><Plus className="w-4 h-4" />创建网关</Button> : undefined}
    >
      <Panel>
        {list.length === 0 ? <EmptyState title="暂无网关" message="创建网关后即可接收客户端流量" /> : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead><tr className="border-b border-slate-200 text-slate-500"><th className="p-3">名称</th><th className="p-3">监听入口</th><th className="p-3">状态</th><th className="p-3">更新时间</th><th className="p-3 text-right">操作</th></tr></thead>
              <tbody className="divide-y divide-slate-100">
                {list.map((gateway) => (
                  <tr key={gateway.id} className={selected?.id === gateway.id ? 'bg-blue-50/40' : ''} onClick={() => setSelectedID(gateway.id)}>
                    <td className="p-3"><div className="flex items-center gap-2"><Layers3 className="w-4 h-4 text-blue-600" /><div><strong>{gateway.name}</strong><div className="font-mono text-[10px] text-slate-400">{gateway.id}</div></div></div></td>
                    <td className="p-3"><div className="flex flex-wrap gap-1.5">{gateway.listeners.map((listener) => <Badge key={listener.name} tone="neutral">{gatewayProtocolLabel(listener.protocol)} · {listener.port} · {listener.hostname || '全部域名'}</Badge>)}</div></td>
                    <td className="p-3"><Badge tone={!gateway.enabled ? 'neutral' : gateway.state === 'Ready' ? 'success' : gateway.state === 'Error' ? 'error' : 'warning'}>{gateway.enabled ? gateway.state : 'Disabled'}</Badge></td>
                    <td className="p-3 text-slate-500">{formatDateTime(gateway.updatedAt || gateway.createdAt)}</td>
                    <td className="p-3 text-right" onClick={(event) => event.stopPropagation()}>{canWriteConfiguration ? <div className="inline-flex gap-1"><Button variant="ghost" size="sm" onClick={() => openEditor(gateway)}><Edit3 className="w-3.5 h-3.5" /></Button><Button variant="ghost" size="sm" onClick={() => setDeleteCandidate(gateway)}><Trash2 className="w-3.5 h-3.5 text-rose-600" /></Button></div> : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      {selected && policies.data ? (
        <Panel title={`网关“${selected.name}”的治理策略`}>
          <GovernancePolicyPanel targetKind="Gateway" targetID={selected.id} targetName={selected.name} workspace={policies.data} onChanged={policies.reload} />
        </Panel>
      ) : null}

      <Drawer title={draft.id ? `编辑网关：${draft.name}` : '创建网关'} subtitle="每个监听入口独立声明协议、端口和域名范围" isOpen={editorOpen} onClose={() => setEditorOpen(false)}>
        <div className="space-y-5">
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
        </div>
      </Drawer>

      <Modal title="删除网关" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}><div className="space-y-5 p-6"><p className="text-sm">确定删除网关“{deleteCandidate?.name}”吗？</p><div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div></div></Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function ListenerEditor({ listener, certificates, onChange, onRemove }: { listener: GatewayListener; certificates: Array<{ id: string; name: string }>; onChange: (listener: GatewayListener) => void; onRemove: () => void }) {
  const setProtocol = (protocol: GatewayProtocol) => onChange({
    ...listener,
    protocol,
    port: protocol === 'GATEWAY_PROTOCOL_HTTPS' ? 8443 : 8080,
    certificateID: protocol === 'GATEWAY_PROTOCOL_HTTPS' ? listener.certificateID : '',
  });
  return (
    <div className="space-y-3 rounded-xl border border-slate-200 bg-slate-50/60 p-4">
      <div className="grid grid-cols-[1fr_120px_100px] gap-3"><Field label="名称"><input className="input" value={listener.name} onChange={(event) => onChange({ ...listener, name: event.target.value })} /></Field><Field label="协议"><select className="select" value={listener.protocol} onChange={(event) => setProtocol(event.target.value as GatewayProtocol)}><option value="GATEWAY_PROTOCOL_HTTP">HTTP</option><option value="GATEWAY_PROTOCOL_HTTPS">HTTPS</option></select></Field><Field label="端口"><input className="input" type="number" value={listener.port} onChange={(event) => onChange({ ...listener, port: Number(event.target.value) })} /></Field></div>
      <Field label="域名（留空表示全部域名）"><div className="relative"><Globe className="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" /><input className="input pl-9 font-mono" placeholder="api.example.com 或 *.example.com" value={listener.hostname} onChange={(event) => onChange({ ...listener, hostname: event.target.value })} /></div></Field>
      {listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' ? certificates.length > 0 ? <Field label="TLS 证书"><select className="select" value={listener.certificateID} onChange={(event) => onChange({ ...listener, certificateID: event.target.value })}><option value="">选择证书</option>{certificates.map((certificate) => <option key={certificate.id} value={certificate.id}>{certificate.name}</option>)}</select></Field> : <p className="text-xs text-amber-700">请先<Link className="mx-1 font-semibold underline" to="/certificates">创建证书</Link>再配置 HTTPS</p> : null}
      <div className="flex justify-end"><Button variant="ghost" size="sm" onClick={onRemove}><Trash2 className="h-3.5 w-3.5 text-rose-600" />删除入口</Button></div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
