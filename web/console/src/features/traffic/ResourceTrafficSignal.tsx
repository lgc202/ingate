import { useCallback, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { batchGetResourceTraffic } from '@/api/traffic';
import { useResource } from '@/api/useResource';
import { recentTimeRange } from '@/domain/timeRange';
import {
  formatTrafficCount,
  metricNumber,
  type ResourceTrafficSummary,
  type TrafficAnalysisFilters,
  type TrafficBreakdownDimension,
} from '@/domain/traffic';

export type TrafficResourceKind = 'gateway' | 'route' | 'service';

export interface ResourceTrafficOverview {
  loading: boolean;
  error: Error | null;
  metrics: Map<string, ResourceTrafficSummary>;
  filters: TrafficAnalysisFilters;
}

// useResourceTrafficOverview 以一次聚合查询读取资源列表所需的最近流量信号
export function useResourceTrafficOverview(kind: TrafficResourceKind, resourceIDs: string[]): ResourceTrafficOverview {
  const [range] = useState(() => recentTimeRange(1));
  const resourceIDsKey = [...new Set(resourceIDs)].join('\n');
  const requestedResourceIDs = useMemo(() => resourceIDsKey ? resourceIDsKey.split('\n') : [], [resourceIDsKey]);
  const filters = useMemo<TrafficAnalysisFilters>(() => ({
    ...range,
    breakdownDimension: resourceDimension(kind),
    breakdownOrder: 'TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT',
  }), [kind, range]);
  const load = useCallback(
    () => requestedResourceIDs.length === 0
      ? Promise.resolve([])
      : batchGetResourceTraffic(filters.startTime, filters.endTime, filters.breakdownDimension, requestedResourceIDs),
    [filters, requestedResourceIDs],
  );
  const traffic = useResource(load);
  const metrics = useMemo(
    () => new Map(traffic.data?.map((item) => [item.resourceID, item])),
    [traffic.data],
  );
  return { loading: traffic.loading, error: traffic.error, metrics, filters };
}

export function ResourceTrafficSignal({ resourceID, overview }: { resourceID: string; overview: ResourceTrafficOverview }) {
  const metrics = overview.metrics.get(resourceID);
  if (overview.loading && !metrics) return <span className="resource-traffic-signal is-muted">读取中</span>;
  if (overview.error) return <span className="resource-traffic-signal is-muted">暂不可用</span>;
  if (!metrics || metricNumber(metrics.requestCount) === 0) return <span className="resource-traffic-signal is-muted">暂无请求</span>;

  const serverErrors = metricNumber(metrics.serverErrorCount);
  const noResponses = metricNumber(metrics.noResponseCount);
  const issues = serverErrors + noResponses;
  return (
    <Link className={`resource-traffic-signal ${issues > 0 ? 'is-abnormal' : ''}`} to={analysisURL(overview.filters, resourceID)}>
      {issues > 0 ? (
        <><strong>{issueLabel(serverErrors, noResponses)}</strong><small>共 {formatTrafficCount(metrics.requestCount)} 次请求</small></>
      ) : (
        <strong>{formatTrafficCount(metrics.requestCount)} 次请求</strong>
      )}
    </Link>
  );
}

function issueLabel(serverErrors: number, noResponses: number): string {
  const labels: string[] = [];
  if (serverErrors > 0) labels.push(`服务端错误 ${formatTrafficCount(serverErrors)}`);
  if (noResponses > 0) labels.push(`无响应 ${formatTrafficCount(noResponses)}`);
  return labels.join(' · ');
}

function resourceDimension(kind: TrafficResourceKind): TrafficBreakdownDimension {
  if (kind === 'gateway') return 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY';
  if (kind === 'service') return 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE';
  return 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE';
}

function analysisURL(filters: TrafficAnalysisFilters, resourceID: string): string {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
    breakdownDimension: filters.breakdownDimension,
    breakdownOrder: 'TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE',
  });
  if (filters.breakdownDimension === 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY') query.set('gatewayID', resourceID);
  if (filters.breakdownDimension === 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE') query.set('routeID', resourceID);
  if (filters.breakdownDimension === 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE') query.set('serviceID', resourceID);
  return `/analysis?${query}`;
}
