import { apiListAllByCursor, apiRequest, type CursorPagedResponse } from './client';
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

export async function getPolicyWorkspace(): Promise<PolicyWorkspace> {
  const [ipRestrictionPolicies, gatewayList, routeList] = await Promise.all([
    listIPRestrictionPolicies(),
    listGateways(),
    listRoutes(),
  ]);
  const policies: GovernancePolicy[] = ipRestrictionPolicies.map((policy) => ({
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
  })).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
  return { policies, ipRestrictionPolicies, targets: policyTargets(gatewayList, routeList) };
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

export function updateGovernancePolicyTargets(policy: GovernancePolicy, targets: PolicyTargetRef[]) {
  const normalized = targets.map((target) => ({ kind: target.kind, id: target.id }));
  return saveIPRestrictionPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled: policy.raw.enabled, targets: normalized, allow: policy.raw.allow, deny: policy.raw.deny });
}

export async function deleteIPRestrictionPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deleteVersionedPolicy('/ip-restriction-policies', id, version);
  return { message: 'IP 访问限制策略已删除' };
}

export function setGovernancePolicyEnabled(policy: GovernancePolicy, enabled: boolean) {
  return saveIPRestrictionPolicy({ id: policy.raw.id, version: policy.raw.version, name: policy.raw.name, enabled, targets: policy.raw.targets, allow: policy.raw.allow, deny: policy.raw.deny });
}

function policyTargetFromResponse(target: PolicyTargetResponse): PolicyTargetRef {
  return { kind: policyTargetKindFromResponse(target.kind), id: target.id, displayName: target.name, status: resourceStatus(target.state, target.message) };
}

function policyTargetKindFromResponse(value: string): PolicyTargetKind {
  if (value === 'POLICY_TARGET_KIND_GATEWAY') return 'Gateway';
  if (value === 'POLICY_TARGET_KIND_ROUTE') return 'Route';
  throw new Error(`服务返回了未知的策略目标类型：${value}`);
}

function resourceStatus(state: string, message: string): ResourceStatus {
  const states: Record<string, ResourceState> = { READY: 'Ready', PENDING: 'Pending', ERROR: 'Error', DISABLED: 'Disabled' };
  return { state: states[state] ?? 'Pending', message };
}

function policyPayloadToRequest(payload: IPRestrictionPolicyPayload) {
  return { ...payload, targets: payload.targets.map((target) => ({ id: target.id, kind: target.kind === 'Gateway' ? 'POLICY_TARGET_KIND_GATEWAY' : 'POLICY_TARGET_KIND_ROUTE' })) };
}

async function savePolicy(basePath: string, payload: { id?: string }) {
  const path = payload.id ? `${basePath}/${encodeURIComponent(payload.id)}` : basePath;
  await apiRequest(path, { method: payload.id ? 'PUT' : 'POST', body: JSON.stringify(payload) });
}

async function deleteVersionedPolicy(basePath: string, id: string, version: number) {
  await apiRequest(`${basePath}/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}

function policyTargets(gateways: GatewayListView, routes: RouteListView): PolicyTargetOption[] {
  return [
    ...gateways.gateways.map((gateway) => ({ id: gateway.id, name: gateway.name || gateway.id, kind: 'Gateway' as const })),
    ...routes.routes.map((route) => ({ id: route.id, name: route.name || route.id, kind: 'Route' as const })),
  ].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'));
}

function ipRestrictionSummary(policy: IPRestrictionPolicy) {
  return policy.allow.length > 0 ? `仅允许 ${policy.allow.length} 个 IP / 网段` : `拒绝 ${policy.deny.length} 个 IP / 网段`;
}
