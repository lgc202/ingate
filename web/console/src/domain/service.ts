export type ServiceType = 'application' | 'model' | 'agent' | 'mcp';
export type ServiceLoadBalancePolicy = 'round_robin' | 'least_request' | 'random';

export const serviceTypeOptions: { value: ServiceType; label: string }[] = [
  { value: 'application', label: '应用服务' },
  { value: 'model', label: '模型服务' },
  { value: 'agent', label: 'Agent 服务' },
  { value: 'mcp', label: 'MCP 服务' },
];

export const serviceLoadBalancePolicyOptions: { value: ServiceLoadBalancePolicy; label: string }[] = [
  { value: 'round_robin', label: '轮询' },
  { value: 'least_request', label: '最少请求' },
  { value: 'random', label: '随机' },
];

export function serviceTypeLabel(type: ServiceType | string): string {
  return serviceTypeOptions.find((option) => option.value === type)?.label ?? type;
}

export function serviceLoadBalancePolicyLabel(policy: ServiceLoadBalancePolicy | string): string {
  return serviceLoadBalancePolicyOptions.find((option) => option.value === policy)?.label ?? policy;
}

export interface ServiceResource {
  id: string;
  version?: string;
  name: string;
  type: ServiceType;
  endpoints: ServiceEndpointPayload[];
  loadBalancePolicy: ServiceLoadBalancePolicy;
  healthCheck?: ServiceHealthCheck;
  createdAt: string;
}

export interface ServiceListView {
  upstreams: ServiceResource[];
}

export interface ServiceMutationPayload {
  id?: string;
  version?: string;
  name: string;
  type: ServiceType;
  endpoints: ServiceEndpointPayload[];
  loadBalancePolicy: ServiceLoadBalancePolicy;
  healthCheck?: ServiceHealthCheck;
}

export interface ServiceEndpointPayload {
  id: string;
  address: string;
  port: number;
  weight: number;
  enabled: boolean;
}

export interface ServiceHealthCheck {
  enabled: boolean;
  path?: string;
  intervalSeconds?: number;
  timeoutSeconds?: number;
}

export interface ServiceMutationPreview {
  title: string;
  subtitle: string;
  diffs: {
    before: string;
    after: string;
  }[];
}

export interface ServiceMutationResult {
  message: string;
  changeId?: string;
}

export interface ServiceValidationItem {
  label: string;
  status: 'healthy' | 'warning' | 'critical' | 'unknown';
  message: string;
}

export interface ServiceValidationReport {
  valid: boolean;
  summary: string;
  items: ServiceValidationItem[];
}
