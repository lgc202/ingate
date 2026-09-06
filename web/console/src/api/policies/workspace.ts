import { listCallers } from '../callers';
import { listGateways } from '../gateways';
import { listWasmPluginMarketInstallations } from '../plugins';
import { listRoutes } from '../routes';
import type { GatewayListView } from '@/domain/gateway';
import type {
  GovernancePolicy,
  HeaderTransformationPolicy,
  IPRestrictionPolicy,
  MockResponsePolicy,
  PolicyTargetOption,
  PolicyTargetRef,
  PolicyWorkspace,
  RateLimitPolicy,
  TokenQuotaPolicy,
} from '@/domain/policy';
import type { RouteListView } from '@/domain/route';
import {
  deleteHeaderTransformationPolicy,
  headerTransformationSummary,
  listHeaderTransformationPolicies,
  saveHeaderTransformationPolicy,
} from './headerTransformation';
import {
  deleteIPRestrictionPolicy,
  ipRestrictionSummary,
  listIPRestrictionPolicies,
  saveIPRestrictionPolicy,
} from './ipRestriction';
import {
  deleteMockResponsePolicy,
  listMockResponsePolicies,
  saveMockResponsePolicy,
} from './mockResponse';
import {
  deleteRateLimitPolicy,
  listRateLimitPolicies,
  rateLimitSummary,
  saveRateLimitPolicy,
} from './rateLimit';
import {
  deleteTokenQuotaPolicy,
  listTokenQuotaPolicies,
  saveTokenQuotaPolicy,
  tokenQuotaSummary,
} from './tokenQuota';

// 策略列表只读取策略本身和低基数的插件安装摘要，避免每次翻页都全量扫描 Gateway、Route 和 Caller。
export async function getPolicyListWorkspace(): Promise<PolicyWorkspace> {
  const [
    ipRestrictionPolicies,
    rateLimitPolicies,
    tokenQuotaPolicies,
    headerTransformationPolicies,
    mockResponsePolicies,
    plugins,
  ] = await Promise.all([
    listIPRestrictionPolicies(),
    listRateLimitPolicies(),
    listTokenQuotaPolicies(),
    listHeaderTransformationPolicies(),
    listMockResponsePolicies(),
    listWasmPluginMarketInstallations(),
  ]);
  const policies = governancePolicies(
    ipRestrictionPolicies,
    rateLimitPolicies,
    tokenQuotaPolicies,
    headerTransformationPolicies,
    mockResponsePolicies,
  );
  return {
    policies,
    ipRestrictionPolicies,
    rateLimitPolicies,
    tokenQuotaPolicies,
    headerTransformationPolicies,
    mockResponsePolicies,
    installedPluginPackages: plugins.map((plugin) => plugin.package),
    targets: referencedPolicyTargets(policies),
  };
}

// 编辑器需要完整的目标候选项，仅在用户打开创建或编辑抽屉时读取。
export async function getPolicyEditorOptions(): Promise<PolicyTargetOption[]> {
  const [gatewayList, routeList, callers] = await Promise.all([listGateways(), listRoutes(), listCallers()]);
  return policyTargets(gatewayList, routeList, callers);
}

export function updateGovernancePolicyTargets(policy: GovernancePolicy, targets: PolicyTargetRef[]) {
  return saveGovernancePolicy(policy, { targets });
}

export function deleteGovernancePolicy(policy: GovernancePolicy) {
  const version = Number(policy.version);
  if (policy.kind === 'IPRestrictionPolicy') return deleteIPRestrictionPolicy(policy.id, version);
  if (policy.kind === 'RateLimitPolicy') return deleteRateLimitPolicy(policy.id, version);
  if (policy.kind === 'TokenQuotaPolicy') return deleteTokenQuotaPolicy(policy.id, version);
  if (policy.kind === 'HeaderTransformationPolicy') return deleteHeaderTransformationPolicy(policy.id, version);
  return deleteMockResponsePolicy(policy.id, version);
}

