import { apiRequest, setQueryParameter } from './client';
import { getRequestRecordWorkspace } from './requestRecords';
import type {
  ResourceTrafficSummary,
  TrafficAnalysis,
  TrafficAnalysisFilters,
  TrafficAnalysisWorkspace,
} from '@/domain/traffic';

const maximumResourceTrafficBatch = 200;

export async function getTrafficAnalysis(filters: TrafficAnalysisFilters): Promise<TrafficAnalysis> {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
    breakdownDimension: filters.breakdownDimension,
    breakdownOrder: filters.breakdownOrder,
    breakdownLimit: String(filters.breakdownLimit ?? 10),
  });
  setQueryParameter(query, 'gatewayID', filters.gatewayID);
  setQueryParameter(query, 'routeID', filters.routeID);
  setQueryParameter(query, 'serviceID', filters.serviceID);
  const analysis = await apiRequest<TrafficAnalysis>(`/traffic-analysis?${query}`);
  return {
    ...analysis,
    trend: analysis.trend ?? [],
    breakdown: analysis.breakdown ?? [],
  };
}

export function getTrafficAnalysisWorkspace(): Promise<TrafficAnalysisWorkspace> {
  return getRequestRecordWorkspace();
}

export async function batchGetResourceTraffic(
  startTime: string,
  endTime: string,
  dimension: TrafficAnalysisFilters['breakdownDimension'],
  resourceIDs: string[],
): Promise<ResourceTrafficSummary[]> {
  const batches: string[][] = [];
  for (let offset = 0; offset < resourceIDs.length; offset += maximumResourceTrafficBatch) {
    batches.push(resourceIDs.slice(offset, offset + maximumResourceTrafficBatch));
  }
  const responses = await Promise.all(batches.map((batch) => apiRequest<{ summaries?: ResourceTrafficSummary[] }>(
    '/traffic-analysis/resource-summaries:batchGet',
    {
      method: 'POST',
      body: JSON.stringify({
        startTime: new Date(startTime).toISOString(),
        endTime: new Date(endTime).toISOString(),
        dimension,
        resourceIDs: batch,
      }),
    },
  )));
  return responses.flatMap((response) => response.summaries ?? []);
}
