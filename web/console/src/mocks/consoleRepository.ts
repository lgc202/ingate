import type { ConsoleRepository } from '@/api/contracts';
import type {
  GatewayListView,
  GatewayMutationPayload,
  GatewayMutationResult,
  GatewayValidationReport,
} from '@/domain/gateway';
import type { HomeDashboard } from '@/domain/home';
import type { ObservabilityOverview } from '@/domain/observability';
import type { PluginListView } from '@/domain/plugin';
import type {
  AccessControlPolicy,
  AccessControlPolicyPayload,
  GovernancePolicy,
  PolicyBinding,
  PolicyBindingPayload,
  PolicyMutationResult,
  PolicyTargetOption,
  PolicyWorkspace,
  RateLimitPolicy,
  RateLimitPolicyPayload,
} from '@/domain/policy';
import type { RuntimeStatusView } from '@/domain/runtime';
import type { RouteActionResult, RouteMutationPayload, RoutePageView, RouteTargetPayload, RouteValidationReport } from '@/domain/route';
import {
  routePolicyCapabilityRequestHeaderModifier,
  routePolicyCapabilityRetry,
  routePolicyCapabilityTimeout,
} from '@/domain/route';
import type {
  ServiceListView,
  ServiceMutationPayload,
  ServiceMutationResult,
  ServiceValidationReport,
} from '@/domain/service';
import { serviceLoadBalancePolicyLabel } from '@/domain/service';
import type { SettingsWorkspace } from '@/domain/settings';

const defaultRouteTimeoutMillis = 30000;

const homeDashboard: HomeDashboard = {
  context: {
    configurationDomain: '当前 Ingate',
    timeRange: '最近 15 分钟',
    timeRangeOptions: ['最近 15 分钟', '最近 1 小时', '最近 24 小时'],
  },
  metrics: [
    { label: '活跃网关', value: '3', meta: '较昨日 0', footer: '全部正常' },
    { label: '在线路由', value: '54', meta: '较昨日 +2', footer: '92.4% 匹配成功率' },
    { label: '健康服务', value: '27', meta: '较昨日 +1', footer: '99.96% 成功率' },
    { label: '待处理风险', value: '2', meta: '影响 1 个网关 / 1 个服务', footer: '需确认' },
  ],
  keyLinks: [
    {
      id: 'gw-prod-orders',
      gatewayName: 'gw-prod',
      routeMethod: 'POST',
      routePath: '/v1/orders',
      serviceName: 'order-svc',
      traffic: '3.2k req/s',
      successRate: '99.3%',
      latencyP95: '186ms',
      status: 'healthy',
      reason: '刚生效且流量稳定',
    },
    {
      id: 'gw-prod-users',
      gatewayName: 'gw-prod',
      routeMethod: 'GET',
      routePath: '/v1/users',
      serviceName: 'user-svc',
      traffic: '12.4k req/s',
      successRate: '98.7%',
      latencyP95: '241ms',
      status: 'healthy',
      reason: '当前环境最高流量链路',
    },
    {
      id: 'gw-prod-products',
      gatewayName: 'gw-prod',
      routeMethod: 'GET',
      routePath: '/v1/products',
      serviceName: 'catalog-svc',
      traffic: '1.8k req/s',
      successRate: '97.1%',
      latencyP95: '304ms',
      status: 'warning',
      reason: '成功率低于基线',
    },
    {
      id: 'gw-staging-inventory',
      gatewayName: 'gw-staging',
      routeMethod: 'GET',
      routePath: '/v1/inventory',
      serviceName: 'inventory-svc',
      traffic: '356 req/s',
      successRate: '92.4%',
      latencyP95: '612ms',
      status: 'critical',
      reason: '异常链路，建议优先处理',
    },
  ],
  actionItems: [
    {
      id: 'route-inventory',
      priority: 'P1',
      title: '异常路由 /v1/inventory',
      description: '当前范围内成功率下降并触发错误峰值。',
      target: 'route',
      status: 'critical',
      actionLabel: '查看路由',
    },
    {
      id: 'service-payment',
      priority: 'P2',
      title: 'payment-svc 健康未知',
      description: '未配置健康检查接口，状态无法自动确认。',
      target: 'service',
      status: 'unknown',
      actionLabel: '查看服务',
    },
    {
      id: 'runtime-pending',
      priority: 'P2',
      title: '3 条配置等待生效',
      description: '涉及 1 个网关、2 条路由、1 个策略，系统会自动生成运行配置。',
      target: 'runtime',
      status: 'warning',
      actionLabel: '查看状态',
    },
    {
      id: 'cert-expire',
      priority: 'P3',
      title: 'api.ingate.io 证书提醒',
      description: '7 天后到期，建议提前续期。',
      target: 'gateway',
      status: 'warning',
      actionLabel: '查看网关',
    },
  ],
  healthSummary: [
    { label: '网关健康', value: '3', status: 'healthy' },
    { label: '路由异常', value: '2', status: 'critical' },
    { label: '服务未知', value: '1', status: 'unknown' },
    { label: 'Envoy 漂移', value: '1', status: 'warning' },
  ],
  changes: [
    { time: '5 分钟前', title: '自动生效成功', description: 'POST /v1/orders / order-svc', status: 'healthy' },
    { time: '18 分钟前', title: '策略更新', description: 'user-rate-limit 调整到 100 req/s', status: 'unknown' },
    { time: '34 分钟前', title: '证书续期', description: 'api.ingate.io 自动续期成功', status: 'healthy' },
  ],
  requestTrend: [12, 16, 14, 19, 18, 22, 21, 24, 20, 26, 23, 25],
  errorDistribution: [
    ['401 Unauthorized', '1.2k', '12.4%'],
    ['429 Too Many Requests', '860', '8.9%'],
    ['400 Bad Request', '320', '3.3%'],
    ['503 Service Unavailable', '210', '2.2%'],
    ['504 Gateway Timeout', '160', '1.6%'],
  ],
};

