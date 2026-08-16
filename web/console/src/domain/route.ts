import type { ResourceStatus } from './common';

export type HttpMethod = 'GET' | 'HEAD' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'OPTIONS';
export type RoutePathMatchType = 'ROUTE_PATH_MATCH_PREFIX' | 'ROUTE_PATH_MATCH_EXACT';
export type HostRewriteMode = 'HOST_REWRITE_MODE_SERVICE_ADDRESS' | 'HOST_REWRITE_MODE_PRESERVE' | 'HOST_REWRITE_MODE_CUSTOM';

export interface HeaderMatch {
  name: string;
  value: string;
}

export interface HeaderValue {
  name: string;
  value: string;
}

export interface HeaderModifier {
  set: HeaderValue[];
  add: HeaderValue[];
  remove: string[];
}

export interface WeightedUpstream {
  upstreamID: string;
  weight: number;
}

export interface HostRewrite {
  mode: HostRewriteMode;
  hostname?: string;
}

export interface RouteResource {
  id: string;
  name: string;
  enabled: boolean;
  gatewayIDs: string[];
  hostnames: string[];
  match: {
    path: { type: RoutePathMatchType; value: string };
    methods: HttpMethod[];
    headers: HeaderMatch[];
  };
  upstreams: WeightedUpstream[];
  hostRewrite: HostRewrite;
  requestHeaderModifier?: HeaderModifier;
  responseHeaderModifier?: HeaderModifier;
  timeout?: { requestMillis: number };
  retry?: { attempts: number; perTryTimeoutMillis: number };
  state: ResourceStatus['state'];
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface RouteListView {
  routes: RouteResource[];
}

export interface RouteGatewayOption {
  id: string;
  name: string;
}

export interface UpstreamOption {
  id: string;
  name: string;
  endpoint: string;
}

export interface RouteWorkspace extends RouteListView {
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
}

export interface RouteMutationPayload {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  gatewayIDs: string[];
  hostnames: string[];
  match: RouteResource['match'];
  upstreams: WeightedUpstream[];
  hostRewrite: HostRewrite;
  requestHeaderModifier?: HeaderModifier;
  responseHeaderModifier?: HeaderModifier;
  timeout?: RouteResource['timeout'];
  retry?: RouteResource['retry'];
}
