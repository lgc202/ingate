import { apiListAll, apiListAllByCursor, apiRequest } from './client';
import type { CursorPagedResponse, PagedResponse } from './client';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import type { GatewayListView } from '@/domain/gateway';
import type { ResourceState, ResourceStatus } from '@/domain/common';
import type {
  GovernancePolicy,
  IPRestrictionPolicy,
  IPRestrictionPolicyPayload,
  PolicyMutationResult,
  PolicyTargetKind,
  PolicyTargetOption,
  PolicyTargetRef,
  PolicyWorkspace,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  RateLimitSubjectType,
  TokenQuotaPolicy,
  TokenQuotaPolicyPayload,
  TokenQuotaSubjectType,
} from '@/domain/policy';
import { isModelRoute, type RouteListView } from '@/domain/route';

interface PolicyTargetResponse {
  kind: string;
  id: string;
  name: string;
  state: string;
  message: string;
}

interface RateLimitPolicyResponse extends Omit<RateLimitPolicy, 'version' | 'targets' | 'subject' | 'limit' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  subject: { type: string; headerName?: string };
  limit: { requests: string | number; windowSeconds: string | number };
  state: string;
  message: string;
}

interface IPRestrictionPolicyResponse extends Omit<IPRestrictionPolicy, 'version' | 'targets' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  state: string;
  message: string;
}

interface TokenQuotaPolicyResponse extends Omit<TokenQuotaPolicy, 'targets' | 'status'> {
  targets: PolicyTargetResponse[];
  status: { state: string; message: string };
}

interface RateLimitPolicyListResponse extends CursorPagedResponse {
  policies?: RateLimitPolicyResponse[];
}

interface IPRestrictionPolicyListResponse extends CursorPagedResponse {
  policies?: IPRestrictionPolicyResponse[];
}

interface TokenQuotaPolicyListResponse extends PagedResponse {
  policies?: TokenQuotaPolicyResponse[];
}

export async function getPolicyWorkspace(): Promise<PolicyWorkspace> {
  const [rateLimitPolicies, ipRestrictionPolicies, tokenQuotaPolicies, gatewayList, routeList] = await Promise.all([
    listRateLimitPolicies(),
    listIPRestrictionPolicies(),
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
      enabled: policy.enabled,
      summary: rateLimitSummary(policy),
      ruleCount: 1,
      targets: policy.targets,
      status: policy.status,
      createdAt: policy.createdAt,
      raw: policy,
    })),
    ...ipRestrictionPolicies.map((policy) => ({
      id: policy.id,
      version: policy.version,
      kind: 'IPRestrictionPolicy' as const,
      name: policy.name,
      enabled: policy.enabled,
      summary: ipRestrictionSummary(policy),
      ruleCount: policy.allow.length + policy.deny.length,
      targets: policy.targets,
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
      targets: policy.targets,
      status: policy.status,
      createdAt: policy.createdAt,
      raw: policy,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));

  return {
    policies,
    rateLimitPolicies,
    ipRestrictionPolicies,
    tokenQuotaPolicies,
    targets: policyTargets(gatewayList, routeList),
  };
}

export async function listRateLimitPolicies(): Promise<RateLimitPolicy[]> {
  const policies = await apiListAllByCursor<RateLimitPolicyListResponse, RateLimitPolicyResponse>(
    '/rate-limit-policies',
    (page) => page.policies ?? [],
  );
  return policies.map(rateLimitPolicyFromResponse);
}

export async function listIPRestrictionPolicies(): Promise<IPRestrictionPolicy[]> {
  const policies = await apiListAllByCursor<IPRestrictionPolicyListResponse, IPRestrictionPolicyResponse>(
    '/ip-restriction-policies',
    (page) => page.policies ?? [],
  );
  return policies.map(ipRestrictionPolicyFromResponse);
}

export async function listTokenQuotaPolicies(): Promise<TokenQuotaPolicy[]> {
  const policies = await apiListAll<TokenQuotaPolicyListResponse, TokenQuotaPolicyResponse>(
    '/token-quota-policies',
    (page) => page.policies ?? [],
  );
  return policies.map(tokenQuotaPolicyFromResponse);
}

export async function saveRateLimitPolicy(payload: RateLimitPolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/rate-limit-policies', rateLimitPayloadToRequest(payload));
  return { message: `限流策略已保存：${payload.name}`, changeId: payload.id };
}

