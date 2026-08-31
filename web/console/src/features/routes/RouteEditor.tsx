import type { ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { Badge, Button } from '@/components/ui';
import type { AIModel, HostRewriteMode, HttpMethod, RouteAccessMode, RoutePathMatchType, RouteWorkspace } from '@/domain/route';
import type { RouteDraft } from './form';

const methods: HttpMethod[] = ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

export function RouteEditor({ draft, workspace, busy, onChange, onCancel, onSave }: { draft: RouteDraft; workspace: RouteWorkspace; busy: boolean; onChange: (draft: RouteDraft) => void; onCancel: () => void; onSave: () => void }) {
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

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block space-y-1"><span className="text-xs font-medium text-slate-700">{label}</span>{children}</label>;
}

function replaceAt<T>(items: T[], index: number, value: T): T[] {
  return items.map((item, current) => current === index ? value : item);
}
