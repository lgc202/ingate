import { apiListAllByCursor, apiRequest, type CursorPagedResponse } from './client';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import { listCallers } from './callers';
import { listWasmPlugins } from './plugins';
import type { GatewayListView } from '@/domain/gateway';
import type { ResourceState, ResourceStatus } from '@/domain/common';
import type {
  GovernancePolicy,
  CallerTokenQuotaUsage,
  HeaderTransformationOperation,
  HeaderTransformationPolicy,
  HeaderTransformationPolicyPayload,
  HeaderTransformationRule,
  IPRestrictionPolicy,
  IPRestrictionPolicyPayload,
  MockResponsePolicy,
  MockResponsePolicyPayload,
  PolicyMutationResult,
  PolicyTargetKind,
  PolicyTargetOption,
  PolicyTargetRef,
  PolicyWorkspace,
  TokenQuotaLimit,
  TokenQuotaPeriod,
  TokenQuotaPolicy,
  TokenQuotaPolicyPayload,
} from '@/domain/policy';
import type { RouteListView } from '@/domain/route';

interface PolicyTargetResponse {
  kind: string;
  id: string;
  name: string;
  state: string;
  message: string;
}

interface IPRestrictionPolicyResponse extends Omit<IPRestrictionPolicy, 'version' | 'targets' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  state: string;
  message: string;
}

interface IPRestrictionPolicyListResponse extends CursorPagedResponse { policies?: IPRestrictionPolicyResponse[] }

interface TokenQuotaLimitResponse {
  period: string;
  tokens: string | number;
}

interface TokenQuotaPolicyResponse extends Omit<TokenQuotaPolicy, 'version' | 'targets' | 'limits' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  limits: TokenQuotaLimitResponse[];
  state: string;
  message: string;
}

interface TokenQuotaPolicyListResponse extends CursorPagedResponse { policies?: TokenQuotaPolicyResponse[] }

interface HeaderTransformationRuleResponse {
  operation: string;
  name: string;
  value: string;
}

interface HeaderTransformationPolicyResponse extends Omit<HeaderTransformationPolicy, 'version' | 'targets' | 'requestRules' | 'responseRules' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  requestRules: HeaderTransformationRuleResponse[];
  responseRules: HeaderTransformationRuleResponse[];
  state: string;
  message: string;
}

interface HeaderTransformationPolicyListResponse extends CursorPagedResponse { policies?: HeaderTransformationPolicyResponse[] }

interface MockResponsePolicyResponse extends Omit<MockResponsePolicy, 'version' | 'targets' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  state: string;
  message: string;
}

interface MockResponsePolicyListResponse extends CursorPagedResponse { policies?: MockResponsePolicyResponse[] }

interface CallerTokenQuotaUsageResponse {
  policyId: string;
  policyName: string;
  period: string;
  usedTokens: string | number;
  limitTokens: string | number;
  remainingTokens: string | number;
  startedAt: string;
  resetsAt: string;
}

interface GetCallerTokenQuotaUsageResponse { usages?: CallerTokenQuotaUsageResponse[] }

