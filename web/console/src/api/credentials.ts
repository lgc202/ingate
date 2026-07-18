import { apiRequest } from './client';
import type {
  UpstreamCredentialList,
  UpstreamCredentialMutationPayload,
  UpstreamCredentialMutationResult,
} from '@/domain/credential';

interface UpstreamCredentialMutationResponse {
  success: boolean;
  id?: string;
}

export async function listUpstreamCredentials(): Promise<UpstreamCredentialList> {
  const response = await apiRequest<Partial<UpstreamCredentialList>>('/upstream-credentials');
  return { credentials: response.credentials ?? [] };
}

export async function saveUpstreamCredential(
  payload: UpstreamCredentialMutationPayload,
): Promise<UpstreamCredentialMutationResult> {
  const path = payload.id
    ? `/upstream-credentials/${encodeURIComponent(payload.id)}`
    : '/upstream-credentials';
  const response = await apiRequest<UpstreamCredentialMutationResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });

  return {
    message: `访问凭据已保存：${payload.name}`,
    id: response.id ?? payload.id,
  };
}

export async function deleteUpstreamCredential(id: string) {
  await apiRequest<UpstreamCredentialMutationResponse>(
    `/upstream-credentials/${encodeURIComponent(id)}`,
    { method: 'DELETE' },
  );
}
