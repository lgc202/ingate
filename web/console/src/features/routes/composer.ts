import type {
  HttpMethod,
  RouteComposerPreview,
  RoutePublishPayload,
  RoutePublishPreview,
  RouteValidationItem,
  RouteValidationReport,
} from '@/domain/route';

export interface RouteComposerDraft {
  id?: string;
  version?: string;
  methods: HttpMethod[];
  path: string;
  gatewayNames: string[];
  hostnames: string[];
  serviceName: string;
  enabled: boolean;
  rateLimit: string;
  timeout: string;
  selectedTargetName: string;
  enabledPolicyNames: string[];
  policySettings: Record<string, Record<string, string>>;
}

export function createRouteComposerDraft(template: RouteComposerPreview): RouteComposerDraft {
  const enabledPolicyNames = template.policies.filter((policy) => policy.enabled).map((policy) => policy.name);

  return {
    methods: template.methods,
    path: template.path || '/',
    gatewayNames: template.gatewayNames,
    hostnames: normalizeHostnames(template.hostnames),
    serviceName: template.serviceName,
    enabled: true,
    rateLimit: template.rateLimit,
    timeout: '30s',
    selectedTargetName: template.serviceName,
    enabledPolicyNames,
    policySettings: Object.fromEntries(template.policies.map((policy) => [
      policy.name,
      Object.fromEntries(policy.params.map((param) => [param.key, param.defaultValue])),
    ])),
  };
}

export function validateRouteComposerDraft(draft: RouteComposerDraft): RouteValidationReport {
  const invalidHostnames = draft.hostnames.filter((hostname) => !isValidHostname(hostname));
  const items: RouteValidationItem[] = [
    {
      label: '匹配规则',
      status: draft.path.startsWith('/') ? 'healthy' : 'critical',
      message: draft.path.startsWith('/') ? '路径格式正确' : '路径必须以 / 开头',
    },
    {
      label: '目标服务',
      status: draft.serviceName ? 'healthy' : 'critical',
      message: draft.serviceName ? `已选择 ${draft.serviceName}` : '请选择目标服务',
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
    subtitle: `目标服务 ${draft.serviceName} · 策略 ${draft.enabledPolicyNames.length} 个`,
    diffs: [
      { before: `methods: ${formatMethods(template.methods)}`, after: `methods: ${formatMethods(draft.methods)}` },
      { before: `path: ${template.path}`, after: `path: ${draft.path}` },
      { before: `hostnames: ${template.hostnames.join(', ') || '不限制'}`, after: `hostnames: ${draft.hostnames.join(', ') || '不限制'}` },
      { before: `service: ${template.serviceName}`, after: `service: ${draft.serviceName}` },
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
    serviceName: draft.serviceName,
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