const gatewayList: GatewayListView = {
  gateways: [
    {
      id: 'gw-public',
      name: '公网入口',
      description: '公网 API 入口',
      listeners: [
        { name: 'gw-public-https', protocol: 'HTTPS', port: 443, certificateId: 'cert-api-ingate' },
        { name: 'gw-public-http', protocol: 'HTTP', port: 80 },
      ],
      hostBindings: [
        { hostname: 'api.ingate.io', listenerRefs: ['gw-public-https', 'gw-public-http'], tls: { certificateRef: 'cert-api-ingate' } },
        { hostname: '*.api.ingate.io', listenerRefs: ['gw-public-https', 'gw-public-http'], tls: { certificateRef: 'cert-api-ingate' } },
      ],
      enabled: true,
      createdAt: '2026-06-05T09:00:00Z',
    },
    {
      id: 'gw-partner',
      name: '合作方入口',
      description: '合作方 API 入口',
      listeners: [
        { name: 'gw-partner-https', protocol: 'HTTPS', port: 10443, certificateId: 'cert-partner' },
      ],
      hostBindings: [
        { hostname: 'partner.ingate.local', listenerRefs: ['gw-partner-https'], tls: { certificateRef: 'cert-partner' } },
      ],
      enabled: true,
      createdAt: '2026-06-05T08:40:00Z',
    },
    {
      id: 'gw-sandbox',
      name: '沙箱入口',
      description: '沙箱联调入口',
      listeners: [
        { name: 'gw-sandbox-http', protocol: 'HTTP', port: 18080 },
      ],
      hostBindings: [
        { listenerRefs: ['gw-sandbox-http'] },
      ],
      enabled: true,
      createdAt: '2026-06-05T07:00:00Z',
    },
    {
      id: 'gw-broken',
      name: '遗留系统入口',
      description: '遗留系统入口',
      listeners: [
        { name: 'gw-broken-http', protocol: 'HTTP', port: 19090 },
      ],
      hostBindings: [
        { hostname: 'legacy.ingate.local', listenerRefs: ['gw-broken-http'] },
      ],
      enabled: false,
      createdAt: '2026-06-04T09:00:00Z',
    },
  ],
};

const routeWorkspace: RoutePageView = {
  routes: [
    { id: 'users-list', name: '用户查询路由', gatewayIDs: ['gw-prod'], hostnames: ['api.ingate.io', 'open.ingate.io'], rules: [{ name: 'main', methods: ['GET'], pathPrefix: '/v1/users', targets: [{ upstreamID: 'user-svc', weight: 100 }] }], enabled: true, createdAt: '2026-06-05T09:00:00Z' },
    { id: 'orders-create', name: '订单创建路由', gatewayIDs: ['gw-prod', 'gw-partner'], hostnames: ['api.ingate.io', 'shop.ingate.io'], rules: [{ name: 'main', methods: ['POST'], pathPrefix: '/v1/orders', targets: [{ upstreamID: 'order-svc', weight: 90 }, { upstreamID: 'user-svc', weight: 10 }] }], enabled: true, createdAt: '2026-06-05T08:40:00Z' },
    { id: 'products-list', name: '商品列表路由', gatewayIDs: ['gw-prod'], hostnames: ['api.ingate.io'], rules: [{ name: 'main', methods: ['GET'], pathPrefix: '/v1/products', targets: [{ upstreamID: 'catalog-svc', weight: 100 }] }], enabled: true, createdAt: '2026-06-05T08:20:00Z' },
    { id: 'inventory-list', name: '库存查询路由', gatewayIDs: ['gw-staging'], hostnames: ['staging-api.ingate.io'], rules: [{ name: 'main', methods: [], pathPrefix: '/v1/inventory', targets: [{ upstreamID: 'inventory-svc', weight: 100 }] }], enabled: false, createdAt: '2026-06-05T08:00:00Z' },
  ],
  composer: {
    name: '',
    methods: ['POST'],
    path: '/v1/orders',
    gatewayIDs: ['gw-prod'],
    gateways: [
      { id: 'gw-prod', name: '生产网关' },
      { id: 'gw-partner', name: '合作方网关' },
      { id: 'gw-staging', name: '预发网关' },
    ],
    hostnames: ['api.ingate.io', 'shop.ingate.io'],
    validations: ['匹配规则生效', '目标服务可用', '转发控制正确', '无冲突'],
    targets: [
      { id: 'order-svc', name: 'order-svc', type: 'application', endpoint: 'order-svc.cluster.local:80', meta: '3/3 个端点' },
      { id: 'user-svc', name: 'user-svc', type: 'application', endpoint: 'user-svc.cluster.local:80', meta: '4/4 个端点' },
      { id: 'gpt-4o-model', name: 'gpt-4o-model', type: 'model', endpoint: 'api.openai.example:443', meta: '1/1 个端点' },
    ],
    policies: [
      {
        capability: routePolicyCapabilityRequestHeaderModifier,
        displayName: '请求 Header 改写',
        meta: '向后端服务写入或删除请求 Header',
        enabled: false,
        params: [
          { key: 'setHeadersOn', label: '写入 Header 名称', defaultValue: '', placeholder: '多个名称用逗号分隔', required: true },
          { key: 'value', label: 'Header 值', defaultValue: '', placeholder: '请输入要写入的 Header 值', required: true },
          { key: 'removeHeadersOn', label: '删除 Header 名称', defaultValue: '', placeholder: '多个名称用逗号分隔' },
        ],
      },
      {
        capability: routePolicyCapabilityTimeout,
        displayName: '超时控制',
        meta: '设置请求从进入网关到返回响应的最长时间，包含失败重试过程',
        enabled: false,
        params: [
          { key: 'timeoutMillis', label: '请求总超时', defaultValue: '30000', inputType: 'number', unit: 'ms', min: 100, max: 300000, required: true },
        ],
      },
      {
        capability: routePolicyCapabilityRetry,
        displayName: '失败重试',
        meta: '后端连接失败或返回异常状态时自动重试；单次尝试超时不能超过请求总超时',
        enabled: false,
        params: [
          { key: 'attempts', label: '重试次数', defaultValue: '2', inputType: 'number', unit: '次', min: 1, max: 5, required: true },
          { key: 'perTryTimeoutMillis', label: '单次尝试超时', defaultValue: '1000', inputType: 'number', unit: 'ms', min: 100, max: 60000, required: true },
        ],
      },
    ],
  },
};