export async function getPolicyWorkspace(): Promise<PolicyWorkspace> {
  const [ipRestrictionPolicies, tokenQuotaPolicies, headerTransformationPolicies, mockResponsePolicies, gatewayList, routeList, callers, plugins] = await Promise.all([
    listIPRestrictionPolicies(),
    listTokenQuotaPolicies(),
    listHeaderTransformationPolicies(),
    listMockResponsePolicies(),
    listGateways(),
    listRoutes(),
    listCallers(),
    listWasmPlugins(),
  ]);
  const policies: GovernancePolicy[] = [...ipRestrictionPolicies.map((policy) => ({
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
    updatedAt: policy.updatedAt,
    raw: policy,
  }) satisfies GovernancePolicy), ...tokenQuotaPolicies.map((policy) => ({
    id: policy.id,
    version: policy.version,
    kind: 'TokenQuotaPolicy' as const,
    name: policy.name,
    enabled: policy.enabled,
    summary: tokenQuotaSummary(policy.limits),
    ruleCount: policy.limits.length,
    targets: policy.targets,
    status: policy.status,
    createdAt: policy.createdAt,
    updatedAt: policy.updatedAt,
    raw: policy,
  }) satisfies GovernancePolicy), ...headerTransformationPolicies.map((policy) => ({
    id: policy.id,
    version: policy.version,
    kind: 'HeaderTransformationPolicy' as const,
    name: policy.name,
    enabled: policy.enabled,
    summary: headerTransformationSummary(policy),
    ruleCount: policy.requestRules.length + policy.responseRules.length,
    targets: policy.targets,
    status: policy.status,
    createdAt: policy.createdAt,
    updatedAt: policy.updatedAt,
    raw: policy,
  }) satisfies GovernancePolicy), ...mockResponsePolicies.map((policy) => ({
    id: policy.id,
    version: policy.version,
    kind: 'MockResponsePolicy' as const,
    name: policy.name,
    enabled: policy.enabled,
    summary: `${policy.statusCode} · ${policy.contentType}`,
    ruleCount: policy.headers.length + 1,
    targets: policy.targets,
    status: policy.status,
    createdAt: policy.createdAt,
    updatedAt: policy.updatedAt,
    raw: policy,
  }) satisfies GovernancePolicy)].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
  return {
    policies,
    ipRestrictionPolicies,
    tokenQuotaPolicies,
    headerTransformationPolicies,
    mockResponsePolicies,
    installedPluginPackages: plugins.map((plugin) => plugin.package),
    targets: policyTargets(gatewayList, routeList, callers),
  };
}

export async function listIPRestrictionPolicies(): Promise<IPRestrictionPolicy[]> {
  const policies = await apiListAllByCursor<IPRestrictionPolicyListResponse, IPRestrictionPolicyResponse>('/ip-restriction-policies', (page) => page.policies ?? []);
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(policyTargetFromResponse),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function saveIPRestrictionPolicy(payload: IPRestrictionPolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/ip-restriction-policies', policyPayloadToRequest(payload));
  return { message: `IP 访问限制策略已保存：${payload.name}`, changeId: payload.id };
}

export async function listTokenQuotaPolicies(): Promise<TokenQuotaPolicy[]> {
  const policies = await apiListAllByCursor<TokenQuotaPolicyListResponse, TokenQuotaPolicyResponse>('/token-quota-policies', (page) => page.policies ?? []);
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(policyTargetFromResponse),
    limits: policy.limits.map((limit) => ({ period: tokenQuotaPeriodFromResponse(limit.period), tokens: Number(limit.tokens) })),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function getCallerTokenQuotaUsage(callerID: string): Promise<CallerTokenQuotaUsage[]> {
  const response = await apiRequest<GetCallerTokenQuotaUsageResponse>(
    `/callers/${encodeURIComponent(callerID)}/token-quota-usage`,
  );
  return (response.usages ?? []).map((usage) => ({
    policyID: usage.policyId,
    policyName: usage.policyName,
    period: tokenQuotaPeriodFromResponse(usage.period),
    usedTokens: Number(usage.usedTokens),
    limitTokens: Number(usage.limitTokens),
    remainingTokens: Number(usage.remainingTokens),
    startedAt: usage.startedAt,
    resetsAt: usage.resetsAt,
  }));
}

export async function saveTokenQuotaPolicy(payload: TokenQuotaPolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/token-quota-policies', {
    ...payload,
    targets: payload.targets.map((target) => ({ id: target.id, kind: 'POLICY_TARGET_KIND_CALLER' })),
    limits: payload.limits.map((limit) => ({ period: tokenQuotaPeriodToRequest(limit.period), tokens: limit.tokens })),
  });
  return { message: `Token 额度策略已保存：${payload.name}`, changeId: payload.id };
}

export async function listHeaderTransformationPolicies(): Promise<HeaderTransformationPolicy[]> {
  const policies = await apiListAllByCursor<HeaderTransformationPolicyListResponse, HeaderTransformationPolicyResponse>(
    '/header-transformation-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(policyTargetFromResponse),
    requestRules: policy.requestRules.map(headerTransformationRuleFromResponse),
    responseRules: policy.responseRules.map(headerTransformationRuleFromResponse),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function saveHeaderTransformationPolicy(payload: HeaderTransformationPolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/header-transformation-policies', {
    ...payload,
    targets: payload.targets.map((target) => ({ id: target.id, kind: 'POLICY_TARGET_KIND_ROUTE' })),
    requestRules: payload.requestRules.map(headerTransformationRuleToRequest),
    responseRules: payload.responseRules.map(headerTransformationRuleToRequest),
  });
  return { message: `请求响应转换策略已保存：${payload.name}`, changeId: payload.id };
}

