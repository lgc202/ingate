import { apiListAllByCursor, apiListPageByCursor, apiRequest, type CursorPage, type CursorPagedResponse } from './client';
import { listRoutes } from './routes';
import type {
  Caller,
  CallerMutationPayload,
  CallerWorkspace,
  CreateCallerResult,
  IssuedAccessKey,
} from '@/domain/caller';

interface CallerListResponse extends CursorPagedResponse {
  callers?: Caller[];
}

export async function getCallerOptions(): Promise<CallerWorkspace> {
  const routeList = await listRoutes();
  return {
    callers: [],
    routes: routeList.routes
      .filter((route) => route.accessMode === 'ROUTE_ACCESS_CALLER')
      .map((route) => ({ id: route.id, name: route.name })),
  };
}

export async function listCallers(): Promise<Caller[]> {
  const callers = await apiListAllByCursor<CallerListResponse, Caller>('/callers', (page) => page.callers ?? []);
  return callers.map(callerFromAPI);
}

export async function listCallerPage(input: {
  limit: number; cursor: string; query?: string; enabled?: boolean;
}): Promise<CursorPage<Caller>> {
  const page = await apiListPageByCursor<CallerListResponse, Caller>('/callers', input, (value) => value.callers ?? []);
  return { ...page, items: page.items.map(callerFromAPI) };
}

export async function createCaller(payload: CallerMutationPayload): Promise<CreateCallerResult> {
  const result = await apiRequest<CreateCallerResult>('/callers', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  return {
    caller: callerFromAPI(result.caller),
    issuedAccessKey: issuedKeyFromAPI(result.issuedAccessKey),
  };
}

export async function updateCaller(payload: CallerMutationPayload): Promise<Caller> {
  const caller = await apiRequest<Caller>(`/callers/${encodeURIComponent(payload.id ?? '')}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
  return callerFromAPI(caller);
}

export async function deleteCaller(id: string, version: number) {
  await apiRequest<Record<string, never>>(`/callers/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}

export async function issueAccessKey(callerID: string, version: number, name: string, expiresAt?: string): Promise<IssuedAccessKey> {
  const key = await apiRequest<IssuedAccessKey>(`/callers/${encodeURIComponent(callerID)}/access-keys`, {
    method: 'POST',
    body: JSON.stringify({ callerID, version, name, expiresAt }),
  });
  return issuedKeyFromAPI(key);
}

export async function disableAccessKey(callerID: string, accessKeyID: string, version: number): Promise<Caller> {
  const caller = await apiRequest<Caller>(
    `/callers/${encodeURIComponent(callerID)}/access-keys/${encodeURIComponent(accessKeyID)}:disable`,
    { method: 'POST', body: JSON.stringify({ callerID, accessKeyID, version }) },
  );
  return callerFromAPI(caller);
}

function callerFromAPI(caller: Caller): Caller {
  return {
    ...caller,
    version: Number(caller.version),
    routeIDs: caller.routeIDs ?? [],
    accessKeys: caller.accessKeys ?? [],
  };
}

function issuedKeyFromAPI(key: IssuedAccessKey): IssuedAccessKey {
  return { ...key, accessKey: key.accessKey };
}
