import { apiListAllByCursor, apiListPageByCursor, apiRequest, type CursorPage, type CursorPagedResponse } from './client';
import { normalizeResourceState } from '@/domain/common';
import type { Gateway, GatewayListView, GatewayMutationPayload } from '@/domain/gateway';

interface GatewayListResponse extends CursorPagedResponse {
  gateways?: Gateway[];
}

export async function listGateways(): Promise<GatewayListView> {
  const gateways = await apiListAllByCursor<GatewayListResponse, Gateway>('/gateways', (page) => page.gateways ?? []);
  return {
    gateways: gateways.map((gateway) => ({
      ...gateway,
      version: Number(gateway.version),
      listeners: gateway.listeners ?? [],
      state: normalizeResourceState(gateway.state),
    })),
  };
}

export async function listGatewayPage(input: {
  limit: number; cursor: string; query?: string; enabled?: boolean; state?: string;
}): Promise<CursorPage<Gateway>> {
  const page = await apiListPageByCursor<GatewayListResponse, Gateway>('/gateways', input, (value) => value.gateways ?? []);
  return { ...page, items: page.items.map(gatewayFromAPI) };
}

function gatewayFromAPI(gateway: Gateway): Gateway {
  return {
    ...gateway,
    version: Number(gateway.version),
    listeners: gateway.listeners ?? [],
    state: normalizeResourceState(gateway.state),
  };
}

export async function saveGateway(payload: GatewayMutationPayload): Promise<Gateway> {
  const path = payload.id ? `/gateways/${encodeURIComponent(payload.id)}` : '/gateways';
  const gateway = await apiRequest<Gateway>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
  return { ...gateway, version: Number(gateway.version), state: normalizeResourceState(gateway.state) };
}

export async function deleteGateway(id: string, version: number) {
  await apiRequest<Record<string, never>>(`/gateways/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}
