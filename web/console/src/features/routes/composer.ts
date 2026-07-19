import type {
  HeaderMatch,
  HeaderModifier,
  HttpMethod,
  ModelRoute,
  RouteMutationPayload,
  RouteResource,
  RouteRetry,
  RouteRule,
  RouteRulePayload,
  RouteTimeout,
  UpstreamOption,
  WeightedUpstream,
} from '@/domain/route';

const defaultUpstreamWeight = 100;
const minUpstreamWeight = 1;
const maxUpstreamWeight = 100;
const defaultRouteTimeoutMillis = 30000;
const minRouteTimeoutMillis = 100;
const maxRouteTimeoutMillis = 300000;
const minRetryAttempts = 1;
const maxRetryAttempts = 5;
const minPerTryTimeoutMillis = 100;
const maxPerTryTimeoutMillis = 60000;
export const modelRoutePath = '/v1/chat/completions';
const aiClusterHeader = 'x-ingate-ai-cluster-v1';
const aiRouteHeader = 'x-ingate-ai-route-v1';
const aiManagedRequestHeaders = new Set([
  ':authority',
  ':path',
  'accept-encoding',
  'anthropic-version',
  'authorization',
  'content-encoding',
  'content-length',
  'content-type',
  aiClusterHeader,
  aiRouteHeader,
  'x-api-key',
  'x-goog-api-key',
]);

export type RouteForwardMode = 'service' | 'model';

export interface RouteComposerDraft {
  id?: string;
  version?: string;
  name: string;
  ruleName: string;
  methods: HttpMethod[];
  path: string;
  gatewayIDs: string[];
  hostnames: string[];
  headers: HeaderMatch[];
  forwardMode: RouteForwardMode;
  weightedUpstreams: WeightedUpstream[];
  modelRoutes: ModelRoute[];
  enabled: boolean;
  requestHeaderModifier?: HeaderModifier;
  responseHeaderModifier?: HeaderModifier;
  timeout?: RouteTimeout;
  retry?: RouteRetry;
  preservedRules: RouteRule[];
}

export interface RouteDraftErrors {
  name?: string;
  ruleName?: string;
  path?: string;
  gateways?: string;
  hostnames?: string;
  headers?: string;
  upstreams?: string;
  models?: string;
  requestHeaderModifier?: string;
  responseHeaderModifier?: string;
  timeout?: string;
  retry?: string;
}

export interface RouteDraftValidation {
  valid: boolean;
  summary: string;
  errors: RouteDraftErrors;
}

export function createRouteComposerDraft(route?: RouteResource): RouteComposerDraft {
  const rule = route?.rules[0];
  const forwardMode: RouteForwardMode = rule?.modelRouting ? 'model' : 'service';

  return {
    id: route?.id,
    version: route?.version,
    name: route?.name ?? '',
    ruleName: rule?.name ?? 'main',
    methods: forwardMode === 'model' ? ['POST'] : rule?.methods ?? [],
    path: forwardMode === 'model' ? modelRoutePath : rule?.pathPrefix ?? '/',
    gatewayIDs: route?.gatewayIDs ?? [],
    hostnames: route?.hostnames ?? [],
    headers: rule?.headers ?? [],
    forwardMode,
    weightedUpstreams: rule?.upstreams ?? [],
    modelRoutes: rule?.modelRouting?.models.map((model) => ({ ...model })) ?? [createModelRoute()],
    enabled: route?.enabled ?? true,
    requestHeaderModifier: cloneHeaderModifier(rule?.requestHeaderModifier),
    responseHeaderModifier: cloneHeaderModifier(rule?.responseHeaderModifier),
    timeout: rule?.timeout ? { ...rule.timeout } : undefined,
    retry: rule?.retry ? { ...rule.retry } : undefined,
    preservedRules: (route?.rules.slice(1) ?? []).map(cloneRouteRule),
  };
}

