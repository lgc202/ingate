import {
  Activity,
  AlertTriangle,
  ArrowRight,
  Bot,
  Braces,
  CheckCircle2,
  ChevronRight,
  CircleGauge,
  Clock3,
  Globe2,
  KeyRound,
  LockKeyhole,
  Network,
  Plus,
  RotateCw,
  Send,
  Server,
  ShieldCheck,
  Sparkles,
  Trash2,
  Wrench,
} from 'lucide-react';
import { useMemo, useState, type FormEvent } from 'react';
import {
  CopyButton,
  Drawer,
  EmptyState,
  FilterTabs,
  FormActions,
  Metric,
  PageHeader,
  PrimaryButton,
  SearchField,
  StatusBadge,
  Toast,
  Topology,
  TypeBadge,
  submitForm,
} from '../components/ui';
import type { Caller, Certificate, Gateway, GatewayListener, GatewayRoute, Policy, Service, ServiceEndpoint, ServiceType, TrafficType } from '../data';
import { usePrototype } from '../prototype-context';

export function GatewayPage() {
  const { gateways, routes, certificates, policies, addGateway } = usePrototype();
  const [selected, setSelected] = useState<Gateway | null>(null);
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState('');
  const [toast, setToast] = useState('');
  const visible = gateways.filter((gateway) => `${gateway.name}${gatewayDomains(gateway).join('')}${gateway.listeners.map(listenerLabel).join('')}`.toLowerCase().includes(query.toLowerCase()));

  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="流量管理" title="网关" description="监听入口、域名与证书" actions={<PrimaryButton onClick={() => setCreating(true)}><Plus />创建网关</PrimaryButton>} />
      <section className="metric-grid four"><Metric label="网关" value={String(gateways.length)} note={`${gateways.filter((gateway) => gateway.state === 'healthy').length} 个配置正常`} /><Metric label="代理实例" value="5 / 5" note="同一环境共享" tone="good" /><Metric label="监听入口" value={String(gateways.reduce((sum, gateway) => sum + gateway.listeners.length, 0))} note={`${gateways.flatMap((gateway) => gateway.listeners).filter((listener) => listener.protocol === 'HTTPS').length} 个 HTTPS`} /><Metric label="生效路由" value={String(routes.filter((route) => route.state !== 'disabled').length)} note="当前配置版本" /></section>
      <section className="card table-card">
        <header className="table-toolbar"><SearchField value={query} onChange={setQuery} placeholder="搜索网关、域名或监听入口" /><span>{visible.length} 个网关</span></header>
        <div className="table-head gateway-columns"><span>网关</span><span>监听入口</span><span>域名</span><span>证书</span><span>路由</span><span>状态</span><span /></div>
        {visible.length ? visible.map((gateway) => (
          <button key={gateway.id} className="table-row gateway-columns" type="button" onClick={() => setSelected(gateway)}>
            <div className="name-cell"><span><Network /></span><div><strong>{gateway.name}</strong><small>{gatewayDomains(gateway)[0]}</small></div></div>
            <span>{gateway.listeners.map(listenerLabel).join(' · ')}</span><span>{gatewayDomains(gateway).join('、')}</span><span>{gatewayCertificateNames(gateway, certificates).join('、') || '—'}</span><strong>{routes.filter((route) => route.gatewayID === gateway.id).length}</strong><StatusBadge state={gateway.state} /><ChevronRight />
          </button>
        )) : <EmptyState title="没有匹配的网关" description="请调整搜索条件。" />}
      </section>
      {selected ? <GatewayDetail gateway={selected} routes={routes.filter((route) => route.gatewayID === selected.id)} certificates={certificates} policies={policies} onClose={() => setSelected(null)} /> : null}
      {creating ? <CreateGateway certificates={certificates} onClose={() => setCreating(false)} onSave={(gateway) => { addGateway(gateway); setCreating(false); setToast('网关已加入演示环境'); }} /> : null}
      {toast ? <Toast message={toast} onDone={() => setToast('')} /> : null}
    </div>
  );
}

function GatewayDetail({ gateway, routes, certificates, policies, onClose }: { gateway: Gateway; routes: GatewayRoute[]; certificates: Certificate[]; policies: Policy[]; onClose: () => void }) {
  const appliedPolicies = policies.filter((policy) => policy.targets.some((target) => target.kind === '网关' && target.id === gateway.id));
  return (
    <Drawer title={gateway.name} description="流量入口与当前生效配置" onClose={onClose} width="wide">
      <div className="detail-hero"><span><Network /></span><div><StatusBadge state={gateway.state} /><h3>{gatewayDomains(gateway).join(' · ')}</h3><p>{gateway.listeners.length} 个监听入口 · 配置已发布到当前环境</p></div></div>
      <div className="detail-kpis"><Metric label="路由" value={String(routes.length)} note="全部已生效" /><Metric label="配置版本" value="142" note="今天 14:31" /><Metric label="可用率" value="99.99%" note="过去 24 小时" /></div>
      <section className="detail-section"><header><h3>监听入口</h3></header>{gateway.listeners.map((listener) => <div className="listener-detail" key={listener.id}><div className="detail-line"><span><Globe2 /></span><div><strong>{listenerLabel(listener)}</strong><small>{listener.bindings.length} 个域名绑定</small></div><StatusBadge state="healthy" /></div>{listener.bindings.map((binding) => <div className="listener-binding" key={binding.domain}><code>{binding.domain}</code><span>{binding.certificateID ? certificates.find((certificate) => certificate.id === binding.certificateID)?.name ?? '证书不存在' : '不使用 TLS'}</span></div>)}</div>)}</section>
      <section className="detail-section"><header><h3>绑定路由</h3></header>{routes.map((route) => <div className="detail-line" key={route.id}><TypeBadge type={route.type} /><div><strong>{route.name}</strong><small>{route.host}{route.path}</small></div><span>{route.targets[0]?.serviceName}</span></div>)}</section>
      <section className="detail-section"><header><h3>入口策略</h3></header>{appliedPolicies.length ? appliedPolicies.map((policy) => <div className="detail-line" key={policy.id}><span><ShieldCheck /></span><div><strong>{policy.name}</strong><small>{policy.rule}</small></div><StatusBadge state={policy.state} /></div>) : <EmptyState title="未应用入口策略" description="路由级策略仍会在对应路由上执行。" />}</section>
    </Drawer>
  );
}

function CreateGateway({ certificates, onClose, onSave }: { certificates: Certificate[]; onClose: () => void; onSave: (gateway: Gateway) => void }) {
  const [name, setName] = useState('新生产网关');
  const [listeners, setListeners] = useState<GatewayListener[]>([{ id: crypto.randomUUID(), protocol: 'HTTPS', port: 443, bindings: [{ domain: 'new-api.example.com', certificateID: '' }] }]);
  const updateListener = (listenerID: string, changes: Partial<GatewayListener>) => setListeners((items) => items.map((listener) => listener.id === listenerID ? { ...listener, ...changes } : listener));
  const updateBinding = (listenerID: string, index: number, domain: string, certificateID = '') => setListeners((items) => items.map((listener) => listener.id === listenerID ? { ...listener, bindings: listener.bindings.map((binding, bindingIndex) => bindingIndex === index ? { domain, certificateID: listener.protocol === 'HTTPS' ? certificateID : undefined } : binding) } : listener));
  const invalidTLS = listeners.some((listener) => listener.protocol === 'HTTPS' && listener.bindings.some((binding) => !binding.certificateID || !certificates.some((certificate) => certificate.id === binding.certificateID && certificate.usage === '服务器证书' && certificate.identities.some((identity) => certificateCoversDomain(identity, binding.domain)))));
  const duplicateSocket = new Set(listeners.map((listener) => listener.port)).size !== listeners.length;
  const duplicateDomain = listeners.some((listener) => new Set(listener.bindings.map((binding) => binding.domain.trim().toLowerCase())).size !== listener.bindings.length);
  const save = () => onSave({ id: `gw-${Date.now()}`, name, listeners, state: 'healthy' });
  return <Drawer title="创建网关" description="一个网关可以包含多个监听入口，每个 HTTPS 域名绑定自己的证书" onClose={onClose} width="wide"><form onSubmit={(event) => submitForm(event, save)}><div className="form-grid"><label className="field-wide"><span>网关名称</span><input required value={name} onChange={(event) => setName(event.target.value)} /></label></div><div className="listener-editor">{listeners.map((listener, listenerIndex) => <section className="form-section" key={listener.id}><header><span>{listenerIndex + 1}</span><div><strong>监听入口</strong><small>{listenerLabel(listener)}</small></div>{listeners.length > 1 ? <button type="button" onClick={() => setListeners((items) => items.filter((item) => item.id !== listener.id))}><Trash2 /></button> : null}</header><div className="form-grid"><label><span>协议</span><select value={listener.protocol} onChange={(event) => { const protocol = event.target.value as 'HTTP' | 'HTTPS'; updateListener(listener.id, { protocol, port: protocol === 'HTTPS' ? 443 : 80, bindings: listener.bindings.map((binding) => ({ domain: binding.domain, certificateID: protocol === 'HTTPS' ? '' : undefined })) }); }}><option>HTTPS</option><option>HTTP</option></select></label><label><span>端口</span><input required type="number" min="1" max="65535" value={listener.port} onChange={(event) => updateListener(listener.id, { port: Number(event.target.value) })} /></label></div><div className="binding-editor">{listener.bindings.map((binding, index) => { const matching = certificates.filter((certificate) => certificate.usage === '服务器证书' && certificate.state !== 'error' && certificate.identities.some((identity) => certificateCoversDomain(identity, binding.domain))); const effectiveCertificateID = matching.some((certificate) => certificate.id === binding.certificateID) ? binding.certificateID ?? '' : ''; return <div className="binding-row" key={index}><label><span>请求域名</span><input required value={binding.domain} onChange={(event) => updateBinding(listener.id, index, event.target.value)} /></label>{listener.protocol === 'HTTPS' ? <label><span>TLS 证书</span><select required value={effectiveCertificateID} onChange={(event) => updateBinding(listener.id, index, binding.domain, event.target.value)}><option value="">选择覆盖该域名的证书</option>{matching.map((certificate) => <option key={certificate.id} value={certificate.id}>{certificate.name}</option>)}</select>{binding.domain && !matching.length ? <small>没有可覆盖该域名的服务器证书</small> : null}</label> : null}<button type="button" aria-label="删除域名绑定" disabled={listener.bindings.length === 1} onClick={() => updateListener(listener.id, { bindings: listener.bindings.filter((_, bindingIndex) => bindingIndex !== index) })}><Trash2 /></button></div>; })}</div><button className="text-action" type="button" onClick={() => updateListener(listener.id, { bindings: [...listener.bindings, { domain: '', certificateID: listener.protocol === 'HTTPS' ? '' : undefined }] })}><Plus />添加域名</button></section>)}</div><button className="text-action" type="button" onClick={() => setListeners((items) => [...items, { id: crypto.randomUUID(), protocol: 'HTTP', port: 80, bindings: [{ domain: '', certificateID: undefined }] }])}><Plus />添加监听入口</button>{duplicateSocket ? <div className="form-note is-error"><AlertTriangle />同一网关内不能创建两个占用相同端口的监听入口；同一端口的多个域名应放在一个入口下。</div> : null}{duplicateDomain ? <div className="form-note is-error"><AlertTriangle />同一监听入口内不能重复绑定相同域名。</div> : null}<FormActions submitLabel="创建网关" submitDisabled={invalidTLS || duplicateSocket || duplicateDomain || listeners.some((listener) => listener.bindings.some((binding) => !binding.domain.trim()))} onCancel={onClose} /></form></Drawer>;
}

