import type { HealthStatus } from './common';

export type PolicyWorkspaceTab = 'access-control' | 'rate-limit' | 'bindings';
export type PolicyKind = 'RateLimitPolicy' | 'AccessControlPolicy';
export type PolicyTargetKind = 'Gateway' | 'Route';

export type RateLimitMode = 'Local' | 'Global';
export type RateLimitAlgorithm = 'FixedWindow' | 'SlidingWindow' | 'TokenBucket';
export type RateLimitKeyType = 'IP' | 'Header' | 'Query' | 'Cookie' | 'Consumer' | 'Route' | 'Gateway' | 'RouteRule' | 'JWTClaim' | 'APIKey' | 'Tenant';
export type RateLimitFailurePolicy = 'FailOpen' | 'FailClose';

export type AccessControlAction = 'Allow' | 'Deny';
export type AccessControlConditionType = 'IP' | 'Header' | 'Consumer' | 'Tenant';

export interface PolicyOption {
  id: string;
  name: string;
  kind?: PolicyKind;
  rules?: string[];
}

export interface RedisStoreOption {
  id: string;
  name: string;
}

export interface PolicyWorkspace {
  rateLimitPolicies: RateLimitPolicy[];
  accessControlPolicies: AccessControlPolicy[];
  bindings: PolicyBinding[];
  gateways: PolicyOption[];
  routes: PolicyOption[];
  redisStores: RedisStoreOption[];
}

export interface RateLimitPolicy {
  id: string;
  version?: string;
  name: string;
  description?: string;
  enabled: boolean;
  mode: RateLimitMode;
  rules: RateLimitRule[];
  global?: GlobalRateLimitConfig;
  response?: RateLimitResponse;
  failurePolicy?: RateLimitFailurePolicy;
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

export interface GlobalRateLimitConfig {
  redisRef: string;
  prefix?: string;
  timeoutMillis?: number;
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

export interface PolicyBinding {
  id: string;
  version?: string;
  name: string;
  description?: string;
  enabled: boolean;
  targetRef: PolicyTargetRef;
  policies: PolicyRef[];
  createdAt?: string;
}

export interface PolicyTargetRef {
  kind: PolicyTargetKind;
  name: string;
  ruleName?: string;
}

export interface PolicyRef {
  kind: PolicyKind;
  name: string;
}

export interface PolicyMutationResult {
  message: string;
  changeId?: string;
}

export interface PolicyValidationItem {
  label: string;
  status: HealthStatus;
  message: string;
}

export interface PolicyValidationReport {
  valid: boolean;
  summary: string;
  items: PolicyValidationItem[];
}

export function policyKindLabel(kind: PolicyKind) {
  const labels: Record<PolicyKind, string> = {
    RateLimitPolicy: '限流策略',
    AccessControlPolicy: '访问控制策略',
  };
  return labels[kind];
}

export function policyTargetKindLabel(kind: PolicyTargetKind) {
  const labels: Record<PolicyTargetKind, string> = {
    Gateway: '网关',
    Route: '路由',
  };
  return labels[kind];
}

export function rateLimitModeLabel(mode: RateLimitMode | string) {
  const labels: Record<RateLimitMode, string> = {
    Local: '本地限流',
    Global: '全局限流',
  };
  return labels[mode as RateLimitMode] ?? mode;
}

export function rateLimitAlgorithmLabel(algorithm: RateLimitAlgorithm | string | undefined) {
  const labels: Record<RateLimitAlgorithm, string> = {
    FixedWindow: '固定窗口',
    SlidingWindow: '滑动窗口',
    TokenBucket: '令牌桶',
  };
  return labels[algorithm as RateLimitAlgorithm] ?? algorithm ?? '固定窗口';
}

export function rateLimitFailurePolicyLabel(policy: RateLimitFailurePolicy | string | undefined) {
  const labels: Record<RateLimitFailurePolicy, string> = {
    FailOpen: '故障放行',
    FailClose: '故障拒绝',
  };
  return labels[policy as RateLimitFailurePolicy] ?? policy ?? '故障放行';
}

export function accessControlActionLabel(action: AccessControlAction | string | undefined) {
  const labels: Record<AccessControlAction, string> = {
    Allow: '允许',
    Deny: '拒绝',
  };
  return labels[action as AccessControlAction] ?? action ?? '允许';
}

export function conditionTypeLabel(type: AccessControlConditionType | string) {
  const labels: Record<AccessControlConditionType, string> = {
    IP: '客户端 IP',
    Header: '请求 Header',
    Consumer: 'Consumer',
    Tenant: '租户',
  };
  return labels[type as AccessControlConditionType] ?? type;
}

export function rateLimitKeyTypeLabel(type: RateLimitKeyType | string) {
  const labels: Record<RateLimitKeyType, string> = {
    IP: '客户端 IP',
    Header: '请求 Header',
    Query: 'Query 参数',
    Cookie: 'Cookie',
    Consumer: 'Consumer',
    Route: '路由',
    Gateway: '网关',
    RouteRule: '路由规则',
    JWTClaim: 'JWT Claim',
    APIKey: 'API Key',
    Tenant: '租户',
  };
  return labels[type as RateLimitKeyType] ?? type;
}
