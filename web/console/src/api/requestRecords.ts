import { apiRequest } from './client';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import { listUpstreams } from './upstreams';
import { listCallers } from './callers';
import type {
  RequestRecord,
  RequestRecordFilters,
  RequestRecordPage,
  RequestRecordWorkspace,
} from '@/domain/requestRecord';

export async function listRequestRecords(
  filters: RequestRecordFilters,
  pageToken = '',
  pageSize = 10,
): Promise<RequestRecordPage> {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
    pageSize: String(pageSize),
  });
  setQuery(query, 'gatewayID', filters.gatewayID);
  setQuery(query, 'routeID', filters.routeID);
  setQuery(query, 'serviceID', filters.serviceID);
  setQuery(query, 'callerID', filters.callerID);
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
  const [gateways, routes, services, callers] = await Promise.all([listGateways(), listRoutes(), listUpstreams(), listCallers()]);
  return {
    gateways: gateways.gateways.map(({ id, name }) => ({ id, name })),
    routes: routes.routes.map(({ id, name, accessMode }) => ({ id, name, accessMode })),
    services: services.upstreams.map(({ id, name }) => ({ id, name })),
    callers: callers.map(({ id, name, accessKeys }) => ({
      id,
      name,
      accessKeys: accessKeys.map((key) => ({ id: key.id, name: key.name })),
    })),
  };
}

function setQuery(query: URLSearchParams, name: string, value?: string) {
  const normalized = value?.trim();
  if (normalized) query.set(name, normalized);
}
