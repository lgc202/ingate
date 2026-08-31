import type {
  Service,
  ServiceEndpoint,
  ServiceLoadBalancing,
  ServiceMutationPayload,
  ServiceType,
  ModelProtocol,
} from '@/domain/service';

export interface ServiceDraft {
  id?: string;
  version?: number;
  name: string;
  endpoints: ServiceEndpoint[];
  httpsEnabled: boolean;
  serverName: string;
  loadBalancing: ServiceLoadBalancing;
  healthCheckEnabled: boolean;
  healthCheckPath: string;
  healthCheckInterval: number;
  healthCheckTimeout: number;
  type: ServiceType;
  modelProtocol: ModelProtocol;
  apiKey: string;
  clearApiKey: boolean;
  apiKeyConfigured: boolean;
}

export function createServiceDraft(service?: Service): ServiceDraft {
  return {
    id: service?.id,
    version: service?.version,
    name: service?.name ?? '',
    endpoints: service?.endpoints.map((endpoint) => ({ ...endpoint })) ?? [
      { address: '', port: 0, weight: 1 },
    ],
    httpsEnabled: Boolean(service?.tls),
    serverName: service?.tls?.serverName ?? '',
    loadBalancing: service?.loadBalancing ?? 'LOAD_BALANCING_POLICY_ROUND_ROBIN',
    healthCheckEnabled: Boolean(service?.healthCheck),
    healthCheckPath: service?.healthCheck?.path ?? '/healthz',
    healthCheckInterval: service?.healthCheck?.intervalSeconds ?? 10,
    healthCheckTimeout: service?.healthCheck?.timeoutSeconds ?? 2,
    type: service?.model ? 'MODEL' : 'HTTP',
    modelProtocol: service?.model?.protocol ?? 'MODEL_PROTOCOL_OPENAI',
    apiKey: '',
    clearApiKey: false,
    apiKeyConfigured: service?.model?.apiKeyConfigured ?? false,
  };
}

export function validateServiceDraft(draft: ServiceDraft): string[] {
  const errors: string[] = [];
  if (!draft.name.trim()) errors.push('请输入服务名称');
  if (draft.endpoints.length === 0) errors.push('至少添加一个服务地址');
  if (draft.endpoints.some((endpoint) => (
    !endpoint.address.trim()
    || endpoint.port < 1
    || endpoint.port > 65535
    || endpoint.weight < 1
    || endpoint.weight > 1000
  ))) {
    errors.push('服务地址、端口或权重不正确');
  }
  if (draft.httpsEnabled && !draft.serverName.trim()) errors.push('启用 HTTPS 后必须填写服务名称');
  if (
    draft.healthCheckEnabled
    && (
      !draft.healthCheckPath.startsWith('/')
      || draft.healthCheckTimeout >= draft.healthCheckInterval
    )
  ) {
    errors.push('健康检查路径必须以 / 开头，且超时应小于检查间隔');
  }
  if (draft.type === 'MODEL' && draft.apiKey && draft.apiKey.trim() !== draft.apiKey) {
    errors.push('API Key 不能包含首尾空格');
  }
  if (draft.apiKey && draft.clearApiKey) {
    errors.push('不能同时填写新 API Key 和清除已有 API Key');
  }
  return errors;
}

export function buildServicePayload(draft: ServiceDraft): ServiceMutationPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    endpoints: draft.endpoints.map((item) => ({
      address: item.address.trim().toLowerCase(),
      port: Number(item.port),
      weight: Number(item.weight),
    })),
    tls: draft.httpsEnabled
      ? { serverName: draft.serverName.trim().toLowerCase() }
      : undefined,
    loadBalancing: draft.loadBalancing,
    healthCheck: draft.healthCheckEnabled
      ? {
        path: draft.healthCheckPath.trim(),
        intervalSeconds: Number(draft.healthCheckInterval),
        timeoutSeconds: Number(draft.healthCheckTimeout),
      }
      : undefined,
    model: draft.type === 'MODEL'
      ? {
        protocol: draft.modelProtocol,
        apiKey: draft.apiKey || undefined,
        clearApiKey: draft.clearApiKey || undefined,
      }
      : undefined,
  };
}
