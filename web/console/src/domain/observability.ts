import type { CountSegment, MetricCard, TimelineEvent } from './common';

export interface CallLogEntry {
  route: string;
  statusCode: string;
  result: string;
}

export interface ObservabilityOverview {
  metrics: MetricCard[];
  requestTrend: number[];
  callLogs: CallLogEntry[];
  serviceHealth: CountSegment[];
  alerts: TimelineEvent[];
}