function listenerLabel(listener: GatewayListener) {
  return `${listener.protocol} · ${listener.port}`;
}

function gatewayDomains(gateway: Gateway) {
  return [...new Set(gateway.listeners.flatMap((listener) => listener.bindings.map((binding) => binding.domain)))];
}

function gatewayCertificateNames(gateway: Gateway, certificates: Certificate[]) {
  const ids = new Set(gateway.listeners.flatMap((listener) => listener.bindings.map((binding) => binding.certificateID).filter(Boolean)));
  return certificates.filter((certificate) => ids.has(certificate.id)).map((certificate) => certificate.name);
}

function certificateCoversDomain(certificateDomain: string, requestedDomain: string) {
  if (!requestedDomain) return false;
  if (certificateDomain === requestedDomain) return true;
  if (!certificateDomain.startsWith('*.')) return false;
  const suffix = certificateDomain.slice(1);
  return requestedDomain.endsWith(suffix) && requestedDomain.slice(0, -suffix.length).length > 0 && !requestedDomain.slice(0, -suffix.length).includes('.');
}

type RouteFilter = 'ALL' | TrafficType;

export function RoutePage() {
  const { routes, gateways, services, policies, callers, addRoute } = usePrototype();
  const [filter, setFilter] = useState<RouteFilter>('ALL');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<GatewayRoute | null>(null);
  const [creating, setCreating] = useState(false);
  const [debugging, setDebugging] = useState<GatewayRoute | null>(null);
  const [toast, setToast] = useState('');
  const visible = routes.filter((route) => (filter === 'ALL' || route.type === filter) && `${route.name}${route.host}${route.published.join('')}`.toLowerCase().includes(query.toLowerCase()));

  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="流量管理" title="路由" actions={<PrimaryButton onClick={() => setCreating(true)}><Plus />创建路由</PrimaryButton>} />
      <section className="card table-card">
        <header className="table-toolbar"><SearchField value={query} onChange={setQuery} placeholder="搜索路由、域名或发布能力" /><FilterTabs value={filter} onChange={setFilter} options={[{ value: 'ALL', label: '全部', count: routes.length }, { value: 'API', label: 'API', count: routes.filter((item) => item.type === 'API').length }, { value: 'AI', label: 'AI', count: routes.filter((item) => item.type === 'AI').length }, { value: 'MCP', label: 'MCP', count: routes.filter((item) => item.type === 'MCP').length }]} /></header>
        <div className="table-head route-columns"><span>路由</span><span>入口</span><span>匹配规则</span><span>目标服务</span><span>请求</span><span>成功率</span><span>性能</span><span>状态</span><span /></div>
        {visible.length ? visible.map((route) => <button key={route.id} className="table-row route-columns" type="button" onClick={() => setSelected(route)}><div className="name-cell"><TypeBadge type={route.type} /><div><strong>{route.name}</strong><small>{route.published.join('、')}</small></div></div><div><strong>{route.gatewayName}</strong><small>{route.host}{route.path}</small></div><span>{route.match}</span><span>{route.targets[0]?.serviceName}{route.targets.length > 1 ? ` 等 ${route.targets.length} 个` : ''}</span><strong>{route.requests}</strong><span>{route.successRate}</span><span>{route.latency}</span><StatusBadge state={route.state} /><ChevronRight /></button>) : <EmptyState title="没有匹配的路由" description="调整筛选条件，或创建一条新路由。" />}
      </section>
      {selected ? <RouteDetail route={selected} policies={policies} onClose={() => setSelected(null)} onDebug={() => { setDebugging(selected); setSelected(null); }} /> : null}
      {debugging ? <RouteDebugger route={debugging} callers={callers} onClose={() => setDebugging(null)} /> : null}
      {creating ? <CreateRoute gateways={gateways} services={services} onClose={() => setCreating(false)} onSave={(route) => { addRoute(route); setCreating(false); setToast('路由已加入演示环境'); }} /> : null}
      {toast ? <Toast message={toast} onDone={() => setToast('')} /> : null}
    </div>
  );
}

function RouteDetail({ route, policies, onClose, onDebug }: { route: GatewayRoute; policies: Policy[]; onClose: () => void; onDebug: () => void }) {
  const performanceLabel = route.type === 'AI' ? '首个 Token' : route.type === 'MCP' ? 'P95 耗时' : 'P95 延迟';
  const accessMode = route.accessMode ?? '需要调用方密钥';
  const appliedPolicies = policies.filter((policy) => policy.targets.some((target) => (target.kind === '路由' && target.id === route.id) || (target.kind === '网关' && target.id === route.gatewayID)));
  return (
    <Drawer title={route.name} description={`${route.host}${route.path}`} onClose={onClose} width="wide">
      <div className="drawer-actions"><TypeBadge type={route.type} /><StatusBadge state={route.state} /><PrimaryButton onClick={onDebug}><Send />在线调试</PrimaryButton></div>
      <Topology gateway={route.gatewayName} route={route.name} service={route.targets[0]?.serviceName ?? '未选择'} detail={route.targets[0]?.detail} />
      <div className="detail-kpis"><Metric label="今日请求" value={route.requests} note="过去 24 小时" /><Metric label="成功率" value={route.successRate} note="全部调用方" /><Metric label={performanceLabel} value={route.latency.replace('TTFT ', '').replace('P95 ', '')} note={route.type === 'AI' ? '流式响应' : '端到端'} /></div>
      <section className="detail-section"><header><h3>匹配与发布</h3></header><dl className="definition-list"><div><dt>访问方式</dt><dd>{accessMode}</dd></div><div><dt>匹配规则</dt><dd>{route.match}</dd></div><div><dt>{route.type === 'AI' ? '客户端模型名' : route.type === 'MCP' ? '开放工具' : '对外接口'}</dt><dd>{route.published.map((item) => <code key={item}>{item}</code>)}</dd></div></dl></section>
      <section className="detail-section"><header><h3>目标服务</h3><span>{route.forwarding.strategy}</span></header>{route.targets.map((target) => <div className="detail-line" key={`${target.serviceID}-${target.detail}-${target.role}`}><span><Server /></span><div><strong>{target.serviceName}</strong><small>{target.publishedCapability ? `${target.publishedCapability} → ${target.detail}` : target.detail}</small></div><span>{target.role}{target.weight ? ` · ${target.weight}%` : ''}</span><StatusBadge state="healthy" /></div>)}</section>
      <section className="detail-section"><header><h3>转发控制</h3></header><dl className="definition-list"><div><dt>请求超时</dt><dd>{route.forwarding.timeout}</dd></div><div><dt>失败重试</dt><dd>{route.forwarding.retries} 次</dd></div>{route.type === 'API' ? <><div><dt>路径处理</dt><dd>{route.forwarding.pathHandling}</dd></div><div><dt>主机头</dt><dd>{route.forwarding.hostRewrite}</dd></div></> : null}</dl></section>
      <section className="detail-section"><header><h3>已应用策略</h3></header>{appliedPolicies.length ? <div className="chip-list">{appliedPolicies.map((policy) => <span key={policy.id}><ShieldCheck />{policy.name}{policy.targets.some((target) => target.kind === '网关' && target.id === route.gatewayID) ? ' · 继承自网关' : ''}</span>)}</div> : <EmptyState title="未应用策略" description="当前路由仅执行访问控制与基础转发。" />}</section>
    </Drawer>
  );
}

