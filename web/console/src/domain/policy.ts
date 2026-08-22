import type { ResourceState, ResourceStatus } from './common';

export type GovernancePolicyKind = 'IPRestrictionPolicy';
export type PolicyTargetKind = 'Gateway' | 'Route';

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

export interface PolicyTargetOption {
  id: string;
  name: string;
  kind: PolicyTargetKind;
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
  updatedAt?: string;
}

export type GovernancePolicy = GovernancePolicyBase & {
  kind: 'IPRestrictionPolicy';
  raw: IPRestrictionPolicy;
};

export interface PolicyWorkspace {
  policies: GovernancePolicy[];
  ipRestrictionPolicies: IPRestrictionPolicy[];
  targets: PolicyTargetOption[];
}

export interface PolicyMutationResult {
  message: string;
  changeId?: string;
}

interface IPRestrictionPolicyConfigPayload {
  name: string;
  targets: PolicyTargetPayload[];
  allow: string[];
  deny: string[];
}

type CreatePolicyIdentity = { id?: never; version?: never };
type VersionedPolicyIdentity<T extends string | number> = { id: string; version: T };

export type IPRestrictionPolicyPayload =
  | IPRestrictionPolicyConfigPayload & CreatePolicyIdentity
  | IPRestrictionPolicyConfigPayload & { enabled: boolean } & VersionedPolicyIdentity<number>;

export function policyKindLabel(kind: GovernancePolicyKind) {
  const labels: Record<GovernancePolicyKind, string> = {
    IPRestrictionPolicy: 'IP 访问限制',
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
    Error: '生效失败',
    Disabled: '已停用',
  };
  return labels[status.state];
}

export function governancePolicyStatusLabel(policy: Pick<GovernancePolicy, 'enabled' | 'targets' | 'status'>) {
  if (policy.enabled && policy.targets.length === 0 && policy.status.state === 'Ready') {
    return '未应用';
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
