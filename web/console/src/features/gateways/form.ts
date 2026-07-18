import type {
  Gateway,
  GatewayListener,
  GatewayMutationPayload,
  GatewayValidationItem,
  GatewayValidationReport,
} from '@/domain/gateway';

export type GatewayHostMode = 'any' | 'specified';

export const GATEWAY_HTTP_PORT = 8080;
export const GATEWAY_HTTPS_PORT = 8443;

export interface GatewayFormDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  listeners: GatewayListener[];
  hostMode: GatewayHostMode;
  hostnames: string[];
}

export function createGatewayDraft(gateway?: Gateway | null): GatewayFormDraft {
  const hostnames = normalizeHostnames(gateway?.hostnames ?? []);

  return {
    id: gateway?.id,
    version: gateway?.version,
    name: gateway?.name ?? '',
    description: gateway?.description ?? '',
    listeners: gateway?.listeners.length
      ? gateway.listeners.map((listener) => ({ ...listener }))
      : [{ protocol: 'HTTP', port: GATEWAY_HTTP_PORT }],
    hostMode: hostnames.length > 0 ? 'specified' : 'any',
    hostnames,
  };
}

export function validateGatewayDraft(draft: GatewayFormDraft): GatewayValidationReport {
  const name = draft.name.trim();
  const normalizedHostnames = draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [];
  const invalidHostnames = normalizedHostnames.filter((hostname) => !isValidHostname(hostname));
  const httpsListener = draft.listeners.find((listener) => listener.protocol === 'HTTPS');
  const items: GatewayValidationItem[] = [
    {
      label: '网关名称',
      status: name ? 'healthy' : 'critical',
      message: name || '请输入网关名称',
    },
    {
      label: '运行入口',
      status: draft.listeners.length > 0 ? 'healthy' : 'critical',
      message: draft.listeners.length > 0
        ? draft.listeners.map((listener) => `${listener.protocol}:${listener.port}`).join(' / ')
        : '至少启用一个运行入口',
    },
    {
      label: 'HTTPS 证书',
      status: !httpsListener || httpsListener.certificateID ? 'healthy' : 'critical',
      message: !httpsListener
        ? '未启用 HTTPS'
        : httpsListener.certificateID
          ? '已选择 HTTPS 证书'
          : '请选择 HTTPS 证书',
    },
    {
      label: '域名范围',
      status: (draft.hostMode === 'specified' && normalizedHostnames.length === 0) || invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: draft.hostMode === 'specified' && normalizedHostnames.length === 0
        ? '指定域名时至少添加一个域名'
        : invalidHostnames.length > 0
          ? `域名格式不正确：${invalidHostnames.join('、')}`
          : normalizedHostnames.length > 0
            ? `限制 ${normalizedHostnames.length} 个域名`
            : '不限制域名',
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '网关配置通过校验，可以保存。' : '网关配置还存在未完成项。',
    items,
  };
}

export function buildGatewayPayload(draft: GatewayFormDraft): GatewayMutationPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    description: draft.description.trim(),
    listeners: draft.listeners.map((listener) => ({ ...listener })),
    hostnames: draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [],
  };
}

export function parseHostnames(input: string): string[] {
  return normalizeHostnames(input.split(/[\s,，、;；]+/));
}

export function normalizeHostnames(hostnames: string[]): string[] {
  return Array.from(new Set(hostnames.map((hostname) => hostname.trim().toLowerCase()).filter(Boolean)));
}

export function formatHostnames(hostnames: string[]) {
  return hostnames.length > 0 ? hostnames.join('、') : '不限制域名';
}

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;

  if (!normalized || normalized.length > 253) {
    return false;
  }

  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}
