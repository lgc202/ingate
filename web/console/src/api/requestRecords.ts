import { apiRequest } from './client';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import { listUpstreams } from './upstreams';
import type {
  RequestRecord,
  RequestRecordFilters,
  RequestRecordPage,
  RequestRecordWorkspace,
} from '@/domain/requestRecord';

export async function listRequestRecords(
  filters: RequestRecordFilters,
  pageToken = '',
): Promise<RequestRecordPage> {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
    pageSize: '50',
  });
  setQuery(query, 'gatewayID', filters.gatewayID);
  setQuery(query, 'routeID', filters.routeID);
  setQuery(query, 'serviceID', filters.serviceID);
  setQuery(query, 'requestID', filters.requestID);
  setQuery(query, 'method', filters.method);
  setQuery(query, 'host', filters.host);
  setQuery(query, 'pathPrefix', filters.pathPrefix);
  setQuery(query, 'outcome', filters.outcome);
  if (filters.statusCode !== undefined) query.set('statusCode', String(filters.statusCode));
  if (pageToken) query.set('pageToken', pageToken);

  const page = await apiRequest<Partial<RequestRecordPage>>(`/request-records?${query}`);
  return {
    records: page.records ?? [],
    nextPageToken: page.nextPageToken ?? '',
  };
}

export async function getRequestRecord(id: string, startedAt: string): Promise<RequestRecord> {
  const query = new URLSearchParams({ startedAt });
  return apiRequest<RequestRecord>(`/request-records/${encodeURIComponent(id)}?${query}`);
}

export async function getRequestRecordWorkspace(): Promise<RequestRecordWorkspace> {
  const [gateways, routes, services] = await Promise.all([listGateways(), listRoutes(), listUpstreams()]);
  return {
    gateways: gateways.gateways.map(({ id, name }) => ({ id, name })),
    routes: routes.routes.map(({ id, name }) => ({ id, name })),
    services: services.upstreams.map(({ id, name }) => ({ id, name })),
  };
}

function setQuery(query: URLSearchParams, name: string, value?: string) {
  const normalized = value?.trim();
  if (normalized) query.set(name, normalized);
}
