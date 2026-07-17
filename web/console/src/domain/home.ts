import type { CountSegment, HealthStatus, MetricCard, TimelineEvent } from './common';
import type { HttpMethod } from './route';

export interface HomeDashboard {
  context: DashboardContext;
  metrics: MetricCard[];
  keyLinks: KeyLinkSummary[];
  actionItems: ActionItem[];
  healthSummary: CountSegment[];
  changes: TimelineEvent[];
  requestTrend: number[];
  errorDistribution: [string, string, string][];
}

export interface DashboardContext {
  configurationDomain: string;
  timeRange: string;
  timeRangeOptions: string[];
}

export interface KeyLinkSummary {
  id: string;
  gatewayName: string;
  routeMethod: HttpMethod;
  routePath: string;
  serviceName: string;
  traffic: string;
  successRate: string;
  latencyP95: string;
  status: HealthStatus;
  reason: string;
}

export type ActionPriority = 'P1' | 'P2' | 'P3';

export interface ActionItem {
  id: string;
  priority: ActionPriority;
  title: string;
  description: string;
  target: string;
  status: HealthStatus;
  actionLabel: string;
}
