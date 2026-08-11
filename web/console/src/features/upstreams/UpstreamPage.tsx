import { useState, type ReactNode } from 'react';
import { Bot, Check, ChevronRight, Cloud, Database, Edit3, Plus, Server, Trash2 } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { deleteUpstream, listUpstreams, saveUpstream } from '@/api/upstreams';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Badge, Button, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, StatusDot, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Upstream } from '@/domain/upstream';
import { upstreamLoadBalancingLabel, upstreamLoadBalancingOptions } from '@/domain/upstream';
import { buildUpstreamPayload, createUpstreamDraft, validateUpstreamDraft, type UpstreamDraft } from './form';

export function UpstreamPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { canWriteConfiguration } = useAuth();
  const resource = useResource(listUpstreams);
  const [draft, setDraft] = useState<UpstreamDraft>(() => createUpstreamDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [activeKind, setActiveKind] = useState<'http' | 'llm'>(searchParams.get('type') === 'model' ? 'llm' : 'http');
  const [modelServiceOpen, setModelServiceOpen] = useState(false);
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
      subtitle="统一管理普通 HTTP 上游、模型厂商连接和实际模型"
      actions={canWriteConfiguration ? <Button onClick={() => activeKind === 'http' ? openEditor() : setModelServiceOpen(true)}><Plus className="w-4 h-4" />{activeKind === 'http' ? '创建 HTTP 服务' : '接入大模型服务'}</Button> : undefined}
    >
      <nav className="resource-kind-tabs"><button type="button" className={activeKind === 'http' ? 'is-active' : ''} onClick={() => { setActiveKind('http'); setSearchParams({ type: 'http' }); }}><Server />HTTP 服务<span>{resource.data.upstreams.length}</span></button><button type="button" className={activeKind === 'llm' ? 'is-active' : ''} onClick={() => { setActiveKind('llm'); setSearchParams({ type: 'model' }); }}><Bot />大模型服务<span>5</span></button></nav>
      {activeKind === 'http' ? <Panel>
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
      </Panel> : <ModelServicePrototype onOpen={() => setModelServiceOpen(true)} />}

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

      <Drawer title="接入模型服务" subtitle="在服务中保存厂商连接，并选择该服务实际提供的模型" isOpen={modelServiceOpen} onClose={() => setModelServiceOpen(false)}>
        <div className="model-service-form">
          <div className="wizard-heading"><h3>连接模型厂商</h3><p>厂商地址、凭据和实际模型都归属于这个服务；AI 路由直接选择这里的服务和模型。</p></div>
          <div className="provider-options"><button type="button" className="is-selected"><Cloud /><strong>通义千问</strong><span>阿里云百炼</span></button><button type="button"><Bot /><strong>Anthropic</strong><span>Claude API</span></button><button type="button"><Server /><strong>自定义服务</strong><span>OpenAI 兼容</span></button><button type="button"><Database /><strong>自托管模型</strong><span>内部推理服务</span></button></div>
          <label className="form-field"><span>服务名称</span><input className="input" defaultValue="通义千问生产" /></label>
          <label className="form-field"><span>服务地址</span><input className="input mono" defaultValue="https://dashscope.aliyuncs.com/compatible-mode" /></label>
          <label className="form-field"><span>API Key</span><input className="input mono" type="password" defaultValue="sk-example-token" /></label>
          <div className="connection-check"><span><Check /></span><div><strong>连接验证通过</strong><p>鉴权成功，已读取该服务可用的实际模型。</p></div><Button variant="ghost" size="sm">重新测试</Button></div>
          <div className="form-field"><span>该服务提供的实际模型</span><div className="discovered-models"><label className="is-selected"><input type="checkbox" defaultChecked /><span><Bot /></span><div><strong>qwen-max</strong><p>对话 · 视觉 · 工具调用 · 128K</p></div><Badge tone="success">已选择</Badge></label><label><input type="checkbox" /><span><Bot /></span><div><strong>qwen-plus</strong><p>对话 · 工具调用 · 128K</p></div></label><label><input type="checkbox" /><span><Database /></span><div><strong>text-embedding-v3</strong><p>文本向量 · 8K</p></div></label></div></div>
          <div className="drawer-actions"><Button variant="ghost" onClick={() => setModelServiceOpen(false)}>取消</Button><Button onClick={() => { setModelServiceOpen(false); setNotice({ message: '模型服务已保存到当前原型', tone: 'success' }); }}>保存模型服务</Button></div>
        </div>
      </Drawer>

      <Modal title="删除服务" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}><div className="p-6 space-y-5"><p className="text-sm">确定删除服务“{deleteCandidate?.name}”吗？</p><div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button></div></div></Modal>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function ModelServicePrototype({ onOpen }: { onOpen: () => void }) {
  const services = [
    { name: '通义千问生产', provider: '阿里云百炼', address: 'dashscope.aliyuncs.com', models: ['qwen-max', 'qwen-plus'], state: 'healthy' as const, latency: '620 ms' },
    { name: '通义千问灾备', provider: '百炼国际站', address: 'dashscope-intl.aliyuncs.com', models: ['qwen-max'], state: 'healthy' as const, latency: '1.3 s' },
    { name: 'Anthropic 公网', provider: 'Anthropic', address: 'api.anthropic.com', models: ['claude-sonnet-4'], state: 'warning' as const, latency: '2.8 s' },
    { name: 'Bedrock 灾备', provider: 'AWS Bedrock', address: 'bedrock-runtime.us-east-1.amazonaws.com', models: ['claude-sonnet-4'], state: 'healthy' as const, latency: '1.7 s' },
    { name: '内部向量服务', provider: '自托管', address: 'embedding.internal:8080', models: ['bge-m3'], state: 'healthy' as const, latency: '88 ms' },
  ];
  return <section className="model-service-list-card"><header><div><h3>大模型服务</h3><p>每个服务保存厂商连接、访问凭据和实际提供的模型，供 AI 路由选择</p></div><Button variant="outline" onClick={onOpen}>接入服务</Button></header><div className="model-service-head"><span>服务</span><span>地址</span><span>实际模型</span><span>运行状态</span><span>P95 延迟</span><span /></div>{services.map((service) => <button type="button" className="model-service-row" key={service.name}><span className="provider-mark"><Cloud /></span><div><strong>{service.name}</strong><small>{service.provider}</small></div><code>{service.address}</code><div className="model-tags">{service.models.map((model) => <code key={model}>{model}</code>)}</div><Badge tone={service.state === 'healthy' ? 'success' : 'warning'}><StatusDot status={service.state} />{service.state === 'healthy' ? '正常' : '延迟升高'}</Badge><strong>{service.latency}</strong><ChevronRight /></button>)}</section>;
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}
