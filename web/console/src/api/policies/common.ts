import { apiRequest } from '../client';
import { normalizeResourceState, type ResourceStatus } from '@/domain/common';
import type { PolicyTargetKind, PolicyTargetRef } from '@/domain/policy';

export interface PolicyTargetResponse {
  kind: string;
  id: string;
  name: string;
  state: string;
  message: string;
}

export function targetFromResponse(target: PolicyTargetResponse): PolicyTargetRef {
  return {
    kind: policyTargetKindFromResponse(target.kind),
    id: target.id,
    displayName: target.name,
    status: resourceStatus(target.state, target.message),
  };
}

export function resourceStatus(state: string, message: string): ResourceStatus {
  return { state: normalizeResourceState(state), message };
}

export function trafficTargetToRequest(target: { kind: PolicyTargetKind; id: string }) {
  if (target.kind === 'Gateway') return { id: target.id, kind: 'POLICY_TARGET_KIND_GATEWAY' };
  if (target.kind === 'Route') return { id: target.id, kind: 'POLICY_TARGET_KIND_ROUTE' };
  throw new Error('流量策略仅支持网关或路由目标');
}

export async function savePolicy(basePath: string, payload: Record<string, unknown> & { id?: string }) {
  const path = payload.id ? `${basePath}/${encodeURIComponent(payload.id)}` : basePath;
  await apiRequest(path, { method: payload.id ? 'PUT' : 'POST', body: JSON.stringify(payload) });
}

export async function deletePolicy(basePath: string, id: string, version: number) {
  await apiRequest(`${basePath}/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}

function policyTargetKindFromResponse(value: string): PolicyTargetKind {
  if (value === 'POLICY_TARGET_KIND_GATEWAY') return 'Gateway';
  if (value === 'POLICY_TARGET_KIND_ROUTE') return 'Route';
  if (value === 'POLICY_TARGET_KIND_CALLER') return 'Caller';
  throw new Error(`服务返回了未知的策略目标类型：${value}`);
}
