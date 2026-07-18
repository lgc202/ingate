import type { ResourceStatus } from './common';

export type ConfigurationResourceKind =
  | 'Gateway'
  | 'Route'
  | 'Upstream'
  | 'Certificate'
  | 'RateLimitPolicy'
  | 'AccessControlPolicy';

export interface ConfigurationStatusItem {
  id: string;
  name: string;
  kind: ConfigurationResourceKind;
  status: ResourceStatus;
  href: string;
}

export interface ConfigurationStatusView {
  items: ConfigurationStatusItem[];
}

export function configurationResourceKindLabel(kind: ConfigurationResourceKind) {
  const labels: Record<ConfigurationResourceKind, string> = {
    Gateway: '网关',
    Route: '路由',
    Upstream: '服务',
    Certificate: '证书',
    RateLimitPolicy: '限流策略',
    AccessControlPolicy: '访问控制策略',
  };

  return labels[kind];
}
