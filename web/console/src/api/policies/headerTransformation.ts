import { apiListAllByCursor, type CursorPagedResponse } from '../client';
import type {
  HeaderTransformationOperation,
  HeaderTransformationPolicy,
  HeaderTransformationPolicyPayload,
  HeaderTransformationRule,
  PolicyMutationResult,
} from '@/domain/policy';
import {
  deletePolicy,
  resourceStatus,
  savePolicy,
  targetFromResponse,
  type PolicyTargetResponse,
} from './common';

interface RuleResponse {
  operation: string;
  name: string;
  value: string;
}

interface PolicyResponse extends Omit<
  HeaderTransformationPolicy,
  'version' | 'targets' | 'requestRules' | 'responseRules' | 'status'
> {
  version: string | number;
  targets: PolicyTargetResponse[];
  requestRules: RuleResponse[];
  responseRules: RuleResponse[];
  state: string;
  message: string;
}

interface PolicyListResponse extends CursorPagedResponse {
  policies?: PolicyResponse[];
}

export async function listHeaderTransformationPolicies(): Promise<HeaderTransformationPolicy[]> {
  const policies = await apiListAllByCursor<PolicyListResponse, PolicyResponse>(
    '/header-transformation-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(targetFromResponse),
    requestRules: policy.requestRules.map(ruleFromResponse),
    responseRules: policy.responseRules.map(ruleFromResponse),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function saveHeaderTransformationPolicy(
  payload: HeaderTransformationPolicyPayload,
): Promise<PolicyMutationResult> {
  await savePolicy('/header-transformation-policies', {
    ...payload,
    targets: payload.targets.map((target) => ({ id: target.id, kind: 'POLICY_TARGET_KIND_ROUTE' })),
    requestRules: payload.requestRules.map(ruleToRequest),
    responseRules: payload.responseRules.map(ruleToRequest),
  });
  return { message: `请求响应转换策略已保存：${payload.name}`, changeId: payload.id };
}

export async function deleteHeaderTransformationPolicy(
  id: string,
  version: number,
): Promise<PolicyMutationResult> {
  await deletePolicy('/header-transformation-policies', id, version);
  return { message: '请求响应转换策略已删除' };
}

export function headerTransformationSummary(policy: HeaderTransformationPolicy) {
  const parts = [];
  if (policy.requestRules.length > 0) parts.push(`${policy.requestRules.length} 条请求规则`);
  if (policy.responseRules.length > 0) parts.push(`${policy.responseRules.length} 条响应规则`);
  return parts.join(' · ');
}

function ruleFromResponse(rule: RuleResponse): HeaderTransformationRule {
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

function ruleToRequest(rule: HeaderTransformationRule) {
  return {
    operation: `HEADER_TRANSFORMATION_OPERATION_${rule.operation.toUpperCase()}`,
    name: rule.name,
    value: rule.value,
  };
}
