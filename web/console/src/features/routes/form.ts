import type {
  AIModel,
  HeaderMatch,
  HostRewriteMode,
  HttpMethod,
  RouteAccessMode,
  RouteMutationPayload,
  RoutePathMatchType,
  RouteResource,
  WeightedService,
} from '@/domain/route';

export interface RouteDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  accessMode: RouteAccessMode;
  gatewayIDs: string[];
  hostnames: string;
  pathType: RoutePathMatchType;
  path: string;
  methods: HttpMethod[];
  headers: HeaderMatch[];
  services: WeightedService[];
  hostRewriteMode: HostRewriteMode;
  customHostname: string;
  timeoutEnabled: boolean;
  timeoutMillis: number;
  retryEnabled: boolean;
  retryAttempts: number;
  perTryTimeoutMillis: number;
  requestHeaderModifier?: RouteResource['requestHeaderModifier'];
  responseHeaderModifier?: RouteResource['responseHeaderModifier'];
  type: 'HTTP' | 'AI';
  aiModels: AIModel[];
}

export function createDraft(route?: RouteResource): RouteDraft {
  return {
    id: route?.id,
    version: route?.version,
    name: route?.name ?? '',
    enabled: route?.enabled ?? true,
    accessMode: route?.accessMode ?? 'ROUTE_ACCESS_MODE_PUBLIC',
    gatewayIDs: route ? route.gatewayIDs : [''],
    hostnames: route?.hostnames.join(', ') ?? '',
    pathType: route?.match.path.type ?? 'ROUTE_PATH_MATCH_TYPE_PREFIX',
    path: route?.match.path.value ?? '/',
    methods: route?.match.methods ?? [],
    headers: route?.match.headers.map((header) => ({ ...header })) ?? [],
    services: route ? route.services.map((service) => ({ ...service })) : [{ serviceID: '', weight: 1 }],
    hostRewriteMode: route?.hostRewrite.mode ?? 'HOST_REWRITE_MODE_SERVICE_HOST',
    customHostname: route?.hostRewrite.hostname ?? '',
    timeoutEnabled: Boolean(route?.timeout),
    timeoutMillis: route?.timeout?.requestMillis ?? 30000,
    retryEnabled: Boolean(route?.retry),
    retryAttempts: route?.retry?.attempts ?? 2,
    perTryTimeoutMillis: route?.retry?.perTryTimeoutMillis ?? 5000,
    requestHeaderModifier: route?.requestHeaderModifier,
    responseHeaderModifier: route?.responseHeaderModifier,
    type: route?.ai ? 'AI' : 'HTTP',
    aiModels: route?.ai?.models.map((model) => ({ ...model, targets: model.targets.map((target) => ({ ...target })) })) ?? [],
  };
}

export function validateDraft(draft: RouteDraft): string | undefined {
  if (!draft.name.trim()) return '请输入路由名称';
  if (draft.gatewayIDs.length === 0 || draft.gatewayIDs.some((id) => !id)) return '至少选择一个有效的网关';
  if (new Set(draft.gatewayIDs).size !== draft.gatewayIDs.length) return '生效网关不能重复';
  if (!draft.path.startsWith('/')) return '请求路径必须以 / 开头';
  if (draft.type === 'HTTP' && (draft.services.length === 0 || draft.services.some((item) => !item.serviceID || item.weight < 1 || item.weight > 1000))) return '至少配置一个有效的目标服务';
  if (draft.type === 'AI') {
    if (draft.aiModels.length === 0) return '至少发布一个客户端模型';
    const names = draft.aiModels.map((model) => model.name.trim());
    if (names.some((name) => !name) || new Set(names).size !== names.length) return '客户端模型名不能为空或重复';
    if (draft.aiModels.some((model) => model.targets.length === 0 || model.targets.some((target) => !target.serviceID || !target.model.trim() || target.weight < 1 || target.weight > 1000))) return '每个客户端模型至少需要一条有效的模型线路';
  }
  if (draft.hostRewriteMode === 'HOST_REWRITE_MODE_CUSTOM' && !validHostname(draft.customHostname)) return '请输入有效的自定义主机名';
  if (draft.timeoutEnabled && (draft.timeoutMillis < 100 || draft.timeoutMillis > 300000)) return '请求超时范围应为 100 到 300000 毫秒';
  if (draft.retryEnabled && (draft.retryAttempts < 1 || draft.retryAttempts > 5 || draft.perTryTimeoutMillis < 100 || draft.perTryTimeoutMillis > 60000)) return '重试配置不正确';
  return undefined;
}

export function toPayload(draft: RouteDraft): RouteMutationPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    enabled: draft.enabled,
    accessMode: draft.accessMode,
    gatewayIDs: draft.gatewayIDs,
    hostnames: draft.hostnames.split(/[,，\s]+/).map((value) => value.trim().toLowerCase()).filter(Boolean),
    match: { path: { type: draft.pathType, value: draft.path.trim() }, methods: draft.type === 'AI' ? ['POST'] : draft.methods, headers: draft.headers },
    services: draft.type === 'HTTP' ? draft.services : [],
    ai: draft.type === 'AI' ? { models: draft.aiModels.map((model) => ({ name: model.name.trim(), targets: model.targets.map((target) => ({ ...target, model: target.model.trim() })) })) } : undefined,
    hostRewrite: {
      mode: draft.hostRewriteMode,
      hostname: draft.hostRewriteMode === 'HOST_REWRITE_MODE_CUSTOM' ? draft.customHostname.trim().toLowerCase() : undefined,
    },
    requestHeaderModifier: draft.requestHeaderModifier,
    responseHeaderModifier: draft.responseHeaderModifier,
    timeout: draft.timeoutEnabled ? { requestMillis: draft.timeoutMillis } : undefined,
    retry: draft.retryEnabled ? { attempts: draft.retryAttempts, perTryTimeoutMillis: draft.perTryTimeoutMillis } : undefined,
  };
}

function validHostname(value: string): boolean {
  const hostname = value.trim();
  return hostname.length > 0 && hostname.length <= 253 && hostname.split('.').every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label));
}
