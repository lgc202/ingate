import type { ResourceState, ResourceStatus } from './common';

export type GovernancePolicyKind = 'RateLimitPolicy' | 'AccessControlPolicy';
export type PolicyTargetKind = 'Gateway' | 'Route';
export type RateLimitKeyType = 'IP' | 'Header' | 'Query' | 'Cookie' | 'Route' | 'Gateway' | 'RouteRule';
export type RateLimitFailurePolicy = '' | 'FailOpen' | 'FailClose';
export type AccessControlAction = '' | 'Allow' | 'Deny';
export type AccessControlConditionType = 'IP' | 'Header';

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
  version: string;
  name: string;
  description?: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
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
  version: string;
  name: string;
  description?: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
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

export interface PolicyTargetOption {
  id: string;
  name: string;
  kind: PolicyTargetKind;
}

interface GovernancePolicyBase {
  id: string;
  version: string;
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
  | GovernancePolicyBase & { kind: 'AccessControlPolicy'; raw: AccessControlPolicy };

export interface PolicyWorkspace {
  policies: GovernancePolicy[];
  rateLimitPolicies: RateLimitPolicy[];
  accessControlPolicies: AccessControlPolicy[];
  targets: PolicyTargetOption[];
}

export interface PolicyMutationResult {
  message: string;
  changeId?: string;
}

interface RateLimitPolicyConfigPayload {
  name: string;
  description?: string;
  enabled: boolean;
  targets: PolicyTargetPayload[];
  rules: RateLimitRule[];
  response?: RateLimitResponse;
  failurePolicy?: RateLimitFailurePolicy;
}

interface AccessControlPolicyConfigPayload {
  name: string;
  description?: string;
  enabled: boolean;
  targets: PolicyTargetPayload[];
  defaultAction?: AccessControlAction;
  rules: AccessControlRule[];
  response?: AccessControlDenyResponse;
}

type CreatePolicyIdentity = { id?: never; version?: never };
type UpdatePolicyIdentity = { id: string; version: string };

export type RateLimitPolicyPayload = RateLimitPolicyConfigPayload & (CreatePolicyIdentity | UpdatePolicyIdentity);
export type AccessControlPolicyPayload = AccessControlPolicyConfigPayload & (CreatePolicyIdentity | UpdatePolicyIdentity);

export function policyKindLabel(kind: GovernancePolicyKind) {
  return kind === 'RateLimitPolicy' ? '限流' : '访问控制';
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