export async function saveIPRestrictionPolicy(payload: IPRestrictionPolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/ip-restriction-policies', policyPayloadToRequest(payload));
  return { message: `IP 访问限制策略已保存：${payload.name}`, changeId: payload.id };
}

export async function saveTokenQuotaPolicy(payload: TokenQuotaPolicyPayload): Promise<PolicyMutationResult> {
  const response = await savePolicy<{ id?: string }>('/token-quota-policies', policyPayloadToRequest(payload));
  return { message: `Token 配额策略已保存：${payload.name}`, changeId: response.id ?? payload.id };
}

export async function updateGovernancePolicyTargets(policy: GovernancePolicy, targets: PolicyTargetRef[]) {
  const normalizedTargets = targets.map((target) => ({ kind: target.kind, id: target.id }));
  switch (policy.kind) {
    case 'RateLimitPolicy':
      return saveRateLimitPolicy({
        id: policy.raw.id,
        version: policy.raw.version,
        name: policy.raw.name,
        enabled: policy.raw.enabled,
        targets: normalizedTargets,
        subject: policy.raw.subject,
        limit: policy.raw.limit,
      });
    case 'IPRestrictionPolicy':
      return saveIPRestrictionPolicy({
        id: policy.raw.id,
        version: policy.raw.version,
        name: policy.raw.name,
        enabled: policy.raw.enabled,
        targets: normalizedTargets,
        allow: policy.raw.allow,
        deny: policy.raw.deny,
      });
    case 'TokenQuotaPolicy':
      return saveTokenQuotaPolicy({
        id: policy.raw.id,
        version: policy.raw.version,
        name: policy.raw.name,
        description: policy.raw.description,
        enabled: policy.raw.enabled,
        targets: normalizedTargets,
        subject: policy.raw.subject,
        quota: policy.raw.quota,
        failurePolicy: policy.raw.failurePolicy,
        response: policy.raw.response,
      });
  }
}

export async function deleteRateLimitPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/rate-limit-policies', id, version);
  return { message: '限流策略已删除' };
}

export async function deleteIPRestrictionPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/ip-restriction-policies', id, version);
  return { message: 'IP 访问限制策略已删除' };
}

export async function deleteTokenQuotaPolicy(id: string): Promise<PolicyMutationResult> {
  await apiRequest<Record<string, never>>(`/token-quota-policies/${encodeURIComponent(id)}`, { method: 'DELETE' });
  return { message: 'Token 配额策略已删除' };
}

export async function setGovernancePolicyEnabled(policy: GovernancePolicy, enabled: boolean) {
  if (policy.kind === 'RateLimitPolicy') {
    return saveRateLimitPolicy({
      id: policy.raw.id,
      version: policy.raw.version,
      name: policy.raw.name,
      enabled,
      targets: policy.raw.targets,
      subject: policy.raw.subject,
      limit: policy.raw.limit,
    });
  }
  if (policy.kind === 'IPRestrictionPolicy') {
    return saveIPRestrictionPolicy({
      id: policy.raw.id,
      version: policy.raw.version,
      name: policy.raw.name,
      enabled,
      targets: policy.raw.targets,
      allow: policy.raw.allow,
      deny: policy.raw.deny,
    });
  }
  await apiRequest<Record<string, never>>(`/token-quota-policies/${encodeURIComponent(policy.id)}/enabled`, {
    method: 'PATCH',
    body: JSON.stringify({ enabled }),
  });
  return { message: `Token 配额策略已${enabled ? '启用' : '停用'}：${policy.name}` };
}

function rateLimitPolicyFromResponse(policy: RateLimitPolicyResponse): RateLimitPolicy {
  return {
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(policyTargetFromResponse),
    subject: {
      type: rateLimitSubjectTypeFromResponse(policy.subject.type),
      headerName: policy.subject.headerName,
    },
    limit: {
      requests: Number(policy.limit.requests),
      windowSeconds: Number(policy.limit.windowSeconds),
    },
    status: resourceStatus(policy.state, policy.message),
  };
}

function ipRestrictionPolicyFromResponse(policy: IPRestrictionPolicyResponse): IPRestrictionPolicy {
  return {
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(policyTargetFromResponse),
    status: resourceStatus(policy.state, policy.message),
  };
}