export async function listMockResponsePolicies(): Promise<MockResponsePolicy[]> {
  const policies = await apiListAllByCursor<MockResponsePolicyListResponse, MockResponsePolicyResponse>(
    '/mock-response-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(policyTargetFromResponse),
    headers: policy.headers.map((header) => ({ name: header.name, value: header.value })),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function saveMockResponsePolicy(payload: MockResponsePolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/mock-response-policies', {
    ...payload,
    targets: payload.targets.map((target) => ({ id: target.id, kind: 'POLICY_TARGET_KIND_ROUTE' })),
  });
  return { message: `模拟响应策略已保存：${payload.name}`, changeId: payload.id };
}

export function updateGovernancePolicyTargets(policy: GovernancePolicy, targets: PolicyTargetRef[]) {
  const normalized = targets.map((target) => ({ kind: target.kind, id: target.id }));
  if (policy.kind === 'IPRestrictionPolicy') {
    return saveIPRestrictionPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled: policy.raw.enabled, targets: normalized, allow: policy.raw.allow, deny: policy.raw.deny });
  }
  if (policy.kind === 'TokenQuotaPolicy') {
    return saveTokenQuotaPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled: policy.raw.enabled, targets: normalized, timeZone: policy.raw.timeZone, limits: policy.raw.limits });
  }
  if (policy.kind === 'HeaderTransformationPolicy') {
    return saveHeaderTransformationPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled: policy.raw.enabled, targets: normalized, requestRules: policy.raw.requestRules, responseRules: policy.raw.responseRules });
  }
  return saveMockResponsePolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled: policy.raw.enabled, targets: normalized, statusCode: policy.raw.statusCode, contentType: policy.raw.contentType, headers: policy.raw.headers, body: policy.raw.body });
}

export async function deleteIPRestrictionPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/ip-restriction-policies', id, version);
  return { message: 'IP 访问限制策略已删除' };
}

export async function deleteTokenQuotaPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/token-quota-policies', id, version);
  return { message: 'Token 额度策略已删除' };
}

export async function deleteHeaderTransformationPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/header-transformation-policies', id, version);
  return { message: '请求响应转换策略已删除' };
}

export async function deleteMockResponsePolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/mock-response-policies', id, version);
  return { message: '模拟响应策略已删除' };
}

export function setGovernancePolicyEnabled(policy: GovernancePolicy, enabled: boolean) {
  if (policy.kind === 'IPRestrictionPolicy') {
    return saveIPRestrictionPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled, targets: policy.raw.targets, allow: policy.raw.allow, deny: policy.raw.deny });
  }
  if (policy.kind === 'TokenQuotaPolicy') {
    return saveTokenQuotaPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled, targets: policy.raw.targets, timeZone: policy.raw.timeZone, limits: policy.raw.limits });
  }
  if (policy.kind === 'HeaderTransformationPolicy') {
    return saveHeaderTransformationPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled, targets: policy.raw.targets, requestRules: policy.raw.requestRules, responseRules: policy.raw.responseRules });
  }
  return saveMockResponsePolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled, targets: policy.raw.targets, statusCode: policy.raw.statusCode, contentType: policy.raw.contentType, headers: policy.raw.headers, body: policy.raw.body });
}

function policyTargetFromResponse(target: PolicyTargetResponse): PolicyTargetRef {
  return { kind: policyTargetKindFromResponse(target.kind), id: target.id, displayName: target.name, status: resourceStatus(target.state, target.message) };
}

function policyTargetKindFromResponse(value: string): PolicyTargetKind {
  if (value === 'POLICY_TARGET_KIND_GATEWAY') return 'Gateway';
  if (value === 'POLICY_TARGET_KIND_ROUTE') return 'Route';
  if (value === 'POLICY_TARGET_KIND_CALLER') return 'Caller';
  throw new Error(`服务返回了未知的策略目标类型：${value}`);
}

