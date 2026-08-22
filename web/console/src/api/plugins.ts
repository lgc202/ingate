import { apiListAllByCursor, apiRequest, type CursorPagedResponse } from './client';
import { normalizeResourceState } from '@/domain/common';
import type { PluginCatalog, PluginCatalogItem, WasmPlugin } from '@/domain/plugin';

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

export async function installWasmPlugin(packageName: string): Promise<WasmPlugin> {
  const plugin = await apiRequest<WasmPluginResponse>('/wasm-plugins', {
    method: 'POST',
    body: JSON.stringify({ package: packageName }),
  });
  return pluginFromAPI(plugin);
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

function catalogFromAPI(response: WasmPluginCatalogResponse): PluginCatalog {
  return {
    plugins: response.plugins ?? [],
  };
}
