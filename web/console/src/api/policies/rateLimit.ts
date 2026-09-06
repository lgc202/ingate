import { apiListAllByCursor, type CursorPagedResponse } from '../client';
import type {
  PolicyMutationResult,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  RateLimitSubjectType,
} from '@/domain/policy';
import {
  deletePolicy,
  resourceStatus,
  savePolicy,
  targetFromResponse,
  trafficTargetToRequest,
  type PolicyTargetResponse,
} from './common';

interface PolicyResponse extends Omit<RateLimitPolicy, 'version' | 'targets' | 'subject' | 'limit' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  subject: { type: string; headerName: string };
  limit: { requests: string | number; windowSeconds: string | number };
  state: string;
  message: string;
}

interface PolicyListResponse extends CursorPagedResponse {
  policies?: PolicyResponse[];
}

export async function listRateLimitPolicies(): Promise<RateLimitPolicy[]> {
  const policies = await apiListAllByCursor<PolicyListResponse, PolicyResponse>(
    '/rate-limit-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(targetFromResponse),
    subject: {
      type: rateLimitSubjectFromResponse(policy.subject.type),
      headerName: policy.subject.headerName,
    },
    limit: {
      requests: Number(policy.limit.requests),
      windowSeconds: Number(policy.limit.windowSeconds),
    },
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function saveRateLimitPolicy(payload: RateLimitPolicyPayload): Promise<PolicyMutationResult> {
  await savePolicy('/rate-limit-policies', {
    ...payload,
    targets: payload.targets.map(trafficTargetToRequest),
    subject: {
      type: `RATE_LIMIT_SUBJECT_TYPE_${payload.subject.type.toUpperCase()}`,
      headerName: payload.subject.type === 'Header' ? payload.subject.headerName : '',
    },
  });
  return { message: `请求限流策略已保存：${payload.name}`, changeId: payload.id };
}

export async function deleteRateLimitPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deletePolicy('/rate-limit-policies', id, version);
  return { message: '请求限流策略已删除' };
}

export function rateLimitSummary(policy: RateLimitPolicy) {
  const subject = policy.subject.type === 'Shared'
    ? '目标共享'
    : policy.subject.type === 'IP'
      ? '每个客户端 IP'
      : `每个 ${policy.subject.headerName} 值`;
  return `${subject} · ${formatDuration(policy.limit.windowSeconds)}内最多 ${policy.limit.requests} 次请求`;
}

function rateLimitSubjectFromResponse(value: string): RateLimitSubjectType {
  if (value === 'RATE_LIMIT_SUBJECT_TYPE_SHARED') return 'Shared';
  if (value === 'RATE_LIMIT_SUBJECT_TYPE_IP') return 'IP';
  if (value === 'RATE_LIMIT_SUBJECT_TYPE_HEADER') return 'Header';
  throw new Error(`服务返回了未知的限流计数方式：${value}`);
}

function formatDuration(seconds: number) {
  if (seconds % 86_400 === 0) return `${seconds / 86_400} 天`;
  if (seconds % 3_600 === 0) return `${seconds / 3_600} 小时`;
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`;
  return `${seconds} 秒`;
}