function resourceStatus(state: string, message: string): ResourceStatus {
  const states: Record<string, ResourceState> = { READY: 'Ready', PENDING: 'Pending', ERROR: 'Error', DISABLED: 'Disabled' };
  return { state: states[state] ?? 'Pending', message };
}

function policyPayloadToRequest(payload: IPRestrictionPolicyPayload) {
  return { ...payload, targets: payload.targets.map((target) => ({ id: target.id, kind: target.kind === 'Gateway' ? 'POLICY_TARGET_KIND_GATEWAY' : 'POLICY_TARGET_KIND_ROUTE' })) };
}

async function savePolicy(basePath: string, payload: Record<string, unknown> & { id?: string }) {
  const path = payload.id ? `${basePath}/${encodeURIComponent(payload.id)}` : basePath;
  await apiRequest(path, { method: payload.id ? 'PUT' : 'POST', body: JSON.stringify(payload) });
}

async function deleteVersionedPolicy(basePath: string, id: string, version: number) {
  await apiRequest(`${basePath}/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}

function policyTargets(gateways: GatewayListView, routes: RouteListView, callers: Array<{ id: string; name: string }>): PolicyTargetOption[] {
  return [
    ...gateways.gateways.map((gateway) => ({ id: gateway.id, name: gateway.name || gateway.id, kind: 'Gateway' as const })),
    ...routes.routes.map((route) => ({ id: route.id, name: route.name || route.id, kind: 'Route' as const })),
    ...callers.map((caller) => ({ id: caller.id, name: caller.name || caller.id, kind: 'Caller' as const })),
  ].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
}

function tokenQuotaPeriodFromResponse(value: string): TokenQuotaPeriod {
  if (value === 'TOKEN_QUOTA_PERIOD_DAY') return 'Day';
  if (value === 'TOKEN_QUOTA_PERIOD_WEEK') return 'Week';
  if (value === 'TOKEN_QUOTA_PERIOD_MONTH') return 'Month';
  throw new Error(`服务返回了未知的额度周期：${value}`);
}

function tokenQuotaPeriodToRequest(value: TokenQuotaPeriod) {
  return `TOKEN_QUOTA_PERIOD_${value.toUpperCase()}`;
}

function headerTransformationRuleFromResponse(rule: HeaderTransformationRuleResponse): HeaderTransformationRule {
  const operations: Record<string, HeaderTransformationOperation> = {
    HEADER_TRANSFORMATION_OPERATION_REMOVE: 'remove',
    HEADER_TRANSFORMATION_OPERATION_RENAME: 'rename',
    HEADER_TRANSFORMATION_OPERATION_REPLACE: 'replace',
    HEADER_TRANSFORMATION_OPERATION_ADD: 'add',
    HEADER_TRANSFORMATION_OPERATION_APPEND: 'append',
  };
  const operation = operations[rule.operation];
  if (!operation) throw new Error(`服务返回了未知的 Header 转换操作：${rule.operation}`);
  return { operation, name: rule.name, value: rule.value };
}

function headerTransformationRuleToRequest(rule: HeaderTransformationRule) {
  return {
    operation: `HEADER_TRANSFORMATION_OPERATION_${rule.operation.toUpperCase()}`,
    name: rule.name,
    value: rule.value,
  };
}

function tokenQuotaSummary(limits: TokenQuotaLimit[]) {
  const labels: Record<TokenQuotaPeriod, string> = { Day: '每日', Week: '每周', Month: '每月' };
  return limits.map((limit) => `${labels[limit.period]} ${formatTokenCount(limit.tokens)}`).join(' · ');
}

function formatTokenCount(tokens: number) {
  return new Intl.NumberFormat('zh-CN', { notation: tokens >= 10_000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(tokens);
}

function ipRestrictionSummary(policy: IPRestrictionPolicy) {
  return policy.allow.length > 0 ? `仅允许 ${policy.allow.length} 个 IP / 网段` : `拒绝 ${policy.deny.length} 个 IP / 网段`;
}

function headerTransformationSummary(policy: HeaderTransformationPolicy) {
  const parts = [];
  if (policy.requestRules.length > 0) parts.push(`${policy.requestRules.length} 条请求规则`);
  if (policy.responseRules.length > 0) parts.push(`${policy.responseRules.length} 条响应规则`);
  return parts.join(' · ');
}
