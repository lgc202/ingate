import type { ResourceState } from './common';

export const standardPluginPackages = {
  transformer: 'ingate-transformer',
  mockResponse: 'ingate-mock-response',
} as const;

export type WasmPluginPullPolicy =
  | 'WASM_PLUGIN_PULL_POLICY_IF_NOT_PRESENT'
  | 'WASM_PLUGIN_PULL_POLICY_ALWAYS';

export interface WasmPluginPolicyUsage {
  policyID: string;
  policyKind: 'HeaderTransformationPolicy' | 'MockResponsePolicy';
  policyName: string;
}

export interface WasmPlugin {
  id: string;
  sourceID: string;
  sourceName: string;
  name: string;
  package: string;
  pluginVersion: string;
  url: string;
  sha256: string;
  pullPolicy: WasmPluginPullPolicy;
  state: ResourceState;
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
  latestVersion: string;
  upgradeAvailable: boolean;
  usages: WasmPluginPolicyUsage[];
}

export interface PluginCatalogItem {
  sourceID: string;
  sourceName: string;
  package: string;
  name: string;
  pluginVersion: string;
  category: string;
  description: string;
  provider: string;
  license: string;
  sourceURL: string;
}

export interface PluginCatalog {
  plugins: PluginCatalogItem[];
}

export type PluginSourceSyncState =
  | 'PLUGIN_SOURCE_SYNC_STATE_READY'
  | 'PLUGIN_SOURCE_SYNC_STATE_ERROR'
  | 'PLUGIN_SOURCE_SYNC_STATE_DISABLED'
  | 'PLUGIN_SOURCE_SYNC_STATE_NOT_SYNCED'
  | 'PLUGIN_SOURCE_SYNC_STATE_UNSPECIFIED';

export interface PluginSource {
  id: string;
  name: string;
  url: string;
  builtin: boolean;
  enabled: boolean;
  syncState: PluginSourceSyncState;
  message: string;
  pluginCount: number;
  lastSyncedAt?: string;
  version: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface PluginSourceInput {
  name: string;
  url: string;
  enabled: boolean;
}

export function pluginSourceStateLabel(state: PluginSourceSyncState): string {
  if (state === 'PLUGIN_SOURCE_SYNC_STATE_READY') return '同步正常';
  if (state === 'PLUGIN_SOURCE_SYNC_STATE_ERROR') return '同步失败';
  if (state === 'PLUGIN_SOURCE_SYNC_STATE_DISABLED') return '已停用';
  return '尚未同步';
}

export function pluginSourceStateTone(state: PluginSourceSyncState): 'success' | 'error' | 'neutral' | 'warning' {
  if (state === 'PLUGIN_SOURCE_SYNC_STATE_READY') return 'success';
  if (state === 'PLUGIN_SOURCE_SYNC_STATE_ERROR') return 'error';
  if (state === 'PLUGIN_SOURCE_SYNC_STATE_DISABLED') return 'neutral';
  return 'warning';
}

export function pluginStatusLabel(state: ResourceState): string {
  if (state === 'Ready') return '可用';
  if (state === 'Error') return '不可用';
  if (state === 'Disabled') return '不可用';
  return '准备中';
}

export function pluginStatusTone(state: ResourceState): 'success' | 'warning' | 'error' | 'neutral' {
  if (state === 'Ready') return 'success';
  if (state === 'Error') return 'error';
  if (state === 'Disabled') return 'neutral';
  return 'warning';
}
