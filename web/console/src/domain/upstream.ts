import type { ResourceStatus } from './common';

export type UpstreamLoadBalancing = 'LOAD_BALANCING_POLICY_ROUND_ROBIN' | 'LOAD_BALANCING_POLICY_LEAST_REQUEST';

export const upstreamLoadBalancingOptions: Array<{ value: UpstreamLoadBalancing; label: string }> = [
  { value: 'LOAD_BALANCING_POLICY_ROUND_ROBIN', label: '轮询' },
  { value: 'LOAD_BALANCING_POLICY_LEAST_REQUEST', label: '最少请求' },
];

export interface UpstreamEndpoint {
  address: string;
  port: number;
  weight: number;
}

export interface UpstreamTLS {
  serverName: string;
}

export interface UpstreamHealthCheck {
  path: string;
  intervalSeconds: number;
  timeoutSeconds: number;
}

export interface Upstream {
  id: string;
  name: string;
  endpoints: UpstreamEndpoint[];
  tls?: UpstreamTLS;
  loadBalancing: UpstreamLoadBalancing;
  healthCheck?: UpstreamHealthCheck;
  state: ResourceStatus['state'];
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface UpstreamList {
  upstreams: Upstream[];
}

export interface UpstreamMutationPayload {
  id?: string;
  version?: number;
  name: string;
  endpoints: UpstreamEndpoint[];
  tls?: UpstreamTLS;
  loadBalancing: UpstreamLoadBalancing;
  healthCheck?: UpstreamHealthCheck;
}

export function upstreamLoadBalancingLabel(value: UpstreamLoadBalancing): string {
  return upstreamLoadBalancingOptions.find((option) => option.value === value)?.label ?? value;
}
