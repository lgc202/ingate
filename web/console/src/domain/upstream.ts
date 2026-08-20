import type { ResourceStatus } from './common';

export type UpstreamLoadBalancing = 'LOAD_BALANCING_POLICY_ROUND_ROBIN' | 'LOAD_BALANCING_POLICY_LEAST_REQUEST';
export type ModelProtocol = 'MODEL_PROTOCOL_OPENAI' | 'MODEL_PROTOCOL_ANTHROPIC';
export type UpstreamType = 'HTTP' | 'MODEL';

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

export interface ModelUpstream {
  protocol: ModelProtocol;
  apiKeyConfigured: boolean;
}

export interface ModelUpstreamInput {
  protocol: ModelProtocol;
  apiKey?: string;
  clearApiKey?: boolean;
}

export interface Upstream {
  id: string;
  name: string;
  endpoints: UpstreamEndpoint[];
  tls?: UpstreamTLS;
  loadBalancing: UpstreamLoadBalancing;
  healthCheck?: UpstreamHealthCheck;
  model?: ModelUpstream;
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
  model?: ModelUpstreamInput;
}

export function modelProtocolLabel(value: ModelProtocol): string {
  switch (value) {
    case 'MODEL_PROTOCOL_OPENAI': return 'OpenAI 兼容';
    case 'MODEL_PROTOCOL_ANTHROPIC': return 'Anthropic Messages';
  }
}

export function upstreamLoadBalancingLabel(value: UpstreamLoadBalancing): string {
  return upstreamLoadBalancingOptions.find((option) => option.value === value)?.label ?? value;
}
