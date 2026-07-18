import { apiRequest } from './client';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import type { GatewayListView } from '@/domain/gateway';
import type {
  AccessControlPolicy,
  AccessControlPolicyPayload,
  GovernancePolicy,
  PolicyBinding,
  PolicyBindingPayload,
  PolicyMutationResult,
  PolicyTargetOption,
  PolicyWorkspace,
  RateLimitPolicy,
  RateLimitPolicyPayload,
} from '@/domain/policy';
import type { RouteListView } from '@/domain/route';

interface PolicyMutationResponse {
  success: boolean;
  id?: string;
}

interface RateLimitPolicyListResponse {
  policies?: RateLimitPolicy[];
}

interface AccessControlPolicyListResponse {
  policies?: AccessControlPolicy[];
}

interface PolicyBindingListResponse {
  bindings?: PolicyBinding[];
}

export async function getPolicyWorkspace(): Promise<PolicyWorkspace> {
  const [rateLimitPolicies, accessControlPolicies, bindings, gatewayList, routeList] = await Promise.all([
    listRateLimitPolicies(),
    listAccessControlPolicies(),
    listPolicyBindings(),
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
      mode: policy.mode === 'Global' ? '全局共享计数' : '单实例计数',
      ruleCount: policy.rules.length,
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
      mode: policy.defaultAction === 'Deny' ? '默认拒绝' : '默认放行',
      ruleCount: policy.rules?.length ?? 0,
      createdAt: policy.createdAt,
      raw: policy,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name));

  return {
    policies,
    rateLimitPolicies,
    accessControlPolicies,
    bindings,
    targets: policyTargets(gatewayList, routeList),
  };
}

export async function listRateLimitPolicies(): Promise<RateLimitPolicy[]> {
  const response = await apiRequest<RateLimitPolicyListResponse>('/rate-limit-policies');
  return response.policies ?? [];
}

export async function listAccessControlPolicies(): Promise<AccessControlPolicy[]> {
  const response = await apiRequest<AccessControlPolicyListResponse>('/access-control-policies');
  return response.policies ?? [];
}

export async function listPolicyBindings(): Promise<PolicyBinding[]> {
  const response = await apiRequest<PolicyBindingListResponse>('/policy-bindings');
  return response.bindings ?? [];
}

export async function saveRateLimitPolicy(payload: RateLimitPolicyPayload) {
  return savePolicy('/rate-limit-policies', payload, `限流策略已保存：${payload.name}`);
}

export async function saveAccessControlPolicy(payload: AccessControlPolicyPayload) {
  return savePolicy('/access-control-policies', payload, `访问控制策略已保存：${payload.name}`);
}

export async function savePolicyBinding(payload: PolicyBindingPayload) {
  return savePolicy('/policy-bindings', payload, `策略绑定已保存：${payload.name}`);
}

export async function deleteRateLimitPolicy(id: string) {
  return deletePolicy('/rate-limit-policies', id, `限流策略已删除：${id}`);
}

export async function deleteAccessControlPolicy(id: string) {
  return deletePolicy('/access-control-policies', id, `访问控制策略已删除：${id}`);
}

export async function deletePolicyBinding(id: string) {
  return deletePolicy('/policy-bindings', id, `策略绑定已删除：${id}`);
}

export async function setRateLimitPolicyEnabled(id: string, enabled: boolean) {
  return setPolicyEnabled('/rate-limit-policies', id, enabled, `限流策略已${enabled ? '启用' : '停用'}：${id}`);
}

export async function setAccessControlPolicyEnabled(id: string, enabled: boolean) {
  return setPolicyEnabled('/access-control-policies', id, enabled, `访问控制策略已${enabled ? '启用' : '停用'}：${id}`);
}

export async function setPolicyBindingEnabled(id: string, enabled: boolean) {
  return setPolicyEnabled('/policy-bindings', id, enabled, `策略绑定已${enabled ? '启用' : '停用'}：${id}`);
}

function policyTargets(gateways: GatewayListView, routes: RouteListView): PolicyTargetOption[] {
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
      ruleNames: route.rules.map((rule) => rule.name),
    })),
  ].sort((a, b) => a.name.localeCompare(b.name));
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
