import type { HealthStatus } from './common';

export interface Gateway {
  id: string;
  version?: string;
  name: string;
  description: string;
  listeners: GatewayListener[];
  hostBindings: GatewayHostBinding[];
  enabled: boolean;
  createdAt: string;
}

export type GatewayListenerProtocol = 'HTTP';

export interface GatewayListener {
  name: string;
  protocol: GatewayListenerProtocol;
  port: number;
}

export interface GatewayHostBinding {
  hostname?: string;
  listenerRefs: string[];
}

export interface GatewayListView {
  gateways: Gateway[];
}

export interface GatewayMutationPayload {
  id?: string;
  version?: string;
  name: string;
  description: string;
  listeners: GatewayListener[];
  hostBindings: GatewayHostBinding[];
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
