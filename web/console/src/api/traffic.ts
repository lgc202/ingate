import { apiRequest } from './client';
import { getRequestRecordWorkspace } from './requestRecords';
import type {
  TrafficAnalysis,
  TrafficAnalysisFilters,
  TrafficAnalysisWorkspace,
} from '@/domain/traffic';

export async function getTrafficAnalysis(filters: TrafficAnalysisFilters): Promise<TrafficAnalysis> {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
    breakdownDimension: filters.breakdownDimension,
    breakdownLimit: '10',
  });
  setQuery(query, 'gatewayID', filters.gatewayID);
  setQuery(query, 'routeID', filters.routeID);
  setQuery(query, 'serviceID', filters.serviceID);
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

function setQuery(query: URLSearchParams, name: string, value?: string) {
  const normalized = value?.trim();
  if (normalized) query.set(name, normalized);
}
