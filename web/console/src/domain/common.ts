export type HealthStatus = 'healthy' | 'warning' | 'critical' | 'unknown';

export type PublishStatus = 'published' | 'pending' | 'failed' | 'disabled';

export type RuntimeSyncStatus = 'synced' | 'syncing' | 'failed' | 'unknown';

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

export function publishStatusLabel(status: PublishStatus) {
  const labels: Record<PublishStatus, string> = {
    published: '已生效',
    pending: '生效中',
    failed: '生效失败',
    disabled: '停用',
  };

  return labels[status];
}

export function runtimeSyncStatusLabel(status: RuntimeSyncStatus) {
  const labels: Record<RuntimeSyncStatus, string> = {
    synced: '已生效',
    syncing: '生效中',
    failed: '生效失败',
    unknown: '未知',
  };

  return labels[status];
}

export function statusTone(status: HealthStatus | PublishStatus | RuntimeSyncStatus) {
  if (status === 'healthy' || status === 'published' || status === 'synced') {
    return 'green';
  }

  if (status === 'warning' || status === 'pending' || status === 'syncing') {
    return 'amber';
  }

  if (status === 'critical' || status === 'failed') {
    return 'red';
  }

  return 'neutral';
}

export function statusColor(status: HealthStatus) {
  const colors: Record<HealthStatus, string> = {
    healthy: 'var(--green)',
    warning: 'var(--amber)',
    critical: 'var(--red)',
    unknown: '#8b918a',
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
