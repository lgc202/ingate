import { apiListAll, apiRequest } from './client';
import type { PagedResponse } from './client';
import type { GatewayListView, GatewayMutationPayload, GatewayMutationResult } from '@/domain/gateway';

interface GatewayMutationResponse {
  success: boolean;
  id?: string;
}

interface GatewayListResponse extends PagedResponse {
  gateways?: Array<Omit<GatewayListView['gateways'][number], 'hostnames' | 'listeners'> & {
    hostnames?: string[];
    listeners?: GatewayListView['gateways'][number]['listeners'];
  }>;
}

export async function listGateways(): Promise<GatewayListView> {
  const gateways = await apiListAll<GatewayListResponse, NonNullable<GatewayListResponse['gateways']>[number]>(
    '/gateways',
    (page) => page.gateways ?? [],
  );
  return {
    gateways: gateways.map((gateway) => ({
      ...gateway,
      hostnames: gateway.hostnames ?? [],
      listeners: gateway.listeners ?? [],
    })),
  };
}

export async function saveGateway(payload: GatewayMutationPayload): Promise<GatewayMutationResult> {
  const path = payload.id ? `/gateways/${encodeURIComponent(payload.id)}` : '/gateways';
  const response = await apiRequest<GatewayMutationResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });

  return {
    message: `网关已保存：${payload.name}`,
    changeId: response.id ?? payload.id,
  };
}

export async function deleteGateway(id: string) {
  await apiRequest<GatewayMutationResponse>(`/gateways/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function setGatewayEnabled(id: string, enabled: boolean) {
  await apiRequest<GatewayMutationResponse>(`/gateways/${encodeURIComponent(id)}/enabled`, {
    method: 'PATCH',
    body: JSON.stringify({ enabled }),
  });
}
