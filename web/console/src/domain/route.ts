import type { ResourceStatus } from './common';
import type { ModelCatalogItem, ModelProvider, UpstreamProtocol } from './upstream';

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export interface RouteResource {
  id: string;
  version?: string;
  name: string;
  gatewayIDs: string[];
  hostnames: string[];
  rules: RouteRule[];
  enabled: boolean;
  status: ResourceStatus;
  createdAt: string;
}

export interface RouteWorkspace {
  routes: RouteResource[];
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
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
  type: string;
  protocol: UpstreamProtocol;
  provider?: ModelProvider;
  models: ModelCatalogItem[];
  endpoint?: string;
  meta: string;
}

export interface RouteRule {
  name: string;
  pathPrefix: string;
  methods: HttpMethod[];
  headers: HeaderMatch[];
  upstreams: WeightedUpstream[];
  modelRouting?: ModelRouting;
  requestHeaderModifier?: HeaderModifier;
  responseHeaderModifier?: HeaderModifier;
  timeout?: RouteTimeout;
  retry?: RouteRetry;
}

export interface ModelRouting {
  models: ModelRoute[];
}

export interface ModelRoute {
  model: string;
  upstreamID: string;
  upstreamModel?: string;
}

export interface WeightedUpstream {
  upstreamID: string;
  weight: number;
}

export interface HeaderMatch {
  name: string;
  value: string;
}

export interface HeaderModifier {
  set: HeaderValue[];
  remove: string[];
}

export interface HeaderValue {
  name: string;
  value: string;
}

export interface RouteTimeout {
  requestMillis: number;
}

export interface RouteRetry {
  attempts: number;
  perTryTimeoutMillis: number;
}

export interface RouteMutationPayload {
  id?: string;
  version?: string;
  name: string;
  gatewayIDs: string[];
  hostnames: string[];
  enabled: boolean;
  rules: RouteRulePayload[];
}

// RouteRulePayload 只在 Console API 边界保留 targets 字段名
export interface RouteRulePayload extends Omit<RouteRule, 'upstreams'> {
  targets?: WeightedUpstream[];
}

export interface RouteActionResult {
  message: string;
  changeId?: string;
}

export function isModelRoute(route: Pick<RouteResource, 'rules'>) {
  return route.rules.some((rule) => Boolean(rule.modelRouting?.models.length));
}
