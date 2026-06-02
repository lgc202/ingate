import type {
  Gateway,
  GatewayCertificateOption,
  GatewayListener,
  GatewayMutationPayload,
  GatewayValidationItem,
  GatewayValidationReport,
} from '@/domain/gateway';

const DEFAULT_RUNTIME_GROUP_ID = 'default';
const DEFAULT_RUNTIME_GROUP_NAME = '默认运行组';

export type GatewayHostMode = 'any' | 'specified';

export interface GatewayFormDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  runtimeGroupId: string;
  runtimeGroupName: string;
  listeners: GatewayListener[];
  hostMode: GatewayHostMode;
  hostnames: string[];
}

export function createGatewayDraft(gateway?: Gateway | null): GatewayFormDraft {
  const hostnames = gateway?.hostnames ?? (gateway?.hostPolicy === '不限制 Host' ? [] : parseHostnames(gateway?.hostPolicy ?? ''));

  return {
    id: gateway?.id,
    version: gateway?.version,
    name: gateway?.name ?? 'gw-new',
    description: gateway?.description ?? '',
    runtimeGroupId: gateway?.runtimeGroupId ?? DEFAULT_RUNTIME_GROUP_ID,
    runtimeGroupName: gateway?.runtimeGroupName ?? DEFAULT_RUNTIME_GROUP_NAME,
    listeners: gateway?.listenerItems?.length ? gateway.listenerItems : [createGatewayListener('HTTP', '8080')],
    hostMode: hostnames.length > 0 ? 'specified' : 'any',
    hostnames,
  };
}

export function createGatewayListener(protocol: GatewayListener['protocol'] = 'HTTP', port = ''): GatewayListener {
  return {
    id: `listener-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    protocol,
    port,
  };
}

export function validateGatewayDraft(draft: GatewayFormDraft): GatewayValidationReport {
  const normalizedHostnames = draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [];
  const invalidHostnames = normalizedHostnames.filter((hostname) => !isValidHostname(hostname));
  const ports = draft.listeners.map((listener) => listener.port.trim()).filter(Boolean);
  const duplicatePorts = ports.filter((port, index) => ports.indexOf(port) !== index);
  const httpsWithoutCertificate = draft.listeners.filter((listener) => listener.protocol === 'HTTPS' && !listener.certificateId);
  const items: GatewayValidationItem[] = [
    {
      label: '网关名称',
      status: draft.name.trim() ? 'healthy' : 'critical',
      message: draft.name.trim() ? draft.name.trim() : '请输入网关名称',
    },
    {
      label: '监听器',
      status: draft.listeners.length > 0 && draft.listeners.every((listener) => listener.port.trim()) && duplicatePorts.length === 0 ? 'healthy' : 'critical',
      message: duplicatePorts.length > 0
        ? `端口重复：${Array.from(new Set(duplicatePorts)).join('、')}`
        : draft.listeners.length > 0 && draft.listeners.every((listener) => listener.port.trim())
          ? formatListeners(draft.listeners)
          : '至少配置一个监听器，并填写端口',
    },
    {
      label: 'HTTPS 证书',
      status: httpsWithoutCertificate.length > 0 ? 'critical' : 'healthy',
      message: httpsWithoutCertificate.length > 0 ? 'HTTPS 监听器必须选择证书' : '证书配置满足要求',
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
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name.trim(),
    description: draft.description.trim(),
    runtimeGroupId: draft.runtimeGroupId,
    runtimeGroupName: draft.runtimeGroupName,
    listeners: draft.listeners.map((listener) => ({
      ...listener,
      port: listener.port.trim(),
    })),
    hostnames: draft.hostMode === 'specified' ? normalizeHostnames(draft.hostnames) : [],
  };
}

export function formatListeners(listeners: GatewayListener[]) {
  return listeners
    .map((listener) => `${listener.protocol}:${listener.port || '-'}`)
    .join(' / ');
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

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;

  if (!normalized.includes('.') || normalized.length > 253) {
    return false;
  }

  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}

function matchesCertificateDomain(domain: string, hostname: string) {
  if (domain.startsWith('*.')) {
    return hostname.endsWith(domain.slice(1));
  }

  return domain === hostname;
}
