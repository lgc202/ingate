import type {
  ModelCatalogItem,
  ModelProvider,
  Upstream,
  UpstreamEndpoint,
  UpstreamHealthCheck,
  UpstreamLoadBalancePolicy,
  UpstreamMutationPayload,
  UpstreamProtocol,
  UpstreamType,
} from '@/domain/upstream';
import { modelProviderDefinition, modelProviderDefinitions } from '@/domain/upstream';

export interface UpstreamFormDraft {
  id?: string;
  version?: string;
  initialType: UpstreamType;
  name: string;
  type: UpstreamType;
  protocol: UpstreamProtocol;
  httpsEnabled: boolean;
  serverName: string;
  apiKey: string;
  apiKeyConfigured: boolean;
  removeAPIKey: boolean;
  modelProvider: ModelProvider;
  modelBaseURL: string;
  initialModelBaseURL: string;
  modelEndpointID: string;
  initialModelHealthCheck?: UpstreamHealthCheck;
  models: ModelCatalogItem[];
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
    apiKey?: string;
    modelBaseURL?: string;
    models?: string;
    endpoints?: string;
    loadBalancePolicy?: string;
    healthCheck?: string;
  };
}

export function createUpstreamDraft(upstream?: Upstream | null): UpstreamFormDraft {
  const provider = upstream?.model?.provider ?? 'openai';
  const model = upstream?.model;
  const endpoints = createEndpointsFromUpstream(upstream);
  const primaryModelEndpoint = upstream?.type === 'model' ? modelPrimaryEndpoint(upstream) : undefined;
  const initialModelBaseURL = upstream?.type === 'model'
    ? modelBaseURL(upstream)
    : modelProviderDefinition(provider).defaultBaseURL;

  return {
    id: upstream?.id,
    version: upstream?.version,
    initialType: upstream?.type ?? 'application',
    name: upstream?.name ?? '',
    type: upstream?.type ?? 'application',
    protocol: upstream?.protocol ?? (upstream?.type === 'model' ? modelProviderDefinition(provider).protocol : 'HTTP'),
    httpsEnabled: Boolean(upstream?.tls),
    serverName: upstream?.tls?.serverName ?? '',
    apiKey: '',
    apiKeyConfigured: upstream?.apiKeyConfigured ?? false,
    removeAPIKey: false,
    modelProvider: provider,
    modelBaseURL: initialModelBaseURL,
    initialModelBaseURL,
    modelEndpointID: primaryModelEndpoint?.id ?? endpoints[0]?.id ?? '',
    initialModelHealthCheck: upstream?.type === 'model' && upstream.healthCheck
      ? { ...upstream.healthCheck }
      : undefined,
    models: model?.models.map((item) => ({ ...item })) ?? [createModelCatalogItem()],
    endpoints,
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

  if (draft.type === 'model') {
    const provider = modelProviderDefinition(draft.modelProvider);
    if (draft.protocol !== provider.protocol) {
      errors.protocol = '模型厂商与上游协议不匹配';
    }

    const parsedURL = parseModelBaseURL(draft.modelBaseURL);
    if (!parsedURL) {
      errors.modelBaseURL = '请输入完整的 HTTP 或 HTTPS API 地址';
    }
    if (draft.apiKey && !isValidAPIKey(draft.apiKey)) {
      errors.apiKey = 'API Key 不能包含换行或控制字符';
    }
    const keepsAPIKey = Boolean(draft.apiKey) || (draft.apiKeyConfigured && !draft.removeAPIKey);
    if (keepsAPIKey && parsedURL?.protocol !== 'https:') {
      errors.modelBaseURL = '配置或保留 API Key 时必须使用 HTTPS 地址';
    }
    errors.models = validateModelCatalog(draft.models);
    if (draft.id && draft.initialType === 'model') {
      errors.endpoints = validateEndpoints(draft.endpoints);
      if (!draft.loadBalancePolicy) {
        errors.loadBalancePolicy = '请选择负载均衡方式';
      }
      errors.healthCheck = validateHealthCheck(draft);
    }
  } else {
    if (draft.protocol !== 'HTTP') {
      errors.protocol = '当前服务类型使用 HTTP 协议';
    }
    if (draft.httpsEnabled) {
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
  }

  const valid = Object.values(errors).every((message) => !message);
  return {
    valid,
    summary: valid ? '服务配置可以保存' : '请先完成服务配置中的必填项',
    errors,
  };
}

export function buildUpstreamPayload(draft: UpstreamFormDraft): UpstreamMutationPayload {
  const apiKey = draft.apiKey;
  const removeAPIKey = Boolean(draft.id) && (
    (draft.type === 'model' && draft.removeAPIKey)
    || (draft.type !== 'model' && draft.apiKeyConfigured)
  );
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

  if (draft.type === 'model') {
    const parsedURL = parseModelBaseURL(draft.modelBaseURL);
    const provider = modelProviderDefinition(draft.modelProvider);
    const hostname = modelEndpointHostname(parsedURL?.hostname ?? '');
    const secure = parsedURL?.protocol === 'https:';
    const port = parsedURL?.port ? Number(parsedURL.port) : secure ? 443 : 80;
    const keepsExistingTopology = Boolean(draft.id) && draft.initialType === 'model' && endpoints.length > 0;
    let modelEndpoints: UpstreamEndpoint[];

    if (keepsExistingTopology) {
      const originalURL = parseModelBaseURL(draft.initialModelBaseURL);
      const originalHostname = modelEndpointHostname(originalURL?.hostname ?? '').toLowerCase();
      const primaryIndex = Math.max(0, endpoints.findIndex((endpoint) => endpoint.id === draft.modelEndpointID));
      const primaryEndpoint = endpoints[primaryIndex];
      const endpointFollowedAuthority = originalURL?.protocol === 'http:'
        || primaryEndpoint.address.toLowerCase() === originalHostname;

      modelEndpoints = endpoints.map((endpoint, index) => {
        const addressFollowedAuthority = endpoint.address.toLowerCase() === originalHostname;
        return {
          ...endpoint,
          address: addressFollowedAuthority || (index === primaryIndex && (!secure || endpointFollowedAuthority))
            ? hostname.toLowerCase()
            : endpoint.address,
          port: index === primaryIndex ? port : endpoint.port,
        };
      });
    } else {
      const endpoint = endpoints[0] ?? createUpstreamEndpoint();
      modelEndpoints = [{
        id: endpoint.id,
        address: hostname.toLowerCase(),
        port,
        weight: 100,
        enabled: true,
      }];
    }

    return {
      id: draft.id,
      version: draft.version,
      name: draft.name.trim(),
      type: 'model',
      protocol: provider.protocol,
      model: {
        provider: draft.modelProvider,
        apiBasePath: normalizeBasePath(parsedURL?.pathname ?? '/'),
        models: draft.models.map((model) => ({
          name: model.name.trim(),
          displayName: model.displayName.trim(),
          enabled: model.enabled,
        })),
      },
      tls: secure ? { serverName: hostname.toLowerCase() } : undefined,
      apiKey: apiKey && !removeAPIKey ? { value: apiKey } : undefined,
      removeAPIKey: removeAPIKey || undefined,
      endpoints: modelEndpoints,
      loadBalancePolicy: draft.loadBalancePolicy,
      healthCheck: keepsExistingTopology ? draft.initialModelHealthCheck : healthCheck,
    };
  }

  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    type: draft.type,
    protocol: 'HTTP',
    tls: draft.httpsEnabled ? { serverName: draft.serverName.trim().toLowerCase() } : undefined,
    removeAPIKey: removeAPIKey || undefined,
    endpoints,
    loadBalancePolicy: draft.loadBalancePolicy,
    healthCheck,
  };
}

export function changeUpstreamType(draft: UpstreamFormDraft, type: UpstreamType): UpstreamFormDraft {
  if (type === 'model') {
    const provider = modelProviderDefinition(draft.modelProvider);
    return {
      ...draft,
      type,
      protocol: provider.protocol,
      modelBaseURL: draft.modelBaseURL || provider.defaultBaseURL,
      models: draft.models.length > 0 ? draft.models : [createModelCatalogItem()],
    };
  }
  return { ...draft, type, protocol: 'HTTP' };
}

export function changeModelProvider(draft: UpstreamFormDraft, modelProvider: ModelProvider): Partial<UpstreamFormDraft> {
  const currentDefinition = modelProviderDefinition(draft.modelProvider);
  const nextDefinition = modelProviderDefinition(modelProvider);
  const knownDefault = modelProviderDefinitions.some((item) => item.defaultBaseURL === draft.modelBaseURL);
  return {
    modelProvider,
    protocol: nextDefinition.protocol,
    modelBaseURL: !draft.modelBaseURL || draft.modelBaseURL === currentDefinition.defaultBaseURL || knownDefault
      ? nextDefinition.defaultBaseURL
      : draft.modelBaseURL,
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

export function createModelCatalogItem(): ModelCatalogItem {
  return { name: '', displayName: '', enabled: true };
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

export function formatModelBaseURL(upstream: Upstream): string {
  return modelBaseURL(upstream);
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

function validateModelCatalog(models: ModelCatalogItem[]) {
  if (models.length === 0) {
    return '至少添加一个厂商模型';
  }
  const names = new Set<string>();
  for (const [index, model] of models.entries()) {
    const name = model.name.trim();
    if (!name) {
      return `第 ${index + 1} 个模型缺少厂商模型名称`;
    }
    if (!model.displayName.trim()) {
      return `第 ${index + 1} 个模型缺少显示名称`;
    }
    if (names.has(name)) {
      return `厂商模型 ${name} 不能重复`;
    }
    names.add(name);
  }
  if (!models.some((model) => model.enabled)) {
    return '至少启用一个厂商模型';
  }
  return undefined;
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

function parseModelBaseURL(value: string): URL | null {
  try {
    const parsed = new URL(value.trim());
    const hostname = modelEndpointHostname(parsed.hostname).toLowerCase();
    if (
      !['http:', 'https:'].includes(parsed.protocol)
      || !isValidServiceName(hostname)
      || parsed.username
      || parsed.password
      || parsed.search
      || parsed.hash
      || parsed.pathname.includes('%')
    ) {
      return null;
    }
    parsed.pathname = normalizeBasePath(parsed.pathname);
    return parsed;
  } catch {
    return null;
  }
}

function modelEndpointHostname(hostname: string): string {
  return hostname.startsWith('[') && hostname.endsWith(']')
    ? hostname.slice(1, -1)
    : hostname;
}

function modelBaseURL(upstream: Upstream): string {
  const endpoint = modelPrimaryEndpoint(upstream);
  if (!endpoint) {
    return modelProviderDefinition(upstream.model?.provider ?? 'custom').defaultBaseURL;
  }
  const secure = Boolean(upstream.tls);
  const defaultPort = secure ? 443 : 80;
  const authority = upstream.tls?.serverName || endpoint.address;
  const address = authority.includes(':') && !authority.startsWith('[') ? `[${authority}]` : authority;
  const port = endpoint.port === defaultPort ? '' : `:${endpoint.port}`;
  return `${secure ? 'https' : 'http'}://${address}${port}${normalizeBasePath(upstream.model?.apiBasePath ?? '/')}`;
}

function normalizeBasePath(value: string): string {
  const segments = (value || '/').split('/').filter(Boolean);
  return segments.length === 0 ? '/' : `/${segments.join('/')}`;
}

function modelPrimaryEndpoint(upstream: Upstream): UpstreamEndpoint | undefined {
  const endpoints = [...upstream.endpoints].sort((left, right) => {
    if (left.address !== right.address) {
      return left.address < right.address ? -1 : 1;
    }
    return left.port - right.port;
  });
  return endpoints.find((endpoint) => endpoint.enabled) ?? endpoints[0];
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

function isValidAPIKey(value: string): boolean {
  if (!value) {
    return false;
  }
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code < 0x20 || code === 0x7f) {
      return false;
    }
  }
  return true;
}
