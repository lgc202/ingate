import { Badge } from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone } from '@/domain/common';
import type { RouteResource } from '@/domain/route';
import type { Service } from '@/domain/service';
import { modelProtocolLabel, serviceLoadBalancingLabel } from '@/domain/service';
import { ResourceTrafficSummary } from '@/features/traffic/ResourceTrafficSummary';

export function ServiceDetail({ service, routes }: { service: Service; routes: RouteResource[] }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div>
          <h3>{service.name}</h3>
        </div>
        <Badge tone={resourceStateTone(service.state)}>
          {resourceStateLabel(service.state)}
        </Badge>
      </section>
      <ResourceTrafficSummary kind="service" resourceID={service.id} />
      {service.model ? (
        <section className="resource-detail-section">
          <h3>模型接入</h3>
          <div className="resource-detail-grid">
            <div>
              <span>服务类型</span>
              <strong>模型服务</strong>
            </div>
            <div>
              <span>接口协议</span>
              <strong>{modelProtocolLabel(service.model.protocol)}</strong>
            </div>
            <div>
              <span>API Key</span>
              <strong>{service.model.apiKeyConfigured ? '已配置' : '未配置'}</strong>
            </div>
            <div>
              <span>客户端模型名</span>
              <strong>由 AI 路由发布</strong>
            </div>
          </div>
        </section>
      ) : null}
      <section className="resource-detail-section">
        <h3>连接设置</h3>
        <div className="resource-detail-grid">
          <div>
            <span>协议</span>
            <strong>{service.tls ? 'HTTPS' : 'HTTP'}</strong>
          </div>
          <div>
            <span>负载均衡</span>
            <strong>{serviceLoadBalancingLabel(service.loadBalancing)}</strong>
          </div>
          {service.tls ? (
            <div>
              <span>TLS 服务名称</span>
              <strong>{service.tls.serverName}</strong>
            </div>
          ) : null}
          <div>
            <span>主动健康检查</span>
            <strong>
              {service.healthCheck
                ? `${service.healthCheck.path} · 每 ${service.healthCheck.intervalSeconds} 秒`
                : '未启用'}
            </strong>
          </div>
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>引用路由</h3>
        {routes.length > 0 ? (
          <div className="resource-detail-list">
            {routes.map((route) => (
              <article key={route.id}>
                <div>
                  <strong>{route.name}</strong>
                  <small>{route.ai ? 'AI 路由' : 'API 路由'}</small>
                </div>
                <Badge tone="neutral">路由</Badge>
              </article>
            ))}
          </div>
        ) : (
          <p className="text-xs text-slate-500">当前没有路由引用此服务</p>
        )}
      </section>
      <section className="resource-detail-section">
        <h3>服务地址</h3>
        <div className="resource-detail-list">
          {service.endpoints.map((endpoint) => (
            <article key={`${endpoint.address}:${endpoint.port}`}>
              <div>
                <strong>{endpoint.address}:{endpoint.port}</strong>
                <small>转发权重 {endpoint.weight}</small>
              </div>
              <Badge tone="neutral">{service.tls ? 'HTTPS' : 'HTTP'}</Badge>
            </article>
          ))}
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>资源信息</h3>
        <div className="resource-detail-grid">
          <div>
            <span>生效状态</span>
            <strong>{service.message || resourceStateLabel(service.state)}</strong>
          </div>
          <div>
            <span>更新时间</span>
            <strong>{formatDateTime(service.updatedAt || service.createdAt)}</strong>
          </div>
          <div>
            <span>创建时间</span>
            <strong>{formatDateTime(service.createdAt)}</strong>
          </div>
        </div>
      </section>
    </div>
  );
}