function tokenQuotaPolicyFromResponse(policy: TokenQuotaPolicyResponse): TokenQuotaPolicy {
  return {
    ...policy,
    targets: policy.targets.map(policyTargetFromResponse),
    quota: {
      tokens: Number(policy.quota.tokens),
      windowSeconds: Number(policy.quota.windowSeconds),
    },
    status: resourceStatus(policy.status.state, policy.status.message),
  };
}

function policyTargetFromResponse(target: PolicyTargetResponse): PolicyTargetRef {
  return {
    kind: policyTargetKindFromResponse(target.kind),
    id: target.id,
    displayName: target.name,
    status: resourceStatus(target.state, target.message),
  };
}

function policyTargetKindFromResponse(value: string): PolicyTargetKind {
  if (value === 'POLICY_TARGET_KIND_GATEWAY') return 'Gateway';
  if (value === 'POLICY_TARGET_KIND_ROUTE') return 'Route';
  throw new Error(`服务返回了未知的策略目标类型：${value}`);
}

function resourceStatus(state: string, message: string): ResourceStatus {
  const states: Record<string, ResourceState> = {
    READY: 'Ready',
    PENDING: 'Pending',
    ERROR: 'Error',
    DISABLED: 'Disabled',
  };
  return { state: states[state] ?? 'Pending', message };
}

function rateLimitSubjectTypeFromResponse(value: string): RateLimitSubjectType {
  const types: Record<string, RateLimitSubjectType> = {
    RATE_LIMIT_SUBJECT_TYPE_SHARED: 'Shared',
    RATE_LIMIT_SUBJECT_TYPE_IP: 'IP',
    RATE_LIMIT_SUBJECT_TYPE_HEADER: 'Header',
  };
  return types[value] ?? 'Shared';
}

function targetKindToRequest(kind: PolicyTargetKind) {
  return kind === 'Gateway' ? 'POLICY_TARGET_KIND_GATEWAY' : 'POLICY_TARGET_KIND_ROUTE';
}

function subjectTypeToRequest(type: RateLimitSubjectType) {
  return `RATE_LIMIT_SUBJECT_TYPE_${type.toUpperCase()}`;
}

function policyPayloadToRequest<T extends { targets: Array<{ kind: PolicyTargetKind; id: string }> }>(payload: T) {
  return {
    ...payload,
    targets: payload.targets.map((target) => ({ id: target.id, kind: targetKindToRequest(target.kind) })),
  };
}

function rateLimitPayloadToRequest(payload: RateLimitPolicyPayload) {
  return {
    ...policyPayloadToRequest(payload),
    subject: { ...payload.subject, type: subjectTypeToRequest(payload.subject.type) },
  };
}

async function savePolicy<TResponse = Record<string, never>>(
  basePath: string,
  payload: { id?: string },
): Promise<TResponse> {
  const path = payload.id ? `${basePath}/${encodeURIComponent(payload.id)}` : basePath;
  return apiRequest<TResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
}

async function deleteVersionedPolicy(basePath: string, id: string, version: number) {
  const query = new URLSearchParams({ version: String(version) });
  await apiRequest<Record<string, never>>(`${basePath}/${encodeURIComponent(id)}?${query}`, { method: 'DELETE' });
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
  ].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
}

function rateLimitSummary(policy: RateLimitPolicy) {
  const subject: Record<RateLimitSubjectType, string> = {
    Shared: '全部请求共用',
    IP: '每个 IP 独立',
    Header: `按 ${policy.subject.headerName || '请求头'} 独立`,
  };
  return `${policy.limit.requests} 次 / ${quotaWindowLabel(policy.limit.windowSeconds)} · ${subject[policy.subject.type]}`;
}

function ipRestrictionSummary(policy: IPRestrictionPolicy) {
  return policy.allow.length > 0
    ? `仅允许 ${policy.allow.length} 个 IP / 网段`
    : `拒绝 ${policy.deny.length} 个 IP / 网段`;
}

function tokenQuotaSummary(policy: TokenQuotaPolicy) {
  return `${policy.quota.tokens.toLocaleString('zh-CN')} Token / ${quotaWindowLabel(policy.quota.windowSeconds)} · ${tokenQuotaSubjectLabel(policy.subject.type)}`;
}

function quotaWindowLabel(windowSeconds: number) {
  if (windowSeconds % 86_400 === 0) return `${windowSeconds / 86_400} 天`;
  if (windowSeconds % 3_600 === 0) return `${windowSeconds / 3_600} 小时`;
  if (windowSeconds % 60 === 0) return `${windowSeconds / 60} 分钟`;
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
