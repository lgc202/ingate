import { request } from './client.ts';
import type { Kind, Resources, Spec } from '../domain/resources.ts';

export interface ListQuery {
  pageNum: number;
  pageSize: number;
  displayName?: string;
  gatewayName?: string;
}
export interface Page<T> {
  items: T[];
  total: number;
}

const replyKeys: Record<Kind, { item: string; list: string }> = {
  gateways: { item: 'gateway', list: 'gateways' },
  backends: { item: 'backend', list: 'backends' },
  'ai-backends': { item: 'backend', list: 'backends' },
  'http-routes': { item: 'route', list: 'routes' },
  'model-routes': { item: 'route', list: 'routes' },
  'mcp-routes': { item: 'route', list: 'routes' },
};

function collectionPath(namespace: string, kind: Kind): string {
  if (!namespace) throw new Error('必须指定 Namespace。');

  return `/namespaces/${encodeURIComponent(namespace)}/${kind}`;
}

function unwrap<K extends Kind>(kind: K, reply: Record<string, unknown>): Resources[K] {
  const resource = reply[replyKeys[kind].item] as Resources[K] | undefined;
  if (!resource?.name || !resource.version || !resource.namespace)
    throw new Error('管理 API 返回的资源缺少身份或版本。');

  return resource;
}

export async function listResources<K extends Kind>(
  namespace: string,
  kind: K,
  query: ListQuery,
  signal?: AbortSignal,
): Promise<Page<Resources[K]>> {
  const params = new URLSearchParams({
    pageNum: String(query.pageNum),
    pageSize: String(query.pageSize),
  });
  if (query.displayName) params.set('displayName', query.displayName);
  if (query.gatewayName && kind.endsWith('-routes')) params.set('gatewayName', query.gatewayName);
  const reply = await request<Record<string, unknown>>(
    `${collectionPath(namespace, kind)}?${params}`,
    { signal },
  );
  const items = reply[replyKeys[kind].list] ?? [];
  const total = Number(reply.totalCount ?? '0');
  if (!Array.isArray(items) || !Number.isSafeInteger(total) || total < 0)
    throw new Error('管理 API 返回了无效的分页数据。');

  return { items: items as Resources[K][], total };
}

export async function getResource<K extends Kind>(
  namespace: string,
  kind: K,
  name: string,
  signal?: AbortSignal,
): Promise<Resources[K]> {
  const reply = await request<Record<string, unknown>>(
    `${collectionPath(namespace, kind)}/${encodeURIComponent(name)}`,
    { signal },
  );

  return unwrap(kind, reply);
}

export async function saveResource<K extends Kind>(
  namespace: string,
  kind: K,
  spec: Spec<Resources[K]>,
  observed?: Resources[K],
): Promise<Resources[K]> {
  const path =
    collectionPath(namespace, kind) + (observed ? '/' + encodeURIComponent(observed.name) : '');
  const body = {
    ...spec,
    namespace,
    ...(observed ? { name: observed.name, version: observed.version } : {}),
  };
  const reply = await request<Record<string, unknown>>(path, {
    method: observed ? 'PUT' : 'POST',
    body: JSON.stringify(body),
  });

  return unwrap(kind, reply);
}

export async function deleteResource(namespace: string, kind: Kind, name: string): Promise<void> {
  // 当前 Delete 契约不接受 version，不能凭空附加并发删除条件。
  await request(`${collectionPath(namespace, kind)}/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  });
}
