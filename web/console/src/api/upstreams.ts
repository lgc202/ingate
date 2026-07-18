import { apiRequest } from './client';
import type { UpstreamList, UpstreamMutationPayload, UpstreamMutationResult } from '@/domain/upstream';

interface UpstreamMutationResponse {
  success: boolean;
  id?: string;
}

export function listUpstreams() {
  return apiRequest<UpstreamList>('/upstreams');
}

export async function saveUpstream(payload: UpstreamMutationPayload): Promise<UpstreamMutationResult> {
  const path = payload.id ? `/upstreams/${encodeURIComponent(payload.id)}` : '/upstreams';
  const response = await apiRequest<UpstreamMutationResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });

  return {
    message: `服务已保存：${payload.name}`,
    changeId: response.id ?? payload.id,
  };
}

export async function deleteUpstream(id: string) {
  await apiRequest<UpstreamMutationResponse>(`/upstreams/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
