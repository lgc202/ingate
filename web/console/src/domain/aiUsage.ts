export type AIUsageBreakdownDimension =
  | 'AI_USAGE_BREAKDOWN_DIMENSION_CALLER'
  | 'AI_USAGE_BREAKDOWN_DIMENSION_ROUTE'
  | 'AI_USAGE_BREAKDOWN_DIMENSION_CLIENT_MODEL'
  | 'AI_USAGE_BREAKDOWN_DIMENSION_SERVICE'
  | 'AI_USAGE_BREAKDOWN_DIMENSION_ACTUAL_MODEL';

export type AIUsageBreakdownOrder =
  | 'AI_USAGE_BREAKDOWN_ORDER_CALL_COUNT'
  | 'AI_USAGE_BREAKDOWN_ORDER_TOTAL_TOKENS';

export interface AIUsageFilters {
  startTime: string;
  endTime: string;
  gatewayID?: string;
  callerID?: string;
  routeID?: string;
  clientModel?: string;
  serviceID?: string;
  actualModel?: string;
  breakdownDimension: AIUsageBreakdownDimension;
  breakdownOrder: AIUsageBreakdownOrder;
  breakdownLimit?: number;
}

export interface AIUsageMetrics {
  callCount: string | number;
  normalResponseCount: string | number;
  tokenReportedCallCount: string | number;
  inputTokens: string | number;
  outputTokens: string | number;
  totalTokens: string | number;
}

export interface AIUsageTrendPoint {
  startedAt: string;
  metrics: AIUsageMetrics;
}

export interface AIUsageBreakdownItem {
  dimensionValue: string;
  metrics: AIUsageMetrics;
}

export interface AIUsageAnalysis {
  summary: AIUsageMetrics;
  trend: AIUsageTrendPoint[];
  breakdownDimension: AIUsageBreakdownDimension;
  breakdownOrder: AIUsageBreakdownOrder;
  breakdown: AIUsageBreakdownItem[];
}

export interface AIUsageResourceOption {
  id: string;
  name: string;
}

export interface AIUsageWorkspace {
  gateways: AIUsageResourceOption[];
  callers: AIUsageResourceOption[];
  routes: AIUsageResourceOption[];
  services: AIUsageResourceOption[];
  clientModels: string[];
  actualModels: string[];
}

export function usageNumber(value: string | number | undefined): number {
  const result = Number(value ?? 0);
  return Number.isFinite(result) ? result : 0;
}

export function formatUsageCount(value: string | number | undefined): string {
  const count = usageNumber(value);
  return new Intl.NumberFormat('zh-CN', {
    notation: count >= 10_000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(count);
}

export function formatUsageCountExact(value: string | number | undefined): string {
  return new Intl.NumberFormat('zh-CN').format(usageNumber(value));
}

export function formatUsagePercent(value: number, total: number): string {
  if (total <= 0) return '—';
  return `${((value / total) * 100).toFixed(2)}%`;
}
