import type { ResourceState, ResourceStatus } from './common';

export type GovernancePolicyKind =
  | 'IPRestrictionPolicy'
  | 'TokenQuotaPolicy'
  | 'HeaderTransformationPolicy'
  | 'MockResponsePolicy';
export type PolicyTargetKind = 'Gateway' | 'Route' | 'Caller';

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

export type TokenQuotaPeriod = 'Day' | 'Week' | 'Month';

export interface TokenQuotaLimit {
  period: TokenQuotaPeriod;
  tokens: number;
}

export interface CallerTokenQuotaUsage {
  policyID: string;
  policyName: string;
  period: TokenQuotaPeriod;
  usedTokens: number;
  limitTokens: number;
  remainingTokens: number;
  startedAt: string;
  resetsAt: string;
}

export interface TokenQuotaPolicy {
  id: string;
  version: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  timeZone: string;
  limits: TokenQuotaLimit[];
  status: ResourceStatus;
  createdAt?: string;
  updatedAt?: string;
}

export type HeaderTransformationOperation = 'remove' | 'rename' | 'replace' | 'add' | 'append';

export interface HeaderTransformationRule {
  operation: HeaderTransformationOperation;
  name: string;
  value: string;
}

export interface HeaderTransformationPolicy {
  id: string;
  version: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  requestRules: HeaderTransformationRule[];
  responseRules: HeaderTransformationRule[];
  status: ResourceStatus;
  createdAt?: string;
  updatedAt?: string;
}

export interface MockResponseHeader {
  name: string;
  value: string;
}

export interface MockResponsePolicy {
  id: string;
  version: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  statusCode: number;
  contentType: string;
  headers: MockResponseHeader[];
  body: string;
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

export type GovernancePolicy = GovernancePolicyBase & (
  | { kind: 'IPRestrictionPolicy'; raw: IPRestrictionPolicy }
  | { kind: 'TokenQuotaPolicy'; raw: TokenQuotaPolicy }
  | { kind: 'HeaderTransformationPolicy'; raw: HeaderTransformationPolicy }
  | { kind: 'MockResponsePolicy'; raw: MockResponsePolicy }
);

export interface PolicyWorkspace {
  policies: GovernancePolicy[];
  ipRestrictionPolicies: IPRestrictionPolicy[];
  tokenQuotaPolicies: TokenQuotaPolicy[];
  headerTransformationPolicies: HeaderTransformationPolicy[];
  mockResponsePolicies: MockResponsePolicy[];
  installedPluginPackages: string[];
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

interface TokenQuotaPolicyConfigPayload {
  name: string;
  enabled: boolean;
  targets: PolicyTargetPayload[];
  timeZone: string;
  limits: TokenQuotaLimit[];
}

export type TokenQuotaPolicyPayload =
  | TokenQuotaPolicyConfigPayload & CreatePolicyIdentity
  | TokenQuotaPolicyConfigPayload & VersionedPolicyIdentity<number>;

interface HeaderTransformationPolicyConfigPayload {
  name: string;
  targets: PolicyTargetPayload[];
  requestRules: HeaderTransformationRule[];
  responseRules: HeaderTransformationRule[];
}

export type HeaderTransformationPolicyPayload =
  | HeaderTransformationPolicyConfigPayload & CreatePolicyIdentity
  | HeaderTransformationPolicyConfigPayload & { enabled: boolean } & VersionedPolicyIdentity<number>;

interface MockResponsePolicyConfigPayload {
  name: string;
  targets: PolicyTargetPayload[];
  statusCode: number;
  contentType: string;
  headers: MockResponseHeader[];
  body: string;
}

export type MockResponsePolicyPayload =
  | MockResponsePolicyConfigPayload & CreatePolicyIdentity
  | MockResponsePolicyConfigPayload & { enabled: boolean } & VersionedPolicyIdentity<number>;

export function policyKindLabel(kind: GovernancePolicyKind) {
  const labels: Record<GovernancePolicyKind, string> = {
    IPRestrictionPolicy: 'IP 访问限制',
    TokenQuotaPolicy: 'Token 额度',
    HeaderTransformationPolicy: '请求响应转换',
    MockResponsePolicy: '模拟响应',
  };
  return labels[kind];
}

export function policyTargetKindLabel(kind: PolicyTargetKind) {
  if (kind === 'Gateway') return '网关';
  if (kind === 'Route') return '路由';
  return '调用方';
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

export function policySupportsTargetKind(policy: GovernancePolicy, kind: PolicyTargetKind) {
  if (policy.kind === 'TokenQuotaPolicy') return kind === 'Caller';
  if (policy.kind === 'HeaderTransformationPolicy' || policy.kind === 'MockResponsePolicy') return kind === 'Route';
  return kind === 'Gateway' || kind === 'Route';
}

export function policyTargetLabel(target: PolicyTargetRef, options: PolicyTargetOption[]) {
  const option = options.find((item) => item.kind === target.kind && item.id === target.id);
  return option?.name ?? target.displayName ?? '目标不可用';
}
