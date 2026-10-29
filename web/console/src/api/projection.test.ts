import { test } from 'node:test';
import assert from 'node:assert/strict';
import { projectionMatches, waitForProjection } from './projection.ts';
import { APIError } from './client.ts';
import type { Gateway, Backend, HTTPRoute } from '../domain/resources.ts';

const saved: Gateway = {
  name: 'edge',
  namespace: 'team-a',
  version: 'opaque-after-write',
  displayName: 'New',
  listeners: [{ name: 'http', port: 8080, protocol: 'LISTENER_PROTOCOL_HTTP', hostname: '' }],
};

test('投影比较保留路由规则的先后顺序', () => {
  const route: HTTPRoute = {
    ...saved,
    parentRefs: [{ name: 'edge' }],
    rules: [
      { name: 'first', backendRefs: [{ name: 'one' }] },
      { name: 'second', backendRefs: [{ name: 'two' }] },
    ],
  };

  assert.equal(
    projectionMatches('http-routes', { ...route, rules: [...route.rules!].reverse() }, route),
    false,
  );
});

test('查询投影尚未更新时继续查询，直到配置与保存结果一致', async (t) => {
  let reads = 0;
  t.mock.method(globalThis, 'fetch', async (_url: string, options: RequestInit) => {
    assert.equal(options.method, undefined, '只能重试读，不能重复写');
    reads++;
    return Response.json({
      gateway:
        reads < 3
          ? { ...saved, displayName: 'Old' }
          : { ...saved, version: 'opaque-status-update' },
    });
  });

  assert.equal(
    await waitForProjection('team-a', 'gateways', { saved }, new AbortController().signal),
    true,
  );
  assert.equal(reads, 3);
});

test('删除后必须等到查询返回 404，不能把一次旧查询当作刷新完成', async (t) => {
  let reads = 0;
  t.mock.method(globalThis, 'fetch', async () =>
    ++reads === 1
      ? Response.json({ gateway: saved })
      : Response.json({ reason: 'NOT_FOUND' }, { status: 404 }),
  );

  assert.equal(
    await waitForProjection(
      'team-a',
      'gateways',
      { deleted: saved.name },
      new AbortController().signal,
    ),
    true,
  );
  assert.equal(reads, 2);
});

test('认证或服务错误不伪装成删除收敛，也不无限重试', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ reason: 'UNAVAILABLE' }, { status: 503 }),
  );

  await assert.rejects(
    waitForProjection('team-a', 'gateways', { deleted: saved.name }, new AbortController().signal),
    (cause) => cause instanceof APIError && cause.status === 503,
  );
  assert.equal(fetch.mock.callCount(), 1);
});

test('离开页面后取消同步，不继续发起后台请求', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => Response.json({ gateway: saved }));
  const controller = new AbortController();
  controller.abort();

  await assert.rejects(waitForProjection('team-a', 'gateways', { saved }, controller.signal));
  assert.equal(fetch.mock.callCount(), 0);
});

test('比较配置而不是解析版本，忽略投影的端点排序但不能忽略实际地址变化', () => {
  const backend: Backend = {
    ...saved,
    endpoints: [
      { address: 'b.example', port: 80, weight: 1 },
      { address: 'a.example', port: 80, weight: 2 },
    ],
  };
  const current = {
    ...backend,
    version: 'status-only',
    endpoints: [...backend.endpoints!].reverse(),
  };

  assert.equal(projectionMatches('backends', current, backend), true);
  assert.equal(projectionMatches('backends', { ...current, displayName: 'Old' }, backend), false);
  assert.equal(projectionMatches('backends', { ...current, name: 'different' }, backend), false);
  assert.equal(
    projectionMatches(
      'backends',
      { ...current, endpoints: [{ address: 'wrong.example', port: 80, weight: 1 }] },
      backend,
    ),
    false,
  );
  assert.equal(backend.endpoints![0].address, 'b.example', '不修改原始保存结果');
});
