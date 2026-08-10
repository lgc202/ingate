import type { ResourceState, ResourceStatus } from './common';

export type GovernancePolicyKind = 'RateLimitPolicy' | 'IPRestrictionPolicy' | 'TokenQuotaPolicy';
export type PolicyTargetKind = 'Gateway' | 'Route';
export type RateLimitSubjectType = 'Shared' | 'IP' | 'Header';
export type TokenQuotaSubjectType = 'Shared' | 'IP' | 'Header';
export type TokenQuotaFailurePolicy = 'FailOpen' | 'FailClose';

export interface PolicyTargetRef {
  kind: PolicyTargetKind;
  id: string;
  displayName?: string;
  status?: ResourceStatus;
}

export interface PolicyTargetPayload {
  kind: PolicyTargetKind;
  id: string;
}

export interface RateLimitPolicy {
  id: string;
  version: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  subject: RateLimitSubject;
  limit: RateLimit;
  status: ResourceStatus;
  createdAt?: string;
  updatedAt?: string;
}

export interface RateLimitSubject {
  type: RateLimitSubjectType;
  headerName?: string;
}

export interface RateLimit {
  requests: number;
  windowSeconds: number;
}

export interface IPRestrictionPolicy {
  id: string;
  version: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  allow: string[];
  deny: string[];
  status: ResourceStatus;
  createdAt?: string;
  updatedAt?: string;
}

export interface TokenQuotaPolicy {
  id: string;
  version: string;
  name: string;
  description?: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  subject: TokenQuotaSubject;
  quota: TokenQuota;
  failurePolicy: TokenQuotaFailurePolicy;
  response: TokenQuotaResponse;
  status: ResourceStatus;
  createdAt?: string;
}

export interface TokenQuotaSubject {
  type: TokenQuotaSubjectType;
  headerName?: string;
}

export interface TokenQuota {
  tokens: number;
  windowSeconds: number;
}

export interface TokenQuotaResponse {
  message: string;
}

export interface PolicyTargetOption {
  id: string;
  name: string;
  kind: PolicyTargetKind;
  supportsTokenQuota: boolean;
}

interface GovernancePolicyBase {
  id: string;
  version: string | number;
  name: string;
  description?: string;
  enabled: boolean;
  summary: string;
  ruleCount: number;
  targets: PolicyTargetRef[];
  status: ResourceStatus;
  createdAt?: string;
}

export type GovernancePolicy =
  | GovernancePolicyBase & { kind: 'RateLimitPolicy'; raw: RateLimitPolicy }
  | GovernancePolicyBase & { kind: 'IPRestrictionPolicy'; raw: IPRestrictionPolicy }
  | GovernancePolicyBase & { kind: 'TokenQuotaPolicy'; raw: TokenQuotaPolicy };

export interface PolicyWorkspace {
  policies: GovernancePolicy[];
  rateLimitPolicies: RateLimitPolicy[];
  ipRestrictionPolicies: IPRestrictionPolicy[];
  tokenQuotaPolicies: TokenQuotaPolicy[];
  targets: PolicyTargetOption[];
}

export interface PolicyMutationResult {
  message: string;
  changeId?: string;
}

interface RateLimitPolicyConfigPayload {
  name: string;
  enabled: boolean;
  targets: PolicyTargetPayload[];
  subject: RateLimitSubject;
  limit: RateLimit;
}

interface IPRestrictionPolicyConfigPayload {
  name: string;
  targets: PolicyTargetPayload[];
  allow: string[];
  deny: string[];
}

interface TokenQuotaPolicyConfigPayload {
  name: string;
  description?: string;
  enabled: boolean;
  targets: PolicyTargetPayload[];
  subject: TokenQuotaSubject;
  quota: TokenQuota;
  failurePolicy: TokenQuotaFailurePolicy;
  response: TokenQuotaResponse;
}

type CreatePolicyIdentity = { id?: never; version?: never };
type VersionedPolicyIdentity<T extends string | number> = { id: string; version: T };

export type RateLimitPolicyPayload = RateLimitPolicyConfigPayload & (CreatePolicyIdentity | VersionedPolicyIdentity<number>);
export type IPRestrictionPolicyPayload =
  | IPRestrictionPolicyConfigPayload & CreatePolicyIdentity
  | IPRestrictionPolicyConfigPayload & { enabled: boolean } & VersionedPolicyIdentity<number>;
export type TokenQuotaPolicyPayload = TokenQuotaPolicyConfigPayload & (CreatePolicyIdentity | VersionedPolicyIdentity<string>);

export function policyKindLabel(kind: GovernancePolicyKind) {
  const labels: Record<GovernancePolicyKind, string> = {
    RateLimitPolicy: '限流',
    IPRestrictionPolicy: 'IP 访问限制',
    TokenQuotaPolicy: 'Token 配额',
  };
  return labels[kind];
}

export function policyTargetKindLabel(kind: PolicyTargetKind) {
  return kind === 'Gateway' ? '网关' : '路由';
}

export function policyStatusLabel(status: ResourceStatus) {
  const labels: Record<ResourceState, string> = {
    Ready: '已生效',
    Pending: '待生效',
    Error: '异常',
    Disabled: '已停用',
  };
  return labels[status.state];
}

export function governancePolicyStatusLabel(policy: Pick<GovernancePolicy, 'enabled' | 'targets' | 'status'>) {
  if (policy.enabled && policy.targets.length === 0 && policy.status.state === 'Ready') {
    return '已保存';
  }
  return policyStatusLabel(policy.status);
}

export function policyStatusTone(status: ResourceStatus): 'success' | 'warning' | 'danger' | 'neutral' {
  const tones: Record<ResourceState, 'success' | 'warning' | 'danger' | 'neutral'> = {
    Ready: 'success',
    Pending: 'warning',
    Error: 'danger',
    Disabled: 'neutral',
  };
  return tones[status.state];
}

export function governancePolicyKey(policy: Pick<GovernancePolicy, 'kind' | 'id'>) {
  return `${policy.kind}:${policy.id}`;
}

export function policyTargetKey(target: Pick<PolicyTargetRef, 'kind' | 'id'>) {
  return `${target.kind}:${target.id}`;
}

export function policyTargetsResource(policy: GovernancePolicy, kind: PolicyTargetKind, id: string) {
  return policy.targets.some((target) => target.kind === kind && target.id === id);
}

export function policyTargetLabel(target: PolicyTargetRef, options: PolicyTargetOption[]) {
  const option = options.find((item) => item.kind === target.kind && item.id === target.id);
  return option?.name ?? target.displayName ?? target.id;
}
