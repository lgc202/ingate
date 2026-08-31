import type { ConfigState, HealthState } from "./resource-state";

export interface CallerPermission {
  routeID: string;
  scopes: string[];
}

export interface CallerMetric {
  label: string;
  value: string;
  note: string;
}

export interface CallerQuota {
  routeID: string;
  used: number;
  limit: number;
  period: "每日" | "每月";
}

export interface CallerAccessKey {
  id: string;
  name: string;
  prefix: string;
  createdAt: string;
  expiresAt: string;
  lastUsed: string;
  state: HealthState;
  graceUntil?: string;
  rotatedFromID?: string;
  replacedByID?: string;
}

export interface Caller {
  id: string;
  name: string;
  slug: string;
  owner: string;
  purpose: string;
  enabled: boolean;
  keys: CallerAccessKey[];
  permissions: CallerPermission[];
  metrics: CallerMetric[];
  quotas: CallerQuota[];
  state: HealthState;
  lastActive: string;
}

export interface IdentitySource {
  id: string;
  name: string;
  provider: string;
  discoveryURL: string;
  audiences: string[];
  state: HealthState;
  lastVerified: string;
}

export interface Policy {
  id: string;
  name: string;
  type: "请求限流" | "IP 访问限制";
  targets: Array<{ kind: "网关" | "路由"; id: string; name: string }>;
  rule: string;
  effect: string;
  state: HealthState;
  configState?: ConfigState;
  settings?: {
    rateLimit?: string;
    ratePeriod?: string;
    rateDimension?: string;
    ipMode?: string;
    ipRanges?: string;
  };
}