export function validateRouteComposerDraft(draft: RouteComposerDraft): RouteDraftValidation {
  const errors: RouteDraftErrors = {};
  const invalidHostnames = draft.hostnames.filter((hostname) => !isValidHostname(hostname));

  if (!draft.name.trim()) {
    errors.name = '请填写路由名称';
  }
  const ruleName = draft.ruleName.trim();
  if (!ruleName) {
    errors.ruleName = '请填写规则名称';
  } else if (draft.preservedRules.some((rule) => rule.name === ruleName)) {
    errors.ruleName = `规则名称不能与附加规则 ${ruleName} 重复`;
  }
  if (!draft.path.trim().startsWith('/')) {
    errors.path = '路径必须以 / 开头';
  }
  if (draft.gatewayIDs.length === 0) {
    errors.gateways = '请选择生效网关';
  }
  if (invalidHostnames.length > 0) {
    errors.hostnames = `域名格式不正确：${invalidHostnames.join('、')}`;
  }

  errors.headers = headerMatchesError(draft.headers, draft.forwardMode === 'model') || undefined;
  if (draft.forwardMode === 'model') {
    errors.models = modelRoutesError(draft.modelRoutes) || undefined;
    if (draft.path !== modelRoutePath) {
      errors.path = '模型路由路径必须为 ' + modelRoutePath;
    }
  } else {
    errors.upstreams = weightedUpstreamsError(draft.weightedUpstreams) || undefined;
  }
  errors.requestHeaderModifier = requestHeaderModifierError(
    draft.requestHeaderModifier,
    draft.forwardMode === 'model',
  ) || undefined;
  errors.responseHeaderModifier = headerModifierError(draft.responseHeaderModifier) || undefined;
  errors.timeout = timeoutError(draft.timeout) || undefined;
  errors.retry = draft.forwardMode === 'model'
    ? draft.retry ? '模型路由暂不支持失败重试' : undefined
    : retryError(draft.retry, draft.timeout?.requestMillis ?? defaultRouteTimeoutMillis) || undefined;

  const valid = Object.values(errors).every((error) => !error);
  return {
    valid,
    summary: valid ? '配置完整，可以保存' : '还有必填项或格式需要调整',
    errors,
  };
}

export function buildRouteMutationPayload(draft: RouteComposerDraft): RouteMutationPayload {
  const modelRouting = draft.forwardMode === 'model'
    ? {
      models: draft.modelRoutes.map((model) => ({
        model: model.model.trim(),
        upstreamID: model.upstreamID.trim(),
        upstreamModel: model.upstreamModel?.trim() || undefined,
      })),
    }
    : undefined;
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    gatewayIDs: draft.gatewayIDs,
    hostnames: normalizeHostnames(draft.hostnames),
    enabled: draft.enabled,
    rules: [{
      name: draft.ruleName.trim(),
      pathPrefix: draft.forwardMode === 'model' ? modelRoutePath : draft.path.trim() || '/',
      methods: draft.forwardMode === 'model' ? ['POST'] : draft.methods,
      headers: normalizeHeaderMatches(draft.headers),
      targets: draft.forwardMode === 'service' ? draft.weightedUpstreams.map((upstream) => ({
        upstreamID: upstream.upstreamID.trim(),
        weight: normalizeUpstreamWeight(upstream.weight),
      })) : undefined,
      modelRouting,
      requestHeaderModifier: normalizeHeaderModifier(draft.requestHeaderModifier),
      responseHeaderModifier: normalizeHeaderModifier(draft.responseHeaderModifier),
      timeout: draft.timeout ? { requestMillis: Number(draft.timeout.requestMillis) } : undefined,
      retry: draft.forwardMode === 'service' && draft.retry ? {
        attempts: Number(draft.retry.attempts),
        perTryTimeoutMillis: Number(draft.retry.perTryTimeoutMillis),
      } : undefined,
    }, ...draft.preservedRules.map(preservedRulePayload)],
  };
}

export function changeRouteForwardMode(draft: RouteComposerDraft, forwardMode: RouteForwardMode): RouteComposerDraft {
  if (forwardMode === draft.forwardMode) {
    return draft;
  }
  if (forwardMode === 'model') {
    return {
      ...draft,
      forwardMode,
      methods: ['POST'],
      path: modelRoutePath,
      retry: undefined,
      modelRoutes: draft.modelRoutes.length > 0 ? draft.modelRoutes : [createModelRoute()],
    };
  }
  return {
    ...draft,
    forwardMode,
    methods: [],
  };
}

export function createModelRoute(): ModelRoute {
  return { model: '', upstreamID: '', upstreamModel: '' };
}

export function formatWeightedUpstreams(upstreams: WeightedUpstream[], options: UpstreamOption[] = []): string {
  if (upstreams.length === 0) {
    return '-';
  }

  const firstUpstreamName = upstreamName(upstreams[0].upstreamID, options);
  if (upstreams.length === 1) {
    return `${firstUpstreamName} (${upstreams[0].weight})`;
  }
  return `${firstUpstreamName} 等 ${upstreams.length} 个`;
}

