import type { ResourceState } from './common';

export type WasmPluginPullPolicy =
  | 'WASM_PLUGIN_PULL_POLICY_IF_NOT_PRESENT'
  | 'WASM_PLUGIN_PULL_POLICY_ALWAYS';

export interface WasmPlugin {
  id: string;
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
}

export interface PluginCatalogItem {
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
  lastCheckedAt: string;
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