export function setGovernancePolicyEnabled(policy: GovernancePolicy, enabled: boolean) {
  return saveGovernancePolicy(policy, { enabled });
}

function governancePolicies(
  ipRestrictionPolicies: IPRestrictionPolicy[],
  rateLimitPolicies: RateLimitPolicy[],
  tokenQuotaPolicies: TokenQuotaPolicy[],
  headerTransformationPolicies: HeaderTransformationPolicy[],
  mockResponsePolicies: MockResponsePolicy[],
): GovernancePolicy[] {
  return [
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
      updatedAt: policy.updatedAt,
      raw: policy,
    }) satisfies GovernancePolicy),
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
      updatedAt: policy.updatedAt,
      raw: policy,
    }) satisfies GovernancePolicy),
    ...tokenQuotaPolicies.map((policy) => ({
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
    }) satisfies GovernancePolicy),
    ...headerTransformationPolicies.map((policy) => ({
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
    }) satisfies GovernancePolicy),
    ...mockResponsePolicies.map((policy) => ({
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
    }) satisfies GovernancePolicy),
  ].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
}

function saveGovernancePolicy(
  policy: GovernancePolicy,
  changes: { enabled?: boolean; targets?: PolicyTargetRef[] },
) {
  const enabled = changes.enabled ?? policy.raw.enabled;
  const targets = (changes.targets ?? policy.raw.targets).map((target) => ({
    kind: target.kind,
    id: target.id,
  }));
  if (policy.kind === 'IPRestrictionPolicy') {
    return saveIPRestrictionPolicy({
      id: policy.raw.id,
      version: policy.raw.version,
      name: policy.raw.name,
      enabled,
      targets,
      allow: policy.raw.allow,
      deny: policy.raw.deny,
    });
  }
  if (policy.kind === 'RateLimitPolicy') {
    return saveRateLimitPolicy({
      id: policy.raw.id,
      version: policy.raw.version,
      name: policy.raw.name,
      enabled,
      targets,
      subject: policy.raw.subject,
      limit: policy.raw.limit,
    });
  }
  if (policy.kind === 'TokenQuotaPolicy') {
    return saveTokenQuotaPolicy({
      id: policy.raw.id,
      version: policy.raw.version,
      name: policy.raw.name,
      enabled,
      targets,
      timeZone: policy.raw.timeZone,
      limits: policy.raw.limits,
    });
  }
  if (policy.kind === 'HeaderTransformationPolicy') {
    return saveHeaderTransformationPolicy({
      id: policy.raw.id,
      version: policy.raw.version,
      name: policy.raw.name,
      enabled,
      targets,
      requestRules: policy.raw.requestRules,
      responseRules: policy.raw.responseRules,
    });
  }
  return saveMockResponsePolicy({
    id: policy.raw.id,
    version: policy.raw.version,
    name: policy.raw.name,
    enabled,
    targets,
    statusCode: policy.raw.statusCode,
    contentType: policy.raw.contentType,
    headers: policy.raw.headers,
    body: policy.raw.body,
  });
}

function policyTargets(
  gateways: GatewayListView,
  routes: RouteListView,
  callers: Array<{ id: string; name: string }>,
): PolicyTargetOption[] {
  return [
    ...gateways.gateways.map((gateway) => ({
      id: gateway.id,
      name: gateway.name || gateway.id,
      kind: 'Gateway' as const,
    })),
    ...routes.routes.map((route) => ({
      id: route.id,
      name: route.name || route.id,
      kind: 'Route' as const,
    })),
    ...callers.map((caller) => ({
      id: caller.id,
      name: caller.name || caller.id,
      kind: 'Caller' as const,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
}

function referencedPolicyTargets(policies: GovernancePolicy[]): PolicyTargetOption[] {
  const targets = new Map<string, PolicyTargetOption>();
  for (const policy of policies) {
    for (const target of policy.targets) {
      targets.set(`${target.kind}:${target.id}`, {
        id: target.id,
        name: target.displayName || '已删除的目标',
        kind: target.kind,
      });
    }
  }
  return [...targets.values()].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
}