const serviceList: ServiceListView = {
  upstreams: [
    { id: 'order-svc', name: 'order-svc', type: 'application', endpoints: [{ id: 'order-1', address: 'order-svc.cluster.local', port: 80, weight: 100, enabled: true }], loadBalancePolicy: 'round_robin', healthCheck: { enabled: true, path: '/healthz', intervalSeconds: 10, timeoutSeconds: 2 }, createdAt: '2026-06-05T09:00:00Z' },
    { id: 'user-svc', name: 'user-svc', type: 'application', endpoints: [{ id: 'user-1', address: 'user-svc.cluster.local', port: 80, weight: 100, enabled: true }], loadBalancePolicy: 'round_robin', healthCheck: { enabled: true, path: '/healthz', intervalSeconds: 10, timeoutSeconds: 2 }, createdAt: '2026-06-05T08:50:00Z' },
    { id: 'catalog-svc', name: 'catalog-svc', type: 'application', endpoints: [{ id: 'catalog-1', address: 'catalog-svc.cluster.local', port: 80, weight: 100, enabled: true }], loadBalancePolicy: 'least_request', healthCheck: { enabled: true, path: '/healthz', intervalSeconds: 10, timeoutSeconds: 2 }, createdAt: '2026-06-05T08:40:00Z' },
    { id: 'inventory-svc', name: 'inventory-svc', type: 'application', endpoints: [{ id: 'inventory-1', address: 'inventory-svc.cluster.local', port: 80, weight: 100, enabled: true }, { id: 'inventory-2', address: 'inventory-canary.cluster.local', port: 80, weight: 20, enabled: false }], loadBalancePolicy: 'round_robin', healthCheck: { enabled: true, path: '/healthz', intervalSeconds: 10, timeoutSeconds: 2 }, createdAt: '2026-06-05T08:30:00Z' },
  ],
};

const runtimeStatus: RuntimeStatusView = {
  available: true,
  message: 'Controller 正常提供配置编译和 xDS 交付服务',
  configReady: true,
  deliveryState: 'Active',
  activeVersion: 'ingate/8f31b7e',
  connectedEnvoys: 3,
  ack: { required: 4, received: 4 },
};

const rateLimitPolicies: RateLimitPolicy[] = [
  {
    id: 'rl-prod-api',
    version: '101',
    name: '生产 API 全局限流',
    description: '公网入口统一限流',
    enabled: true,
    mode: 'Global',
    rules: [
      {
        name: 'by-ip',
        key: { parts: [{ type: 'IP' }] },
        limit: { requests: 1000, windowSeconds: 60, burst: 100 },
        algorithm: 'SlidingWindow',
      },
    ],
    response: { statusCode: 429, message: 'Too many requests', quotaHeaderEnabled: true },
    failurePolicy: 'FailOpen',
    createdAt: '2026-06-05T09:20:00Z',
  },
  {
    id: 'rl-partner-login',
    version: '87',
    name: '合作方登录限流',
    description: '按合作方 Header 控制登录请求频率',
    enabled: true,
    mode: 'Local',
    rules: [
      {
        name: 'by-partner',
        key: { parts: [{ type: 'Header', name: 'x-partner-id' }] },
        limit: { requests: 120, windowSeconds: 60 },
        algorithm: 'FixedWindow',
      },
    ],
    response: { statusCode: 429, message: 'Too many requests' },
    failurePolicy: 'FailClose',
    createdAt: '2026-06-05T08:50:00Z',
  },
];

