export type HealthStatus = 'healthy' | 'warning' | 'critical' | 'unknown';

export type ResourceState = 'Pending' | 'Ready' | 'Error' | 'Disabled';

export interface ResourceStatus {
  state: ResourceState;
  message: string;
}

export interface MetricCard {
  label: string;
  value: string;
  meta: string;
  footer: string;
}

export interface CountSegment {
  label: string;
  value: string;
  status: HealthStatus;
}

export interface TimelineEvent {
  time: string;
  title: string;
  description: string;
  status: HealthStatus;
}

export interface KeyValue {
  label: string;
  value: string;
}

export function healthLabel(status: HealthStatus) {
  const labels: Record<HealthStatus, string> = {
    healthy: '健康',
    warning: '警告',
    critical: '异常',
    unknown: '未知',
  };

  return labels[status];
}

export function statusTone(status: HealthStatus) {
  if (status === 'healthy') {
    return 'success';
  }

  if (status === 'warning') {
    return 'warning';
  }

  if (status === 'critical') {
    return 'danger';
  }

  return 'neutral';
}

export function statusColor(status: HealthStatus) {
  const colors: Record<HealthStatus, string> = {
    healthy: 'var(--success)',
    warning: 'var(--amber)',
    critical: 'var(--red)',
    unknown: 'var(--muted)',
  };

  return colors[status];
}

export function healthStatusClass(status: HealthStatus) {
  const classes: Record<HealthStatus, string> = {
    healthy: 'ok',
    warning: 'warn',
    critical: 'bad',
    unknown: 'neutral',
  };

  return classes[status];
}

export function formatDateTime(value: string) {
  if (!value) {
    return '-';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hour = String(date.getHours()).padStart(2, '0');
  const minute = String(date.getMinutes()).padStart(2, '0');

  return `${year}-${month}-${day} ${hour}:${minute}`;
}

export function normalizeResourceState(value: unknown): ResourceState {
  const states: Record<string, ResourceState> = {
    DISABLED: 'Disabled',
    PENDING: 'Pending',
    READY: 'Ready',
    ERROR: 'Error',
  };
  return states[String(value)] ?? 'Pending';
}
