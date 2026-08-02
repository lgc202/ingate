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

export interface ParsedEndpointInput {
  address: string;
  port?: number;
  isHttps?: boolean;
}

export function parseEndpointInput(input: string): ParsedEndpointInput {
  let raw = input.trim();
  let isHttps: boolean | undefined = undefined;
  let extractedPort: number | undefined = undefined;

  if (/^https:\/\//i.test(raw)) {
    isHttps = true;
  } else if (/^http:\/\//i.test(raw)) {
    isHttps = false;
  }

  // Remove protocol
  raw = raw.replace(/^https?:\/\//i, '');
  // Remove path or query string
  raw = raw.split('/')[0] ?? raw;
  raw = raw.split('?')[0] ?? raw;

  // Check if port is specified e.g. baidu.com:8443 or 192.168.1.100:9000
  if (raw.includes(':') && !raw.startsWith('[')) {
    const parts = raw.split(':');
    const host = parts[0] ?? '';
    const portNum = Number(parts[1]);
    if (host && !Number.isNaN(portNum) && portNum >= 1 && portNum <= 65535) {
      extractedPort = portNum;
      raw = host;
    }
  }

  return {
    address: raw.toLowerCase(),
    port: extractedPort,
    isHttps,
  };
}

export function cleanEndpointAddress(addressInput: string): string {
  return parseEndpointInput(addressInput).address;
}

export function createUpstreamEndpoint(): UpstreamEndpoint {
  return {
    id: `ep-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
    address: '127.0.0.1',
    port: 8080,
    weight: 100,
    enabled: true,
  };
}

export function createModelCatalogItem(name = ''): ModelCatalogItem {
  return {
    name,
    displayName: name,
    enabled: true,
  };
}

export function createUpstreamDraft(upstream?: Upstream): UpstreamFormDraft {
  if (!upstream) {
    const defaultProvider = modelProviderDefinitions[0];
    return {
      initialType: 'application',
      name: '',
      type: 'application',
      protocol: 'HTTP',
      httpsEnabled: false,
      serverName: '',
      apiKey: '',
      apiKeyConfigured: false,
      removeAPIKey: false,
      modelProvider: defaultProvider.value,
      modelBaseURL: defaultProvider.defaultBaseURL,
      initialModelBaseURL: '',
      modelEndpointID: '',
      models: [createModelCatalogItem()],
      endpoints: [createUpstreamEndpoint()],
      loadBalancePolicy: 'round_robin',
      healthCheck: {
        enabled: false,
        path: '/healthz',
        intervalSeconds: 15,
        timeoutSeconds: 5,
      },
    };
  }

  const modelSpec = (upstream as any).spec?.model || upstream.model;
  const isModel = upstream.type === 'model';
  const primaryEndpoint = isModel ? modelPrimaryEndpoint(upstream) : undefined;
  const modelBaseURL = formatModelBaseURL(upstream);
  const provider = modelSpec?.provider ?? 'custom';

  return {
    id: upstream.id,
    version: upstream.version,
    initialType: upstream.type,
    name: upstream.name,
    type: upstream.type,
    protocol: upstream.protocol,
    httpsEnabled: Boolean(upstream.tls),
    serverName: upstream.tls?.serverName ?? '',
    apiKey: '',
    apiKeyConfigured: upstream.apiKeyConfigured ?? false,
    removeAPIKey: false,
    modelProvider: provider,
    modelBaseURL: isModel ? modelBaseURL : modelProviderDefinition('openai').defaultBaseURL,
    initialModelBaseURL: isModel ? modelBaseURL : '',
    modelEndpointID: primaryEndpoint?.id ?? '',
    initialModelHealthCheck: upstream.healthCheck,
    models: isModel && modelSpec?.models && modelSpec.models.length > 0
      ? modelSpec.models.map((item: any) => ({ ...item }))
      : [createModelCatalogItem()],
    endpoints: upstream.endpoints.length > 0
      ? upstream.endpoints.map((ep) => ({ ...ep }))
      : [createUpstreamEndpoint()],
    loadBalancePolicy: upstream.loadBalancePolicy,
    healthCheck: upstream.healthCheck
      ? {
        enabled: upstream.healthCheck.enabled,
        path: upstream.healthCheck.path ?? '/healthz',
        intervalSeconds: upstream.healthCheck.intervalSeconds ?? 15,
        timeoutSeconds: upstream.healthCheck.timeoutSeconds ?? 5,
      }
      : {
        enabled: false,
        path: '/healthz',
        intervalSeconds: 15,
        timeoutSeconds: 5,
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
  } else {
    errors.endpoints = validateEndpoints(draft.endpoints);
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
    address: cleanEndpointAddress(endpoint.address),
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
    const port = parsedURL?.port ? Number(parsedURL.port) : (secure ? 443 : 80);

    const keepsExistingTopology = draft.id
      && draft.initialType === 'model'
      && draft.initialModelBaseURL === draft.modelBaseURL;

    let modelEndpoints: UpstreamEndpoint[];
    if (keepsExistingTopology && endpoints.length > 0) {
      const originalURL = parseModelBaseURL(draft.initialModelBaseURL);
      const originalHostname = modelEndpointHostname(originalURL?.hostname ?? '');
      const primaryIndex = Math.max(0, endpoints.findIndex((item) => item.id === draft.modelEndpointID));
      const primaryEndpoint = endpoints[primaryIndex];
      const endpointFollowedAuthority = originalURL?.protocol === 'http:'
        || primaryEndpoint.address.toLowerCase() === originalHostname;

      modelEndpoints = endpoints.map((endpoint, index) => {
        const addressFollowedAuthority = endpoint.address.toLowerCase() === originalHostname;
        return {
          ...endpoint,
          address: addressFollowedAuthority || (index === primaryIndex && (!secure || endpointFollowedAuthority))
            ? hostname.toLowerCase()
            : cleanEndpointAddress(endpoint.address),
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
    protocol: draft.protocol,
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
  const nextDefinition = modelProviderDefinition(modelProvider);
  return {
    modelProvider,
    protocol: nextDefinition.protocol,
    modelBaseURL: nextDefinition.defaultBaseURL,
  };
}

function parseModelBaseURL(value: string): URL | null {
  try {
    const parsed = new URL(value.trim());
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function modelEndpointHostname(hostname: string): string {
  if (!hostname) {
    return 'localhost';
  }
  return hostname.startsWith('[') && hostname.endsWith(']') ? hostname.slice(1, -1) : hostname;
}

function validateModelCatalog(items: ModelCatalogItem[]): string | undefined {
  if (items.length === 0) {
    return '请至少添加一个可用的模型标识';
  }
  const names = new Set<string>();
  for (const item of items) {
    const name = item.name.trim();
    if (!name) {
      return '模型名称不能为空';
    }
    if (names.has(name.toLowerCase())) {
      return `模型名称【${name}】不能重复`;
    }
    names.add(name.toLowerCase());
  }
  return undefined;
}

function validateEndpoints(endpoints: UpstreamEndpoint[]): string | undefined {
  if (endpoints.length === 0) {
    return '请至少配置一个有效的 Endpoint';
  }
  for (const endpoint of endpoints) {
    const address = cleanEndpointAddress(endpoint.address);
    if (!address) {
      return 'Endpoint 地址不能为空';
    }
    if (!isValidEndpointHost(address)) {
      return `Endpoint 地址【${endpoint.address}】格式不正确，请输入纯 Hostname 或 IP（不含 http://）`;
    }
    if (!Number.isInteger(endpoint.port) || endpoint.port < 1 || endpoint.port > 65535) {
      return `Endpoint【${endpoint.address}】的端口必须在 1-65535 之间`;
    }
    if (!Number.isInteger(endpoint.weight) || endpoint.weight < 1 || endpoint.weight > 256) {
      return `Endpoint【${endpoint.address}】的权重必须在 1-256 之间`;
    }
  }
  return undefined;
}

function isValidEndpointHost(value: string): boolean {
  if (isIPv4(value) || value === 'localhost') {
    return true;
  }
  return value.split('.').every((label) => (
    /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label)
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

function formatModelBaseURL(upstream: Upstream): string {
  const endpoint = upstream.endpoints[0];
  if (!endpoint) {
    return '';
  }
  const secure = Boolean(upstream.tls);
  const defaultPort = secure ? 443 : 80;
  const authority = upstream.tls?.serverName || endpoint.address;
  const address = authority.includes(':') && !authority.startsWith('[') ? `[${authority}]` : authority;
  const port = endpoint.port === defaultPort ? '' : `:${endpoint.port}`;
  const modelSpec = (upstream as any).spec?.model || upstream.model;
  return `${secure ? 'https' : 'http'}://${address}${port}${normalizeBasePath(modelSpec?.apiBasePath ?? '/')}`;
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
  return endpoints[0];
}
