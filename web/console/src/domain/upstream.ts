import type { ResourceStatus } from './common';

export type UpstreamType = 'application' | 'model' | 'agent' | 'mcp';
export type UpstreamLoadBalancePolicy = 'round_robin' | 'least_request' | 'random';

export const upstreamTypeOptions: { value: UpstreamType; label: string }[] = [
  { value: 'application', label: '应用服务' },
  { value: 'model', label: '大模型' },
  { value: 'agent', label: 'Agent' },
  { value: 'mcp', label: 'MCP' },
];

export const upstreamLoadBalancePolicyOptions: { value: UpstreamLoadBalancePolicy; label: string }[] = [
  { value: 'round_robin', label: '轮询' },
  { value: 'least_request', label: '最少请求' },
  { value: 'random', label: '随机' },
];

export function upstreamTypeLabel(type: UpstreamType | string): string {
  return upstreamTypeOptions.find((option) => option.value === type)?.label ?? type;
}

export function upstreamLoadBalancePolicyLabel(policy: UpstreamLoadBalancePolicy | string): string {
  return upstreamLoadBalancePolicyOptions.find((option) => option.value === policy)?.label ?? policy;
}

export interface Upstream {
  id: string;
  version?: string;
  name: string;
  type: UpstreamType;
  endpoints: UpstreamEndpoint[];
  loadBalancePolicy: UpstreamLoadBalancePolicy;
  healthCheck?: UpstreamHealthCheck;
  status: ResourceStatus;
  createdAt: string;
}

export interface UpstreamList {
  upstreams: Upstream[];
}

export interface UpstreamMutationPayload {
  id?: string;
  version?: string;
  name: string;
  type: UpstreamType;
  endpoints: UpstreamEndpoint[];
  loadBalancePolicy: UpstreamLoadBalancePolicy;
  healthCheck?: UpstreamHealthCheck;
}

export interface UpstreamEndpoint {
  id: string;
  address: string;
  port: number;
  weight: number;
  enabled: boolean;
}

export interface UpstreamHealthCheck {
  enabled: boolean;
  path?: string;
  intervalSeconds?: number;
  timeoutSeconds?: number;
}

export interface UpstreamMutationResult {
  message: string;
  changeId?: string;
}
