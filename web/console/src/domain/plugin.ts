import type { CountSegment, HealthStatus, TimelineEvent } from './common';

export interface PluginResource {
  id: string;
  name: string;
  type: string;
  version: string;
  source: string;
  checksum: string;
  deploymentScope: string;
  healthStatus: HealthStatus;
  usedRoutes: number;
  lastUpdatedAt: string;
}

export interface PluginListView {
  plugins: PluginResource[];
  health: CountSegment[];
  incidents: TimelineEvent[];
}
