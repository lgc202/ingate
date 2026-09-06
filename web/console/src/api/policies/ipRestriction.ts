import { apiListAllByCursor, type CursorPagedResponse } from '../client';
import type {
  IPRestrictionPolicy,
  IPRestrictionPolicyPayload,
  PolicyMutationResult,
} from '@/domain/policy';
import {
  deletePolicy,
  resourceStatus,
  savePolicy,
  targetFromResponse,
  trafficTargetToRequest,
  type PolicyTargetResponse,
} from './common';

interface PolicyResponse extends Omit<IPRestrictionPolicy, 'version' | 'targets' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  state: string;
  message: string;
}

interface PolicyListResponse extends CursorPagedResponse {
  policies?: PolicyResponse[];
}

export async function listIPRestrictionPolicies(): Promise<IPRestrictionPolicy[]> {
  const policies = await apiListAllByCursor<PolicyListResponse, PolicyResponse>(
    '/ip-restriction-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(targetFromResponse),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function saveIPRestrictionPolicy(
  payload: IPRestrictionPolicyPayload,
): Promise<PolicyMutationResult> {
  await savePolicy('/ip-restriction-policies', {
    ...payload,
    targets: payload.targets.map(trafficTargetToRequest),
  });
  return { message: `IP 访问限制策略已保存：${payload.name}`, changeId: payload.id };
}

export async function deleteIPRestrictionPolicy(
  id: string,
  version: number,
): Promise<PolicyMutationResult> {
  await deletePolicy('/ip-restriction-policies', id, version);
  return { message: 'IP 访问限制策略已删除' };
}

export function ipRestrictionSummary(policy: IPRestrictionPolicy) {
  return policy.allow.length > 0
    ? `仅允许 ${policy.allow.length} 个 IP / 网段`
    : `拒绝 ${policy.deny.length} 个 IP / 网段`;
}