interface ModelMappingDraft {
  id: string;
  published: string;
  primaryServiceID: string;
  primaryModel: string;
  backupEnabled: boolean;
  backupServiceID: string;
  backupModel: string;
}

function CreateRoute({ gateways, services, onClose, onSave }: { gateways: Gateway[]; services: Service[]; onClose: () => void; onSave: (route: GatewayRoute) => void }) {
  const [type, setType] = useState<TrafficType>('API');
  const [name, setName] = useState('新路由');
  const [gatewayID, setGatewayID] = useState(gateways[0]?.id ?? '');
  const [host, setHost] = useState(gateways[0] ? gatewayDomains(gateways[0])[0] ?? '' : '');
  const [path, setPath] = useState('/new');
  const [method, setMethod] = useState('ANY');
  const [matchMode, setMatchMode] = useState<'精确匹配' | '前缀匹配'>('前缀匹配');
  const [accessMode, setAccessMode] = useState<'需要调用方密钥' | '公开访问'>('需要调用方密钥');
  const [serviceID, setServiceID] = useState('');
  const [selectedTools, setSelectedTools] = useState<string[]>([]);
  const [secondLineEnabled, setSecondLineEnabled] = useState(false);
  const [secondServiceID, setSecondServiceID] = useState('');
  const [strategy, setStrategy] = useState<'主备切换' | '权重分流'>('主备切换');
  const [primaryWeight, setPrimaryWeight] = useState(80);
  const [modelMappings, setModelMappings] = useState<ModelMappingDraft[]>([newModelMapping(services)]);
  const [timeout, setTimeout] = useState('30');
  const [retries, setRetries] = useState('2');
  const [pathHandling, setPathHandling] = useState('保持原路径');
  const [hostRewrite, setHostRewrite] = useState('使用服务地址');
  const gateway = gateways.find((item) => item.id === gatewayID) ?? gateways[0];
  const effectiveHost = gateway && gatewayDomains(gateway).includes(host) ? host : gateway ? gatewayDomains(gateway)[0] ?? '' : '';
  const compatible = services.filter((service) => service.type === (type === 'API' ? 'HTTP' : 'MCP'));
  const effectiveServiceID = compatible.some((service) => service.id === serviceID) ? serviceID : compatible[0]?.id ?? '';
  const primaryService = compatible.find((service) => service.id === effectiveServiceID);
  const effectiveTools = selectedTools.filter((tool) => primaryService?.capabilities.includes(tool));
  const secondCandidates = compatible.filter((service) => service.id !== effectiveServiceID);
  const effectiveSecondServiceID = secondCandidates.some((service) => service.id === secondServiceID) ? secondServiceID : secondCandidates[0]?.id ?? '';
  const secondService = secondCandidates.find((service) => service.id === effectiveSecondServiceID);
  const normalizedMappings = modelMappings.map((mapping) => normalizeModelMapping(mapping, services));
  const duplicateModelNames = new Set(normalizedMappings.map((mapping) => mapping.published.trim())).size !== normalizedMappings.length;
  const modelMappingsComplete = normalizedMappings.length > 0 && normalizedMappings.every((mapping) => mapping.published.trim() && mapping.primaryServiceID && mapping.primaryModel && (!mapping.backupEnabled || (mapping.backupServiceID && mapping.backupModel)));
  const routeReady = Boolean(gateway) && (type === 'AI' ? modelMappingsComplete && !duplicateModelNames : Boolean(primaryService) && (type !== 'MCP' || effectiveTools.length > 0));

  const changeType = (next: TrafficType) => {
    setType(next); setServiceID(''); setSelectedTools([]); setSecondLineEnabled(false); setSecondServiceID('');
    setPath(next === 'API' ? '/new' : next === 'AI' ? '/v1' : '/mcp');
    setTimeout(next === 'AI' ? '120' : '30'); setRetries(next === 'API' ? '2' : '1');
    if (next === 'AI') setModelMappings([newModelMapping(services)]);
  };
  const updateMapping = (id: string, changes: Partial<ModelMappingDraft>) => setModelMappings((items) => items.map((item) => item.id === id ? { ...item, ...changes } : item));
  const save = () => {
    if (!routeReady || !gateway) return;
    const apiCapability = `${method} ${path}${matchMode === '前缀匹配' ? '/*' : ''}`;
    const published = type === 'API' ? [apiCapability] : type === 'MCP' ? effectiveTools : normalizedMappings.map((mapping) => mapping.published.trim());
    const targets: GatewayRoute['targets'] = [];
    if (type === 'AI') {
      normalizedMappings.forEach((mapping) => {
        const primary = services.find((service) => service.id === mapping.primaryServiceID)!;
        targets.push({ serviceID: primary.id, serviceName: primary.name, publishedCapability: mapping.published.trim(), detail: mapping.primaryModel, role: '主线路' });
        if (mapping.backupEnabled) {
          const backup = services.find((service) => service.id === mapping.backupServiceID)!;
          targets.push({ serviceID: backup.id, serviceName: backup.name, publishedCapability: mapping.published.trim(), detail: mapping.backupModel, role: '备用线路' });
        }
      });
    } else if (primaryService) {
      targets.push({ serviceID: primaryService.id, serviceName: primaryService.name, publishedCapability: published.join('、'), detail: type === 'MCP' ? `${effectiveTools.length} 个开放工具` : `${primaryService.endpoints.length} 个端点`, role: secondLineEnabled && strategy === '权重分流' ? '加权线路' : '主线路', weight: secondLineEnabled && strategy === '权重分流' ? primaryWeight : undefined });
      if (secondLineEnabled && secondService) targets.push({ serviceID: secondService.id, serviceName: secondService.name, publishedCapability: published.join('、'), detail: `${secondService.endpoints.length} 个端点`, role: strategy === '权重分流' ? '加权线路' : '备用线路', weight: strategy === '权重分流' ? 100 - primaryWeight : undefined });
    }
    const forwardingStrategy = type === 'AI' ? normalizedMappings.some((mapping) => mapping.backupEnabled) ? '主备切换' : '单线路' : secondLineEnabled ? strategy : '单线路';
    onSave({ id: `route-${Date.now()}`, name, type, gatewayID: gateway.id, gatewayName: gateway.name, host: effectiveHost, path, accessMode, match: type === 'API' ? apiCapability : type === 'AI' ? 'OpenAI API · 请求体 model' : 'Streamable HTTP · tools/call', published, targets, forwarding: { strategy: forwardingStrategy, timeout: `${timeout} 秒`, retries: Number(retries), pathHandling, hostRewrite }, requests: '0', successRate: '—', latency: '—', state: 'healthy' });
  };

  return (
    <Drawer title="创建路由" description="定义外部请求如何匹配并转发到服务" onClose={onClose} width="wide">
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="type-selector">{(['API', 'AI', 'MCP'] as const).map((item) => <button key={item} type="button" className={type === item ? 'is-selected' : ''} onClick={() => changeType(item)}><TypeBadge type={item} /><strong>{item === 'API' ? 'HTTP API' : item === 'AI' ? '模型 API' : 'MCP 工具'}</strong><small>{item === 'API' ? '一个方法与路径匹配' : item === 'AI' ? '发布一个或多个客户端模型名' : '发布一个服务中的多个工具'}</small></button>)}</div>
        <section className="form-section">
          <header><span>1</span><div><strong>请求入口</strong><small>请求只会命中符合域名和路径的一条路由</small></div></header>
          <div className="form-grid">
            <label><span>路由名称</span><input required value={name} onChange={(event) => setName(event.target.value)} /></label>
            <label><span>生效网关</span><select value={gatewayID} onChange={(event) => { setGatewayID(event.target.value); setHost(''); }} required>{gateways.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
            <label><span>请求域名</span><select value={effectiveHost} onChange={(event) => setHost(event.target.value)} required>{gateway ? gatewayDomains(gateway).map((domain) => <option key={domain}>{domain}</option>) : null}</select></label>
            <label><span>请求路径</span><input required value={path} onChange={(event) => setPath(event.target.value)} /></label>
            <label className="field-wide"><span>访问方式</span><select value={accessMode} onChange={(event) => setAccessMode(event.target.value as '需要调用方密钥' | '公开访问')}><option>需要调用方密钥</option><option>公开访问</option></select><small>{accessMode === '需要调用方密钥' ? '校验密钥及该调用方在本路由下的接口、模型或工具权限' : '不识别调用方，调用方权限与用量上限不适用'}</small></label>
            {type === 'API' ? <><label><span>请求方法</span><select value={method} onChange={(event) => setMethod(event.target.value)}><option value="ANY">全部方法</option><option>GET</option><option>POST</option><option>PUT</option><option>PATCH</option><option>DELETE</option></select></label><label><span>路径方式</span><select value={matchMode} onChange={(event) => setMatchMode(event.target.value as '精确匹配' | '前缀匹配')}><option>精确匹配</option><option>前缀匹配</option></select></label></> : null}
          </div>
        </section>
        <section className="form-section">
          <header><span>2</span><div><strong>{type === 'AI' ? '模型映射' : '目标服务'}</strong><small>{type === 'AI' ? '每个客户端模型名拥有独立的主线路和可选备用线路' : type === 'MCP' ? '开放已从 MCP 服务发现的工具' : '配置主线路及可选的第二线路'}</small></div></header>
          {type === 'AI' ? <div className="model-mapping-editor">{normalizedMappings.map((mapping, index) => {
            const primary = services.find((service) => service.id === mapping.primaryServiceID);
            const backupCandidates = services.filter((service) => service.type === 'MODEL' && service.id !== mapping.primaryServiceID);
            const backup = backupCandidates.find((service) => service.id === mapping.backupServiceID);
            return <article key={mapping.id}><header><strong>模型映射 {index + 1}</strong>{normalizedMappings.length > 1 ? <button type="button" onClick={() => setModelMappings((items) => items.filter((item) => item.id !== mapping.id))}><Trash2 />移除</button> : null}</header><div className="form-grid"><label><span>客户端模型名</span><input required value={mapping.published} onChange={(event) => updateMapping(mapping.id, { published: event.target.value })} placeholder="例如 reasoning-pro" /></label><label><span>主线路服务</span><select value={mapping.primaryServiceID} onChange={(event) => updateMapping(mapping.id, { primaryServiceID: event.target.value, primaryModel: '', backupServiceID: '', backupModel: '' })}>{services.filter((service) => service.type === 'MODEL').map((service) => <option key={service.id} value={service.id}>{service.name}</option>)}</select></label><label className="field-wide"><span>主线路真实模型</span><select value={mapping.primaryModel} onChange={(event) => updateMapping(mapping.id, { primaryModel: event.target.value })}>{primary?.capabilities.map((model) => <option key={model}>{model}</option>)}</select></label></div><button className="text-action" type="button" onClick={() => updateMapping(mapping.id, { backupEnabled: !mapping.backupEnabled, backupServiceID: '', backupModel: '' })}>{mapping.backupEnabled ? '移除备用线路' : '+ 添加备用线路'}</button>{mapping.backupEnabled ? <div className="form-grid"><label><span>备用线路服务</span><select value={mapping.backupServiceID} onChange={(event) => updateMapping(mapping.id, { backupServiceID: event.target.value, backupModel: '' })}><option value="">选择备用服务</option>{backupCandidates.map((service) => <option key={service.id} value={service.id}>{service.name}</option>)}</select></label><label><span>备用真实模型</span><select value={mapping.backupModel} onChange={(event) => updateMapping(mapping.id, { backupModel: event.target.value })}><option value="">选择真实模型</option>{backup?.capabilities.map((model) => <option key={model}>{model}</option>)}</select></label></div> : null}</article>;
          })}<button className="text-action" type="button" onClick={() => setModelMappings((items) => [...items, newModelMapping(services)])}><Plus />添加模型映射</button>{duplicateModelNames ? <div className="form-note is-error"><AlertTriangle />客户端模型名不能重复</div> : null}</div> : <><div className="form-grid"><label className="field-wide"><span>{type === 'API' ? 'HTTP 服务' : 'MCP 服务'}</span><select value={effectiveServiceID} onChange={(event) => { setServiceID(event.target.value); setSelectedTools([]); setSecondServiceID(''); }} required>{compatible.map((service) => <option key={service.id} value={service.id}>{service.name} · {service.type === 'HTTP' ? `${service.endpoints.length} 个端点` : service.provider}</option>)}</select></label>{type === 'MCP' ? <fieldset className="field-wide tool-picker"><legend>开放工具</legend>{primaryService?.capabilities.map((tool) => <label key={tool}><input type="checkbox" checked={effectiveTools.includes(tool)} onChange={() => setSelectedTools((items) => items.includes(tool) ? items.filter((item) => item !== tool) : [...items, tool])} /><span><code>{tool}</code></span></label>)}</fieldset> : null}</div>{type === 'API' && secondCandidates.length ? <div className="optional-target"><button type="button" onClick={() => setSecondLineEnabled((value) => !value)}>{secondLineEnabled ? '移除第二线路' : '+ 添加第二线路'}</button>{secondLineEnabled ? <div className="form-grid"><label><span>第二线路服务</span><select value={effectiveSecondServiceID} onChange={(event) => setSecondServiceID(event.target.value)}>{secondCandidates.map((service) => <option key={service.id} value={service.id}>{service.name}</option>)}</select></label><label><span>转发方式</span><select value={strategy} onChange={(event) => setStrategy(event.target.value as '主备切换' | '权重分流')}><option>主备切换</option><option>权重分流</option></select></label>{strategy === '权重分流' ? <label><span>主线路权重（%）</span><input type="number" min="1" max="99" value={primaryWeight} onChange={(event) => setPrimaryWeight(Number(event.target.value))} /><small>第二线路权重为 {100 - primaryWeight}%</small></label> : null}</div> : null}</div> : null}</>}
        </section>
        <details className="advanced-config"><summary>高级转发设置</summary><div className="form-grid"><label><span>请求超时（秒）</span><input type="number" min="1" value={timeout} onChange={(event) => setTimeout(event.target.value)} /></label><label><span>失败重试次数</span><select value={retries} onChange={(event) => setRetries(event.target.value)}><option value="0">不重试</option><option value="1">1 次</option><option value="2">2 次</option><option value="3">3 次</option></select></label>{type === 'API' ? <><label><span>路径处理</span><select value={pathHandling} onChange={(event) => setPathHandling(event.target.value)}><option>保持原路径</option><option>移除匹配前缀</option></select></label><label><span>主机头</span><select value={hostRewrite} onChange={(event) => setHostRewrite(event.target.value)}><option>使用服务地址</option><option>保持请求主机</option></select></label></> : null}</div></details>
        <FormActions submitLabel="创建路由" submitDisabled={!routeReady} onCancel={onClose} />
      </form>
    </Drawer>
  );
}

function newModelMapping(services: Service[]): ModelMappingDraft {
  const primary = services.find((service) => service.type === 'MODEL');
  return { id: crypto.randomUUID(), published: '', primaryServiceID: primary?.id ?? '', primaryModel: primary?.capabilities[0] ?? '', backupEnabled: false, backupServiceID: '', backupModel: '' };
}

function normalizeModelMapping(mapping: ModelMappingDraft, services: Service[]) {
  const modelServices = services.filter((service) => service.type === 'MODEL');
  const primary = modelServices.find((service) => service.id === mapping.primaryServiceID) ?? modelServices[0];
  const primaryModel = primary?.capabilities.includes(mapping.primaryModel) ? mapping.primaryModel : primary?.capabilities[0] ?? '';
  const backupCandidates = modelServices.filter((service) => service.id !== primary?.id);
  const backup = backupCandidates.find((service) => service.id === mapping.backupServiceID);
  const backupModel = backup?.capabilities.includes(mapping.backupModel) ? mapping.backupModel : backup?.capabilities[0] ?? '';
  return { ...mapping, primaryServiceID: primary?.id ?? '', primaryModel, backupServiceID: mapping.backupEnabled ? backup?.id ?? '' : '', backupModel: mapping.backupEnabled ? backupModel : '' };
}

function RouteDebugger({ route, callers, onClose }: { route: GatewayRoute; callers: Caller[]; onClose: () => void }) {
  const [sent, setSent] = useState(false);
  const authenticated = (route.accessMode ?? '需要调用方密钥') === '需要调用方密钥';
  const eligibleCallers = callers.filter((caller) => caller.permissions.some((permission) => permission.routeID === route.id) && caller.keys.some((key) => key.state !== 'disabled'));
  const [callerID, setCallerID] = useState(eligibleCallers[0]?.id ?? '');
  const caller = eligibleCallers.find((item) => item.id === callerID) ?? eligibleCallers[0];
  const grantedScopes = authenticated ? caller?.permissions.find((permission) => permission.routeID === route.id)?.scopes ?? [] : route.published;
  const availableCapabilities = route.published.filter((capability) => grantedScopes.includes(capability));
  const [capability, setCapability] = useState(availableCapabilities[0] ?? '');
  const effectiveCapability = availableCapabilities.includes(capability) ? capability : availableCapabilities[0] ?? '';
  const endpoint = `https://${route.host}${route.path}`;
  const policyResult = route.type === 'API'
    ? authenticated ? '路由权限、请求限流和 IP 限制通过' : '请求限流和 IP 限制通过'
    : route.type === 'AI'
      ? authenticated ? '模型权限、用量上限和参数约束通过' : '参数约束通过'
      : authenticated ? '工具权限和调用限流通过' : '调用限流通过';
  const target = route.targets.find((item) => item.publishedCapability?.split('、').includes(effectiveCapability) && item.role !== '备用线路') ?? route.targets[0];
  const steps = [authenticated ? '调用方认证' : '公开访问', '访问策略', route.name, target?.serviceName];

  return <Drawer title={`调试 · ${route.name}`} description="按真实授权范围构造演示请求" onClose={onClose} width="wide"><div className="debug-layout"><section className="debug-console"><div className="debug-context"><TypeBadge type={route.type} /><span>{authenticated ? caller ? `调用方：${caller.name}` : '没有可用的调用方身份' : '公开访问'}</span><StatusBadge state="healthy" label="配置已生效" /></div>{authenticated ? <label><span>调用方</span><select value={caller?.id ?? ''} onChange={(event) => { setCallerID(event.target.value); setCapability(''); setSent(false); }} disabled={!eligibleCallers.length}><option value="">选择具有本路由权限和有效密钥的调用方</option>{eligibleCallers.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label> : null}{route.published.length > 1 ? <label><span>{route.type === 'AI' ? '客户端模型名' : route.type === 'MCP' ? '工具' : '接口'}</span><select value={effectiveCapability} onChange={(event) => { setCapability(event.target.value); setSent(false); }}>{availableCapabilities.map((item) => <option key={item}>{item}</option>)}</select></label> : null}<label><span>请求地址</span><div className="endpoint-input"><code>{endpoint}</code><CopyButton value={endpoint} /></div></label><label><span>{route.type === 'API' ? '请求内容' : route.type === 'AI' ? '用户消息' : '工具参数'}</span><textarea defaultValue={route.type === 'API' ? '{\n  "query": "78421"\n}' : route.type === 'AI' ? '请查询订单 78421 的当前状态。' : '{\n  "query": "AI 网关运行状态"\n}'} /></label>{authenticated && !caller ? <div className="form-note is-error"><AlertTriangle />请先为调用方授予本路由权限并签发有效密钥。</div> : null}<PrimaryButton disabled={authenticated && !caller} onClick={() => setSent(true)}><Send />发送演示请求</PrimaryButton>{sent ? <div className="debug-response"><CheckCircle2 /><div><strong>200 · 演示响应成功</strong><p>{target?.serviceName} 已完成请求，执行记录已写入请求日志。</p></div></div> : null}</section><section className="execution-panel"><span className="eyebrow">执行过程</span>{steps.map((step, index) => <div className="execution-step" key={`${step}-${index}`}><i>{index + 1}</i><div><strong>{step}</strong><span>{index === 0 ? authenticated ? `${caller?.name} · ${caller?.keys.find((key) => key.state !== 'disabled')?.name}` : '未执行身份认证' : index === 1 ? policyResult : index === 2 ? `${effectiveCapability || route.published[0]} → ${target?.serviceName}` : `演示响应 · ${route.latency}`}</span></div><small>{index + 1} ms</small></div>)}</section></div></Drawer>;
}

type ServiceFilter = 'ALL' | ServiceType;

export function ServicePage() {
  const { services, routes, certificates, addService, updateServiceCredential } = usePrototype();
  const [filter, setFilter] = useState<ServiceFilter>('ALL');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<Service | null>(null);
  const [creating, setCreating] = useState(false);
  const [rotatingCredential, setRotatingCredential] = useState<Service | null>(null);
  const [toast, setToast] = useState('');
  const visible = services.filter((service) => (filter === 'ALL' || service.type === filter) && `${service.name}${service.provider}${service.endpoints.map((item) => item.address).join('')}${service.capabilities.join('')}`.toLowerCase().includes(query.toLowerCase()));
  const healthyHTTPEndpoints = services.filter((item) => item.type === 'HTTP').flatMap((item) => item.endpoints).filter((item) => item.state === 'healthy').length;
  const modelCount = new Set(services.filter((item) => item.type === 'MODEL').flatMap((item) => item.capabilities)).size;
  const toolCount = services.filter((item) => item.type === 'MCP').reduce((sum, item) => sum + item.capabilities.length, 0);
  const warnings = services.filter((item) => item.state === 'warning').length;
  return <div className="page-stack page-enter"><PageHeader eyebrow="流量管理" title="服务" description="HTTP、模型与 MCP 服务连接" actions={<PrimaryButton onClick={() => setCreating(true)}><Plus />创建服务</PrimaryButton>} /><section className="metric-grid four"><Metric label="HTTP 服务" value={String(services.filter((item) => item.type === 'HTTP').length)} note={`${healthyHTTPEndpoints} 个健康端点`} /><Metric label="模型服务" value={String(services.filter((item) => item.type === 'MODEL').length)} note={`${modelCount} 个实际模型`} /><Metric label="MCP 服务" value={String(services.filter((item) => item.type === 'MCP').length)} note={`${toolCount} 个已发现工具`} /><Metric label="运行告警" value={String(warnings)} note={warnings ? '服务或端点异常' : '全部服务正常'} tone={warnings ? 'warning' : 'good'} /></section><section className="card table-card"><header className="table-toolbar"><SearchField value={query} onChange={setQuery} placeholder="搜索服务、地址或能力" /><FilterTabs value={filter} onChange={setFilter} options={[{ value: 'ALL', label: '全部', count: services.length }, { value: 'HTTP', label: 'HTTP', count: services.filter((item) => item.type === 'HTTP').length }, { value: 'MODEL', label: '模型', count: services.filter((item) => item.type === 'MODEL').length }, { value: 'MCP', label: 'MCP', count: services.filter((item) => item.type === 'MCP').length }]} /></header><div className="table-head service-columns"><span>服务</span><span>协议与地址</span><span>端点 / 模型 / 工具</span><span>引用路由</span><span>成功率</span><span>关键性能</span><span>状态</span><span /></div>{visible.length ? visible.map((service) => <button key={service.id} className="table-row service-columns" type="button" onClick={() => setSelected(service)}><div className="name-cell"><TypeBadge type={service.type} /><div><strong>{service.name}</strong><small>{service.provider}</small></div></div><div><strong>{service.protocol}</strong><small>{service.endpoints[0]?.address}{service.endpoints.length > 1 ? ` 等 ${service.endpoints.length} 个` : ''}</small></div><div className="tag-cell">{(service.type === 'HTTP' ? service.endpoints.map((item) => item.address) : service.capabilities).slice(0, 3).map((item) => <code key={item}>{item}</code>)}</div><strong>{routes.filter((route) => route.targets.some((target) => target.serviceID === service.id)).length} 条</strong><span>{service.successRate}</span><span>{service.latency}</span><StatusBadge state={service.state} /><ChevronRight /></button>) : <EmptyState title="没有匹配的服务" description="请调整搜索或筛选条件。" />}</section>{selected ? <ServiceDetail service={services.find((service) => service.id === selected.id) ?? selected} routes={routes.filter((route) => route.targets.some((target) => target.serviceID === selected.id))} certificates={certificates} onRotateCredential={() => { setRotatingCredential(services.find((service) => service.id === selected.id) ?? selected); setSelected(null); }} onClose={() => setSelected(null)} /> : null}{creating ? <CreateService certificates={certificates} onClose={() => setCreating(false)} onSave={(service) => { addService(service); setCreating(false); setToast('服务已创建'); }} /> : null}{rotatingCredential ? <RotateServiceCredential service={rotatingCredential} certificates={certificates} onClose={() => setRotatingCredential(null)} onSave={(clientCertificateID) => { updateServiceCredential(rotatingCredential.id, clientCertificateID); setRotatingCredential(null); setToast('服务凭据已更新'); }} /> : null}{toast ? <Toast message={toast} onDone={() => setToast('')} /> : null}</div>;
}

function ServiceDetail({ service, routes, certificates, onRotateCredential, onClose }: { service: Service; routes: GatewayRoute[]; certificates: Certificate[]; onRotateCredential: () => void; onClose: () => void }) {
  const performanceLabel = service.type === 'MODEL' ? '首个 Token' : 'P95 耗时';
  const [checked, setChecked] = useState(false);
  const clientCertificate = certificates.find((certificate) => certificate.id === service.clientCertificateID);
  const trustCertificate = certificates.find((certificate) => certificate.id === service.trustCertificateID);
  return <Drawer title={service.name} description={`${service.protocol} · ${service.provider}`} onClose={onClose} width="wide"><div className="drawer-actions"><TypeBadge type={service.type} /><StatusBadge state={service.state} />{service.authentication !== '无认证' ? <button className="button button-secondary" type="button" onClick={onRotateCredential}><KeyRound />更新凭据</button> : null}<button className="button button-secondary" type="button" onClick={() => setChecked(true)}>{checked ? <CheckCircle2 /> : <RotateCw />}{checked ? '检测通过' : '重新检测'}</button></div><div className="detail-hero"><span>{service.type === 'HTTP' ? <Braces /> : service.type === 'MODEL' ? <Bot /> : <Wrench />}</span><div><StatusBadge state={service.state} /><h3>{service.endpoints[0]?.address}</h3><p>{service.authentication} · {service.healthCheck}</p></div></div><div className="detail-kpis"><Metric label="成功率" value={service.successRate} note="过去 24 小时" /><Metric label={performanceLabel} value={service.latency} note="过去 24 小时" /><Metric label="引用路由" value={String(routes.length)} note="当前环境" /></div><section className="detail-section"><header><h3>服务端点</h3><span>{service.endpoints.length > 1 ? service.loadBalancing : '单端点'}</span></header>{service.endpoints.map((endpoint) => <div className="detail-line" key={endpoint.address}><span><Server /></span><div><strong>{endpoint.address}</strong><small>{service.endpoints.length > 1 ? `权重 ${endpoint.weight}%` : service.protocol}</small></div><StatusBadge state={endpoint.state} label={endpoint.state === 'healthy' ? '可用' : '异常'} /></div>)}<dl className="definition-list"><div><dt>认证方式</dt><dd>{service.authentication}</dd></div>{service.authentication !== '无认证' ? <div><dt>凭据更新</dt><dd>{service.credentialUpdatedAt ?? '创建服务时'}</dd></div> : null}{clientCertificate ? <div><dt>客户端证书</dt><dd>{clientCertificate.name}</dd></div> : null}<div><dt>连接加密</dt><dd>{service.transportSecurity}</dd></div>{service.transportSecurity === 'TLS' ? <><div><dt>服务端名称</dt><dd>{service.serverName || '从端点地址推导'}</dd></div><div><dt>证书信任</dt><dd>{trustCertificate?.name ?? '系统信任'}</dd></div></> : null}<div><dt>健康检查</dt><dd>{service.healthCheck}</dd></div>{service.endpoints.length > 1 ? <div><dt>负载方式</dt><dd>{service.loadBalancing}</dd></div> : null}</dl></section>{service.type !== 'HTTP' ? <section className="detail-section"><header><h3>{service.type === 'MODEL' ? '实际模型' : '已发现工具'}</h3><span>{service.capabilities.length} 项</span></header>{service.capabilities.map((capability) => <div className="detail-line" key={capability}><span>{service.type === 'MODEL' ? <Sparkles /> : <Wrench />}</span><div><strong>{capability}</strong><small>{service.type === 'MODEL' ? '可映射为客户端模型名' : '可在路由中选择对外开放'}</small></div><StatusBadge state="healthy" label="可用" /></div>)}</section> : null}<section className="detail-section"><header><h3>引用路由</h3></header>{routes.length ? routes.flatMap((route) => route.targets.filter((target) => target.serviceID === service.id).map((target) => <div className="detail-line" key={`${route.id}-${target.publishedCapability}-${target.role}`}><TypeBadge type={route.type} /><div><strong>{route.name}</strong><small>{target.publishedCapability ? `${target.publishedCapability} · ` : ''}{route.host}{route.path}</small></div><span>{target.role}</span></div>)) : <EmptyState title="尚未被路由使用" description="可以在创建路由时选择该服务。" />}</section></Drawer>;
}

function RotateServiceCredential({ service, certificates, onClose, onSave }: { service: Service; certificates: Certificate[]; onClose: () => void; onSave: (clientCertificateID?: string) => void }) {
  const isMTLS = service.authentication.startsWith('mTLS');
  const isAWS = service.authentication === 'AWS 签名';
  const isBasic = service.authentication === 'Basic';
  const [identity, setIdentity] = useState('');
  const [secret, setSecret] = useState('');
  const [region, setRegion] = useState('');
  const [clientCertificateID, setClientCertificateID] = useState(service.clientCertificateID ?? '');
  const [tested, setTested] = useState(false);
  const clientCertificates = certificates.filter((certificate) => certificate.usage === '客户端证书' && certificate.state !== 'error');
  const complete = isMTLS ? Boolean(clientCertificateID) : isAWS ? Boolean(identity && secret && region) : isBasic ? Boolean(identity && secret) : Boolean(secret);
  return <Drawer title="更新服务凭据" description={`${service.name} · ${service.authentication}`} onClose={onClose}><form onSubmit={(event) => submitForm(event, () => onSave(isMTLS ? clientCertificateID : undefined))}><div className="form-grid">{isMTLS ? <label className="field-wide"><span>客户端证书</span><select required value={clientCertificateID} onChange={(event) => { setClientCertificateID(event.target.value); setTested(false); }}><option value="">选择客户端证书</option>{clientCertificates.map((certificate) => <option key={certificate.id} value={certificate.id}>{certificate.name}</option>)}</select></label> : <>{isAWS || isBasic ? <label><span>{isAWS ? 'Access Key ID' : '用户名'}</span><input required value={identity} onChange={(event) => { setIdentity(event.target.value); setTested(false); }} /></label> : null}<label className={isAWS || isBasic ? '' : 'field-wide'}><span>新凭据</span><input required type="password" value={secret} onChange={(event) => { setSecret(event.target.value); setTested(false); }} placeholder="保存后不再显示明文" /></label>{isAWS ? <label className="field-wide"><span>AWS 区域</span><input required value={region} onChange={(event) => { setRegion(event.target.value); setTested(false); }} /></label> : null}</>}</div><button className={`connection-test ${tested ? 'is-success' : ''}`} type="button" disabled={!complete} onClick={() => setTested(true)}>{tested ? <CheckCircle2 /> : <CircleGauge />}<div><strong>{tested ? '新凭据验证通过' : '验证新凭据'}</strong><span>旧凭据会继续使用，直到新凭据验证通过并保存</span></div></button><div className="form-note"><KeyRound />保存后立即切换到新凭据；敏感值不会再次显示，也不会进入操作日志。</div><FormActions submitLabel="保存并切换" submitDisabled={!complete || !tested} onCancel={onClose} /></form></Drawer>;
}

function CreateService({ certificates, onClose, onSave }: { certificates: Certificate[]; onClose: () => void; onSave: (service: Service) => void }) {
  const [type, setType] = useState<ServiceType>('HTTP');
  const [name, setName] = useState('新服务');
  const [provider, setProvider] = useState('内部服务');
  const [protocol, setProtocol] = useState('HTTP/1.1');
  const [authentication, setAuthentication] = useState('无认证');
  const [credential, setCredential] = useState('');
  const [credentialName, setCredentialName] = useState('');
  const [credentialRegion, setCredentialRegion] = useState('');
  const [clientCertificateID, setClientCertificateID] = useState('');
  const [transportSecurity, setTransportSecurity] = useState<Service['transportSecurity']>('明文连接');
  const [serverName, setServerName] = useState('');
  const [trustMode, setTrustMode] = useState<'系统信任' | '指定信任证书'>('系统信任');
  const [trustCertificateID, setTrustCertificateID] = useState('');
  const [endpoints, setEndpoints] = useState<Array<{ address: string; weight: number }>>([{ address: 'service.internal:8080', weight: 100 }]);
  const [loadBalancing, setLoadBalancing] = useState<Service['loadBalancing']>('轮询');
  const [healthMode, setHealthMode] = useState('HTTP');
  const [healthPath, setHealthPath] = useState('/health');
  const [healthInterval, setHealthInterval] = useState('10');
  const [models, setModels] = useState('');
  const [discoveredTools, setDiscoveredTools] = useState<string[]>([]);
  const [tested, setTested] = useState(false);
  const protocolOptions = type === 'HTTP' ? ['HTTP/1.1', 'HTTP/2'] : type === 'MODEL' ? ['OpenAI 兼容 API', 'Anthropic Messages', 'AWS Bedrock'] : ['Streamable HTTP'];
  const authenticationOptions = type === 'HTTP' ? ['无认证', 'Bearer Token', 'Basic', 'mTLS'] : type === 'MODEL' ? ['Bearer Token', '自定义请求头', 'AWS 签名'] : ['无认证', 'Bearer Token', 'mTLS'];
  const clientCertificates = certificates.filter((item) => item.usage === '客户端证书' && item.state !== 'error');
  const trustCertificates = certificates.filter((item) => item.usage === '信任证书' && item.state !== 'error');
  const effectiveClientCertificateID = clientCertificates.some((item) => item.id === clientCertificateID) ? clientCertificateID : '';
  const effectiveTrustCertificateID = trustCertificates.some((item) => item.id === trustCertificateID) ? trustCertificateID : '';
  const capabilities = type === 'MODEL' ? models.split(',').map((item) => item.trim()).filter(Boolean) : type === 'MCP' ? discoveredTools : [];

  const balanceWeights = (items: Array<{ address: string; weight: number }>) => {
    const baseWeight = Math.floor(100 / items.length);
    const remainder = 100 - baseWeight * items.length;
    return items.map((item, index) => ({ ...item, weight: baseWeight + (index < remainder ? 1 : 0) }));
  };
  const updateEndpoint = (index: number, changes: Partial<{ address: string; weight: number }>) => {
    setEndpoints((items) => items.map((item, itemIndex) => itemIndex === index ? { ...item, ...changes } : item));
    setTested(false);
  };
  const changeType = (next: ServiceType) => {
    setType(next);
    setProvider(next === 'HTTP' ? '内部服务' : next === 'MODEL' ? '模型服务商' : 'MCP 服务');
    setProtocol(next === 'HTTP' ? 'HTTP/1.1' : next === 'MODEL' ? 'OpenAI 兼容 API' : 'Streamable HTTP');
    setAuthentication(next === 'HTTP' ? '无认证' : next === 'MODEL' ? 'Bearer Token' : '无认证');
    setEndpoints([{ address: next === 'MCP' ? 'tools.internal:443/mcp' : next === 'MODEL' ? 'api.provider.com:443' : 'service.internal:8080', weight: 100 }]);
    setTransportSecurity(next === 'HTTP' ? '明文连接' : 'TLS');
    setCredential(''); setCredentialName(''); setCredentialRegion(''); setClientCertificateID('');
    setServerName(''); setTrustMode('系统信任'); setTrustCertificateID('');
    setModels(''); setDiscoveredTools([]); setHealthMode(next === 'MODEL' ? '被动检查' : 'HTTP'); setTested(false);
  };
  const testConnection = () => {
    if (type === 'MCP') setDiscoveredTools(['web_search', 'fetch_page', 'extract_text']);
    setTested(true);
  };
  const needsCredential = authentication !== '无认证';
  const credentialComplete = !needsCredential
    || (authentication === 'mTLS' ? Boolean(effectiveClientCertificateID)
      : authentication === 'Basic' || authentication === '自定义请求头' ? Boolean(credentialName.trim() && credential.trim())
        : authentication === 'AWS 签名' ? Boolean(credentialName.trim() && credential.trim() && credentialRegion.trim())
          : Boolean(credential.trim()));
  const tlsComplete = transportSecurity === '明文连接' || trustMode === '系统信任' || Boolean(effectiveTrustCertificateID);
  const canTest = endpoints.every((item) => item.address.trim()) && credentialComplete && tlsComplete && (type !== 'MODEL' || capabilities.length > 0);
  const endpointRecords: ServiceEndpoint[] = endpoints.map((item) => ({ ...item, weight: endpoints.length === 1 ? 100 : item.weight, state: 'healthy' }));
  const healthCheck = healthMode === '被动检查' ? '被动健康检查' : healthMode === 'TCP' ? `TCP 连接 · ${healthInterval} 秒` : `HTTP GET ${healthPath} · ${healthInterval} 秒`;
  const savedAuthentication = authentication === 'mTLS' ? `mTLS · ${clientCertificates.find((item) => item.id === effectiveClientCertificateID)?.name}` : authentication;
  const save = () => onSave({
    id: `svc-${Date.now()}`, name, type, endpoints: endpointRecords, provider, protocol,
    authentication: savedAuthentication,
    clientCertificateID: authentication === 'mTLS' ? effectiveClientCertificateID : undefined,
    transportSecurity,
    serverName: transportSecurity === 'TLS' ? serverName || undefined : undefined,
    trustCertificateID: transportSecurity === 'TLS' && trustMode === '指定信任证书' ? effectiveTrustCertificateID : undefined,
    loadBalancing, healthCheck, capabilities, successRate: '—', latency: '—', state: 'healthy',
  });

  return (
    <Drawer title="创建服务" description="连接、认证、TLS、端点与健康检查" onClose={onClose} width="wide">
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="type-selector">{(['HTTP', 'MODEL', 'MCP'] as const).map((item) => <button key={item} type="button" className={type === item ? 'is-selected' : ''} onClick={() => changeType(item)}><TypeBadge type={item} /><strong>{item === 'HTTP' ? 'HTTP 服务' : item === 'MODEL' ? '模型服务' : 'MCP 服务'}</strong><small>{item === 'HTTP' ? '普通业务接口' : item === 'MODEL' ? '模型厂商或自托管推理服务' : '远程 MCP 服务'}</small></button>)}</div>
        <section className="form-section">
          <header><span>1</span><div><strong>连接信息</strong><small>服务协议和上游身份认证</small></div></header>
          <div className="form-grid">
            <label><span>服务名称</span><input required value={name} onChange={(event) => setName(event.target.value)} /></label>
            <label><span>服务提供方</span><input required value={provider} onChange={(event) => setProvider(event.target.value)} /></label>
            <label><span>{type === 'MCP' ? '传输方式' : '请求协议'}</span><select value={protocol} onChange={(event) => { setProtocol(event.target.value); setTested(false); }}>{protocolOptions.map((item) => <option key={item}>{item}</option>)}</select></label>
            <label><span>认证方式</span><select value={authentication} onChange={(event) => { setAuthentication(event.target.value); setCredential(''); setCredentialName(''); setCredentialRegion(''); setClientCertificateID(''); setTested(false); }}>{authenticationOptions.map((item) => <option key={item}>{item}</option>)}</select></label>
            {authentication === 'mTLS' ? <label className="field-wide"><span>客户端证书</span><select required value={effectiveClientCertificateID} onChange={(event) => { setClientCertificateID(event.target.value); setTested(false); }}><option value="">选择客户端证书</option>{clientCertificates.map((item) => <option key={item.id} value={item.id}>{item.name} · {item.issuer}</option>)}</select></label> : null}
            {authentication === 'Basic' ? <><label><span>用户名</span><input required value={credentialName} onChange={(event) => { setCredentialName(event.target.value); setTested(false); }} /></label><label><span>密码</span><input required type="password" value={credential} onChange={(event) => { setCredential(event.target.value); setTested(false); }} placeholder="保存后不再显示" /></label></> : null}
            {authentication === '自定义请求头' ? <><label><span>请求头名称</span><input required value={credentialName} onChange={(event) => { setCredentialName(event.target.value); setTested(false); }} placeholder="例如 x-api-key" /></label><label><span>请求头值</span><input required type="password" value={credential} onChange={(event) => { setCredential(event.target.value); setTested(false); }} placeholder="保存后不再显示" /></label></> : null}
            {authentication === 'AWS 签名' ? <><label><span>Access Key ID</span><input required value={credentialName} onChange={(event) => { setCredentialName(event.target.value); setTested(false); }} /></label><label><span>Secret Access Key</span><input required type="password" value={credential} onChange={(event) => { setCredential(event.target.value); setTested(false); }} /></label><label className="field-wide"><span>AWS 区域</span><input required value={credentialRegion} onChange={(event) => { setCredentialRegion(event.target.value); setTested(false); }} placeholder="例如 us-east-1" /></label></> : null}
            {authentication === 'Bearer Token' ? <label className="field-wide"><span>Bearer Token</span><input required type="password" value={credential} onChange={(event) => { setCredential(event.target.value); setTested(false); }} placeholder="保存后不再显示明文" /></label> : null}
            {type === 'MODEL' ? <label className="field-wide"><span>模型或部署名称</span><input required value={models} onChange={(event) => { setModels(event.target.value); setTested(false); }} placeholder="例如 qwen-max, qwen-plus" /><small>填写厂商接口中实际可调用的名称</small></label> : null}
          </div>
        </section>
        <section className="form-section">
          <header><span>2</span><div><strong>服务端点</strong><small>同一服务的多个实例共享协议、认证和 TLS 配置</small></div></header>
          <div className="endpoint-editor">{endpoints.map((endpoint, index) => <div className="endpoint-row" key={index}><label><span>端点地址</span><input required value={endpoint.address} onChange={(event) => updateEndpoint(index, { address: event.target.value })} /></label>{endpoints.length > 1 ? <label><span>权重（%）</span><input type="number" min="1" max="100" value={endpoint.weight} onChange={(event) => updateEndpoint(index, { weight: Number(event.target.value) })} /></label> : null}<button type="button" aria-label="删除端点" disabled={endpoints.length === 1} onClick={() => { setEndpoints((items) => balanceWeights(items.filter((_, itemIndex) => itemIndex !== index))); setTested(false); }}><Trash2 /></button></div>)}</div>
          <button className="text-action" type="button" onClick={() => { setEndpoints((items) => balanceWeights([...items, { address: '', weight: 0 }])); setTested(false); }}><Plus />添加端点</button>
          {endpoints.length > 1 ? <div className="form-grid load-balance-field"><label><span>负载方式</span><select value={loadBalancing} onChange={(event) => setLoadBalancing(event.target.value as Service['loadBalancing'])}><option>轮询</option><option>最少请求</option><option>随机</option></select></label><div className="weight-summary"><span>当前权重合计</span><strong className={endpoints.reduce((sum, item) => sum + item.weight, 0) === 100 ? '' : 'is-warning'}>{endpoints.reduce((sum, item) => sum + item.weight, 0)}%</strong></div></div> : null}
        </section>
        <section className="form-section">
          <header><span>3</span><div><strong>传输安全</strong><small>TLS 服务端校验与可选的客户端证书是两件事</small></div></header>
          <div className="form-grid">
            <label><span>连接加密</span><select value={transportSecurity} onChange={(event) => { setTransportSecurity(event.target.value as Service['transportSecurity']); setTested(false); }}><option>明文连接</option><option>TLS</option></select></label>
            {transportSecurity === 'TLS' ? <><label><span>服务端名称</span><input value={serverName} onChange={(event) => { setServerName(event.target.value); setTested(false); }} placeholder="默认从端点地址推导" /></label><label><span>证书信任</span><select value={trustMode} onChange={(event) => { setTrustMode(event.target.value as '系统信任' | '指定信任证书'); setTrustCertificateID(''); setTested(false); }}><option>系统信任</option><option>指定信任证书</option></select></label>{trustMode === '指定信任证书' ? <label><span>信任证书</span><select required value={effectiveTrustCertificateID} onChange={(event) => { setTrustCertificateID(event.target.value); setTested(false); }}><option value="">选择 CA 证书</option>{trustCertificates.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label> : null}</> : null}
          </div>
        </section>
        <details className="advanced-config"><summary>健康检查</summary><div className="form-grid"><label><span>检查方式</span><select value={healthMode} onChange={(event) => setHealthMode(event.target.value)}><option>HTTP</option><option>TCP</option><option>被动检查</option></select></label>{healthMode !== '被动检查' ? <label><span>检查周期（秒）</span><input type="number" min="2" value={healthInterval} onChange={(event) => setHealthInterval(event.target.value)} /></label> : null}{healthMode === 'HTTP' ? <label className="field-wide"><span>检查路径</span><input value={healthPath} onChange={(event) => setHealthPath(event.target.value)} /></label> : null}</div></details>
        <button className={`connection-test ${tested ? 'is-success' : ''}`} type="button" disabled={!canTest} onClick={testConnection}>{tested ? <CheckCircle2 /> : <CircleGauge />}<div><strong>{tested ? '连接验证通过' : type === 'MCP' ? '测试连接并发现工具' : '测试连接'}</strong><span>{tested ? type === 'MCP' ? `已发现 ${discoveredTools.length} 个工具` : `已验证 ${endpoints.length} 个端点、认证和 TLS` : '验证端点、认证、TLS 和协议兼容性'}</span></div></button>
        {tested && type === 'MCP' ? <div className="discovery-result"><strong>已发现工具</strong><div className="tag-cell">{discoveredTools.map((tool) => <code key={tool}>{tool}</code>)}</div></div> : null}
        <FormActions submitLabel="保存服务" submitDisabled={!tested || !canTest || (endpoints.length > 1 && endpoints.reduce((sum, item) => sum + item.weight, 0) !== 100)} onCancel={onClose} />
      </form>
    </Drawer>
  );
}

export function CertificatePage() {
  const { certificates, gateways, services, addCertificate } = usePrototype();
  const [selected, setSelected] = useState<(typeof certificates)[number] | null>(null);
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState('');
  const [toast, setToast] = useState('');
  const visible = certificates.filter((certificate) => `${certificate.name}${certificate.identities.join('')}${certificate.issuer}${certificate.usage}`.toLowerCase().includes(query.toLowerCase()));
  const referencesFor = (certificateID: string) => [
    ...gateways.filter((gateway) => gateway.listeners.some((listener) => listener.bindings.some((binding) => binding.certificateID === certificateID))).map((gateway) => ({ name: gateway.name, detail: '网关 HTTPS 域名绑定' })),
    ...services.filter((service) => service.clientCertificateID === certificateID).map((service) => ({ name: service.name, detail: '上游 mTLS 客户端身份' })),
    ...services.filter((service) => service.trustCertificateID === certificateID).map((service) => ({ name: service.name, detail: '上游 TLS 服务端信任' })),
  ];
  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="流量管理" title="证书" description="网关 HTTPS 与上游 TLS 证书" actions={<PrimaryButton onClick={() => setCreating(true)}><Plus />导入证书</PrimaryButton>} />
      <section className="certificate-ribbon"><div><LockKeyhole /><span><strong>{certificates.length}</strong> 张证书</span></div><div><CheckCircle2 /><span><strong>{certificates.filter((item) => item.state === 'healthy').length}</strong> 张状态正常</span></div><div className="warning"><Clock3 /><span><strong>{certificates.filter((item) => item.remainingDays < 30).length}</strong> 张将在 30 天内过期</span></div></section>
      <section className="card table-card">
        <header className="table-toolbar"><SearchField value={query} onChange={setQuery} placeholder="搜索证书、标识或签发机构" /><span>{visible.length} 张证书</span></header>
        <div className="table-head certificate-columns"><span>证书</span><span>证书标识</span><span>签发机构</span><span>到期时间</span><span>资源引用</span><span>状态</span><span /></div>
        {visible.length ? visible.map((certificate) => { const references = referencesFor(certificate.id); return <button key={certificate.id} className="table-row certificate-columns" type="button" onClick={() => setSelected(certificate)}><div className="name-cell"><span><KeyRound /></span><div><strong>{certificate.name}</strong><small>{certificate.usage}</small></div></div><div className="tag-cell">{certificate.identities.map((identity) => <code key={identity}>{identity}</code>)}</div><span>{certificate.issuer}</span><div><strong>{certificate.expiresAt}</strong><small>剩余 {certificate.remainingDays} 天</small></div><span>{references.length ? references.map((reference) => reference.name).join('、') : '未引用'}</span><StatusBadge state={certificate.state} /><ChevronRight /></button>; }) : <EmptyState title="没有匹配的证书" description="请调整搜索条件。" />}
      </section>
      {selected ? <Drawer title={selected.name} description={selected.usage} onClose={() => setSelected(null)}><div className="detail-hero"><span><KeyRound /></span><div><StatusBadge state={selected.state} /><h3>{selected.identities.join('、')}</h3><p>{selected.issuer} · {selected.expiresAt} 到期</p></div></div><section className="detail-section"><header><h3>引用关系</h3></header>{referencesFor(selected.id).length ? referencesFor(selected.id).map((reference) => <div className="detail-line" key={`${reference.name}-${reference.detail}`}><span><Network /></span><div><strong>{reference.name}</strong><small>{reference.detail}</small></div><StatusBadge state="healthy" /></div>) : <EmptyState title="证书未被引用" description={selected.usage === '服务器证书' ? '可以绑定到 HTTPS 监听入口的域名。' : selected.usage === '客户端证书' ? '可以用于服务的 mTLS 客户端身份。' : '可以用于校验上游 TLS 服务端证书。'} />}</section><div className="form-note"><LockKeyhole />{selected.usage === '信任证书' ? '信任证书不包含私钥。' : '私钥仅在导入时提交，不会再次显示，也不会写入审计详情。'}</div></Drawer> : null}
      {creating ? <CreateCertificate onClose={() => setCreating(false)} onSave={(certificate) => { addCertificate(certificate); setCreating(false); setToast('证书已导入演示环境'); }} /> : null}
      {toast ? <Toast message={toast} onDone={() => setToast('')} /> : null}
    </div>
  );
}

function CreateCertificate({ onClose, onSave }: { onClose: () => void; onSave: (certificate: Certificate) => void }) {
  const [name, setName] = useState('新域名证书');
  const [intendedUsage, setIntendedUsage] = useState<Certificate['usage']>('服务器证书');
  const [certificatePEM, setCertificatePEM] = useState('-----BEGIN CERTIFICATE-----\n••••••••\n-----END CERTIFICATE-----');
  const [privateKeyPEM, setPrivateKeyPEM] = useState('-----BEGIN PRIVATE KEY-----\n••••••••\n-----END PRIVATE KEY-----');
  const [parsed, setParsed] = useState(false);
  const canParse = certificatePEM.includes('-----BEGIN CERTIFICATE-----')
    && certificatePEM.includes('-----END CERTIFICATE-----')
    && (intendedUsage === '信任证书' || privateKeyPEM.includes('-----BEGIN PRIVATE KEY-----') || privateKeyPEM.includes('-----BEGIN RSA PRIVATE KEY-----'));
  const identities = intendedUsage === '服务器证书' ? ['*.new.example.com', 'new.example.com'] : intendedUsage === '客户端证书' ? ['ingate-client.internal'] : ['企业内部根 CA'];
  const issuer = intendedUsage === '服务器证书' ? "Let's Encrypt R13" : '企业内部根 CA';
  const save = () => onSave({ id: `cert-${Date.now()}`, name, identities, issuer, usage: intendedUsage, expiresAt: '2027-08-12', remainingDays: 365, state: 'healthy' });
  return <Drawer title="导入证书" description="系统读取证书用途、标识、签发机构和有效期" onClose={onClose}><form onSubmit={(event) => submitForm(event, save)}><div className="form-grid"><label><span>证书名称</span><input required value={name} onChange={(event) => setName(event.target.value)} /></label><label><span>用于</span><select value={intendedUsage} onChange={(event) => { setIntendedUsage(event.target.value as Certificate['usage']); setParsed(false); }}><option>服务器证书</option><option>客户端证书</option><option>信任证书</option></select><small>选择使用场景；解析结果会校验证书是否具备对应能力</small></label><label className="field-wide"><span>证书链 PEM</span><textarea required value={certificatePEM} onChange={(event) => { setCertificatePEM(event.target.value); setParsed(false); }} /></label>{intendedUsage !== '信任证书' ? <label className="field-wide"><span>私钥 PEM</span><textarea required value={privateKeyPEM} onChange={(event) => { setPrivateKeyPEM(event.target.value); setParsed(false); }} /></label> : null}</div><button className={`connection-test ${parsed ? 'is-success' : ''}`} type="button" disabled={!canParse} onClick={() => setParsed(true)}>{parsed ? <CheckCircle2 /> : <CircleGauge />}<div><strong>{parsed ? intendedUsage === '信任证书' ? '信任证书校验通过' : '证书与私钥匹配' : '解析并校验证书'}</strong><span>{parsed ? '用途、标识、签发机构和有效期已读取' : '保存前检查证书格式、用途、有效期和私钥匹配关系'}</span></div></button>{parsed ? <dl className="definition-list certificate-preview"><div><dt>用途</dt><dd>{intendedUsage}</dd></div><div><dt>{intendedUsage === '服务器证书' ? '覆盖域名' : intendedUsage === '客户端证书' ? '客户端标识' : '证书主体'}</dt><dd>{identities.map((identity) => <code key={identity}>{identity}</code>)}</dd></div><div><dt>签发机构</dt><dd>{issuer}</dd></div><div><dt>有效期</dt><dd>2026-08-12 至 2027-08-12</dd></div><div><dt>指纹</dt><dd><code>SHA256 · A8:31:7F:••:9C</code></dd></div></dl> : null}<FormActions submitLabel="导入证书" submitDisabled={!parsed || !canParse} onCancel={onClose} /></form></Drawer>;
}
