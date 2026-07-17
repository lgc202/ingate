import type {
  Gateway,
  GatewayHostBinding,
  GatewayListener,
  GatewayMutationPayload,
  GatewayValidationItem,
  GatewayValidationReport,
} from '@/domain/gateway';

export const GATEWAY_ENTRY_PORT = 8080;

export type GatewayHostMode = 'any' | 'specified';

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
  const hostnames = gateway ? hostnamesFromBindings(gateway.hostBindings) : [];

  return {
    id: gateway?.id,
    version: gateway?.version,
    name: gateway?.name ?? '',
    description: gateway?.description ?? '',
    listeners: gateway?.listeners?.length ? gateway.listeners.map((listener) => ({ ...listener })) : [createGatewayListener()],
    hostMode: hostnames.length > 0 ? 'specified' : 'any',
    hostnames,
  };
}

export function createGatewayListener(): GatewayListener {
  return {
    name: `listener-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    protocol: 'HTTP',
    port: GATEWAY_ENTRY_PORT,
  };
}

export function validateGatewayDraft(draft: GatewayFormDraft): GatewayValidationReport {
  const name = draft.name.trim();
  const normalizedHostnames = draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [];
  const invalidHostnames = normalizedHostnames.filter((hostname) => !isValidHostname(hostname));
  const ports = draft.listeners.map(() => GATEWAY_ENTRY_PORT);
  const duplicatePorts = ports.filter((port, index) => ports.indexOf(port) !== index);
  const items: GatewayValidationItem[] = [
    {
      label: '网关名称',
      status: name ? 'healthy' : 'critical',
      message: name || '请输入网关名称',
    },
    {
      label: '运行入口',
      status: draft.listeners.length > 0 && duplicatePorts.length === 0 ? 'healthy' : 'critical',
      message: duplicatePorts.length > 0
        ? `入口重复：${Array.from(new Set(duplicatePorts)).join('、')}`
        : draft.listeners.length > 0
          ? formatListeners(draft.listeners)
          : '至少启用一个运行入口',
    },
    {
      label: 'Host 策略',
      status: (draft.hostMode === 'specified' && normalizedHostnames.length === 0) || invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: draft.hostMode === 'specified' && normalizedHostnames.length === 0
        ? '指定 Host 时至少添加一个域名'
        : invalidHostnames.length > 0
          ? `域名格式不正确：${invalidHostnames.join('、')}`
          : normalizedHostnames.length > 0
            ? `限制 ${normalizedHostnames.length} 个 Host`
            : '不限制 Host',
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
  const listeners = draft.listeners.map((listener) => ({
    name: listener.name,
    protocol: listener.protocol,
    port: GATEWAY_ENTRY_PORT,
  }));

  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    description: draft.description.trim(),
    listeners,
    hostBindings: buildHostBindings(draft, listeners),
  };
}

export function formatListeners(listeners: GatewayListener[]) {
  return listeners
    .map((listener) => `${listener.protocol}:${GATEWAY_ENTRY_PORT}`)
    .join(' / ');
}

export function parseHostnames(input: string): string[] {
  return normalizeHostnames(input.split(/[\s,，、;；]+/));
}

export function normalizeHostnames(hostnames: string[]): string[] {
  return Array.from(new Set(hostnames.map((hostname) => hostname.trim().toLowerCase()).filter(Boolean)));
}

export function formatHostnames(hostnames: string[]) {
  return hostnames.length > 0 ? hostnames.join('、') : '不限制 Host';
}

export function hostnamesFromBindings(bindings: GatewayHostBinding[]) {
  return normalizeHostnames(bindings.map((binding) => binding.hostname ?? '').filter(Boolean));
}

function buildHostBindings(draft: GatewayFormDraft, listeners: GatewayListener[]): GatewayHostBinding[] {
  const listenerRefs = listeners.map((listener) => listener.name);
  const hostnames = draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [''];

  return hostnames.map((hostname) => ({
    hostname: hostname || undefined,
    listenerRefs,
  }));
}

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;

  if (!normalized.includes('.') || normalized.length > 253) {
    return false;
  }

  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}
