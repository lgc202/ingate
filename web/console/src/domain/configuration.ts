import type { ResourceStatus } from './common';

export type ConfigurationResourceKind =
  | 'Gateway'
  | 'Route'
  | 'Upstream'
  | 'Certificate'
  | 'RateLimitPolicy'
  | 'AccessControlPolicy'
  | 'TokenQuotaPolicy';

export interface ConfigurationStatusItem {
  id: string;
  name: string;
  kind: ConfigurationResourceKind;
  status: ResourceStatus;
}

export interface ConfigurationStatusSummary {
  total: number;
  ready: number;
  pending: number;
  error: number;
  disabled: number;
}

export interface ConfigurationStatusView {
  summary: ConfigurationStatusSummary;
  items: ConfigurationStatusItem[];
}

const statePriority: Record<ResourceStatus['state'], number> = {
  Error: 0,
  Pending: 1,
  Ready: 2,
  Disabled: 3,
};

const kindPriority: Record<ConfigurationResourceKind, number> = {
  Gateway: 0,
  Route: 1,
  Upstream: 2,
  Certificate: 3,
  RateLimitPolicy: 4,
  AccessControlPolicy: 5,
  TokenQuotaPolicy: 6,
};

export function sortConfigurationItems(items: ConfigurationStatusItem[]) {
  return [...items].sort((left, right) => (
    statePriority[left.status.state] - statePriority[right.status.state]
    || kindPriority[left.kind] - kindPriority[right.kind]
    || left.name.localeCompare(right.name, 'zh-CN')
    || left.id.localeCompare(right.id)
  ));
}

export function configurationResourceKindLabel(kind: ConfigurationResourceKind) {
  const labels: Record<ConfigurationResourceKind, string> = {
    Gateway: '网关',
    Route: '路由',
    Upstream: '服务',
    Certificate: '证书',
    RateLimitPolicy: '限流策略',
    AccessControlPolicy: '访问控制策略',
    TokenQuotaPolicy: 'Token 配额策略',
  };

  return labels[kind];
}
