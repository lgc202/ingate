import type {
  Upstream,
  UpstreamEndpoint,
  UpstreamLoadBalancing,
  UpstreamMutationPayload,
} from '@/domain/upstream';

export interface UpstreamDraft {
  id?: string;
  version?: number;
  name: string;
  endpoints: UpstreamEndpoint[];
  httpsEnabled: boolean;
  serverName: string;
  loadBalancing: UpstreamLoadBalancing;
  healthCheckEnabled: boolean;
  healthCheckPath: string;
  healthCheckInterval: number;
  healthCheckTimeout: number;
}

export function createUpstreamDraft(upstream?: Upstream): UpstreamDraft {
  return {
    id: upstream?.id,
    version: upstream?.version,
    name: upstream?.name ?? '',
    endpoints: upstream?.endpoints.map((endpoint) => ({ ...endpoint })) ?? [{ address: '127.0.0.1', port: 8080, weight: 1 }],
    httpsEnabled: Boolean(upstream?.tls),
    serverName: upstream?.tls?.serverName ?? '',
    loadBalancing: upstream?.loadBalancing ?? 'LOAD_BALANCING_POLICY_ROUND_ROBIN',
    healthCheckEnabled: Boolean(upstream?.healthCheck),
    healthCheckPath: upstream?.healthCheck?.path ?? '/healthz',
    healthCheckInterval: upstream?.healthCheck?.intervalSeconds ?? 10,
    healthCheckTimeout: upstream?.healthCheck?.timeoutSeconds ?? 2,
  };
}

export function validateUpstreamDraft(draft: UpstreamDraft): string[] {
  const errors: string[] = [];
  if (!draft.name.trim()) errors.push('请输入服务名称');
  if (draft.endpoints.length === 0) errors.push('至少添加一个服务地址');
  if (draft.endpoints.some((item) => !item.address.trim() || item.port < 1 || item.port > 65535 || item.weight < 1 || item.weight > 1000)) {
    errors.push('服务地址、端口或权重不正确');
  }
  if (draft.httpsEnabled && !draft.serverName.trim()) errors.push('启用 HTTPS 后必须填写服务名称');
  if (draft.healthCheckEnabled && (!draft.healthCheckPath.startsWith('/') || draft.healthCheckTimeout >= draft.healthCheckInterval)) {
    errors.push('健康检查路径必须以 / 开头，且超时应小于检查间隔');
  }
  return errors;
}

export function buildUpstreamPayload(draft: UpstreamDraft): UpstreamMutationPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    endpoints: draft.endpoints.map((item) => ({
      address: item.address.trim().toLowerCase(),
      port: Number(item.port),
      weight: Number(item.weight),
    })),
    tls: draft.httpsEnabled ? { serverName: draft.serverName.trim().toLowerCase() } : undefined,
    loadBalancing: draft.loadBalancing,
    healthCheck: draft.healthCheckEnabled ? {
      path: draft.healthCheckPath.trim(),
      intervalSeconds: Number(draft.healthCheckInterval),
      timeoutSeconds: Number(draft.healthCheckTimeout),
    } : undefined,
  };
}