const accessControlPolicies: AccessControlPolicy[] = [
  {
    id: 'acl-internal-only',
    version: '45',
    name: '内网访问控制',
    description: '只允许内网网段访问管理接口',
    enabled: true,
    defaultAction: 'Deny',
    rules: [
      {
        name: 'allow-office',
        action: 'Allow',
        conditions: [{ type: 'IP', value: '10.0.0.0/8' }],
      },
      {
        name: 'allow-vpc',
        action: 'Allow',
        conditions: [{ type: 'IP', value: '172.16.0.0/12' }],
      },
    ],
    response: { statusCode: 403, message: 'Forbidden' },
    createdAt: '2026-06-05T09:10:00Z',
  },
  {
    id: 'acl-partner-header',
    version: '33',
    name: '合作方 Header 准入',
    description: '合作方流量必须携带来源标识',
    enabled: false,
    defaultAction: 'Allow',
    rules: [
      {
        name: 'deny-missing-partner',
        action: 'Deny',
        conditions: [{ type: 'Header', name: 'x-partner-id', value: '' }],
      },
    ],
    response: { statusCode: 403, message: 'Forbidden' },
    createdAt: '2026-06-05T08:30:00Z',
  },
];

const policyBindings: PolicyBinding[] = [
  {
    id: 'bind-public-rate-limit',
    version: '17',
    name: '公网入口限流',
    description: '生产公网网关绑定全局限流策略',
    enabled: true,
    targetRef: { kind: 'Gateway', name: 'gw-public' },
    policies: [{ kind: 'RateLimitPolicy', name: 'rl-prod-api' }],
    createdAt: '2026-06-05T09:30:00Z',
  },
  {
    id: 'bind-orders-acl',
    version: '12',
    name: '订单路由访问控制',
    description: '订单创建路由绑定访问控制',
    enabled: true,
    targetRef: { kind: 'Route', name: 'orders-create', ruleName: 'main' },
    policies: [
      { kind: 'AccessControlPolicy', name: 'acl-internal-only' },
      { kind: 'RateLimitPolicy', name: 'rl-partner-login' },
    ],
    createdAt: '2026-06-05T09:25:00Z',
  },
];

const pluginList: PluginListView = {
  plugins: [
    { id: 'auth-plugin', name: 'auth-plugin', type: '认证', version: 'v2.1.0', source: '包仓库', checksum: 'sha256:9b..f1', deploymentScope: 'gw-prod / gw-staging', healthStatus: 'healthy', usedRoutes: 12, lastUpdatedAt: '4 分钟前' },
    { id: 'ratelimit-plugin', name: 'ratelimit-plugin', type: '限流', version: 'v2.1.0', source: '包仓库', checksum: 'sha256:2c..84', deploymentScope: 'gw-prod', healthStatus: 'critical', usedRoutes: 8, lastUpdatedAt: '18 分钟前' },
    { id: 'ai-safety-plugin', name: 'ai-safety-plugin', type: 'AI 安全', version: 'v1.9.3', source: '包仓库', checksum: 'sha256:7f..aa', deploymentScope: 'AI 路由', healthStatus: 'healthy', usedRoutes: 4, lastUpdatedAt: '34 分钟前' },
    { id: 'observability-plugin', name: 'observability-plugin', type: '观测', version: 'v1.2.0', source: '包仓库', checksum: 'sha256:3d..bc', deploymentScope: '全部网关', healthStatus: 'healthy', usedRoutes: 16, lastUpdatedAt: '1 小时前' },
  ],
  health: [
    { label: '健康', value: '3', status: 'healthy' },
    { label: '异常', value: '1', status: 'critical' },
  ],
  incidents: [
    { time: '2 分钟前', title: 'ratelimit-plugin', description: 'Redis 连接超时', status: 'critical' },
    { time: '8 分钟前', title: '影响网关', description: 'gw-prod', status: 'unknown' },
    { time: '立即处理', title: '处理建议', description: '检查限流存储连接', status: 'warning' },
  ],
};

const observabilityOverview: ObservabilityOverview = {
  metrics: [
    { label: '请求量', value: '18.7k', meta: '较上一周期 +12%', footer: '按网关聚合' },
    { label: '错误率', value: '1.8%', meta: '较上一周期 -0.4%', footer: '按路由聚合' },
    { label: 'P95 延迟', value: '242ms', meta: '较上一周期 +18ms', footer: '按服务聚合' },
    { label: 'AI Token', value: '9.6M', meta: '较上一周期 +8%', footer: '按模型聚合' },
    { label: '插件异常', value: '3', meta: '较上一周期 -1', footer: '最近 1 小时' },
  ],
  requestTrend: [14, 15, 13, 17, 20, 18, 22, 25, 21, 24, 27, 26],
  callLogs: [
    { route: 'POST /v1/orders', statusCode: '200', result: '成功' },
    { route: 'GET /v1/users', statusCode: '200', result: '成功' },
    { route: 'POST /v1/login', statusCode: '429', result: '限流' },
    { route: 'GET /v1/catalog', statusCode: '503', result: '异常' },
    { route: 'POST /v1/chat', statusCode: '200', result: '成功' },
  ],
  serviceHealth: [
    { label: '健康', value: '23 (85.2%)', status: 'healthy' },
    { label: '警告', value: '3 (11.1%)', status: 'warning' },
    { label: '异常', value: '1 (3.7%)', status: 'critical' },
    { label: '未知', value: '0 (0%)', status: 'unknown' },
  ],
  alerts: [
    { time: '2 分钟前', title: 'fraud-agent', description: '实例健康检查失败', status: 'critical' },
    { time: '15 分钟前', title: 'inventory-svc', description: '部分实例响应慢', status: 'warning' },
    { time: '22 分钟前', title: 'order-svc', description: 'CPU 使用率偏高', status: 'warning' },
  ],
};

