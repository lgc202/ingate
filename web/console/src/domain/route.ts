import type { HealthStatus, RuntimeSyncStatus } from './common';

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
export type RoutePolicyCapability = 'RequestHeaderModifier' | 'Timeout' | 'Retry';
export type RoutePolicySource = 'RouteNative';

export const routePolicyCapabilityRequestHeaderModifier: RoutePolicyCapability = 'RequestHeaderModifier';
export const routePolicyCapabilityTimeout: RoutePolicyCapability = 'Timeout';
export const routePolicyCapabilityRetry: RoutePolicyCapability = 'Retry';

export interface RouteResource {
  id: string;
  version?: string;
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  targets?: RouteTargetPayload[];
  policyCount: number;
  policyBindings?: RoutePolicyBindingPayload[];
  traffic: string;
  successRate: string;
  enabled: boolean;
  runtimeStatus: RuntimeSyncStatus;
  lastChangedAt: string;
}

export interface RouteComposerPreview {
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  policyCount: number;
  rateLimit: string;
  validations: string[];
  targets: RouteTargetOption[];
  policies: RoutePolicyOption[];
}

export interface RouteListView {
  routes: RouteResource[];
}

export interface RouteTargetOption {
  name: string;
  type: string;
  endpoint?: string;
  meta: string;
  healthStatus: HealthStatus;
  referencedRoutes?: number;
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
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  targets: RouteTargetPayload[];
  enabled: boolean;
  policyBindings: RoutePolicyBindingPayload[];
}

export interface RouteTargetPayload {
  name: string;
  weight: number;
}

export interface RoutePolicyBindingPayload {
  capability: RoutePolicyCapability;
  source: RoutePolicySource;
  parameters: Record<string, string | string[]>;
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
