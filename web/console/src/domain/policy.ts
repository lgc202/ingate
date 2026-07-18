import type { ResourceStatus } from './common';

export type GovernancePolicyKind = 'RateLimitPolicy' | 'AccessControlPolicy';
export type PolicyTargetKind = 'Gateway' | 'Route';
export type RateLimitMode = 'Local' | 'Global';
export type RateLimitAlgorithm = 'FixedWindow' | 'SlidingWindow' | 'TokenBucket';
export type RateLimitKeyType = 'IP' | 'Header' | 'Query' | 'Cookie' | 'Consumer' | 'Route' | 'Gateway' | 'RouteRule' | 'JWTClaim' | 'APIKey' | 'Tenant';
export type RateLimitFailurePolicy = '' | 'FailOpen' | 'FailClose';
export type AccessControlAction = '' | 'Allow' | 'Deny';
export type AccessControlConditionType = 'IP' | 'Header' | 'Consumer' | 'Tenant';

export interface RateLimitPolicy {
  id: string;
  version?: string;
  name: string;
  description?: string;
  enabled: boolean;
  mode: RateLimitMode;
  rules: RateLimitRule[];
  response?: RateLimitResponse;
  failurePolicy?: RateLimitFailurePolicy;
  status: ResourceStatus;
  createdAt?: string;
}

export interface RateLimitRule {
  name: string;
  key: RateLimitKey;
  limit: RateLimitQuota;
  algorithm?: RateLimitAlgorithm;
}

export interface RateLimitKey {
  parts: RateLimitKeyPart[];
}

export interface RateLimitKeyPart {
  type: RateLimitKeyType;
  name?: string;
}

export interface RateLimitQuota {
  requests: number;
  windowSeconds: number;
  burst?: number;
}

export interface RateLimitResponse {
  statusCode?: number;
  message?: string;
  quotaHeaderEnabled?: boolean;
}

export interface AccessControlPolicy {
  id: string;
  version?: string;
  name: string;
  description?: string;
  enabled: boolean;
  defaultAction?: AccessControlAction;
  rules?: AccessControlRule[];
  response?: AccessControlDenyResponse;
  status: ResourceStatus;
  createdAt?: string;
}

export interface AccessControlRule {
  name: string;
  action: AccessControlAction;
  conditions?: AccessControlCondition[];
}

export interface AccessControlCondition {
  type: AccessControlConditionType;
  name?: string;
  value: string;
}

export interface AccessControlDenyResponse {
  statusCode?: number;
  message?: string;
}

export interface PolicyTargetRef {
  kind: PolicyTargetKind;
  name: string;
  ruleName?: string;
}

export interface PolicyRef {
  kind: GovernancePolicyKind;
  name: string;
}

export interface PolicyBinding {
  id: string;
  version?: string;
  name: string;
  description?: string;
  enabled: boolean;
  targetRef: PolicyTargetRef;
  policies: PolicyRef[];
  status: ResourceStatus;
  createdAt?: string;
}

export interface PolicyTargetOption {
  id: string;
  name: string;
  kind: PolicyTargetKind;
  ruleNames?: string[];
}

export interface GovernancePolicy {
  id: string;
  version?: string;
  kind: GovernancePolicyKind;
  name: string;
  description?: string;
  enabled: boolean;
  mode: string;
  ruleCount: number;
  createdAt?: string;
  raw: RateLimitPolicy | AccessControlPolicy;
}

export interface PolicyWorkspace {
  policies: GovernancePolicy[];
  rateLimitPolicies: RateLimitPolicy[];
  accessControlPolicies: AccessControlPolicy[];
  bindings: PolicyBinding[];
  targets: PolicyTargetOption[];
}

export interface PolicyMutationResult {
  message: string;
  changeId?: string;
}

export type RateLimitPolicyPayload = Omit<RateLimitPolicy, 'id' | 'status' | 'createdAt'> & { id?: string };
export type AccessControlPolicyPayload = Omit<AccessControlPolicy, 'id' | 'status' | 'createdAt'> & { id?: string };
export type PolicyBindingPayload = Omit<PolicyBinding, 'id' | 'status' | 'createdAt'> & { id?: string };

export function policyKindLabel(kind: GovernancePolicyKind) {
  if (kind === 'RateLimitPolicy') {
    return '限流';
  }
  return '访问控制';
}

export function policyTargetKindLabel(kind: PolicyTargetKind) {
  if (kind === 'Gateway') {
    return '网关';
  }
  return '路由';
}

export function policyStatusLabel(enabled: boolean) {
  return enabled ? '启用' : '停用';
}

export function policyStatusTone(enabled: boolean) {
  return enabled ? 'accent' : 'neutral';
}

export function policyRefKey(policy: PolicyRef) {
  return `${policy.kind}:${policy.name}`;
}

export function governancePolicyRef(policy: GovernancePolicy): PolicyRef {
  return {
    kind: policy.kind,
    name: policy.id,
  };
}

export function governancePolicyKey(policy: GovernancePolicy) {
  return policyRefKey(governancePolicyRef(policy));
}

export function policyBindingTargetLabel(binding: PolicyBinding, targets: PolicyTargetOption[]) {
  const target = targets.find((item) => item.kind === binding.targetRef.kind && item.id === binding.targetRef.name);
  const name = target?.name ?? binding.targetRef.name;
  const prefix = policyTargetKindLabel(binding.targetRef.kind);
  return binding.targetRef.ruleName ? `${prefix} / ${name} / ${binding.targetRef.ruleName}` : `${prefix} / ${name}`;
}

export function policyNamesForBinding(binding: PolicyBinding, policies: GovernancePolicy[]) {
  return binding.policies.map((ref) => {
    const policy = policies.find((item) => item.kind === ref.kind && item.id === ref.name);
    return policy?.name ?? ref.name;
  });
}
