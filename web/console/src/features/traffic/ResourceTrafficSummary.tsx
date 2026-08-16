import { useCallback, useMemo, useState } from 'react';
import { ArrowUpRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { getTrafficAnalysis } from '@/api/traffic';
import { useResource } from '@/api/useResource';
import { formatDuration } from '@/domain/requestRecord';
import { recentTimeRange } from '@/domain/timeRange';
import {
  formatTrafficCount,
  formatTrafficPercent,
  metricNumber,
  type TrafficAnalysisFilters,
  type TrafficBreakdownDimension,
  type TrafficMetrics,
} from '@/domain/traffic';

type ResourceKind = 'gateway' | 'route' | 'service';

export function ResourceTrafficSummary({ kind, resourceID }: { kind: ResourceKind; resourceID: string }) {
  const [range] = useState(() => recentTimeRange(1));
  const filters = useMemo(() => resourceFilters(kind, resourceID, range), [kind, range, resourceID]);
  const load = useCallback(() => getTrafficAnalysis(filters), [filters]);
  const traffic = useResource(load);

  return (
    <section className="resource-detail-section resource-observability">
      <header>
        <h3>最近 1 小时</h3>
        <nav aria-label="运行数据下钻">
          <Link to={analysisURL(filters)}>流量分析<ArrowUpRight /></Link>
          <Link to={requestURL(filters)}>请求记录<ArrowUpRight /></Link>
        </nav>
      </header>
      <ResourceTrafficContent loading={traffic.loading} error={traffic.error} metrics={traffic.data?.summary} />
    </section>
  );
}

function ResourceTrafficContent({ loading, error, metrics }: { loading: boolean; error: Error | null; metrics?: TrafficMetrics }) {
  if (loading && !metrics) return <div className="resource-observability-state">正在读取运行数据...</div>;
  if (error || !metrics) return <div className="resource-observability-state is-error">运行数据暂时不可用，仍可查看流量分析或请求记录</div>;

  const requests = metricNumber(metrics.requestCount);
  return (
    <div className="resource-detail-grid resource-observability-grid">
      <div><span>请求量</span><strong>{formatTrafficCount(metrics.requestCount)}</strong></div>
      <div><span>正常响应率</span><strong>{formatTrafficPercent(metricNumber(metrics.nonErrorCount), requests)}</strong></div>
      <div><span>平均总耗时</span><strong>{requests > 0 ? formatDuration(metrics.averageDuration) : '—'}</strong></div>
      <div><span>P95 总耗时</span><strong>{requests > 0 ? formatDuration(metrics.p95Duration) : '—'}</strong><small>{requests > 0 ? `基于 ${formatTrafficCount(requests)} 次请求` : '当前没有请求'}</small></div>
    </div>
  );
}

function resourceFilters(kind: ResourceKind, resourceID: string, range: { startTime: string; endTime: string }): TrafficAnalysisFilters {
  return {
    ...range,
    breakdownDimension: resourceDimension(kind),
    gatewayID: kind === 'gateway' ? resourceID : undefined,
    routeID: kind === 'route' ? resourceID : undefined,
    serviceID: kind === 'service' ? resourceID : undefined,
  };
}

function resourceDimension(kind: ResourceKind): TrafficBreakdownDimension {
  if (kind === 'gateway') return 'TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY';
  if (kind === 'service') return 'TRAFFIC_BREAKDOWN_DIMENSION_SERVICE';
  return 'TRAFFIC_BREAKDOWN_DIMENSION_ROUTE';
}

function analysisURL(filters: TrafficAnalysisFilters): string {
  const query = resourceQuery(filters);
  query.set('breakdownDimension', filters.breakdownDimension);
  return `/analysis?${query}`;
}

function requestURL(filters: TrafficAnalysisFilters): string {
  return `/requests?${resourceQuery(filters)}`;
}

function resourceQuery(filters: TrafficAnalysisFilters): URLSearchParams {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
  });
  if (filters.gatewayID) query.set('gatewayID', filters.gatewayID);
  if (filters.routeID) query.set('routeID', filters.routeID);
  if (filters.serviceID) query.set('serviceID', filters.serviceID);
  return query;
}
