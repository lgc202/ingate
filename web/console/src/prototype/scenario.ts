export type RequestState = 'success' | 'fallback' | 'blocked' | 'error';
export type RequestKind = 'api' | 'ai';

export interface RequestRecord {
  id: string;
  time: string;
  kind: RequestKind;
  caller: string;
  route: string;
  method: string;
  path: string;
  model: string;
  service: string;
  status: RequestState;
  code: string;
  tokens: string;
  inputTokens: string;
  outputTokens: string;
  cachedTokens: string;
  bytes: string;
  latency: string;
  cost: string;
}

export const prototypeScenario = {
  summary: {
    gateways: 2,
    apiRoutes: 7,
    aiRoutes: 2,
    httpServices: 4,
    modelServices: 5,
    callers: 4,
  },
  delivery: {
    version: 142,
    state: 'healthy' as const,
    updatedAt: '14:31:08',
    instances: '3 / 3',
    resources: 26,
  },
  requests: [
    { id: 'req_4Ft7Xs', time: '14:32:31', kind: 'api', caller: '电商 Web', route: '订单查询 API', method: 'GET', path: '/api/orders/78421', model: '—', service: '订单服务', status: 'success', code: '200', tokens: '—', inputTokens: '—', outputTokens: '—', cachedTokens: '—', bytes: '12.4 KB', latency: '86 ms', cost: '—' },
    { id: 'req_8Km3Qr', time: '14:32:24', kind: 'api', caller: '客服工作台', route: '客户资料 API', method: 'GET', path: '/api/customers/9038', model: '—', service: '客户中心', status: 'success', code: '200', tokens: '—', inputTokens: '—', outputTokens: '—', cachedTokens: '—', bytes: '8.7 KB', latency: '124 ms', cost: '—' },
    { id: 'req_7Jv1Kq', time: '14:32:18', kind: 'ai', caller: '客服助手', route: '生产 AI 路由', method: 'POST', path: '/v1/chat/completions', model: 'qwen-max', service: '通义千问生产', status: 'success', code: '200', tokens: '1,842', inputTokens: '1,246', outputTokens: '596', cachedTokens: '312', bytes: '—', latency: '1.8 s', cost: '¥ 0.08' },
    { id: 'req_6Ce2Np', time: '14:32:07', kind: 'api', caller: '商家后台', route: '文件上传 API', method: 'POST', path: '/api/files', model: '—', service: '文件服务', status: 'error', code: '502', tokens: '—', inputTokens: '—', outputTokens: '—', cachedTokens: '—', bytes: '0 KB', latency: '1.2 s', cost: '—' },
    { id: 'req_2Pq9Lm', time: '14:31:56', kind: 'ai', caller: '研发知识库', route: '生产 AI 路由', method: 'POST', path: '/v1/chat/completions', model: 'claude-sonnet', service: 'Bedrock 灾备', status: 'fallback', code: '200', tokens: '3,106', inputTokens: '2,450', outputTokens: '656', cachedTokens: '780', bytes: '—', latency: '3.4 s', cost: '¥ 0.32' },
    { id: 'req_9Ab4Xe', time: '14:31:42', kind: 'ai', caller: '内部自动化', route: '生产 AI 路由', method: 'POST', path: '/v1/chat/completions', model: 'qwen-max', service: '—', status: 'blocked', code: '429', tokens: '—', inputTokens: '—', outputTokens: '—', cachedTokens: '—', bytes: '—', latency: '18 ms', cost: '—' },
    { id: 'req_5Nr8Tw', time: '14:30:11', kind: 'ai', caller: '客服助手', route: '内部 AI 路由', method: 'POST', path: '/v1/embeddings', model: 'text-embedding', service: '内部向量服务', status: 'success', code: '200', tokens: '926', inputTokens: '926', outputTokens: '0', cachedTokens: '0', bytes: '—', latency: '112 ms', cost: '¥ 0.01' },
    { id: 'req_1Hx6Bd', time: '14:29:48', kind: 'ai', caller: '研发知识库', route: '生产 AI 路由', method: 'POST', path: '/v1/chat/completions', model: 'claude-sonnet', service: 'Anthropic 公网', status: 'success', code: '200', tokens: '4,772', inputTokens: '3,800', outputTokens: '972', cachedTokens: '600', bytes: '—', latency: '2.7 s', cost: '¥ 0.46' },
  ] satisfies RequestRecord[],
};
