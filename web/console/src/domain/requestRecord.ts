export type RequestOutcome =
  | 'REQUEST_OUTCOME_UNSPECIFIED'
  | 'REQUEST_OUTCOME_SUCCESS'
  | 'REQUEST_OUTCOME_CLIENT_ERROR'
  | 'REQUEST_OUTCOME_SERVER_ERROR';

// RequestRecord 只描述一次请求的排障元数据，不包含 Header、查询参数或正文。
export interface RequestRecord {
  id: string;
  requestID: string;
  startedAt: string;
  duration?: string;
  timeToFirstByte?: string;
  clientIP: string;
  method: string;
  host: string;
  path: string;
  statusCode: number;
  outcome: RequestOutcome;
  requestBytes: string | number;
  responseBytes: string | number;
  gatewayID: string;
  routeID: string;
  serviceID: string;
  protocol: string;
  responseCodeDetails: string;
  upstreamAttempts: number;
  upstreamAddress: string;
  proxyInstanceID: string;
}

export interface RequestRecordFilters {
  startTime: string;
  endTime: string;
  gatewayID?: string;
  routeID?: string;
  serviceID?: string;
  requestID?: string;
  method?: string;
  host?: string;
  pathPrefix?: string;
  outcome?: RequestOutcome;
  statusCode?: number;
}

export interface RequestRecordPage {
  records: RequestRecord[];
  nextPageToken: string;
}

export interface RequestResourceOption {
  id: string;
  name: string;
}

export interface RequestRecordWorkspace {
  gateways: RequestResourceOption[];
  routes: RequestResourceOption[];
  services: RequestResourceOption[];
}

export function requestOutcomeLabel(outcome: RequestOutcome): string {
  if (outcome === 'REQUEST_OUTCOME_SUCCESS') return '成功';
  if (outcome === 'REQUEST_OUTCOME_CLIENT_ERROR') return '客户端错误';
  if (outcome === 'REQUEST_OUTCOME_SERVER_ERROR') return '服务端错误';
  return '未知';
}

export function requestOutcomeTone(outcome: RequestOutcome): 'success' | 'warning' | 'error' | 'neutral' {
  if (outcome === 'REQUEST_OUTCOME_SUCCESS') return 'success';
  if (outcome === 'REQUEST_OUTCOME_CLIENT_ERROR') return 'warning';
  if (outcome === 'REQUEST_OUTCOME_SERVER_ERROR') return 'error';
  return 'neutral';
}

export function formatRequestTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value || '-';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
    hour12: false,
  }).format(date);
}

export function formatDuration(value?: string): string {
  if (!value) return '-';
  const seconds = Number(value.replace(/s$/, ''));
  if (!Number.isFinite(seconds)) return value;
  const milliseconds = seconds * 1000;
  if (milliseconds < 1) return `${Math.round(milliseconds * 1000)} μs`;
  if (milliseconds < 1000) return `${milliseconds.toFixed(milliseconds < 10 ? 2 : 1)} ms`;
  return `${seconds.toFixed(2)} s`;
}

export function formatBytes(value: string | number): string {
  const bytes = Number(value);
  if (!Number.isFinite(bytes)) return '-';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
}
