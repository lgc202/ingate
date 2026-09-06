import { apiListAllByCursor, apiRequest, type CursorPagedResponse } from '../client';
import type {
  CallerTokenQuotaUsage,
  PolicyMutationResult,
  TokenQuotaLimit,
  TokenQuotaPeriod,
  TokenQuotaPolicy,
  TokenQuotaPolicyPayload,
} from '@/domain/policy';
import {
  deletePolicy,
  resourceStatus,
  savePolicy,
  targetFromResponse,
  type PolicyTargetResponse,
} from './common';

interface LimitResponse {
  period: string;
  tokens: string | number;
}

interface PolicyResponse extends Omit<TokenQuotaPolicy, 'version' | 'targets' | 'limits' | 'status'> {
  version: string | number;
  targets: PolicyTargetResponse[];
  limits: LimitResponse[];
  state: string;
  message: string;
}

interface PolicyListResponse extends CursorPagedResponse {
  policies?: PolicyResponse[];
}

interface UsageResponse {
  policyId: string;
  policyName: string;
  period: string;
  usedTokens: string | number;
  limitTokens: string | number;
  remainingTokens: string | number;
  startedAt: string;
  resetsAt: string;
}

interface UsageListResponse {
  usages?: UsageResponse[];
}

export async function listTokenQuotaPolicies(): Promise<TokenQuotaPolicy[]> {
  const policies = await apiListAllByCursor<PolicyListResponse, PolicyResponse>(
    '/token-quota-policies',
    (page) => page.policies ?? [],
  );
  return policies.map((policy) => ({
    ...policy,
    version: Number(policy.version),
    targets: policy.targets.map(targetFromResponse),
    limits: policy.limits.map((limit) => ({
      period: tokenQuotaPeriodFromResponse(limit.period),
      tokens: Number(limit.tokens),
    })),
    status: resourceStatus(policy.state, policy.message),
  }));
}

export async function getCallerTokenQuotaUsage(callerID: string): Promise<CallerTokenQuotaUsage[]> {
  const response = await apiRequest<UsageListResponse>(
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
    limits: payload.limits.map((limit) => ({
      period: `TOKEN_QUOTA_PERIOD_${limit.period.toUpperCase()}`,
      tokens: limit.tokens,
    })),
  });
  return { message: `Token 额度策略已保存：${payload.name}`, changeId: payload.id };
}

export async function deleteTokenQuotaPolicy(id: string, version: number): Promise<PolicyMutationResult> {
  await deletePolicy('/token-quota-policies', id, version);
  return { message: 'Token 额度策略已删除' };
}

export function tokenQuotaSummary(limits: TokenQuotaLimit[]) {
  const labels: Record<TokenQuotaPeriod, string> = { Day: '每日', Week: '每周', Month: '每月' };
  return limits.map((limit) => `${labels[limit.period]} ${formatTokenCount(limit.tokens)}`).join(' · ');
}

function tokenQuotaPeriodFromResponse(value: string): TokenQuotaPeriod {
  if (value === 'TOKEN_QUOTA_PERIOD_DAY') return 'Day';
  if (value === 'TOKEN_QUOTA_PERIOD_WEEK') return 'Week';
  if (value === 'TOKEN_QUOTA_PERIOD_MONTH') return 'Month';
  throw new Error(`服务返回了未知的额度周期：${value}`);
}

function formatTokenCount(tokens: number) {
  return new Intl.NumberFormat('zh-CN', {
    notation: tokens >= 10_000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(tokens);
}
