import { APIError } from './client.ts';
import { getResource } from './resources.ts';
import { resourceSpec, type AnyResource, type Kind } from '../domain/resources.ts';

export type SavedChange = { saved: AnyResource } | { deleted: string };

// 写响应来自 API Server，Get/List 来自异步投影。只重试查询，不重放写操作。
// 8 秒内未收敛会返回 false，由页面保留“已保存，但列表尚未同步”的明确状态。
export async function waitForProjection(
  namespace: string,
  kind: Kind,
  change: SavedChange,
  signal: AbortSignal,
): Promise<boolean> {
  const deadline = AbortSignal.timeout(8000);
  const combined = AbortSignal.any([signal, deadline]);
  const name = 'saved' in change ? change.saved.name : change.deleted;
  try {
    for (const delay of [0, 150, 300, 600, 1000, 1500, 2000, 2000]) {
      await pause(delay, combined);
      try {
        const current = await getResource(namespace, kind, name, combined);
        if ('saved' in change && projectionMatches(kind, current, change.saved)) return true;
      } catch (cause) {
        if (!(cause instanceof APIError && cause.status === 404)) throw cause;
        if ('deleted' in change) return true;
      }
    }
    return false;
  } catch (cause) {
    if (signal.aborted) throw cause;
    if (deadline.aborted) return false;
    throw cause;
  }
}

export function projectionMatches(kind: Kind, current: AnyResource, saved: AnyResource): boolean {
  if (current.name !== saved.name || current.namespace !== saved.namespace) return false;
  // resourceVersion 是不透明字符串；控制器写 Status 也会改变它，不解析或比较版本大小。
  // 只比较可写配置。Listener/Endpoint 在查询投影中按键排序，其他有序规则保持原序。
  const spec = (resource: AnyResource) => {
    const value = resourceSpec(kind, resource);
    if ('listeners' in value) value.listeners?.sort((a, b) => a.name.localeCompare(b.name));
    if ('endpoints' in value)
      value.endpoints?.sort((a, b) => a.address.localeCompare(b.address) || a.port - b.port);
    return canonical(value);
  };
  return JSON.stringify(spec(current)) === JSON.stringify(spec(saved));
}

function canonical(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object')
    return Object.fromEntries(
      Object.entries(value)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([key, item]) => [key, canonical(item)]),
    );
  return value;
}

function pause(milliseconds: number, signal: AbortSignal): Promise<void> {
  signal.throwIfAborted();
  return new Promise((resolve, reject) => {
    const abort = () => {
      clearTimeout(timer);
      reject(signal.reason);
    };
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', abort);
      resolve();
    }, milliseconds);
    signal.addEventListener('abort', abort, { once: true });
  });
}
