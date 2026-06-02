import type { HealthStatus, RuntimeSyncStatus } from './common';

export interface Gateway {
  id: string;
  version?: string;
  name: string;
  description: string;
  runtimeGroupId: string;
  runtimeGroupName: string;
  listeners: string;
  listenerItems: GatewayListener[];
  hostPolicy: string;
  hostnames: string[];
  routeCount: number;
  serviceCount: number;
  enabled: boolean;
  runtimeStatus: RuntimeSyncStatus;
  healthStatus: HealthStatus;
  latestSnapshotVersion?: string;
  lastChangedAt: string;
}

export type GatewayListenerProtocol = 'HTTP' | 'HTTPS';

export interface GatewayListener {
  id: string;
  protocol: GatewayListenerProtocol;
  port: string;
  certificateId?: string;
  certificateName?: string;
}

export interface GatewayCertificateOption {
  id: string;
  name: string;
  domains: string[];
  expiresAt: string;
  status: HealthStatus;
}

export interface GatewayListView {
  gateways: Gateway[];
  certificates: GatewayCertificateOption[];
}

export interface GatewayMutationPayload {
  id?: string;
  version?: string;
  name: string;
  description: string;
  runtimeGroupId: string;
  runtimeGroupName: string;
  listeners: GatewayListener[];
  hostnames: string[];
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