const settingsWorkspace: SettingsWorkspace = {
  sections: {
    'users-roles': {
      key: 'users-roles',
      title: '用户与角色',
      table: {
        title: '用户与角色',
        subtitle: '管理控制台用户、角色权限和访问范围',
        headers: ['用户', '邮箱', '角色', '访问范围', '最近登录', 'MFA', '状态'],
        rows: [
          ['张晓鹏', 'zhangxp@ingate.io', '平台管理员', '生产环境项目', '2 小时前', '已开启', '启用'],
          ['刘维', 'liuwei@ingate.io', '网关管理员', '预发布环境项目', '8 小时前', '已开启', '启用'],
          ['陈清', 'chenqing@ingate.io', '开发者', '生产环境项目', '1 天前', '未开启', '启用'],
          ['系统审计', 'system@ingate.io', '只读观察者', '全部环境', '7 天前', '已开启', '禁用'],
        ],
      },
    },
    audit: {
      key: 'audit',
      title: '审计设置',
      toggleGroups: [
        {
          title: '审计设置',
          subtitle: '配置操作审计、敏感操作复核和日志保留策略',
          items: [
            { label: '记录所有操作日志', enabled: true },
            { label: '日志保留 180 天', enabled: true },
            { label: '敏感操作二次确认', enabled: true },
          ],
        },
        {
          title: '记录范围',
          items: [
            { label: '登录', enabled: true },
            { label: '资源变更', enabled: true },
            { label: '配置生效', enabled: true },
            { label: '权限变更', enabled: true },
            { label: '策略变更', enabled: true },
          ],
        },
      ],
    },
    notifications: {
      key: 'notifications',
      title: '通知设置',
      table: {
        title: '通知渠道',
        subtitle: '配置生效结果、告警、证书到期和安全事件通知',
        headers: ['渠道名称', '类型', '接收范围', '状态'],
        rows: [
          ['Slack 生产群', 'Slack', '生效 / 告警', '启用'],
          ['邮件告警组', '邮件', '证书 / 安全', '启用'],
          ['Webhook 运维平台', 'Webhook', '全部事件', '待校验'],
          ['短信高危告警', '短信', '严重告警', '启用'],
        ],
      },
      toggleGroups: [
        {
          title: '事件订阅',
          items: [
            { label: '生效成功', enabled: true },
            { label: '生效失败', enabled: true },
            { label: '网关异常', enabled: true },
            { label: '服务异常', enabled: true },
            { label: '证书到期', enabled: true },
            { label: '安全事件', enabled: true },
          ],
        },
      ],
    },
    'global-security': {
      key: 'global-security',
      title: '全局安全',
      toggleGroups: [
        {
          title: '访问安全',
          subtitle: '配置控制台访问安全、密钥策略和敏感操作保护',
          items: [
            { label: 'MFA 强制开启', enabled: true },
            { label: '登录失败锁定', enabled: true },
            { label: '会话超时 30 分钟', enabled: true },
            { label: '敏感字段脱敏', enabled: true },
          ],
        },
      ],
      keyValues: [
        { label: '安全评分', value: '92', status: 'healthy' },
        { label: '高风险操作', value: '2' },
        { label: '待处理风险', value: '3' },
        { label: '最近事件', value: '密码策略更新' },
      ],
    },
    'system-params': {
      key: 'system-params',
      title: '系统参数',
      table: {
        title: '系统参数',
        subtitle: '管理控制台默认值、生效策略和平台级开关',
        headers: ['参数项', '当前值', '默认值', '生效范围', '风险等级'],
        rows: [
          ['默认生效窗口', '09:00-18:00', '全天', '全局', '低'],
          ['变更审批', '开启', '开启', '全局', '低'],
          ['自动回滚', '开启', '关闭', '生效', '中'],
          ['路由冲突阻断', '开启', '开启', '生效', '低'],
          ['生效前校验', '严格模式', '标准模式', '生效', '低'],
        ],
      },
    },
  },
  inspector: {
    configurationDomain: [
      { label: '配置域', value: '当前 Ingate', status: 'healthy' },
      { label: '创建时间', value: '2024-01-10 09:30:21' },
      { label: '创建人', value: 'alex@ingate.io' },
      { label: '配置分发', value: '单一 Envoy 配置' },
    ],
    envoyHealth: [
      { label: 'envoy-1', status: 'healthy' },
      { label: 'envoy-2', status: 'healthy' },
      { label: 'envoy-3', status: 'warning' },
    ],
    securityBaseline: [
      { label: 'HTTPS 强制跳转', status: 'healthy' },
      { label: '最小 TLS 版本 1.2+', status: 'healthy' },
      { label: '敏感字段脱敏', status: 'healthy' },
      { label: '访问令牌有效期 <= 24h', status: 'healthy' },
    ],
  },
};

function clone<T>(data: T): T {
  return structuredClone(data);
}

