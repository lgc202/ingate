import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  resourceSpec,
  newSpec,
  title,
  type HTTPRoute,
  type ModelRoute,
  type Backend,
} from './resources.ts';

const identity = {
  name: 'stable-id',
  namespace: 'team-a',
  displayName: '展示名',
  version: '9007199254740993',
  createTime: '2026-10-24T00:00:00Z',
};

test('完整 PUT 保留全部规则、OR 匹配和父引用，不提交只读元数据', () => {
  const route: HTTPRoute = {
    ...identity,
    parentRefs: [{ name: 'g1' }, { name: 'g2', sectionName: 'tools' }],
    rules: [
      {
        name: 'first',
        matches: [
          { path: { type: 'PATH_MATCH_TYPE_EXACT', value: '/a' } },
          { path: { type: 'PATH_MATCH_TYPE_PREFIX', value: '/b' } },
        ],
        backendRefs: [{ name: 'b1' }],
      },
      { name: 'second', matches: [], backendRefs: [{ name: 'b2' }] },
    ],
  };

  const spec = resourceSpec('http-routes', route);

  assert.deepEqual(spec.rules, route.rules);
  assert.deepEqual(spec.parentRefs, route.parentRefs);
  assert.equal('version' in spec, false);
  assert.equal('createTime' in spec, false);
  assert.equal('name' in spec, false);
  spec.rules![0].name = 'changed';
  assert.equal(route.rules![0].name, 'first');
});

test('模型精确匹配和覆盖值不自动修剪或改变大小写', () => {
  const route: ModelRoute = {
    ...identity,
    parentRefs: [{ name: 'g' }],
    rules: [
      {
        matches: [
          {
            headers: [{ name: 'x-ai-eg-model', type: 'Exact', value: ' Coding ' }],
          },
        ],
        backendRefs: [{ name: 'ai', modelNameOverride: ' Code-V2 ' }],
      },
    ],
  };

  const spec = resourceSpec('model-routes', route);

  assert.equal(spec.rules![0].matches![0].headers![0].value, ' Coding ');
  assert.equal(spec.rules![0].backendRefs![0].modelNameOverride, ' Code-V2 ');
});

test('网络服务保留多端点权重；无 TLS 时显式提交 null 清除', () => {
  const backend: Backend = {
    ...identity,
    endpoints: [
      { address: 'a.example.com', port: 8080, weight: 3 },
      { address: 'b.example.com', port: 8081, weight: 2 },
    ],
  };

  const spec = resourceSpec('backends', backend);

  assert.deepEqual(spec.endpoints, backend.endpoints);
  assert.equal(spec.tls, null);
});

test('默认配置不分配资源稳定名称，展示名为空时可辨认', () => {
  for (const kind of [
    'gateways',
    'backends',
    'ai-backends',
    'http-routes',
    'model-routes',
    'mcp-routes',
  ] as const) {
    assert.equal('name' in newSpec(kind), false);
  }
  assert.equal(title({ ...identity, displayName: '' }), identity.name);
});
