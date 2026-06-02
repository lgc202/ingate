import type { HealthStatus } from './common';

export type RuntimeTarget = 'xds' | 'debug';

export type SnapshotStatus = 'generated' | 'published' | 'failed' | 'unknown';

export interface PublishSnapshot {
  id: string;
  name: string;
  gateway: string;
  target: RuntimeTarget;
  version: string;
  routeCount: number;
  clusterCount: number;
  endpointCount: number;
  status: SnapshotStatus;
  createdAt: string;
  message: string;
}

export interface PublishDiagnostic {
  label: string;
  value: string;
  status: HealthStatus;
}

export interface PublishListView {
  snapshots: PublishSnapshot[];
  diagnostics: PublishDiagnostic[];
}

export function snapshotStatusLabel(status: SnapshotStatus) {
  const labels: Record<SnapshotStatus, string> = {
    generated: '已生成',
    published: '已生效',
    failed: '生效失败',
    unknown: '未知',
  };

  return labels[status];
}
