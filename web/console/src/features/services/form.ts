import type {
  ServiceEndpointPayload,
  ServiceLoadBalancePolicy,
  ServiceMutationPayload,
  ServiceResource,
  ServiceType,
  ServiceValidationItem,
  ServiceValidationReport,
} from '@/domain/service';
import { serviceLoadBalancePolicyLabel } from '@/domain/service';

export interface ServiceFormDraft {
  id?: string;
  version?: string;
  name: string;
  type: ServiceType;
  endpoints: ServiceEndpointPayload[];
  loadBalancePolicy: ServiceLoadBalancePolicy;
  healthCheck: {
    enabled: boolean;
    path: string;
    intervalSeconds: number;
    timeoutSeconds: number;
  };
}

export function createServiceDraft(service?: ServiceResource | null): ServiceFormDraft {
  return {
    id: service?.id,
    version: service?.version,
    name: service?.name ?? '',
    type: service?.type ?? 'application',
    endpoints: createEndpointsFromService(service),
    loadBalancePolicy: service?.loadBalancePolicy ?? 'round_robin',
    healthCheck: {
      enabled: service?.healthCheck?.enabled ?? true,
      path: service?.healthCheck?.path ?? '/healthz',
      intervalSeconds: service?.healthCheck?.intervalSeconds ?? 10,
      timeoutSeconds: service?.healthCheck?.timeoutSeconds ?? 2,
    },
  };
}

export function validateServiceDraft(draft: ServiceFormDraft): ServiceValidationReport {
  const endpointErrors = validateEndpoints(draft.endpoints);
  const enabledEndpoints = draft.endpoints.filter((endpoint) => endpoint.enabled);
  const healthInterval = draft.healthCheck.intervalSeconds;
  const healthTimeout = draft.healthCheck.timeoutSeconds;
  const items: ServiceValidationItem[] = [
    {
      label: '服务名称',
      status: draft.name.trim() ? 'healthy' : 'critical',
      message: draft.name.trim() ? draft.name.trim() : '请输入服务名称',
    },
    {
      label: '服务端点',
      status: endpointErrors.length === 0 && enabledEndpoints.length > 0 ? 'healthy' : 'critical',
      message: endpointErrors[0] ?? (enabledEndpoints.length > 0 ? `已配置 ${draft.endpoints.length} 个端点` : '至少保留一个启用端点'),
    },
    {
      label: '负载均衡',
      status: draft.loadBalancePolicy ? 'healthy' : 'critical',
      message: draft.loadBalancePolicy ? serviceLoadBalancePolicyLabel(draft.loadBalancePolicy) : '请选择负载均衡方式',
    },
    {
      label: '健康检查',
      status: validateHealthCheck(draft, healthInterval, healthTimeout),
      message: healthCheckMessage(draft, healthInterval, healthTimeout),
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '服务配置通过校验，可以保存。' : '服务配置还存在未完成项。',
    items,
  };
}

export function buildServicePayload(draft: ServiceFormDraft): ServiceMutationPayload {
  const endpoints = draft.endpoints.map((endpoint) => ({
    ...endpoint,
    address: endpoint.address.trim(),
  }));
  const healthCheck = draft.healthCheck.enabled
    ? {
      enabled: true,
      path: draft.healthCheck.path.trim(),
      intervalSeconds: draft.healthCheck.intervalSeconds,
      timeoutSeconds: draft.healthCheck.timeoutSeconds,
    }
    : { enabled: false };

  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    type: draft.type,
    endpoints,
    loadBalancePolicy: draft.loadBalancePolicy,
    healthCheck,
  };
}

export function createServiceEndpoint(): ServiceEndpointPayload {
  return {
    id: `endpoint-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    address: '',
    port: 80,
    weight: 100,
    enabled: true,
  };
}

export function formatEndpointSummary(endpoints: ServiceEndpointPayload[]) {
  const enabledEndpoints = endpoints.filter((endpoint) => endpoint.enabled);
  const visibleEndpoints = enabledEndpoints.length > 0 ? enabledEndpoints : endpoints;

  if (visibleEndpoints.length === 0) {
    return '-';
  }

  const first = visibleEndpoints[0];
  const suffix = visibleEndpoints.length > 1 ? ` 等 ${visibleEndpoints.length} 个端点` : '';

  return `${first.address}:${first.port}${suffix}`;
}

export function formatInstanceSummary(endpoints: ServiceEndpointPayload[]) {
  const enabledCount = endpoints.filter((endpoint) => endpoint.enabled).length;

  return `${enabledCount}/${endpoints.length}`;
}

function createEndpointsFromService(service?: ServiceResource | null): ServiceEndpointPayload[] {
  if (!service) {
    return [createServiceEndpoint()];
  }

  if (service.endpoints.length > 0) {
    return service.endpoints.map((endpoint) => ({ ...endpoint }));
  }
  return [createServiceEndpoint()];
}

function validateEndpoints(endpoints: ServiceEndpointPayload[]) {
  if (endpoints.length === 0) {
    return ['至少配置一个服务端点'];
  }

  return endpoints.flatMap((endpoint, index) => {
    const messages: string[] = [];
    const port = endpoint.port;
    const weight = endpoint.weight;

    if (!endpoint.address.trim()) {
      messages.push(`第 ${index + 1} 个端点缺少地址`);
    }

    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      messages.push(`第 ${index + 1} 个端点端口不合法`);
    }

    if (!Number.isInteger(weight) || weight < 1 || weight > 100) {
      messages.push(`第 ${index + 1} 个端点权重需要在 1-100 之间`);
    }

    return messages;
  });
}

function validateHealthCheck(draft: ServiceFormDraft, interval: number, timeout: number) {
  if (!draft.healthCheck.enabled) {
    return 'healthy';
  }

  if (!draft.healthCheck.path.trim().startsWith('/')) {
    return 'critical';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return 'critical';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return 'critical';
  }

  return 'healthy';
}

function healthCheckMessage(draft: ServiceFormDraft, interval: number, timeout: number) {
  if (!draft.healthCheck.enabled) {
    return '未启用健康检查';
  }

  if (!draft.healthCheck.path.trim().startsWith('/')) {
    return '探活路径必须以 / 开头';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return '检查间隔需要在 1-300 秒之间';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return '超时时间需要在 1-60 秒之间，并且小于检查间隔';
  }

  return `${draft.healthCheck.path} / ${interval}s / ${timeout}s`;
}
