import type { Gateway, GatewayListener, GatewayMutationPayload } from '@/domain/gateway';

export interface GatewayDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  listeners: GatewayListener[];
}

export function createGatewayDraft(gateway?: Gateway): GatewayDraft {
  return {
    id: gateway?.id,
    version: gateway?.version,
    name: gateway?.name ?? '',
    enabled: gateway?.enabled ?? true,
    listeners: gateway?.listeners.map((listener) => ({ ...listener })) ?? [newListener(0)],
  };
}

export function newListener(index: number): GatewayListener {
  return {
    name: `http-${index + 1}`,
    protocol: 'GATEWAY_PROTOCOL_HTTP',
    port: 0,
    hostname: '',
    certificateID: '',
  };
}

export function validateGatewayDraft(draft: GatewayDraft): string | undefined {
  if (!draft.name.trim()) return '请输入网关名称';
  if (draft.listeners.length === 0) return '至少配置一个监听入口';

  const names = new Set<string>();
  for (const listener of draft.listeners) {
    if (!/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(listener.name)) return '监听名称只能包含小写字母、数字和连字符';
    if (names.has(listener.name)) return `监听名称 ${listener.name} 不能重复`;
    names.add(listener.name);
    if (listener.port < 1 || listener.port > 65535) return '监听端口必须在 1 到 65535 之间';
    if (listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' && !listener.certificateID) return 'HTTPS 监听入口必须选择证书';
  }
  return undefined;
}

export function buildGatewayPayload(draft: GatewayDraft): GatewayMutationPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    enabled: draft.enabled,
    listeners: draft.listeners.map((listener) => ({
      ...listener,
      name: listener.name.trim(),
      hostname: listener.hostname.trim().toLowerCase(),
      certificateID: listener.protocol === 'GATEWAY_PROTOCOL_HTTPS' ? listener.certificateID : '',
    })),
  };
}
