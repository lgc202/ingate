import type { HealthStatus, ResourceStatus } from './common';

export type GatewayProtocol = 'HTTP' | 'HTTPS';

export interface GatewayListener {
  protocol: GatewayProtocol;
  port: number;
  certificateID?: string;
}

export interface Gateway {
  id: string;
  version?: string;
  name: string;
  description: string;
  listeners: GatewayListener[];
  hostnames: string[];
  enabled: boolean;
  status: ResourceStatus;
  createdAt: string;
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
  hostnames: string[];
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
