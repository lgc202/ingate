import type {
  HeaderMatch,
  HttpMethod,
  RouteComposerPreview,
  RoutePolicyCapability,
  RoutePublishPayload,
  RoutePublishPreview,
  RouteTargetOption,
  RouteTargetPayload,
  RouteValidationItem,
  RouteValidationReport,
} from '@/domain/route';

const defaultTargetWeight = 100;
const minTargetWeight = 1;
const maxTargetWeight = 100;

export interface RouteComposerDraft {
  id?: string;
  version?: string;
  ruleName: string;
  methods: HttpMethod[];
  path: string;
  gatewayIDs: string[];
  hostnames: string[];
  headers: HeaderMatch[];
  targetServices: RouteTargetPayload[];
  enabled: boolean;
  enabledPolicyCapabilities: RoutePolicyCapability[];
  policySettings: Record<string, Record<string, string>>;
}

export function createRouteComposerDraft(template: RouteComposerPreview): RouteComposerDraft {
  const enabledPolicyCapabilities = template.policies.filter((policy) => policy.enabled).map((policy) => policy.capability);
  return {
    ruleName: 'main',
    methods: template.methods,
    path: template.path || '/',
    gatewayIDs: template.gatewayIDs,
    hostnames: normalizeHostnames(template.hostnames),
    headers: [],
    targetServices: [],
    enabled: true,
    enabledPolicyCapabilities,
    policySettings: Object.fromEntries(template.policies.map((policy) => [
      policy.capability,
      Object.fromEntries(policy.params.map((param) => [param.key, param.defaultValue])),
    ])),
  };
}

export function validateRouteComposerDraft(draft: RouteComposerDraft): RouteValidationReport {
  const invalidHostnames = draft.hostnames.filter((hostname) => !isValidHostname(hostname));
  const headerError = headerMatchesError(draft.headers);
  const targetError = targetServicesError(draft.targetServices);
  const items: RouteValidationItem[] = [
    {
      label: '规则名称',
      status: draft.ruleName.trim() ? 'healthy' : 'critical',
      message: draft.ruleName.trim() ? draft.ruleName : '请填写规则名称',
    },
    {
      label: '匹配规则',
      status: draft.path.startsWith('/') ? 'healthy' : 'critical',
      message: draft.path.startsWith('/') ? '路径格式正确' : '路径必须以 / 开头',
    },
    {
      label: '目标服务',
      status: targetError ? 'critical' : 'healthy',
      message: targetError || `已选择 ${draft.targetServices.length} 个目标服务，总权重 ${targetWeightSum(draft.targetServices)}`,
    },
    {
      label: '网关',
      status: draft.gatewayIDs.length > 0 ? 'healthy' : 'critical',
      message: draft.gatewayIDs.length > 0 ? `生效于 ${draft.gatewayIDs.join('、')}` : '请选择生效网关',
    },
    {
      label: '匹配域名',
      status: invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: invalidHostnames.length > 0
          ? `域名格式不正确：${invalidHostnames.join('、')}`
          : draft.hostnames.length > 0
            ? `已配置 ${draft.hostnames.length} 个匹配域名`
            : '不限制 Host',
    },
    {
      label: 'Header 匹配',
      status: headerError ? 'critical' : 'healthy',
      message: headerError || (draft.headers.length > 0 ? `已配置 ${draft.headers.length} 个 Header 条件` : '不限制 Header'),
    },
  ];

  const valid = items.every((item) => item.status !== 'critical');
  const summary = valid ? '配置通过校验，可以保存。' : '配置还存在未完成项，请先补齐。';

  return { valid, summary, items };
}

export function buildRoutePublishPreview(template: RouteComposerPreview, draft: RouteComposerDraft): RoutePublishPreview {
  return {
    title: `${formatMethods(draft.methods)} ${draft.path}`,
    subtitle: `目标服务 ${formatTargetServices(draft.targetServices)} · 策略 ${draft.enabledPolicyCapabilities.length} 个`,
    diffs: [
      { before: `methods: ${formatMethods(template.methods)}`, after: `methods: ${formatMethods(draft.methods)}` },
      { before: `path: ${template.path}`, after: `path: ${draft.path}` },
      { before: `hostnames: ${template.hostnames.join(', ') || '不限制'}`, after: `hostnames: ${draft.hostnames.join(', ') || '不限制'}` },
      { before: 'rules: 未保存配置', after: `rules: ${draft.path}` },
      { before: `native_policies: ${template.policyCount}`, after: `native_policies: ${draft.enabledPolicyCapabilities.length}` },
    ],
  };
}

export function buildRoutePublishPayload(draft: RouteComposerDraft): RoutePublishPayload {
  return {
    id: draft.id,
    version: draft.version,
    gatewayIDs: draft.gatewayIDs,
    hostnames: normalizeHostnames(draft.hostnames),
    enabled: draft.enabled,
    rules: [buildRouteRule(draft)],
  };
}

