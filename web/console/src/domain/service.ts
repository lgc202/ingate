import type { ResourceStatus } from './common';

export type ServiceLoadBalancing =
  | 'LOAD_BALANCING_POLICY_ROUND_ROBIN'
  | 'LOAD_BALANCING_POLICY_LEAST_REQUEST';
export type ModelProtocol = 'MODEL_PROTOCOL_OPENAI' | 'MODEL_PROTOCOL_ANTHROPIC';
export type ServiceType = 'HTTP' | 'MODEL';

export const serviceLoadBalancingOptions: Array<{ value: ServiceLoadBalancing; label: string }> = [
  { value: 'LOAD_BALANCING_POLICY_ROUND_ROBIN', label: '轮询' },
  { value: 'LOAD_BALANCING_POLICY_LEAST_REQUEST', label: '最少请求' },
];

export interface ServiceEndpoint {
  address: string;
  port: number;
  weight: number;
}

export interface ServiceTLS {
  serverName: string;
}

export interface ServiceHealthCheck {
  path: string;
  intervalSeconds: number;
  timeoutSeconds: number;
}

export interface ModelService {
  protocol: ModelProtocol;
  apiKeyConfigured: boolean;
}

export interface ModelServiceInput {
  protocol: ModelProtocol;
  apiKey?: string;
  clearApiKey?: boolean;
}

export interface Service {
  id: string;
  name: string;
  endpoints: ServiceEndpoint[];
  tls?: ServiceTLS;
  loadBalancing: ServiceLoadBalancing;
  healthCheck?: ServiceHealthCheck;
  model?: ModelService;
  state: ResourceStatus['state'];
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface ServiceList {
  services: Service[];
}

export interface ServiceMutationPayload {
  id?: string;
  version?: number;
  name: string;
  endpoints: ServiceEndpoint[];
  tls?: ServiceTLS;
  loadBalancing: ServiceLoadBalancing;
  healthCheck?: ServiceHealthCheck;
  model?: ModelServiceInput;
}

export function modelProtocolLabel(value: ModelProtocol): string {
  switch (value) {
    case 'MODEL_PROTOCOL_OPENAI':
      return 'OpenAI 兼容';
    case 'MODEL_PROTOCOL_ANTHROPIC':
      return 'Anthropic Messages';
  }
}

export function serviceLoadBalancingLabel(value: ServiceLoadBalancing): string {
  return serviceLoadBalancingOptions.find((option) => option.value === value)?.label ?? value;
}
