export type AccessKeyStatus = 'active' | 'disabled' | 'expired';

export interface AccessKey {
  id: string;
  name: string;
  prefix: string;
  suffix: string;
  enabled: boolean;
  allowedModels: string[];
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt: string;
}

export interface AccessKeyList {
  accessKeys: AccessKey[];
}

export interface AccessKeyMutationPayload {
  name: string;
  allowedModels: string[];
  expiresAt?: string;
}

export interface AccessKeyMutationResponse {
  accessKey: AccessKey;
}

export interface AccessKeySecretResponse {
  accessKey: AccessKey;
  secret: string;
}

export function getAccessKeyStatus(key: AccessKey): AccessKeyStatus {
  if (!key.enabled) {
    return 'disabled';
  }
  if (key.expiresAt) {
    const expireTime = new Date(key.expiresAt).getTime();
    if (!Number.isNaN(expireTime) && expireTime <= Date.now()) {
      return 'expired';
    }
  }
  return 'active';
}

export function formatMaskedKey(prefix: string, suffix: string): string {
  const p = prefix || '';
  const s = suffix || '';
  return `${p}••••${s}`;
}
