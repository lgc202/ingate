import { apiListAllByCursor, apiListPageByCursor, apiRequest } from './client';
import type { CursorPage, CursorPagedResponse } from './client';
import { normalizeResourceState } from '@/domain/common';
import type { Service, ServiceList, ServiceMutationPayload } from '@/domain/service';

interface ServiceListResponse extends ServiceList, CursorPagedResponse {}

export async function listServices(): Promise<ServiceList> {
  const services = await apiListAllByCursor<ServiceListResponse, Service>(
    '/services',
    (page) => page.services ?? [],
  );
  return { services: services.map(serviceFromAPI) };
}

export async function listServicePage(input: {
  limit: number;
  cursor: string;
  query?: string;
  state?: string;
  type?: string;
}): Promise<CursorPage<Service>> {
  const page = await apiListPageByCursor<ServiceListResponse, Service>(
    '/services',
    input,
    (value) => value.services ?? [],
  );
  return { ...page, items: page.items.map(serviceFromAPI) };
}

function serviceFromAPI(service: Service): Service {
  return {
    ...service,
    version: Number(service.version),
    state: normalizeResourceState(service.state),
  };
}

export async function saveService(payload: ServiceMutationPayload): Promise<Service> {
  const path = payload.id ? `/services/${encodeURIComponent(payload.id)}` : '/services';
  const service = await apiRequest<Service>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
  return serviceFromAPI(service);
}

export async function deleteService(id: string, version: number): Promise<void> {
  await apiRequest<Record<string, never>>(
    `/services/${encodeURIComponent(id)}?version=${version}`,
    { method: 'DELETE' },
  );
}
