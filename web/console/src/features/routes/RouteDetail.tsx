import type { ReactNode } from 'react';
import { Badge } from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone } from '@/domain/common';
import type { Caller } from '@/domain/caller';
import type { PolicyWorkspace } from '@/domain/policy';
import type { RouteResource, RouteWorkspace } from '@/domain/route';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';
import { ResourceTrafficSummary } from '@/features/traffic/ResourceTrafficSummary';
import {
  accessModeLabel,
  hostRewriteLabel,
  methodLabel,
  pathMatchLabel,
  resourceName,
  resourceNames,
  routeServiceIDs,
} from './presentation';

export function RouteDetail({ route, workspace, callers, policies, onPoliciesChanged }: { route: RouteResource; workspace: RouteWorkspace; callers: Caller[]; policies: PolicyWorkspace | null; onPoliciesChanged: () => Promise<void> }) {
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

export function RouteCallExample({ route, workspace }: { route: RouteResource; workspace: RouteWorkspace }) {
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

export function AIModelDetail({ route, workspace }: { route: RouteResource; workspace: RouteWorkspace }) {
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

function DetailSection({ title, children }: { title: string; children: ReactNode }) {
  return <section className="resource-detail-section"><h3>{title}</h3><div className="resource-detail-grid">{children}</div></section>;
}

function DetailItem({ label, value, code = false }: { label: string; value: string; code?: boolean }) {
  return <div><span>{label}</span>{code ? <code>{value}</code> : <strong>{value}</strong>}</div>;
}
