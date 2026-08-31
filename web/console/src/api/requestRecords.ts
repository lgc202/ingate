import { apiRequest, setQueryParameter } from './client';
import { listCallers } from './callers';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import { listServices } from './services';
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
  setQueryParameter(query, 'gatewayID', filters.gatewayID);
  setQueryParameter(query, 'routeID', filters.routeID);
  setQueryParameter(query, 'serviceID', filters.serviceID);
  setQueryParameter(query, 'callerID', filters.callerID);
  setQueryParameter(query, 'requestID', filters.requestID);
  setQueryParameter(query, 'method', filters.method);
  setQueryParameter(query, 'host', filters.host);
  setQueryParameter(query, 'pathPrefix', filters.pathPrefix);
  setQueryParameter(query, 'outcome', filters.outcome);
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
  const [gateways, routes, services, callers] = await Promise.all([
    listGateways(),
    listRoutes(),
    listServices(),
    listCallers(),
  ]);
  return {
    gateways: gateways.gateways.map(({ id, name }) => ({ id, name })),
    routes: routes.routes.map(({ id, name, accessMode }) => ({ id, name, accessMode })),
    services: services.services.map(({ id, name }) => ({ id, name })),
    callers: callers.map(({ id, name, accessKeys }) => ({
      id,
      name,
      accessKeys: accessKeys.map((key) => ({ id: key.id, name: key.name })),
    })),
  };
}
