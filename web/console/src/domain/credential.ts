import type { ResourceStatus } from './common';

export type UpstreamCredentialType = 'APIKey';

export interface UpstreamCredential {
  id: string;
  version?: string;
  status: ResourceStatus;
  name: string;
  type: UpstreamCredentialType;
  configured: boolean;
  createdAt: string;
}

export interface UpstreamCredentialList {
  credentials: UpstreamCredential[];
}

export interface UpstreamCredentialMutationPayload {
  id?: string;
  version?: string;
  name: string;
  type: UpstreamCredentialType;
  apiKey?: {
    value: string;
  };
}

export interface UpstreamCredentialMutationResult {
  message: string;
  id?: string;
}

export function upstreamCredentialTypeLabel(type: UpstreamCredentialType | string): string {
  return type === 'APIKey' ? 'API Key' : type;
}
