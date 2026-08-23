import { apiListAllByCursor, apiRequest, type CursorPagedResponse } from './client';
import { normalizeResourceState } from '@/domain/common';
import type {
  PluginCatalog,
  PluginCatalogItem,
  PluginSource,
  PluginSourceInput,
  WasmPlugin,
} from '@/domain/plugin';

interface WasmPluginResponse extends Omit<WasmPlugin, 'version'> {
  version: string | number;
}

interface WasmPluginListResponse extends CursorPagedResponse {
  plugins?: WasmPluginResponse[];
}

interface WasmPluginCatalogResponse {
  plugins?: PluginCatalogItem[];
}

export async function listWasmPluginCatalog(): Promise<PluginCatalog> {
  const response = await apiRequest<WasmPluginCatalogResponse>('/wasm-plugin-catalog');
  return catalogFromAPI(response);
}

export async function listWasmPlugins(): Promise<WasmPlugin[]> {
  const plugins = await apiListAllByCursor<WasmPluginListResponse, WasmPluginResponse>(
    '/wasm-plugins',
    (page) => page.plugins ?? [],
  );
  return plugins.map(pluginFromAPI);
}

interface PluginSourceResponse extends Omit<PluginSource, 'version'> {
  version: string | number;
}

interface PluginSourceListResponse {
  sources?: PluginSourceResponse[];
}

export async function installWasmPlugin(sourceID: string, packageName: string): Promise<WasmPlugin> {
  const plugin = await apiRequest<WasmPluginResponse>('/wasm-plugins', {
    method: 'POST',
    body: JSON.stringify({ sourceID, package: packageName }),
  });
  return pluginFromAPI(plugin);
}

export async function listPluginSources(): Promise<PluginSource[]> {
  const response = await apiRequest<PluginSourceListResponse>('/plugin-sources');
  return (response.sources ?? []).map(pluginSourceFromAPI);
}

export async function createPluginSource(input: PluginSourceInput): Promise<PluginSource> {
  const response = await apiRequest<PluginSourceResponse>('/plugin-sources', {
    method: 'POST',
    body: JSON.stringify(input),
  });
  return pluginSourceFromAPI(response);
}

export async function updatePluginSource(source: PluginSource, input: PluginSourceInput): Promise<PluginSource> {
  const response = await apiRequest<PluginSourceResponse>(`/plugin-sources/${encodeURIComponent(source.id)}`, {
    method: 'PUT',
    body: JSON.stringify({ id: source.id, version: source.version, ...input }),
  });
  return pluginSourceFromAPI(response);
}

export async function deletePluginSource(id: string, version: number): Promise<void> {
  await apiRequest<Record<string, never>>(
    `/plugin-sources/${encodeURIComponent(id)}?version=${version}`,
    { method: 'DELETE' },
  );
}

export async function syncPluginSource(id: string): Promise<PluginSource> {
  const response = await apiRequest<PluginSourceResponse>(`/plugin-sources/${encodeURIComponent(id)}:sync`, {
    method: 'POST',
    body: JSON.stringify({ id }),
  });
  return pluginSourceFromAPI(response);
}

export async function upgradeWasmPlugin(plugin: Pick<WasmPlugin, 'id' | 'version'>): Promise<WasmPlugin> {
  const response = await apiRequest<WasmPluginResponse>(`/wasm-plugins/${encodeURIComponent(plugin.id)}`, {
    method: 'PUT',
    body: JSON.stringify({ id: plugin.id, version: plugin.version }),
  });
  return pluginFromAPI(response);
}

export async function deleteWasmPlugin(id: string, version: number): Promise<void> {
  await apiRequest<Record<string, never>>(
    `/wasm-plugins/${encodeURIComponent(id)}?version=${version}`,
    { method: 'DELETE' },
  );
}

function pluginFromAPI(plugin: WasmPluginResponse): WasmPlugin {
  return {
    ...plugin,
    version: Number(plugin.version),
    state: normalizeResourceState(plugin.state),
  };
}

function pluginSourceFromAPI(source: PluginSourceResponse): PluginSource {
  return {
    ...source,
    version: Number(source.version),
  };
}

function catalogFromAPI(response: WasmPluginCatalogResponse): PluginCatalog {
  return {
    plugins: response.plugins ?? [],
  };
}
