import type { CountSegment, HealthStatus, PublishStatus, TimelineEvent } from './common';

export type PolicyRiskLevel = 'low' | 'medium' | 'high';

export interface PolicyResource {
  id: string;
  name: string;
  type: string;
  scope: string;
  boundRoutes: number;
  publishStatus: PublishStatus;
  riskLevel: PolicyRiskLevel;
  lastChangedAt: string;
}

export interface PolicyListView {
  policies: PolicyResource[];
  coverage: CountSegment[];
  changes: TimelineEvent[];
}

export function riskLevelLabel(level: PolicyRiskLevel) {
  const labels: Record<PolicyRiskLevel, string> = {
    low: '低',
    medium: '中',
    high: '高',
  };

  return labels[level];
}

export function riskLevelStatus(level: PolicyRiskLevel): HealthStatus {
  if (level === 'high') {
    return 'critical';
  }

  if (level === 'medium') {
    return 'warning';
  }

  return 'healthy';
}
