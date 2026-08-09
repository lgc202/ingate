import { apiListAll, apiRequest } from './client';
import type { PagedResponse } from './client';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import type { GatewayListView } from '@/domain/gateway';
import type {
  AccessControlPolicy,
  AccessControlPolicyPayload,
  GovernancePolicy,
  PolicyMutationResult,
  PolicyTargetOption,
  PolicyTargetRef,
  PolicyWorkspace,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  TokenQuotaPolicy,
  TokenQuotaPolicyPayload,
  TokenQuotaSubjectType,
} from '@/domain/policy';
import { isModelRoute, type RouteListView } from '@/domain/route';

interface PolicyMutationResponse {
  success: boolean;
  id?: string;
}

interface RateLimitPolicyListResponse extends PagedResponse {
  policies?: RateLimitPolicy[];
}

interface AccessControlPolicyListResponse extends PagedResponse {
  policies?: AccessControlPolicy[];
}

interface TokenQuotaPolicyListResponse extends PagedResponse {
  policies?: TokenQuotaPolicy[];
}

export async function getPolicyWorkspace(): Promise<PolicyWorkspace> {
  const [rateLimitPolicies, accessControlPolicies, tokenQuotaPolicies, gatewayList, routeList] = await Promise.all([
    listRateLimitPolicies(),
    listAccessControlPolicies(),
    listTokenQuotaPolicies(),
    listGateways(),
    listRoutes(),
  ]);

  const policies: GovernancePolicy[] = [
    ...rateLimitPolicies.map((policy) => ({
      id: policy.id,
      version: policy.version,
      kind: 'RateLimitPolicy' as const,
      name: policy.name,
      description: policy.description,
      enabled: policy.enabled,
      summary: rateLimitSummary(policy),
      ruleCount: policy.rules.length,
      targets: policy.targets ?? [],
      status: policy.status,
      createdAt: policy.createdAt,
      raw: policy,
    })),
    ...accessControlPolicies.map((policy) => ({
      id: policy.id,
      version: policy.version,
      kind: 'AccessControlPolicy' as const,
      name: policy.name,
      description: policy.description,
      enabled: policy.enabled,
      summary: policy.defaultAction === 'Deny' ? '未命中时拒绝' : '未命中时放行',
      ruleCount: policy.rules?.length ?? 0,
      targets: policy.targets ?? [],
      status: policy.status,
      createdAt: policy.createdAt,
      raw: policy,
    })),
    ...tokenQuotaPolicies.map((policy) => ({
      id: policy.id,
      version: policy.version,
      kind: 'TokenQuotaPolicy' as const,
      name: policy.name,
      description: policy.description,
      enabled: policy.enabled,
      summary: tokenQuotaSummary(policy),
      ruleCount: 1,
      targets: policy.targets ?? [],
      status: policy.status,
      createdAt: policy.createdAt,
      raw: policy,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name));

  return {
    policies,
    rateLimitPolicies,
    accessControlPolicies,
    tokenQuotaPolicies,
    targets: policyTargets(gatewayList, routeList),
  };
}

export async function listRateLimitPolicies(): Promise<RateLimitPolicy[]> {
  return apiListAll<RateLimitPolicyListResponse, RateLimitPolicy>('/rate-limit-policies', (page) => page.policies ?? []);
}

export async function listAccessControlPolicies(): Promise<AccessControlPolicy[]> {
  return apiListAll<AccessControlPolicyListResponse, AccessControlPolicy>('/access-control-policies', (page) => page.policies ?? []);
}

export async function listTokenQuotaPolicies(): Promise<TokenQuotaPolicy[]> {
  return apiListAll<TokenQuotaPolicyListResponse, TokenQuotaPolicy>('/token-quota-policies', (page) => page.policies ?? []);
}

export async function saveRateLimitPolicy(payload: RateLimitPolicyPayload) {
  return savePolicy('/rate-limit-policies', payload, `限流策略已保存：${payload.name}`);
}

export async function saveAccessControlPolicy(payload: AccessControlPolicyPayload) {
  return savePolicy('/access-control-policies', payload, `访问控制策略已保存：${payload.name}`);
}

export async function saveTokenQuotaPolicy(payload: TokenQuotaPolicyPayload) {
  return savePolicy('/token-quota-policies', payload, `Token 配额策略已保存：${payload.name}`);
}

export async function updateGovernancePolicyTargets(policy: GovernancePolicy, targets: PolicyTargetRef[]) {
  const normalizedTargets = targets.map((target) => ({ kind: target.kind, id: target.id }));
  if (policy.kind === 'RateLimitPolicy') {
    const source = policy.raw;
    if (!source.version) {
      throw new Error('限流策略版本缺失，请刷新后重试');
    }
    return savePolicy('/rate-limit-policies', {
      id: source.id,
      version: source.version,
      name: source.name,
      description: source.description,
      enabled: source.enabled,
      targets: normalizedTargets,
      rules: source.rules,
      response: source.response,
      failurePolicy: source.failurePolicy,
    } satisfies RateLimitPolicyPayload, `限流策略应用范围已更新：${source.name}`);
  }

  if (policy.kind === 'AccessControlPolicy') {
    const source = policy.raw;
    if (!source.version) {
      throw new Error('访问控制策略版本缺失，请刷新后重试');
    }
    return savePolicy('/access-control-policies', {
      id: source.id,
      version: source.version,
      name: source.name,
      description: source.description,
      enabled: source.enabled,
      targets: normalizedTargets,
      defaultAction: source.defaultAction,
      rules: source.rules ?? [],
      response: source.response,
    } satisfies AccessControlPolicyPayload, `访问控制策略应用范围已更新：${source.name}`);
  }

  const source = policy.raw;
  if (!source.version) {
    throw new Error('Token 配额策略版本缺失，请刷新后重试');
  }
  return savePolicy('/token-quota-policies', {
    id: source.id,
    version: source.version,
    name: source.name,
    description: source.description,
    enabled: source.enabled,
    targets: normalizedTargets,
    subject: source.subject,
    quota: source.quota,
    failurePolicy: source.failurePolicy,
    response: source.response,
  } satisfies TokenQuotaPolicyPayload, `Token 配额策略应用范围已更新：${source.name}`);
}

export async function deleteRateLimitPolicy(id: string, name: string) {
  return deletePolicy('/rate-limit-policies', id, `限流策略已删除：${name}`);
}

export async function deleteAccessControlPolicy(id: string, name: string) {
  return deletePolicy('/access-control-policies', id, `访问控制策略已删除：${name}`);
}

export async function deleteTokenQuotaPolicy(id: string, name: string) {
  return deletePolicy('/token-quota-policies', id, `Token 配额策略已删除：${name}`);
}

export async function setRateLimitPolicyEnabled(id: string, name: string, enabled: boolean) {
  return setPolicyEnabled('/rate-limit-policies', id, enabled, `限流策略已${enabled ? '启用' : '停用'}：${name}`);
}

export async function setAccessControlPolicyEnabled(id: string, name: string, enabled: boolean) {
  return setPolicyEnabled('/access-control-policies', id, enabled, `访问控制策略已${enabled ? '启用' : '停用'}：${name}`);
}

export async function setTokenQuotaPolicyEnabled(id: string, name: string, enabled: boolean) {
  return setPolicyEnabled('/token-quota-policies', id, enabled, `Token 配额策略已${enabled ? '启用' : '停用'}：${name}`);
}

function policyTargets(gateways: GatewayListView, routes: RouteListView): PolicyTargetOption[] {
  return [
    ...gateways.gateways.map((gateway) => ({
      id: gateway.id,
      name: gateway.name || gateway.id,
      kind: 'Gateway' as const,
      supportsTokenQuota: true,
    })),
    ...routes.routes.map((route) => ({
      id: route.id,
      name: route.name || route.id,
      kind: 'Route' as const,
      supportsTokenQuota: isModelRoute(route),
    })),
  ].sort((a, b) => a.name.localeCompare(b.name));
}

function rateLimitSummary(policy: RateLimitPolicy) {
  const limit = policy.rules[0]?.limit;
  if (!limit) {
    return '未配置额度';
  }
  return `${limit.requests} 次 / ${limit.windowSeconds} 秒`;
}

function tokenQuotaSummary(policy: TokenQuotaPolicy) {
  return `${policy.quota.tokens.toLocaleString('zh-CN')} Token / ${quotaWindowLabel(policy.quota.windowSeconds)} · ${tokenQuotaSubjectLabel(policy.subject.type)}`;
}

function quotaWindowLabel(windowSeconds: number) {
  if (windowSeconds % 86_400 === 0) {
    return `${windowSeconds / 86_400} 天`;
  }
  if (windowSeconds % 3_600 === 0) {
    return `${windowSeconds / 3_600} 小时`;
  }
  if (windowSeconds % 60 === 0) {
    return `${windowSeconds / 60} 分钟`;
  }
  return `${windowSeconds} 秒`;
}

function tokenQuotaSubjectLabel(type: TokenQuotaSubjectType) {
  const labels: Record<TokenQuotaSubjectType, string> = {
    Shared: '所有命中请求共用',
    IP: '每个来源 IP 独立',
    Header: '每个请求头值独立',
  };
  return labels[type];
}

async function savePolicy<T extends { id?: string }>(basePath: string, payload: T, message: string): Promise<PolicyMutationResult> {
  const path = payload.id ? `${basePath}/${encodeURIComponent(payload.id)}` : basePath;
  const response = await apiRequest<PolicyMutationResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
  return { message, changeId: response.id ?? payload.id };
}

async function deletePolicy(basePath: string, id: string, message: string): Promise<PolicyMutationResult> {
  await apiRequest<PolicyMutationResponse>(`${basePath}/${encodeURIComponent(id)}`, { method: 'DELETE' });
  return { message };
}

async function setPolicyEnabled(basePath: string, id: string, enabled: boolean, message: string): Promise<PolicyMutationResult> {
  await apiRequest<PolicyMutationResponse>(`${basePath}/${encodeURIComponent(id)}/enabled`, {
    method: 'PATCH',
    body: JSON.stringify({ enabled }),
  });
  return { message };
}