function validateRoutePayload(payload: RouteMutationPayload): RouteValidationReport {
  const invalidHostnames = payload.hostnames.filter((hostname) => !isValidHostname(hostname));
  const rule = routeRule(payload);
  const policyValidationMessage = rule ? validateRoutePolicyRelationship(rule) : '';
  const targets = rule?.targets ?? [];
  const targetError = rule ? routeTargetValidationMessage(targets) : '请至少配置一条路由规则';
  const items: RouteValidationReport['items'] = [
    {
      label: '路由名称',
      status: payload.name.trim() ? 'healthy' : 'critical',
      message: payload.name.trim() ? payload.name.trim() : '请填写路由名称',
    },
    {
      label: '匹配规则',
      status: rule && rule.pathPrefix.startsWith('/') ? 'healthy' : 'critical',
      message: rule && rule.pathPrefix.startsWith('/') ? '路径格式正确' : '路径必须以 / 开头',
    },
    {
      label: '目标服务',
      status: targetError ? 'critical' : 'healthy',
      message: targetError || `已选择 ${targets.length} 个目标服务，总权重 ${routeTargetWeightSum(targets)}`,
    },
    {
      label: '网关',
      status: payload.gatewayIDs.length > 0 ? 'healthy' : 'critical',
      message: payload.gatewayIDs.length > 0 ? `生效于 ${payload.gatewayIDs.join('、')}` : '请选择生效网关',
    },
    {
      label: '匹配域名',
      status: invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: invalidHostnames.length > 0
          ? `域名格式不正确：${invalidHostnames.join('、')}`
          : payload.hostnames.length > 0
            ? `已配置 ${payload.hostnames.length} 个匹配域名`
            : '不限制 Host',
    },
    {
      label: '转发控制',
      status: policyValidationMessage ? 'critical' : routePolicyCount(rule) > 0 ? 'healthy' : 'warning',
      message: policyValidationMessage || (routePolicyCount(rule) > 0 ? `已配置 ${routePolicyCount(rule)} 项转发控制` : '未配置转发控制'),
    },
  ];
  const valid = items.every((item) => item.status !== 'critical');

  return {
    valid,
    summary: valid ? '服务端校验通过，配置可以保存。' : '服务端校验发现未完成项。',
    items,
  };
}

function validateRoutePolicyRelationship(rule: RouteMutationPayload['rules'][number]) {
  if (!rule.retry) {
    return '';
  }

  const totalTimeoutMillis = rule.timeout?.requestMillis ?? defaultRouteTimeoutMillis;
  const perTryTimeoutMillis = rule.retry.perTryTimeoutMillis;
  if (Number.isFinite(perTryTimeoutMillis) && Number.isFinite(totalTimeoutMillis) && perTryTimeoutMillis > totalTimeoutMillis) {
    return `单次尝试超时不能大于请求总超时 ${totalTimeoutMillis}ms`;
  }

  return '';
}

function routeRule(payload: RouteMutationPayload) {
  return payload.rules[0];
}

function routeTargetValidationMessage(targets: RouteTargetPayload[]) {
  if (targets.length === 0) {
    return '请选择目标服务';
  }

  const seenIDs = new Set<string>();
  for (const target of targets) {
    if (!target.upstreamID.trim()) {
      return '目标服务不能为空';
    }
    if (seenIDs.has(target.upstreamID)) {
      return '目标服务不能重复';
    }
    seenIDs.add(target.upstreamID);
    if (target.weight < 1 || target.weight > 100) {
      return '目标权重必须在 1-100 之间';
    }
  }
  return '';
}

function routePolicyCount(rule?: RouteMutationPayload['rules'][number]) {
  if (!rule) {
    return 0;
  }
  return [rule.requestHeaderModifier, rule.responseHeaderModifier, rule.timeout, rule.retry].filter(Boolean).length;
}

function routeTargetWeightSum(targets: RouteTargetPayload[]) {
  return targets.reduce((sum, target) => sum + target.weight, 0);
}

function routeAction(message: string, changeId?: string): RouteActionResult {
  return { message, changeId };
}

function routeActionSummary(payload: RouteMutationPayload) {
  if (payload.name.trim()) {
    return payload.name.trim();
  }
  const rule = routeRule(payload);
  if (!rule) {
    return payload.id ?? '未配置规则';
  }
  const methods = rule.methods.length > 0 ? rule.methods.join('、') : '全部方法';
  return `${methods} ${rule.pathPrefix}`;
}

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;

  if (!normalized.includes('.') || normalized.length > 253) {
    return false;
  }

  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}

