import { apiListAllByCursor, type CursorPagedResponse } from '../client';
import type {
  MockResponsePolicy,
  MockResponsePolicyPayload,
  PolicyMutationResult,
} from '@/domain/policy';
import {
  deletePolicy,
  resourceStatus,
  savePolicy,
  targetFromResponse,
  type PolicyTargetResponse,
} from './common';

interface PolicyResponse extends Omit<MockResponsePolicy, 'version' | 'targets' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  state: string;
  message: string;
}

interface PolicyListResponse extends CursorPagedResponse {
  policies?: PolicyResponse[];
}

export async function listMockResponsePolicies(): Promise<MockResponsePolicy[]> {
  const policies = await apiListAllByCursor<PolicyListResponse, PolicyResponse>(
    '/mock-response-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(targetFromResponse),
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

export async function deleteMockResponsePolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deletePolicy('/mock-response-policies', id, version);
  return { message: '模拟响应策略已删除' };
}
