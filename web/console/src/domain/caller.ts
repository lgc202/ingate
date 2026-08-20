export interface AccessKey {
  id: string;
  name: string;
  enabled: boolean;
  createdAt: string;
  expiresAt?: string;
}

export interface Caller {
  id: string;
  name: string;
  enabled: boolean;
  routeIDs: string[];
  accessKeys: AccessKey[];
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface CallerRouteOption {
  id: string;
  name: string;
}

export interface CallerWorkspace {
  callers: Caller[];
  routes: CallerRouteOption[];
}

export interface CallerMutationPayload {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  routeIDs: string[];
  accessKeyName?: string;
  accessKeyExpiresAt?: string;
}

export interface IssuedAccessKey {
  accessKey: AccessKey;
  secret: string;
}

export interface CreateCallerResult {
  caller: Caller;
  issuedAccessKey: IssuedAccessKey;
}