function buildRouteRule(draft: RouteComposerDraft) {
  const requestHeaderModifier = draft.enabledPolicyCapabilities.includes('RequestHeaderModifier')
    ? headerModifierFromSettings(draft.policySettings.RequestHeaderModifier ?? {})
    : undefined;
  const responseHeaderModifier = draft.enabledPolicyCapabilities.includes('ResponseHeaderModifier')
    ? headerModifierFromSettings(draft.policySettings.ResponseHeaderModifier ?? {})
    : undefined;
  const timeout = draft.enabledPolicyCapabilities.includes('Timeout')
    ? { requestMillis: Number(draft.policySettings.Timeout?.timeoutMillis ?? 30000) }
    : undefined;
  const retry = draft.enabledPolicyCapabilities.includes('Retry')
    ? {
      attempts: Number(draft.policySettings.Retry?.attempts ?? 2),
      perTryTimeoutMillis: Number(draft.policySettings.Retry?.perTryTimeoutMillis ?? 1000),
    }
    : undefined;

  return {
    name: draft.ruleName.trim(),
    pathPrefix: draft.path,
    methods: draft.methods,
    headers: normalizeHeaderMatches(draft.headers),
    targets: draft.targetServices,
    requestHeaderModifier,
    responseHeaderModifier,
    timeout,
    retry,
  };
}

function headerModifierFromSettings(settings: Record<string, string>) {
  const setNames = parsePolicyNames(settings.setHeadersOn ?? '');
  const value = settings.value ?? '';
  const remove = parsePolicyNames(settings.removeHeadersOn ?? '');

  return {
    set: setNames.map((name) => ({ name, value })),
    remove,
  };
}

function parsePolicyNames(value: string): string[] {
  return value.split(/[,，、]/).map((item) => item.trim()).filter(Boolean);
}

function formatMethods(methods: HttpMethod[]): string {
  return methods.length > 0 ? methods.join('、') : '全部方法';
}

function normalizeTargetServices(
  availableTargets: RouteComposerPreview['targets'],
  targets: RouteTargetPayload[],
): RouteTargetPayload[] {
  const availableIDs = new Set(availableTargets.map((target) => target.id));
  const seenIDs = new Set<string>();
  const normalizedTargets = targets
    .map((target) => ({ upstreamID: target.upstreamID.trim(), weight: normalizeTargetWeight(target.weight) }))
    .filter((target) => {
      if (!target.upstreamID || !availableIDs.has(target.upstreamID) || seenIDs.has(target.upstreamID)) {
        return false;
      }
      seenIDs.add(target.upstreamID);
      return true;
    });
  return normalizedTargets;
}

export function formatTargetServices(targets: RouteTargetPayload[], options: RouteTargetOption[] = []): string {
  if (targets.length === 0) {
    return '-';
  }
  const firstTargetName = targetName(targets[0].upstreamID, options);
  if (targets.length === 1) {
    return `${firstTargetName}(${targets[0].weight})`;
  }
  return `${firstTargetName} 等 ${targets.length} 个`;
}

function targetName(upstreamID: string, options: RouteTargetOption[]) {
  const name = options.find((option) => option.id === upstreamID)?.name;
  if (name) {
    return name;
  }
  if (upstreamID.length <= 12) {
    return upstreamID;
  }
  return `${upstreamID.slice(0, 8)}...${upstreamID.slice(-4)}`;
}

export function targetWeightSum(targets: RouteTargetPayload[]): number {
  return targets.reduce((sum, target) => sum + (Number(target.weight) || 0), 0);
}

function targetServicesError(targets: RouteTargetPayload[]): string {
  if (targets.length === 0) {
    return '请选择目标服务';
  }

  const seenIDs = new Set<string>();
  for (const target of targets) {
    if (!target.upstreamID.trim()) {
      return '目标服务不能为空';
    }
    if (seenIDs.has(target.upstreamID)) {
      return '目标服务不能重复';
    }
    seenIDs.add(target.upstreamID);
    if (target.weight < minTargetWeight || target.weight > maxTargetWeight) {
      return `目标权重必须在 ${minTargetWeight}-${maxTargetWeight} 之间`;
    }
  }

  return '';
}

function headerMatchesError(headers: HeaderMatch[]): string {
  for (const header of headers) {
    if (!header.name.trim()) {
      return 'Header 名称不能为空';
    }
    if (!header.value.trim()) {
      return 'Header 值不能为空';
    }
  }
  return '';
}

export function normalizeHeaderMatches(headers: HeaderMatch[]): HeaderMatch[] {
  return headers
    .map((header) => ({
      name: header.name.trim(),
      value: header.value.trim(),
    }))
    .filter((header) => header.name && header.value);
}

function normalizeTargetWeight(weight: number): number {
  const value = Math.trunc(Number(weight));
  if (!Number.isFinite(value)) {
    return defaultTargetWeight;
  }
  return Math.min(Math.max(value, minTargetWeight), maxTargetWeight);
}

export function parseHostnames(input: string): string[] {
  return normalizeHostnames(input.split(/[\s,，;；]+/));
}

export function normalizeHostnames(hostnames: string[]): string[] {
  return Array.from(new Set(hostnames.map((hostname) => hostname.trim().toLowerCase()).filter(Boolean)));
}

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;

  if (!normalized.includes('.') || normalized.length > 253) {
    return false;
  }

  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}
