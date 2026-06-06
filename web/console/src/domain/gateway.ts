import type { HealthStatus } from './common';

export interface Gateway {
  id: string;
  version?: string;
  name: string;
  description: string;
  runtimeGroup: string;
  runtimeGroupName: string;
  listenerSummary: string;
  hostBindingSummary: string;
  listeners: GatewayListener[];
  hostBindings: GatewayHostBinding[];
  enabled: boolean;
  healthStatus: HealthStatus;
  createdAt: string;
}

export type GatewayListenerProtocol = 'HTTP' | 'HTTPS';

export interface GatewayListener {
  name: string;
  protocol: GatewayListenerProtocol;
  port: number;
  certificateId?: string;
}

export interface GatewayHostBinding {
  hostname?: string;
  listenerRefs: string[];
  tls?: GatewayTLS;
}

export interface GatewayTLS {
  certificateRef?: string;
}

export interface GatewayCertificateOption {
  id: string;
  name: string;
  domains: string[];
  expiresAt: string;
  status: HealthStatus;
}

export interface GatewayRuntimeGroupOption {
  id: string;
  name: string;
}

export interface GatewayListView {
  gateways: Gateway[];
}

export interface GatewayWorkspace {
  gateways: Gateway[];
  runtimeGroups: GatewayRuntimeGroupOption[];
  certificates: GatewayCertificateOption[];
}

export interface GatewayMutationPayload {
  id?: string;
  version?: string;
  name: string;
  description: string;
  runtimeGroup: string;
  listeners: GatewayListener[];
  hostBindings: GatewayHostBinding[];
}

export interface GatewayMutationPreview {
  title: string;
  subtitle: string;
  diffs: {
    before: string;
    after: string;
  }[];
}

export interface GatewayMutationResult {
  message: string;
  changeId?: string;
}

export interface GatewayValidationItem {
  label: string;
  status: HealthStatus;
  message: string;
}

export interface GatewayValidationReport {
  valid: boolean;
  summary: string;
  items: GatewayValidationItem[];
}