export function formatModelRoutes(models: ModelRoute[], options: UpstreamOption[] = []): string {
  if (models.length === 0) {
    return '-';
  }
  const first = models[0];
  const service = upstreamName(first.upstreamID, options);
  const clientModel = first.model || '未命名模型';
  const upstreamModel = first.upstreamModel || clientModel;
  const firstMapping = `${clientModel} → ${upstreamModel}`;
  if (models.length === 1) {
    return `${service} · ${firstMapping}`;
  }
  const serviceCount = new Set(models.map((model) => model.upstreamID).filter(Boolean)).size;
  return `${service} · ${firstMapping} 等 ${models.length} 个模型 / ${serviceCount} 个服务`;
}

export function upstreamWeightSum(upstreams: WeightedUpstream[]): number {
  return upstreams.reduce((sum, upstream) => sum + (Number(upstream.weight) || 0), 0);
}

export function forwardControlCount(draft: Pick<RouteComposerDraft, 'requestHeaderModifier' | 'responseHeaderModifier' | 'timeout' | 'retry'>): number {
  return [draft.requestHeaderModifier, draft.responseHeaderModifier, draft.timeout, draft.retry].filter(Boolean).length;
}

export function parseHostnames(input: string): string[] {
  return normalizeHostnames(input.split(/[\s,，;；]+/));
}

export function normalizeHostnames(hostnames: string[]): string[] {
  return Array.from(new Set(hostnames.map((hostname) => hostname.trim().toLowerCase()).filter(Boolean)));
}

export function normalizeHeaderMatches(headers: HeaderMatch[]): HeaderMatch[] {
  return headers
    .map((header) => ({
      name: header.name.trim().toLowerCase(),
      value: header.value.trim(),
    }))
    .filter((header) => header.name && header.value);
}

function cloneHeaderModifier(modifier: HeaderModifier | undefined): HeaderModifier | undefined {
  if (!modifier) {
    return undefined;
  }
  return {
    set: modifier.set.map((header) => ({ ...header })),
    remove: [...modifier.remove],
  };
}

function cloneRouteRule(rule: RouteRule): RouteRule {
  return {
    ...rule,
    methods: [...rule.methods],
    headers: rule.headers.map((header) => ({ ...header })),
    upstreams: rule.upstreams.map((upstream) => ({ ...upstream })),
    modelRouting: rule.modelRouting ? {
      models: rule.modelRouting.models.map((model) => ({ ...model })),
    } : undefined,
    requestHeaderModifier: cloneHeaderModifier(rule.requestHeaderModifier),
    responseHeaderModifier: cloneHeaderModifier(rule.responseHeaderModifier),
    timeout: rule.timeout ? { ...rule.timeout } : undefined,
    retry: rule.retry ? { ...rule.retry } : undefined,
  };
}

function preservedRulePayload(rule: RouteRule): RouteRulePayload {
  return {
    name: rule.name,
    pathPrefix: rule.pathPrefix,
    methods: rule.methods,
    headers: rule.headers,
    targets: rule.upstreams,
    modelRouting: rule.modelRouting,
    requestHeaderModifier: rule.requestHeaderModifier,
    responseHeaderModifier: rule.responseHeaderModifier,
    timeout: rule.timeout,
    retry: rule.retry,
  };
}

function modelRoutesError(models: ModelRoute[]): string {
  if (models.length === 0) {
    return '至少配置一个模型';
  }
  const seenModels = new Set<string>();
  for (const [index, model] of models.entries()) {
    const clientModel = model.model.trim();
    if (!clientModel) {
      return `第 ${index + 1} 个模型缺少客户端模型名称`;
    }
    if (!model.upstreamID.trim()) {
      return `第 ${index + 1} 个模型缺少模型服务`;
    }
    if (seenModels.has(clientModel)) {
      return `客户端模型名称不能重复：${clientModel}`;
    }
    seenModels.add(clientModel);
  }
  return '';
}

function normalizeHeaderModifier(modifier: HeaderModifier | undefined): HeaderModifier | undefined {
  if (!modifier) {
    return undefined;
  }
  return {
    set: modifier.set.map((header) => ({
      name: header.name.trim().toLowerCase(),
      value: header.value.trim(),
    })),
    remove: modifier.remove.map((name) => name.trim().toLowerCase()).filter(Boolean),
  };
}

