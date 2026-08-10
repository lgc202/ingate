import type { ResourceStatus } from './common';

export type GatewayProtocol = 'GATEWAY_PROTOCOL_HTTP' | 'GATEWAY_PROTOCOL_HTTPS';

export interface GatewayListener {
  name: string;
  protocol: GatewayProtocol;
  port: number;
  hostname: string;
  certificateID: string;
}

export interface Gateway {
  id: string;
  name: string;
  enabled: boolean;
  listeners: GatewayListener[];
  state: ResourceStatus['state'];
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface GatewayListView {
  gateways: Gateway[];
}

export interface GatewayMutationPayload {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  listeners: GatewayListener[];
}

export function gatewayProtocolLabel(protocol: GatewayProtocol): string {
  return protocol === 'GATEWAY_PROTOCOL_HTTPS' ? 'HTTPS' : 'HTTP';
}
