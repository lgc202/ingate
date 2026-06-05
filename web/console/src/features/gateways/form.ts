import type {
  Gateway,
  GatewayCertificateOption,
  GatewayHostBinding,
  GatewayListener,
  GatewayMutationPayload,
  GatewayRuntimeGroupOption,
  GatewayValidationItem,
  GatewayValidationReport,
} from '@/domain/gateway';

export const GATEWAY_ENTRY_PORTS: Record<GatewayListener['protocol'], number> = {
  HTTP: 8080,
  HTTPS: 8443,
};

export type GatewayHostMode = 'any' | 'specified';

export interface GatewayFormDraft {
  id?: string;
  version?: string;
  displayName: string;
  description: string;
  runtimeGroup: string;
  listeners: GatewayListener[];
  hostMode: GatewayHostMode;
  hostnames: string[];
}

export function createGatewayDraft(gateway?: Gateway | null, defaultRuntimeGroup = ''): GatewayFormDraft {
  const hostnames = gateway ? hostnamesFromBindings(gateway.hostBindings) : [];

  return {
    id: gateway?.id,
    version: gateway?.version,
    displayName: gateway?.displayName ?? '',
    description: gateway?.description ?? '',
    runtimeGroup: gateway?.runtimeGroup ?? defaultRuntimeGroup,
    listeners: gateway?.listeners?.length ? listenersWithCertificates(gateway.listeners, gateway.hostBindings) : [createGatewayListener('HTTP')],
    hostMode: hostnames.length > 0 ? 'specified' : 'any',
    hostnames,
  };
}

export function createGatewayListener(protocol: GatewayListener['protocol'] = 'HTTP', port = gatewayEntryPort(protocol)): GatewayListener {
  return {
    name: `listener-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    protocol,
    port,
  };
}

export function validateGatewayDraft(
  draft: GatewayFormDraft,
  gateways: Gateway[] = [],
  originalGatewayId?: string,
  runtimeGroups: GatewayRuntimeGroupOption[] = [],
): GatewayValidationReport {
  const displayName = draft.displayName.trim();
  const runtimeGroup = draft.runtimeGroup.trim();
  const normalizedHostnames = draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [];
  const invalidHostnames = normalizedHostnames.filter((hostname) => !isValidHostname(hostname));
  const ports = draft.listeners.map((listener) => gatewayEntryPort(listener.protocol));
  const duplicatePorts = ports.filter((port, index) => ports.indexOf(port) !== index);
  const duplicateName = gateways.some((gateway) => gateway.id !== originalGatewayId && gateway.displayName === displayName);
  const runtimeGroupExists = runtimeGroups.some((item) => item.id === runtimeGroup);
  const httpsWithoutCertificate = draft.listeners.filter((listener) => listener.protocol === 'HTTPS' && !listener.certificateId);
  const hostlessConflict = draft.hostMode === 'any' ? hostlessGatewayConflict(draft, gateways, originalGatewayId) : null;
  const items: GatewayValidationItem[] = [
    {
      label: '网关名称',
      status: displayName && !duplicateName ? 'healthy' : 'critical',
      message: !displayName
        ? '请输入网关名称'
        : duplicateName
          ? '网关名称已存在'
          : displayName,
    },
    {
      label: '运行组',
      status: runtimeGroup && runtimeGroupExists ? 'healthy' : 'critical',
      message: !runtimeGroup
        ? '请选择运行组'
        : runtimeGroupExists
          ? runtimeGroups.find((item) => item.id === runtimeGroup)?.name ?? runtimeGroup
          : '运行组不存在',
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
      label: 'HTTPS 证书',
      status: httpsWithoutCertificate.length > 0 ? 'critical' : 'healthy',
      message: httpsWithoutCertificate.length > 0 ? 'HTTPS 监听器必须选择证书' : '证书配置满足要求',
    },
    {
      label: 'Host 策略',
      status: hostlessConflict || (draft.hostMode === 'specified' && normalizedHostnames.length === 0) || invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: hostlessConflict
        ? hostlessConflict
        : draft.hostMode === 'specified' && normalizedHostnames.length === 0
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
    port: gatewayEntryPort(listener.protocol),
    certificateId: listener.protocol === 'HTTPS' ? listener.certificateId : undefined,
  }));

  return {
    id: draft.id,
    version: draft.version,
    displayName: draft.displayName.trim(),
    description: draft.description.trim(),
    runtimeGroup: draft.runtimeGroup,
    listeners,
    hostBindings: buildHostBindings(draft, listeners),
  };
}

export function formatListeners(listeners: GatewayListener[]) {
  return listeners
    .map((listener) => `${listener.protocol}:${gatewayEntryPort(listener.protocol)}`)
    .join(' / ');
}

export function gatewayEntryPort(protocol: GatewayListener['protocol']) {
  return GATEWAY_ENTRY_PORTS[protocol];
}

export function certificateCoverage(certificate: GatewayCertificateOption | undefined, hostnames: string[]) {
  if (!certificate || hostnames.length === 0) {
    return '不检查';
  }

  const uncovered = hostnames.filter((hostname) => !certificate.domains.some((domain) => matchesCertificateDomain(domain, hostname)));

  return uncovered.length === 0 ? '已覆盖 Host' : `未覆盖：${uncovered.join('、')}`;
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
  const httpsListener = listeners.find((listener) => listener.protocol === 'HTTPS');
  const listenerRefs = listeners.map((listener) => listener.name);
  const hostnames = draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [''];

  return hostnames.map((hostname) => ({
    hostname: hostname || undefined,
    listenerRefs,
    tls: httpsListener?.certificateId ? { certificateRef: httpsListener.certificateId } : undefined,
  }));
}

function listenersWithCertificates(listeners: GatewayListener[], bindings: GatewayHostBinding[]) {
  return listeners.map((listener) => {
    if (listener.protocol !== 'HTTPS') {
      return listener;
    }
    const binding = bindings.find((item) => item.listenerRefs.includes(listener.name) && item.tls?.certificateRef);
    return {
      ...listener,
      certificateId: binding?.tls?.certificateRef,
    };
  });
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

function hostlessGatewayConflict(draft: GatewayFormDraft, gateways: Gateway[], originalGatewayId?: string): string | null {
  const entries = new Set(draft.listeners.map((listener) => `${listener.protocol}:${gatewayEntryPort(listener.protocol)}`));
  const conflict = gateways.find((gateway) => {
    if (gateway.id === originalGatewayId || !gateway.enabled || hostnamesFromBindings(gateway.hostBindings).length > 0) {
      return false;
    }

    return gateway.listeners.some((listener) => entries.has(`${listener.protocol}:${listener.port || gatewayEntryPort(listener.protocol)}`));
  });

  if (!conflict) {
    return null;
  }

  const entry = conflict.listeners.find((listener) => entries.has(`${listener.protocol}:${listener.port || gatewayEntryPort(listener.protocol)}`));

  return `${entry ? `${entry.protocol}:${entry.port || gatewayEntryPort(entry.protocol)}` : '当前运行入口'} 已有不限制 Host 的网关 ${conflict.displayName}。请指定 Host，或先停用/删除该网关`;
}

function matchesCertificateDomain(domain: string, hostname: string) {
  if (domain.startsWith('*.')) {
    return hostname.endsWith(domain.slice(1));
  }

  return domain === hostname;
}
