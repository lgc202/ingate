import { test } from 'node:test';
import assert from 'node:assert/strict';
import { listResources, saveResource, deleteResource } from './resources.ts';
import { APIError } from './client.ts';

const gateway = {
  name: 'edge',
  namespace: 'team-a',
  displayName: '入口',
  version: '9007199254740993',
  listeners: [],
};

test('分页查询发送明确条件，不引入视觉稿中的未知筛选字段', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async (url: string, options: RequestInit) => {
    assert.equal(options.credentials, 'same-origin');
    assert.equal(new Headers(options.headers).get('Authorization'), null);
    assert.equal(new Headers(options.headers).get('X-Ingate-Browser'), '1');
    const parsed = new URL(url, 'https://ingate.example');
    assert.equal(parsed.pathname, '/api/admin/v1/namespaces/team-a/http-routes');
    assert.equal(parsed.searchParams.get('displayName'), '订单%');
    assert.equal(parsed.searchParams.get('gatewayName'), 'edge');
    assert.equal(parsed.searchParams.get('pageNum'), '2');
    assert.equal(parsed.searchParams.get('pageSize'), '20');
    assert.equal(parsed.searchParams.has('query'), false);
    return Response.json({ routes: [], totalCount: '32' });
  });

  const page = await listResources('team-a', 'http-routes', {
    pageNum: 2,
    pageSize: 20,
    displayName: '订单%',
    gatewayName: 'edge',
  });

  assert.equal(page.total, 32);
  assert.deepEqual(page.items, []);
  assert.equal(fetch.mock.callCount(), 1);
});

test('更新使用字符串版本，创建不自己生成 name', async (t) => {
  const bodies: Record<string, unknown>[] = [];
  t.mock.method(globalThis, 'fetch', async (_url: string, options: RequestInit) => {
    bodies.push(JSON.parse(options.body as string));
    return Response.json({ gateway });
  });

  await saveResource('team-a', 'gateways', { displayName: '新名称', listeners: [] }, gateway);
  await saveResource('team-a', 'gateways', {
    displayName: '新入口',
    listeners: [],
  });

  assert.equal(bodies[0].version, '9007199254740993');
  assert.equal(bodies[0].name, 'edge');
  assert.equal(bodies[1].name, undefined);
  assert.equal(bodies[1].namespace, 'team-a');
});

test('版本冲突只返回错误，不隐式覆盖或自动重试', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ reason: 'CONFLICT', message: 'changed' }, { status: 409 }),
  );

  await assert.rejects(
    saveResource('team-a', 'gateways', { displayName: 'x', listeners: [] }, gateway),
    (error) => error instanceof APIError && error.status === 409,
  );

  assert.equal(fetch.mock.callCount(), 1);
});

test('Delete 不附加契约不存在的 version 或请求体', async (t) => {
  t.mock.method(globalThis, 'fetch', async (url: string, options: RequestInit) => {
    assert.equal(url, '/api/admin/v1/namespaces/team-a/gateways/edge');
    assert.equal(options.method, 'DELETE');
    assert.equal(options.body, undefined);
    return Response.json({});
  });

  await deleteResource('team-a', 'gateways', 'edge');
});

test('拒绝丢失精度的 totalCount，不展示错误总数', async (t) => {
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ gateways: [], totalCount: '9007199254740993' }),
  );

  await assert.rejects(
    listResources('team-a', 'gateways', { pageNum: 1, pageSize: 20 }),
    /无效的分页数据/,
  );
});

test('传输中断不重试写入，并提示先确认结果', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => {
    throw new TypeError('network failed');
  });

  await assert.rejects(
    saveResource('team-a', 'gateways', { displayName: 'x', listeners: [] }),
    (error) =>
      error instanceof APIError &&
      error.reason === 'NETWORK_ERROR' &&
      error.message.includes('避免重复提交'),
  );

  assert.equal(fetch.mock.callCount(), 1);
});