function validateGatewayPayload(payload: GatewayMutationPayload): GatewayValidationReport {
  const hostnames = payload.hostBindings.map((binding) => binding.hostname ?? '').filter(Boolean);
  const invalidHostnames = hostnames.filter((hostname) => !isValidHostname(hostname));
  const ports = payload.listeners.map((listener) => String(listener.port)).filter(Boolean);
  const duplicatePorts = ports.filter((port, index) => ports.indexOf(port) !== index);
  const httpsWithoutCertificate = payload.listeners.filter((listener) => listener.protocol === 'HTTPS' && !listener.certificateId);
  const items: GatewayValidationReport['items'] = [
    {
      label: '网关名称',
      status: payload.name.trim() ? 'healthy' : 'critical',
      message: payload.name.trim() ? payload.name.trim() : '请输入网关名称',
    },
    {
      label: '监听器',
      status: payload.listeners.length > 0 && payload.listeners.every((listener) => listener.port > 0) && duplicatePorts.length === 0 ? 'healthy' : 'critical',
      message: duplicatePorts.length > 0
        ? `端口重复：${Array.from(new Set(duplicatePorts)).join('、')}`
        : payload.listeners.length > 0 && payload.listeners.every((listener) => listener.port > 0)
          ? payload.listeners.map((listener) => `${listener.protocol}:${listener.port}`).join(' / ')
          : '至少配置一个监听器，并填写端口',
    },
    {
      label: 'HTTPS 证书',
      status: httpsWithoutCertificate.length > 0 ? 'critical' : 'healthy',
      message: httpsWithoutCertificate.length > 0 ? 'HTTPS 监听器必须选择证书' : '证书配置满足要求',
    },
    {
      label: 'Host 策略',
      status: invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: invalidHostnames.length > 0
        ? `域名格式不正确：${invalidHostnames.join('、')}`
        : hostnames.length > 0
          ? `限制 ${hostnames.length} 个 Host`
          : '不限制 Host',
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '网关服务端校验通过。' : '网关服务端校验发现未完成项。',
    items,
  };
}

function gatewayAction(message: string, changeId?: string): GatewayMutationResult {
  return { message, changeId };
}

function validateServicePayload(payload: ServiceMutationPayload): ServiceValidationReport {
  const endpointErrors = validateServiceEndpoints(payload.endpoints);
  const enabledEndpointCount = payload.endpoints.filter((endpoint) => endpoint.enabled).length;
  const healthInterval = payload.healthCheck?.intervalSeconds ?? 0;
  const healthTimeout = payload.healthCheck?.timeoutSeconds ?? 0;
  const items: ServiceValidationReport['items'] = [
    {
      label: '服务名称',
      status: payload.name.trim() ? 'healthy' : 'critical',
      message: payload.name.trim() ? payload.name.trim() : '请输入服务名称',
    },
    {
      label: '服务端点',
      status: endpointErrors.length === 0 && enabledEndpointCount > 0 ? 'healthy' : 'critical',
      message: endpointErrors[0] ?? (enabledEndpointCount > 0 ? `已配置 ${payload.endpoints.length} 个端点` : '至少保留一个启用端点'),
    },
    {
      label: '负载均衡',
      status: payload.loadBalancePolicy ? 'healthy' : 'critical',
      message: payload.loadBalancePolicy ? serviceLoadBalancePolicyLabel(payload.loadBalancePolicy) : '请选择负载均衡方式',
    },
    {
      label: '健康检查',
      status: validateServiceHealth(payload, healthInterval, healthTimeout),
      message: serviceHealthMessage(payload, healthInterval, healthTimeout),
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '服务服务端校验通过。' : '服务服务端校验发现未完成项。',
    items,
  };
}

function validateServiceEndpoints(endpoints: ServiceMutationPayload['endpoints']) {
  if (endpoints.length === 0) {
    return ['至少配置一个服务端点'];
  }

  return endpoints.flatMap((endpoint, index) => {
    const messages: string[] = [];
    const port = Number(endpoint.port);
    const weight = Number(endpoint.weight);

    if (!endpoint.address.trim()) {
      messages.push(`第 ${index + 1} 个端点缺少地址`);
    }

    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      messages.push(`第 ${index + 1} 个端点端口不合法`);
    }

    if (!Number.isInteger(weight) || weight < 1 || weight > 100) {
      messages.push(`第 ${index + 1} 个端点权重需要在 1-100 之间`);
    }

    return messages;
  });
}

function validateServiceHealth(payload: ServiceMutationPayload, interval: number, timeout: number) {
  if (!payload.healthCheck?.enabled) {
    return 'healthy';
  }

  if (!payload.healthCheck.path?.startsWith('/')) {
    return 'critical';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return 'critical';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return 'critical';
  }

  return 'healthy';
}

function serviceHealthMessage(payload: ServiceMutationPayload, interval: number, timeout: number) {
  if (!payload.healthCheck?.enabled) {
    return '未启用健康检查';
  }

  if (!payload.healthCheck.path?.startsWith('/')) {
    return '探活路径必须以 / 开头';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return '检查间隔需要在 1-300 秒之间';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return '超时时间需要在 1-60 秒之间，并且小于检查间隔';
  }

  return `${payload.healthCheck.path} / ${interval}s / ${timeout}s`;
}

function serviceAction(message: string, changeId?: string): ServiceMutationResult {
  return { message, changeId };
}

function getPolicyWorkspace(): PolicyWorkspace {
  const policies: GovernancePolicy[] = [
    ...rateLimitPolicies.map((policy) => ({
      id: policy.id,
      version: policy.version,
      kind: 'RateLimitPolicy' as const,
      name: policy.name,
      description: policy.description,
      enabled: policy.enabled,
      mode: policy.mode === 'Global' ? 'Global / 系统 Redis' : 'Local',
      ruleCount: policy.rules.length,
      createdAt: policy.createdAt,
      raw: policy,
    })),
    ...accessControlPolicies.map((policy) => ({
      id: policy.id,
      version: policy.version,
      kind: 'AccessControlPolicy' as const,
      name: policy.name,
      description: policy.description,
      enabled: policy.enabled,
      mode: policy.defaultAction === 'Deny' ? '默认拒绝' : '默认放行',
      ruleCount: policy.rules?.length ?? 0,
      createdAt: policy.createdAt,
      raw: policy,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name));

  return {
    policies,
    rateLimitPolicies,
    accessControlPolicies,
    bindings: policyBindings,
    targets: policyTargets(),
  };
}

function policyTargets(): PolicyTargetOption[] {
  return [
    ...gatewayList.gateways.map((gateway) => ({
      id: gateway.id,
      name: gateway.name,
      kind: 'Gateway' as const,
    })),
    ...routeWorkspace.routes.map((route) => ({
      id: route.id,
      name: route.name,
      kind: 'Route' as const,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name));
}

function saveRateLimitPolicy(payload: RateLimitPolicyPayload): PolicyMutationResult {
  const id = payload.id ?? crypto.randomUUID();
  const index = rateLimitPolicies.findIndex((policy) => policy.id === id);
  const nextPolicy: RateLimitPolicy = { ...payload, id, createdAt: index >= 0 ? rateLimitPolicies[index].createdAt : new Date().toISOString() };
  if (index >= 0) {
    rateLimitPolicies[index] = nextPolicy;
  } else {
    rateLimitPolicies.push(nextPolicy);
  }
  return { message: `限流策略已保存：${payload.name}`, changeId: id };
}

function saveAccessControlPolicy(payload: AccessControlPolicyPayload): PolicyMutationResult {
  const id = payload.id ?? crypto.randomUUID();
  const index = accessControlPolicies.findIndex((policy) => policy.id === id);
  const nextPolicy: AccessControlPolicy = { ...payload, id, createdAt: index >= 0 ? accessControlPolicies[index].createdAt : new Date().toISOString() };
  if (index >= 0) {
    accessControlPolicies[index] = nextPolicy;
  } else {
    accessControlPolicies.push(nextPolicy);
  }
  return { message: `访问控制策略已保存：${payload.name}`, changeId: id };
}

function savePolicyBinding(payload: PolicyBindingPayload): PolicyMutationResult {
  const id = payload.id ?? crypto.randomUUID();
  const index = policyBindings.findIndex((binding) => binding.id === id);
  const nextBinding: PolicyBinding = { ...payload, id, createdAt: index >= 0 ? policyBindings[index].createdAt : new Date().toISOString() };
  if (index >= 0) {
    policyBindings[index] = nextBinding;
  } else {
    policyBindings.push(nextBinding);
  }
  return { message: `策略绑定已保存：${payload.name}`, changeId: id };
}

function deletePolicyResource(kind: 'RateLimitPolicy' | 'AccessControlPolicy' | 'PolicyBinding', id: string): PolicyMutationResult {
  const resources = kind === 'RateLimitPolicy' ? rateLimitPolicies : kind === 'AccessControlPolicy' ? accessControlPolicies : policyBindings;
  const index = resources.findIndex((resource) => resource.id === id);
  if (index >= 0) {
    resources.splice(index, 1);
  }
  return { message: `已删除：${id}` };
}

function setPolicyEnabled(kind: 'RateLimitPolicy' | 'AccessControlPolicy' | 'PolicyBinding', id: string, enabled: boolean): PolicyMutationResult {
  const resources = kind === 'RateLimitPolicy' ? rateLimitPolicies : kind === 'AccessControlPolicy' ? accessControlPolicies : policyBindings;
  const resource = resources.find((item) => item.id === id);
  if (resource) {
    resource.enabled = enabled;
  }
  return { message: `已${enabled ? '启用' : '停用'}：${id}` };
}

export const mockConsoleRepository: ConsoleRepository = {
  async getHomeDashboard() {
    return clone(homeDashboard);
  },
  async listGateways() {
    return clone(gatewayList);
  },
  async saveGatewayDraft(payload) {
    return gatewayAction(`网关草稿已保存：${payload.name}`, payload.id ?? crypto.randomUUID());
  },
  async deleteGateway(id) {
    return gatewayAction(`网关已删除：${id}`);
  },
  async setGatewayEnabled(id, enabled) {
    return gatewayAction(`网关已${enabled ? '启用' : '停用'}：${id}`);
  },
  async validateGatewayDraft(payload) {
    return validateGatewayPayload(payload);
  },
  async getRouteWorkspace() {
    return clone(routeWorkspace);
  },
  async saveRouteDraft(payload) {
    return routeAction(`路由已保存：${routeActionSummary(payload)}`);
  },
  async deleteRoute(id) {
    return routeAction(`路由已删除：${id}`);
  },
  async setRouteEnabled(id, enabled) {
    return routeAction(`路由已${enabled ? '启用' : '停用'}：${id}`);
  },
  async validateRouteDraft(payload) {
    return validateRoutePayload(payload);
  },
  async listServices() {
    return clone(serviceList);
  },
  async saveServiceDraft(payload) {
    return serviceAction(`服务草稿已保存：${payload.name}`);
  },
  async deleteService(id) {
    return serviceAction(`服务已删除：${id}`);
  },
  async validateServiceDraft(payload) {
    return validateServicePayload(payload);
  },
  async getRuntimeStatus() {
    return clone(runtimeStatus);
  },
  async getPolicyWorkspace() {
    return clone(getPolicyWorkspace());
  },
  async saveRateLimitPolicy(payload) {
    return saveRateLimitPolicy(payload);
  },
  async saveAccessControlPolicy(payload) {
    return saveAccessControlPolicy(payload);
  },
  async savePolicyBinding(payload) {
    return savePolicyBinding(payload);
  },
  async deleteRateLimitPolicy(id) {
    return deletePolicyResource('RateLimitPolicy', id);
  },
  async deleteAccessControlPolicy(id) {
    return deletePolicyResource('AccessControlPolicy', id);
  },
  async deletePolicyBinding(id) {
    return deletePolicyResource('PolicyBinding', id);
  },
  async setRateLimitPolicyEnabled(id, enabled) {
    return setPolicyEnabled('RateLimitPolicy', id, enabled);
  },
  async setAccessControlPolicyEnabled(id, enabled) {
    return setPolicyEnabled('AccessControlPolicy', id, enabled);
  },
  async setPolicyBindingEnabled(id, enabled) {
    return setPolicyEnabled('PolicyBinding', id, enabled);
  },
  async listPlugins() {
    return clone(pluginList);
  },
  async getObservabilityOverview() {
    return clone(observabilityOverview);
  },
  async getSettingsWorkspace() {
    return clone(settingsWorkspace);
  },
};