function headerMatchesError(headers: HeaderMatch[], modelRouting: boolean): string {
  for (const header of headers) {
    const name = header.name.trim().toLowerCase();
    if (!name) {
      return '请求头名称不能为空';
    }
    if (!header.value.trim()) {
      return '请求头值不能为空';
    }
    if (modelRouting && (name === aiClusterHeader || name === aiRouteHeader)) {
      return '模型路由不能匹配系统内部请求头';
    }
  }
  return '';
}

function weightedUpstreamsError(upstreams: WeightedUpstream[]): string {
  if (upstreams.length === 0) {
    return '请选择目标服务';
  }

  const seenIDs = new Set<string>();
  for (const upstream of upstreams) {
    if (!upstream.upstreamID.trim()) {
      return '目标服务不能为空';
    }
    if (seenIDs.has(upstream.upstreamID)) {
      return '目标服务不能重复';
    }
    seenIDs.add(upstream.upstreamID);
    if (!Number.isInteger(upstream.weight) || upstream.weight < minUpstreamWeight || upstream.weight > maxUpstreamWeight) {
      return `目标服务权重必须在 ${minUpstreamWeight}-${maxUpstreamWeight} 之间`;
    }
  }
  return '';
}

function headerModifierError(modifier: HeaderModifier | undefined): string {
  if (!modifier) {
    return '';
  }
  if (modifier.set.length === 0 && modifier.remove.length === 0) {
    return '请至少配置一个写入或删除动作';
  }
  for (const header of modifier.set) {
    if (!header.name.trim() || !header.value.trim()) {
      return '写入请求头的名称和值不能为空';
    }
  }
  if (modifier.remove.some((name) => !name.trim())) {
    return '删除请求头的名称不能为空';
  }
  return '';
}

function requestHeaderModifierError(modifier: HeaderModifier | undefined, modelRouting: boolean): string {
  const error = headerModifierError(modifier);
  if (error || !modifier) {
    return error;
  }

  const names = [
    ...modifier.set.map((header) => header.name),
    ...modifier.remove,
  ].map((name) => name.trim().toLowerCase());
  if (modelRouting && names.some((name) => aiManagedRequestHeaders.has(name))) {
    return '模型路由的认证和请求体相关 Header 由系统管理';
  }
  return '';
}

function timeoutError(timeout: RouteTimeout | undefined): string {
  if (!timeout) {
    return '';
  }
  if (!Number.isInteger(timeout.requestMillis) || timeout.requestMillis < minRouteTimeoutMillis || timeout.requestMillis > maxRouteTimeoutMillis) {
    return `请求超时必须在 ${minRouteTimeoutMillis}-${maxRouteTimeoutMillis}ms 之间`;
  }
  return '';
}

function retryError(retry: RouteRetry | undefined, totalTimeoutMillis: number): string {
  if (!retry) {
    return '';
  }
  if (!Number.isInteger(retry.attempts) || retry.attempts < minRetryAttempts || retry.attempts > maxRetryAttempts) {
    return `重试次数必须在 ${minRetryAttempts}-${maxRetryAttempts} 之间`;
  }
  if (!Number.isInteger(retry.perTryTimeoutMillis) || retry.perTryTimeoutMillis < minPerTryTimeoutMillis || retry.perTryTimeoutMillis > maxPerTryTimeoutMillis) {
    return `单次超时必须在 ${minPerTryTimeoutMillis}-${maxPerTryTimeoutMillis}ms 之间`;
  }
  if (retry.perTryTimeoutMillis > totalTimeoutMillis) {
    return `单次超时不能大于请求总超时 ${totalTimeoutMillis}ms`;
  }
  return '';
}

function normalizeUpstreamWeight(weight: number): number {
  const value = Math.trunc(Number(weight));
  if (!Number.isFinite(value)) {
    return defaultUpstreamWeight;
  }
  return Math.min(Math.max(value, minUpstreamWeight), maxUpstreamWeight);
}

function upstreamName(upstreamID: string, options: UpstreamOption[]) {
  const name = options.find((option) => option.id === upstreamID)?.name;
  if (name) {
    return name;
  }
  if (upstreamID.length <= 12) {
    return upstreamID;
  }
  return `${upstreamID.slice(0, 8)}...${upstreamID.slice(-4)}`;
}

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;
  if (!normalized || normalized.length > 253) {
    return false;
  }
  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}
