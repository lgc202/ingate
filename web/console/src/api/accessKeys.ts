import { apiListAll, apiRequest } from './client';
import type { PagedResponse } from './client';
import type {
  AccessKey,
  AccessKeyList,
  AccessKeyMutationPayload,
  AccessKeyMutationResponse,
  AccessKeySecretResponse,
  AccessKeyUpdatePayload,
} from '@/domain/accessKey';

interface AccessKeyListResponse extends AccessKeyList, PagedResponse {}

export async function listAccessKeys(): Promise<AccessKeyList> {
  const accessKeys = await apiListAll<AccessKeyListResponse, AccessKey>(
    '/access-keys',
    (page) => page.accessKeys ?? [],
  );
  return { accessKeys };
}

export function createAccessKey(payload: AccessKeyMutationPayload): Promise<AccessKeySecretResponse> {
  return apiRequest<AccessKeySecretResponse>('/access-keys', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateAccessKey(id: string, payload: AccessKeyUpdatePayload): Promise<AccessKeyMutationResponse> {
  return apiRequest<AccessKeyMutationResponse>(`/access-keys/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function setAccessKeyEnabled(id: string, version: string, enabled: boolean): Promise<AccessKeyMutationResponse> {
  return apiRequest<AccessKeyMutationResponse>(`/access-keys/${encodeURIComponent(id)}/enabled`, {
    method: 'PATCH',
    body: JSON.stringify({ version, enabled }),
  });
}

export function rotateAccessKey(id: string, version: string): Promise<AccessKeySecretResponse> {
  return apiRequest<AccessKeySecretResponse>(`/access-keys/${encodeURIComponent(id)}/rotate`, {
    method: 'POST',
    body: JSON.stringify({ version }),
  });
}

export function deleteAccessKey(id: string, version: string): Promise<{ success: boolean }> {
  const query = new URLSearchParams({ version });
  return apiRequest<{ success: boolean }>(`/access-keys/${encodeURIComponent(id)}?${query}`, {
    method: 'DELETE',
  });
}
