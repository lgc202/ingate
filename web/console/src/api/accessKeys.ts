import { apiRequest } from './client';
import type {
  AccessKeyList,
  AccessKeyMutationPayload,
  AccessKeyMutationResponse,
  AccessKeySecretResponse,
} from '@/domain/accessKey';

export function listAccessKeys(): Promise<AccessKeyList> {
  return apiRequest<AccessKeyList>('/access-keys');
}

export function createAccessKey(payload: AccessKeyMutationPayload): Promise<AccessKeySecretResponse> {
  return apiRequest<AccessKeySecretResponse>('/access-keys', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateAccessKey(id: string, payload: AccessKeyMutationPayload): Promise<AccessKeyMutationResponse> {
  return apiRequest<AccessKeyMutationResponse>(`/access-keys/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function setAccessKeyEnabled(id: string, enabled: boolean): Promise<AccessKeyMutationResponse> {
  return apiRequest<AccessKeyMutationResponse>(`/access-keys/${encodeURIComponent(id)}/enabled`, {
    method: 'PATCH',
    body: JSON.stringify({ enabled }),
  });
}

export function rotateAccessKey(id: string): Promise<AccessKeySecretResponse> {
  return apiRequest<AccessKeySecretResponse>(`/access-keys/${encodeURIComponent(id)}/rotate`, {
    method: 'POST',
  });
}

export function deleteAccessKey(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/access-keys/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}
