import type {
  Upstream,
  UpstreamEndpoint,
  UpstreamLoadBalancePolicy,
  UpstreamMutationPayload,
  UpstreamProtocol,
  UpstreamType,
} from '@/domain/upstream';

export interface UpstreamFormDraft {
  id?: string;
  version?: string;
  name: string;
  type: UpstreamType;
  protocol: UpstreamProtocol;
  httpsEnabled: boolean;
  serverName: string;
  credentialID: string;
  endpoints: UpstreamEndpoint[];
  loadBalancePolicy: UpstreamLoadBalancePolicy;
  healthCheck: {
    enabled: boolean;
    path: string;
    intervalSeconds: number;
    timeoutSeconds: number;
  };
}

export interface UpstreamFormValidation {
  valid: boolean;
  summary: string;
  errors: {
    name?: string;
    protocol?: string;
    tls?: string;
    endpoints?: string;
    loadBalancePolicy?: string;
    healthCheck?: string;
  };
}

export function createUpstreamDraft(upstream?: Upstream | null): UpstreamFormDraft {
  return {
    id: upstream?.id,
    version: upstream?.version,
    name: upstream?.name ?? '',
    type: upstream?.type ?? 'application',
    protocol: upstream?.protocol ?? (upstream?.type === 'model' ? 'OpenAI' : 'HTTP'),
    httpsEnabled: Boolean(upstream?.tls),
    serverName: upstream?.tls?.serverName ?? '',
    credentialID: upstream?.credentialID ?? '',
    endpoints: createEndpointsFromUpstream(upstream),
    loadBalancePolicy: upstream?.loadBalancePolicy ?? 'round_robin',
    healthCheck: {
      enabled: upstream?.healthCheck?.enabled ?? false,
      path: upstream?.healthCheck?.path ?? '/healthz',
      intervalSeconds: upstream?.healthCheck?.intervalSeconds ?? 10,
      timeoutSeconds: upstream?.healthCheck?.timeoutSeconds ?? 2,
    },
  };
}

export function validateUpstreamDraft(draft: UpstreamFormDraft): UpstreamFormValidation {
  const errors: UpstreamFormValidation['errors'] = {};

  if (!draft.name.trim()) {
    errors.name = '请输入服务名称';
  }

  if (draft.type === 'model' && draft.protocol !== 'OpenAI') {
    errors.protocol = '大模型服务必须使用 OpenAI 兼容协议';
  }
  if (draft.type !== 'model' && draft.protocol !== 'HTTP') {
    errors.protocol = '当前服务类型使用 HTTP 协议';
  }
  if (draft.credentialID && !draft.httpsEnabled) {
    errors.tls = '使用访问凭据时必须开启 HTTPS';
  } else if (draft.httpsEnabled) {
    const serverName = draft.serverName.trim().toLowerCase();
    if (!serverName) {
      errors.tls = '启用 HTTPS 后需要填写服务名称';
    } else if (!isValidServiceName(serverName)) {
      errors.tls = 'HTTPS 服务名称格式不正确';
    }
  }

  errors.endpoints = validateEndpoints(draft.endpoints);

  if (!draft.loadBalancePolicy) {
    errors.loadBalancePolicy = '请选择负载均衡方式';
  }

  errors.healthCheck = validateHealthCheck(draft);

  const valid = Object.values(errors).every((message) => !message);
  return {
    valid,
    summary: valid ? '服务配置可以保存' : '请先完成服务配置中的必填项',
    errors,
  };
}

export function buildUpstreamPayload(draft: UpstreamFormDraft): UpstreamMutationPayload {
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
    protocol: draft.type === 'model' ? 'OpenAI' : 'HTTP',
    tls: draft.httpsEnabled ? { serverName: draft.serverName.trim().toLowerCase() } : undefined,
    credentialID: draft.type === 'model' ? draft.credentialID : undefined,
    endpoints,
    loadBalancePolicy: draft.loadBalancePolicy,
    healthCheck,
  };
}

