import type { ModelProtocol } from './upstream';
import type { RouteAccessMode } from './route';

export type RequestOutcome =
  | 'REQUEST_OUTCOME_UNSPECIFIED'
  | 'REQUEST_OUTCOME_SUCCESS'
  | 'REQUEST_OUTCOME_CLIENT_ERROR'
  | 'REQUEST_OUTCOME_SERVER_ERROR'
  | 'REQUEST_OUTCOME_NO_RESPONSE';

export type RequestRejectionReason =
  | 'REQUEST_REJECTION_REASON_UNSPECIFIED'
  | 'REQUEST_REJECTION_REASON_TOKEN_QUOTA_EXCEEDED';

export interface AIModelCall {
  clientModel: string;
  upstreamModel: string;
  protocol: ModelProtocol;
  responseModel: string;
  finishReason: string;
  inputTokens?: string | number;
  outputTokens?: string | number;
  totalTokens?: string | number;
}

// RequestRecordSummary 只包含列表扫描和进入详情所需的字段。
export interface RequestRecordSummary {
  id: string;
  startedAt: string;
  duration?: string;
  method: string;
  host: string;
  path: string;
  statusCode: number;
  outcome: RequestOutcome;
  gatewayID: string;
  routeID: string;
  serviceID: string;
  callerID: string;
  accessKeyID: string;
  rejectionReason: RequestRejectionReason;
  aiModelCall?: AIModelCall;
}

// RequestRecord 描述一次请求的完整排障元数据，不包含 Header、查询参数或正文。
export interface RequestRecord extends RequestRecordSummary {
  requestID: string;
  timeToFirstByte?: string;
  clientIP: string;
  requestBytes: string | number;
  responseBytes: string | number;
  protocol: string;
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
  callerID?: string;
  requestID?: string;
  method?: string;
  host?: string;
  pathPrefix?: string;
  outcome?: RequestOutcome;
  statusCode?: number;
}

export interface RequestRecordPage {
  records: RequestRecordSummary[];
  nextPageToken: string;
}

export interface RequestResourceOption {
  id: string;
  name: string;
}

export interface RequestRecordWorkspace {
  gateways: RequestResourceOption[];
  routes: Array<RequestResourceOption & { accessMode: RouteAccessMode }>;
  services: RequestResourceOption[];
  callers: Array<RequestResourceOption & { accessKeys: RequestResourceOption[] }>;
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

export function formatTokenCount(value?: string | number): string {
  if (value === undefined) return '-';
  const count = Number(value);
  if (!Number.isFinite(count)) return String(value);
  return new Intl.NumberFormat('zh-CN').format(count);
}
