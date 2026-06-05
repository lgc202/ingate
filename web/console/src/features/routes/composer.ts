import type {
  HttpMethod,
  RouteComposerPreview,
  RoutePublishPayload,
  RoutePublishPreview,
  RouteTargetPayload,
  RouteValidationItem,
  RouteValidationReport,
} from '@/domain/route';

const defaultTargetWeight = 100;
const minTargetWeight = 1;
const maxTargetWeight = 1000;

export interface RouteComposerDraft {
  id?: string;
  version?: string;
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  targetServices: RouteTargetPayload[];
  enabled: boolean;
  rateLimit: string;
  timeout: string;
  enabledPolicyNames: string[];
  policySettings: Record<string, Record<string, string>>;
}

export function createRouteComposerDraft(template: RouteComposerPreview): RouteComposerDraft {
  const enabledPolicyNames = template.policies.filter((policy) => policy.enabled).map((policy) => policy.name);
  const targetServices = normalizeTargetServices(template.targets, template.serviceName ? [{ name: template.serviceName, weight: defaultTargetWeight }] : []);

  return {
    methods: template.methods,
    path: template.path || '/',
    gatewayNames: template.gatewayNames,
    hostnames: normalizeHostnames(template.hostnames),
    serviceName: targetServices[0]?.name ?? template.serviceName,
    targetServices,
    enabled: true,
    rateLimit: template.rateLimit,
    timeout: '30s',
    enabledPolicyNames,
    policySettings: Object.fromEntries(template.policies.map((policy) => [
      policy.name,
      Object.fromEntries(policy.params.map((param) => [param.key, param.defaultValue])),
    ])),
  };
}

export function validateRouteComposerDraft(draft: RouteComposerDraft): RouteValidationReport {
  const invalidHostnames = draft.hostnames.filter((hostname) => !isValidHostname(hostname));
  const targetError = targetServicesError(draft.targetServices);
  const items: RouteValidationItem[] = [
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
      status: draft.gatewayNames.length > 0 ? 'healthy' : 'critical',
      message: draft.gatewayNames.length > 0 ? `生效于 ${draft.gatewayNames.join('、')}` : '请选择生效网关',
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
      label: '限流策略',
      status: draft.rateLimit ? 'healthy' : 'warning',
      message: draft.rateLimit ? draft.rateLimit : '未配置限流规则',
    },
  ];

  const valid = items.every((item) => item.status !== 'critical');
  const summary = valid ? '配置通过校验，可以保存。' : '配置还存在未完成项，请先补齐。';

  return { valid, summary, items };
}

export function buildRoutePublishPreview(template: RouteComposerPreview, draft: RouteComposerDraft): RoutePublishPreview {
  return {
    title: `${formatMethods(draft.methods)} ${draft.path}`,
    subtitle: `目标服务 ${formatTargetServices(draft.targetServices)} · 策略 ${draft.enabledPolicyNames.length} 个`,
    diffs: [
      { before: `methods: ${formatMethods(template.methods)}`, after: `methods: ${formatMethods(draft.methods)}` },
      { before: `path: ${template.path}`, after: `path: ${draft.path}` },
      { before: `hostnames: ${template.hostnames.join(', ') || '不限制'}`, after: `hostnames: ${draft.hostnames.join(', ') || '不限制'}` },
      { before: `service: ${template.serviceName}`, after: `service: ${formatTargetServices(draft.targetServices)}` },
      { before: `policy_bindings: ${template.policyCount}`, after: `policy_bindings: ${draft.enabledPolicyNames.length}` },
    ],
  };
}

export function buildRoutePublishPayload(draft: RouteComposerDraft): RoutePublishPayload {
  return {
    id: draft.id,
    version: draft.version,
    methods: draft.methods,
    path: draft.path,
    gatewayNames: draft.gatewayNames,
    hostnames: normalizeHostnames(draft.hostnames),
    serviceName: draft.targetServices[0]?.name ?? draft.serviceName,
    targets: draft.targetServices,
    enabled: draft.enabled,
    policyBindings: draft.enabledPolicyNames.map((policyName) => ({
      policyName,
      source: 'route',
      parameters: serializePolicyParameters(draft.policySettings[policyName] ?? {}),
    })),
  };
}

function serializePolicyParameters(settings: Record<string, string>): Record<string, string | string[]> {
  return Object.fromEntries(Object.entries(settings).map(([key, value]) => {
    if (key.endsWith('On')) {
      return [key, value.split(/[,，、]/).map((item) => item.trim()).filter(Boolean)];
    }

    return [key, value];
  }));
}

function formatMethods(methods: HttpMethod[]): string {
  return methods.length > 0 ? methods.join('、') : '全部方法';
}

function normalizeTargetServices(
  availableTargets: RouteComposerPreview['targets'],
  targets: RouteTargetPayload[],
): RouteTargetPayload[] {
  const availableNames = new Set(availableTargets.map((target) => target.name));
  const seenNames = new Set<string>();
  const normalizedTargets = targets
    .map((target) => ({ name: target.name.trim(), weight: normalizeTargetWeight(target.weight) }))
    .filter((target) => {
      if (!target.name || !availableNames.has(target.name) || seenNames.has(target.name)) {
        return false;
      }
      seenNames.add(target.name);
      return true;
    });
  if (normalizedTargets.length > 0) {
    return normalizedTargets;
  }
  return availableTargets[0] ? [{ name: availableTargets[0].name, weight: defaultTargetWeight }] : [];
}

export function formatTargetServices(targets: RouteTargetPayload[]): string {
  if (targets.length === 0) {
    return '-';
  }
  if (targets.length === 1) {
    return `${targets[0].name}(${targets[0].weight})`;
  }
  return `${targets[0].name} 等 ${targets.length} 个`;
}

export function targetWeightSum(targets: RouteTargetPayload[]): number {
  return targets.reduce((sum, target) => sum + (Number(target.weight) || 0), 0);
}

function targetServicesError(targets: RouteTargetPayload[]): string {
  if (targets.length === 0) {
    return '请选择目标服务';
  }

  const seenNames = new Set<string>();
  for (const target of targets) {
    if (!target.name.trim()) {
      return '目标服务不能为空';
    }
    if (seenNames.has(target.name)) {
      return '目标服务不能重复';
    }
    seenNames.add(target.name);
    if (target.weight < minTargetWeight || target.weight > maxTargetWeight) {
      return `目标权重必须在 ${minTargetWeight}-${maxTargetWeight} 之间`;
    }
  }

  return '';
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
