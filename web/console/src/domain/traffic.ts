import type { RequestRecordWorkspace } from './requestRecord';

export type TrafficBreakdownDimension =
  | 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY'
  | 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE'
  | 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE';

export interface TrafficAnalysisFilters {
  startTime: string;
  endTime: string;
  gatewayID?: string;
  routeID?: string;
  serviceID?: string;
  breakdownDimension: TrafficBreakdownDimension;
}

export interface TrafficMetrics {
  requestCount: string | number;
  nonErrorCount: string | number;
  clientErrorCount: string | number;
  serverErrorCount: string | number;
  noResponseCount: string | number;
  averageDuration?: string;
  p50Duration?: string;
  p95Duration?: string;
  p99Duration?: string;
}

export interface TrafficTrendPoint {
  startedAt: string;
  metrics: TrafficMetrics;
}

export interface TrafficBreakdownItem {
  resourceID: string;
  metrics: TrafficMetrics;
}

export interface TrafficAnalysis {
  summary: TrafficMetrics;
  trend: TrafficTrendPoint[];
  breakdownDimension: TrafficBreakdownDimension;
  breakdown: TrafficBreakdownItem[];
}

export type TrafficAnalysisWorkspace = RequestRecordWorkspace;

export function metricNumber(value: string | number | undefined): number {
  const result = Number(value ?? 0);
  return Number.isFinite(result) ? result : 0;
}

export function formatTrafficCount(value: string | number | undefined): string {
  const count = metricNumber(value);
  return new Intl.NumberFormat('zh-CN', {
    notation: count >= 10_000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(count);
}

export function formatTrafficPercent(value: number, total: number): string {
  if (total <= 0) return '—';
  return `${((value / total) * 100).toFixed(2)}%`;
}
