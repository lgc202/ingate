import type { CountSegment, HealthStatus, RuntimeSyncStatus } from './common';

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
  endpoint: string;
  endpoints?: ServiceEndpointPayload[];
  instances: string;
  healthStatus: HealthStatus;
  runtimeStatus: RuntimeSyncStatus;
  referencedRoutes: number;
  traffic: string;
  successRate: string;
  lastUpdatedAt: string;
}

export interface ServiceIncident {
  serviceName: string;
  description: string;
  time: string;
  status: HealthStatus;
}

export interface ServiceListView {
  services: ServiceResource[];
  health: CountSegment[];
  incidents: ServiceIncident[];
}

export interface ServiceMutationPayload {
  id?: string;
  version?: string;
  name: string;
  type: ServiceType;
  endpoint: string;
  instances: string;
  endpoints: ServiceEndpointPayload[];
  loadBalancePolicy: ServiceLoadBalancePolicy;
  healthCheckEnabled: boolean;
  healthCheckPath: string;
  healthCheckIntervalSeconds: string;
  healthCheckTimeoutSeconds: string;
}

export interface ServiceEndpointPayload {
  id: string;
  address: string;
  port: string;
  weight: string;
  enabled: boolean;
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
  status: HealthStatus;
  message: string;
}

export interface ServiceValidationReport {
  valid: boolean;
  summary: string;
  items: ServiceValidationItem[];
}