export function changeUpstreamType(draft: UpstreamFormDraft, type: UpstreamType): UpstreamFormDraft {
  return {
    ...draft,
    type,
    protocol: type === 'model' ? 'OpenAI' : 'HTTP',
    credentialID: type === 'model' ? draft.credentialID : '',
  };
}

export function createUpstreamEndpoint(): UpstreamEndpoint {
  return {
    id: `endpoint-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    address: '',
    port: 80,
    weight: 100,
    enabled: true,
  };
}

export function formatEndpointSummary(endpoints: UpstreamEndpoint[]) {
  const enabledEndpoints = endpoints.filter((endpoint) => endpoint.enabled);
  const visibleEndpoints = enabledEndpoints.length > 0 ? enabledEndpoints : endpoints;

  if (visibleEndpoints.length === 0) {
    return '-';
  }

  const first = visibleEndpoints[0];
  const suffix = visibleEndpoints.length > 1 ? ` 等 ${visibleEndpoints.length} 个端点` : '';

  return `${first.address}:${first.port}${suffix}`;
}

export function formatInstanceSummary(endpoints: UpstreamEndpoint[]) {
  const enabledCount = endpoints.filter((endpoint) => endpoint.enabled).length;

  return `${enabledCount}/${endpoints.length}`;
}

function createEndpointsFromUpstream(upstream?: Upstream | null): UpstreamEndpoint[] {
  if (!upstream) {
    return [createUpstreamEndpoint()];
  }

  if (upstream.endpoints.length > 0) {
    return upstream.endpoints.map((endpoint) => ({ ...endpoint }));
  }

  return [createUpstreamEndpoint()];
}

function validateEndpoints(endpoints: UpstreamEndpoint[]) {
  if (endpoints.length === 0) {
    return '至少配置一个服务端点';
  }

  for (const [index, endpoint] of endpoints.entries()) {
    if (!endpoint.address.trim()) {
      return `第 ${index + 1} 个端点缺少地址`;
    }

    if (!Number.isInteger(endpoint.port) || endpoint.port < 1 || endpoint.port > 65535) {
      return `第 ${index + 1} 个端点端口不合法`;
    }

    if (!Number.isInteger(endpoint.weight) || endpoint.weight < 1 || endpoint.weight > 100) {
      return `第 ${index + 1} 个端点权重需要在 1-100 之间`;
    }
  }

  if (!endpoints.some((endpoint) => endpoint.enabled)) {
    return '至少保留一个启用端点';
  }

  return undefined;
}

function validateHealthCheck(draft: UpstreamFormDraft) {
  if (!draft.healthCheck.enabled) {
    return undefined;
  }

  if (!draft.healthCheck.path.trim().startsWith('/')) {
    return '探活路径必须以 / 开头';
  }

  const { intervalSeconds, timeoutSeconds } = draft.healthCheck;
  if (!Number.isInteger(intervalSeconds) || intervalSeconds < 1 || intervalSeconds > 300) {
    return '检查间隔需要在 1-300 秒之间';
  }

  if (!Number.isInteger(timeoutSeconds) || timeoutSeconds < 1 || timeoutSeconds > 60 || timeoutSeconds >= intervalSeconds) {
    return '超时时间需要在 1-60 秒之间，并且小于检查间隔';
  }

  return undefined;
}

function isValidServiceName(value: string): boolean {
  if (!value || value.length > 253) {
    return false;
  }
  if (isIPv4(value) || (value.includes(':') && /^[0-9a-f:]+$/i.test(value))) {
    return true;
  }
  return value.split('.').every((label) => (
    /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label)
  ));
}

function isIPv4(value: string): boolean {
  const parts = value.split('.');
  return parts.length === 4 && parts.every((part) => {
    const number = Number(part);
    return /^\d+$/.test(part) && number >= 0 && number <= 255;
  });
}
