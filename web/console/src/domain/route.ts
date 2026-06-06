import type { HealthStatus, RuntimeSyncStatus } from './common';

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
export type RoutePolicyCapability = 'RequestHeaderModifier' | 'ResponseHeaderModifier' | 'Timeout' | 'Retry';

export const routePolicyCapabilityRequestHeaderModifier: RoutePolicyCapability = 'RequestHeaderModifier';
export const routePolicyCapabilityResponseHeaderModifier: RoutePolicyCapability = 'ResponseHeaderModifier';
export const routePolicyCapabilityTimeout: RoutePolicyCapability = 'Timeout';
export const routePolicyCapabilityRetry: RoutePolicyCapability = 'Retry';

export interface RouteResource {
  id: string;
  version?: string;
  gatewayIDs: string[];
  hostnames: string[];
  rules: RouteRule[];
  policyCount: number;
  traffic: string;
  successRate: string;
  enabled: boolean;
  runtimeStatus: RuntimeSyncStatus;
  createdAt: string;
}

export interface RouteComposerPreview {
  methods: HttpMethod[];
  path: string;
  gatewayIDs: string[];
  gateways: RouteGatewayOption[];
  hostnames: string[];
  policyCount: number;
  validations: string[];
  targets: RouteTargetOption[];
  policies: RoutePolicyOption[];
}

export interface RouteListView {
  routes: RouteResource[];
}

export interface RouteGatewayOption {
  id: string;
  name: string;
}

export interface RouteTargetOption {
  id: string;
  name: string;
  type: string;
  endpoint?: string;
  meta: string;
  healthStatus: HealthStatus;
}

export interface RoutePolicyOption {
  capability: RoutePolicyCapability;
  displayName: string;
  meta: string;
  enabled: boolean;
  params: RoutePolicyParam[];
}

export interface RoutePolicyCapabilities {
  policies: RoutePolicyOption[];
}

export interface RoutePolicyParam {
  key: string;
  label: string;
  defaultValue: string;
  inputType?: 'text' | 'number' | 'select' | 'multiselect';
  placeholder?: string;
  required?: boolean;
  options?: string[];
  unit?: string;
  min?: number;
  max?: number;
}

export interface RoutePublishPreview {
  title: string;
  subtitle: string;
  diffs: {
    before: string;
    after: string;
  }[];
}

export interface RoutePageView {
  routes: RouteResource[];
  composer: RouteComposerPreview;
}

export interface RouteMutationPayload {
  id?: string;
  version?: string;
  gatewayIDs: string[];
  hostnames: string[];
  enabled: boolean;
  rules: RouteRule[];
}

export interface RouteTargetPayload {
  upstreamID: string;
  weight: number;
}

export interface RouteRule {
  name: string;
  pathPrefix: string;
  methods: HttpMethod[];
  headers?: HeaderMatch[];
  targets: RouteTargetPayload[];
  requestHeaderModifier?: HeaderModifier;
  responseHeaderModifier?: HeaderModifier;
  timeout?: RouteTimeout;
  retry?: RouteRetry;
}

export interface HeaderMatch {
  name: string;
  value: string;
}

export interface HeaderModifier {
  set?: HeaderValue[];
  remove?: string[];
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

export type RoutePublishPayload = RouteMutationPayload;

export interface RouteActionResult {
  message: string;
  changeId?: string;
}

export interface RouteValidationItem {
  label: string;
  status: HealthStatus;
  message: string;
}

export interface RouteValidationReport {
  valid: boolean;
  summary: string;
  items: RouteValidationItem[];
}
