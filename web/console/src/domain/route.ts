import type { HealthStatus, KeyValue, RuntimeSyncStatus } from './common';

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export interface RouteResource {
  id: string;
  version?: string;
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  policyCount: number;
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

export interface RouteTargetOption {
  name: string;
  type: string;
  endpoint?: string;
  meta: string;
  healthStatus: HealthStatus;
  referencedRoutes?: number;
}

export interface RoutePolicyOption {
  name: string;
  meta: string;
  enabled: boolean;
  params: RoutePolicyParam[];
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

export interface RouteDetailView {
  title: string;
  tabs: Record<string, KeyValue[]>;
}

export interface RoutePageView {
  routes: RouteResource[];
  composer: RouteComposerPreview;
  publishPreview: RoutePublishPreview;
  detail: RouteDetailView;
}

export interface RouteMutationPayload {
  id?: string;
  version?: string;
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  enabled: boolean;
  policyBindings: RoutePolicyBindingPayload[];
}

export interface RoutePolicyBindingPayload {
  policyName: string;
  source: 'route';
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
