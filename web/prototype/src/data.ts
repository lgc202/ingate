export type TrafficType = "API" | "AI" | "MCP";
export type ServiceType = "HTTP" | "MODEL" | "MCP";
export type HealthState =
  | "healthy"
  | "warning"
  | "error"
  | "disabled"
  | "pending"
  | "unverified";
export type ConfigState = "active" | "failed" | "not-applied";
export type RouteAccessMode =
  | "公开访问"
  | "API Key"
  | "JWT 访问令牌"
  | "浏览器登录"
  | "客户端证书";

export interface AuditRecord {
  id: string;
  time: string;
  actor: string;
  action: string;
  resourceType:
    | "网关"
    | "路由"
    | "服务"
    | "证书"
    | "调用方"
    | "流量策略";
  resource: string;
  detail: string;
  result: "成功" | "失败";
}

export interface GatewayListenerBinding {
  domain: string;
  certificateID?: string;
}

export interface GatewayListener {
  id: string;
  protocol: "HTTP" | "HTTPS";
  port: number;
  bindings: GatewayListenerBinding[];
}

export interface Gateway {
  id: string;
  name: string;
  listeners: GatewayListener[];
  state: HealthState;
  configState?: ConfigState;
}

export interface RouteTarget {
  serviceID: string;
  serviceName: string;
  publishedCapability?: string;
  detail: string;
  role: "主线路" | "备用线路" | "加权线路";
  weight?: number;
}

export interface RouteForwarding {
  strategy: "单线路" | "主备切换" | "权重分流";
  timeout: string;
  retries: number;
  pathHandling: string;
  hostRewrite: string;
  failoverOn?: string[];
  circuitBreaker?: {
    consecutiveFailures: number;
    ejectionTime: string;
  };
}

export interface RouteMatchCondition {
  kind: "Header" | "Query";
  name: string;
  value: string;
  mode: "精确匹配" | "存在";
}

export interface RouteRewrite {
  pathPrefix?: string;
  requestHeaders: Array<{ name: string; value: string }>;
  removeHeaders: string[];
}

export interface GatewayRoute {
  id: string;
  name: string;
  type: TrafficType;
  gatewayID: string;
  gatewayName: string;
  host: string;
  path: string;
  accessMode: RouteAccessMode;
  identitySourceID?: string;
  match: string;
  published: string[];
  targets: RouteTarget[];
  forwarding: RouteForwarding;
  conditions?: RouteMatchCondition[];
  rewrite?: RouteRewrite;
  requests: string;
  successRate: string;
  latency: string;
  state: HealthState;
  configState?: ConfigState;
}

export interface ServiceEndpoint {
  address: string;
  weight: number;
  state: HealthState;
}

export interface Service {
  id: string;
  name: string;
  type: ServiceType;
  protocol: string;
  endpoints: ServiceEndpoint[];
  loadBalancing: "轮询" | "最少请求" | "随机";
  healthCheck: string;
  authentication: string;
  credentialUpdatedAt?: string;
  clientCertificateID?: string;
  transportSecurity: "明文连接" | "TLS";
  serverName?: string;
  trustCertificateID?: string;
  provider: string;
  capabilities: string[];
  modelPrices?: Record<
    string,
    {
      input: number;
      cachedInput?: number;
      output: number;
      unit: "每百万 Token";
      updatedAt: string;
    }
  >;
  successRate: string;
  latency: string;
  state: HealthState;
  configState?: ConfigState;
  verificationState?: "verified" | "unverified" | "failed";
}

export interface Certificate {
  id: string;
  name: string;
  identities: string[];
  issuer: string;
  usage: "服务器证书" | "客户端证书" | "信任证书";
  expiresAt: string;
  remainingDays: number;
  sourceName?: string;
  state: HealthState;
  configState?: ConfigState;
}

export interface CallerPermission {
  routeID: string;
  scopes: string[];
}

export interface CallerMetric {
  label: string;
  value: string;
  note: string;
}

export interface CallerQuota {
  routeID: string;
  used: number;
  limit: number;
  period: "每日" | "每月";
}

export interface CallerAccessKey {
  id: string;
  name: string;
  prefix: string;
  createdAt: string;
  expiresAt: string;
  lastUsed: string;
  state: HealthState;
  graceUntil?: string;
  rotatedFromID?: string;
  replacedByID?: string;
}

export interface Caller {
  id: string;
  name: string;
  slug: string;
  owner: string;
  purpose: string;
  enabled: boolean;
  keys: CallerAccessKey[];
  permissions: CallerPermission[];
  metrics: CallerMetric[];
  quotas: CallerQuota[];
  state: HealthState;
  lastActive: string;
}

export interface IdentitySource {
  id: string;
  name: string;
  provider: string;
  discoveryURL: string;
  audiences: string[];
  state: HealthState;
  lastVerified: string;
}

export interface Policy {
  id: string;
  name: string;
  type: "请求限流" | "IP 访问限制";
  targets: Array<{ kind: "网关" | "路由"; id: string; name: string }>;
  rule: string;
  effect: string;
  state: HealthState;
  configState?: ConfigState;
  settings?: {
    rateLimit?: string;
    ratePeriod?: string;
    rateDimension?: string;
    ipMode?: string;
    ipRanges?: string;
  };
}

export interface RequestRecord {
  id: string;
  time: string;
  type: TrafficType;
  method: string;
  host: string;
  path: string;
  query?: string;
  detail?: string;
  clientIP: string;
  gatewayID: string;
  gateway: string;
  routeID: string;
  caller: string;
  callerID?: string;
  route: string;
  target: string;
  result: "成功" | "主备切换" | "策略拒绝" | "失败";
  code: string;
  latency: string;
  usage: string;
  cost: string;
  attempts: Array<{
    serviceID: string;
    service: string;
    endpoint: string;
    provider?: string;
    actualModel?: string;
    result: "成功" | "失败";
    code: string;
    latency: string;
    ttft?: string;
    inputTokens?: number;
    outputTokens?: number;
    cachedTokens?: number;
    cost?: string;
    error?: string;
    state: HealthState;
  }>;
  decisions: Array<{
    name: string;
    detail: string;
    state: HealthState;
  }>;
}

export const initialGateways: Gateway[] = [
  {
    id: "gw-prod",
    name: "生产网关",
    listeners: [
      {
        id: "listener-prod-https",
        protocol: "HTTPS",
        port: 443,
        bindings: [
          { domain: "api.example.com", certificateID: "cert-wildcard" },
          { domain: "mcp.example.com", certificateID: "cert-wildcard" },
        ],
      },
      {
        id: "listener-prod-http",
        protocol: "HTTP",
        port: 80,
        bindings: [
          { domain: "api.example.com" },
          { domain: "mcp.example.com" },
        ],
      },
    ],
    state: "healthy",
  },
  {
    id: "gw-internal",
    name: "内部网关",
    listeners: [
      {
        id: "listener-internal-https",
        protocol: "HTTPS",
        port: 8443,
        bindings: [
          { domain: "inside.example.com", certificateID: "cert-internal" },
        ],
      },
    ],
    state: "healthy",
  },
];

export const initialServices: Service[] = [
  {
    id: "svc-orders",
    name: "订单服务",
    type: "HTTP",
    protocol: "HTTP/1.1",
    endpoints: [
      { address: "orders-01.internal:8080", weight: 50, state: "healthy" },
      { address: "orders-02.internal:8080", weight: 50, state: "healthy" },
    ],
    loadBalancing: "轮询",
    healthCheck: "HTTP GET /health · 10 秒",
    authentication: "无认证",
    transportSecurity: "明文连接",
    provider: "内部服务",
    capabilities: [],
    successRate: "99.96%",
    latency: "86 ms",
    state: "healthy",
    verificationState: "verified",
  },
  {
    id: "svc-customers",
    name: "客户中心",
    type: "HTTP",
    protocol: "HTTP/2",
    endpoints: [
      { address: "customers-01.internal:8080", weight: 34, state: "healthy" },
      { address: "customers-02.internal:8080", weight: 33, state: "healthy" },
      { address: "customers-03.internal:8080", weight: 33, state: "healthy" },
    ],
    loadBalancing: "最少请求",
    healthCheck: "HTTP GET /ready · 10 秒",
    authentication: "mTLS",
    clientCertificateID: "cert-upstream-client",
    transportSecurity: "TLS",
    serverName: "customers.internal",
    trustCertificateID: "cert-internal-ca",
    provider: "内部服务",
    capabilities: [],
    successRate: "99.91%",
    latency: "124 ms",
    state: "healthy",
    verificationState: "verified",
  },
  {
    id: "svc-files",
    name: "文件服务",
    type: "HTTP",
    protocol: "HTTP/1.1",
    endpoints: [
      { address: "files-01.internal:9000", weight: 50, state: "warning" },
      { address: "files-02.internal:9000", weight: 50, state: "healthy" },
    ],
    loadBalancing: "最少请求",
    healthCheck: "TCP 连接 · 5 秒",
    authentication: "无认证",
    transportSecurity: "明文连接",
    provider: "内部服务",
    capabilities: [],
    successRate: "99.42%",
    latency: "680 ms",
    state: "warning",
    verificationState: "verified",
  },
  {
    id: "svc-qwen",
    name: "通义千问生产",
    type: "MODEL",
    protocol: "OpenAI 兼容 API",
    endpoints: [
      { address: "dashscope.aliyuncs.com", weight: 100, state: "healthy" },
    ],
    loadBalancing: "轮询",
    healthCheck: "被动健康检查",
    authentication: "API Key",
    transportSecurity: "TLS",
    serverName: "dashscope.aliyuncs.com",
    provider: "阿里云百炼",
    capabilities: ["qwen-max", "qwen-plus", "text-embedding-v3"],
    modelPrices: {
      "qwen-max": {
        input: 20,
        cachedInput: 5,
        output: 60,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
      "qwen-plus": {
        input: 4,
        cachedInput: 1,
        output: 12,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
      "text-embedding-v3": {
        input: 0.7,
        output: 0,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
    },
    successRate: "99.98%",
    latency: "TTFT 620 ms",
    state: "healthy",
    verificationState: "verified",
  },
  {
    id: "svc-anthropic",
    name: "Anthropic 公网",
    type: "MODEL",
    protocol: "Anthropic Messages",
    endpoints: [
      { address: "api.anthropic.com", weight: 100, state: "warning" },
    ],
    loadBalancing: "轮询",
    healthCheck: "被动健康检查",
    authentication: "API Key",
    transportSecurity: "TLS",
    serverName: "api.anthropic.com",
    provider: "Anthropic",
    capabilities: ["claude-sonnet-4"],
    modelPrices: {
      "claude-sonnet-4": {
        input: 21,
        cachedInput: 2.1,
        output: 105,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
    },
    successRate: "99.72%",
    latency: "TTFT 2.8 s",
    state: "warning",
    verificationState: "verified",
  },
  {
    id: "svc-bedrock",
    name: "Bedrock 灾备",
    type: "MODEL",
    protocol: "AWS Bedrock",
    endpoints: [
      {
        address: "bedrock-runtime.us-east-1.amazonaws.com",
        weight: 100,
        state: "healthy",
      },
    ],
    loadBalancing: "轮询",
    healthCheck: "被动健康检查",
    authentication: "AWS 签名",
    transportSecurity: "TLS",
    serverName: "bedrock-runtime.us-east-1.amazonaws.com",
    provider: "AWS",
    capabilities: ["claude-sonnet-4"],
    modelPrices: {
      "claude-sonnet-4": {
        input: 21,
        cachedInput: 2.1,
        output: 105,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
    },
    successRate: "99.95%",
    latency: "TTFT 1.7 s",
    state: "healthy",
    verificationState: "verified",
  },
  {
    id: "svc-search-mcp",
    name: "搜索工具服务",
    type: "MCP",
    protocol: "Streamable HTTP",
    endpoints: [
      {
        address: "search-tools.internal:443/mcp",
        weight: 100,
        state: "healthy",
      },
    ],
    loadBalancing: "轮询",
    healthCheck: "HTTP GET /health · 10 秒",
    authentication: "Bearer Token",
    transportSecurity: "TLS",
    serverName: "search-tools.internal",
    trustCertificateID: "cert-internal-ca",
    provider: "内部 MCP",
    capabilities: ["web_search", "fetch_page", "extract_text"],
    successRate: "99.90%",
    latency: "238 ms",
    state: "healthy",
    verificationState: "verified",
  },
  {
    id: "svc-ticket-mcp",
    name: "工单工具服务",
    type: "MCP",
    protocol: "Streamable HTTP",
    endpoints: [
      {
        address: "ticket-tools.internal:443/mcp",
        weight: 100,
        state: "healthy",
      },
    ],
    loadBalancing: "轮询",
    healthCheck: "HTTP GET /health · 10 秒",
    authentication: "Bearer Token",
    transportSecurity: "TLS",
    serverName: "ticket-tools.internal",
    trustCertificateID: "cert-internal-ca",
    provider: "内部 MCP",
    capabilities: ["ticket_get", "ticket_create"],
    successRate: "99.84%",
    latency: "310 ms",
    state: "healthy",
    verificationState: "verified",
  },
];

export const initialRoutes: GatewayRoute[] = [
  {
    id: "route-orders",
    name: "订单查询 API",
    type: "API",
    gatewayID: "gw-prod",
    gatewayName: "生产网关",
    host: "api.example.com",
    path: "/api/orders",
    accessMode: "API Key",
    match: "GET /api/orders/*",
    published: ["GET /api/orders/{id}"],
    targets: [
      {
        serviceID: "svc-orders",
        serviceName: "订单服务",
        publishedCapability: "GET /api/orders/{id}",
        detail: "2 个端点",
        role: "主线路",
      },
    ],
    forwarding: {
      strategy: "单线路",
      timeout: "30 秒",
      retries: 2,
      pathHandling: "保持原路径",
      hostRewrite: "使用服务地址",
      circuitBreaker: { consecutiveFailures: 5, ejectionTime: "30 秒" },
    },
    conditions: [
      { kind: "Header", name: "x-api-version", value: "v2", mode: "精确匹配" },
    ],
    rewrite: {
      pathPrefix: "/orders",
      requestHeaders: [{ name: "x-ingate-route", value: "orders-v2" }],
      removeHeaders: [],
    },
    requests: "58.4K",
    successRate: "99.96%",
    latency: "P95 86 ms",
    state: "healthy",
  },
  {
    id: "route-customers",
    name: "客户资料 API",
    type: "API",
    gatewayID: "gw-prod",
    gatewayName: "生产网关",
    host: "api.example.com",
    path: "/api/customers",
    accessMode: "JWT 访问令牌",
    identitySourceID: "idp-enterprise",
    match: "GET /api/customers/*",
    published: ["GET /api/customers/{id}"],
    targets: [
      {
        serviceID: "svc-customers",
        serviceName: "客户中心",
        detail: "3 个端点",
        role: "主线路",
      },
    ],
    forwarding: {
      strategy: "单线路",
      timeout: "30 秒",
      retries: 2,
      pathHandling: "保持原路径",
      hostRewrite: "使用服务地址",
    },
    requests: "33.8K",
    successRate: "99.91%",
    latency: "P95 124 ms",
    state: "healthy",
  },
  {
    id: "route-files",
    name: "文件上传 API",
    type: "API",
    gatewayID: "gw-prod",
    gatewayName: "生产网关",
    host: "api.example.com",
    path: "/api/files",
    accessMode: "API Key",
    match: "POST /api/files",
    published: ["POST /api/files"],
    targets: [
      {
        serviceID: "svc-files",
        serviceName: "文件服务",
        detail: "2 个端点",
        role: "主线路",
      },
    ],
    forwarding: {
      strategy: "单线路",
      timeout: "60 秒",
      retries: 1,
      pathHandling: "保持原路径",
      hostRewrite: "使用服务地址",
    },
    requests: "6.1K",
    successRate: "99.42%",
    latency: "P95 680 ms",
    state: "warning",
  },
  {
    id: "route-ai-prod",
    name: "生产 AI 路由",
    type: "AI",
    gatewayID: "gw-prod",
    gatewayName: "生产网关",
    host: "api.example.com",
    path: "/v1",
    accessMode: "API Key",
    match: "OpenAI API · 请求体 model",
    published: ["qwen-max", "claude-sonnet"],
    targets: [
      {
        serviceID: "svc-qwen",
        serviceName: "通义千问生产",
        publishedCapability: "qwen-max",
        detail: "qwen-max",
        role: "主线路",
      },
      {
        serviceID: "svc-anthropic",
        serviceName: "Anthropic 公网",
        publishedCapability: "claude-sonnet",
        detail: "claude-sonnet-4",
        role: "主线路",
      },
      {
        serviceID: "svc-bedrock",
        serviceName: "Bedrock 灾备",
        publishedCapability: "claude-sonnet",
        detail: "claude-sonnet-4",
        role: "备用线路",
      },
    ],
    forwarding: {
      strategy: "主备切换",
      timeout: "120 秒",
      retries: 1,
      pathHandling: "保持原路径",
      hostRewrite: "使用服务地址",
      failoverOn: ["连接失败", "超时", "HTTP 429", "HTTP 5xx"],
    },
    requests: "28.4K",
    successRate: "99.72%",
    latency: "TTFT 612 ms",
    state: "warning",
  },
  {
    id: "route-ai-internal",
    name: "内部 AI 路由",
    type: "AI",
    gatewayID: "gw-internal",
    gatewayName: "内部网关",
    host: "inside.example.com",
    path: "/v1",
    accessMode: "API Key",
    match: "OpenAI API · embeddings",
    published: ["text-embedding"],
    targets: [
      {
        serviceID: "svc-qwen",
        serviceName: "通义千问生产",
        publishedCapability: "text-embedding",
        detail: "text-embedding-v3",
        role: "主线路",
      },
    ],
    forwarding: {
      strategy: "单线路",
      timeout: "60 秒",
      retries: 1,
      pathHandling: "保持原路径",
      hostRewrite: "使用服务地址",
    },
    requests: "16.0K",
    successRate: "99.99%",
    latency: "P95 112 ms",
    state: "healthy",
  },
  {
    id: "route-mcp-research",
    name: "研究工具 MCP",
    type: "MCP",
    gatewayID: "gw-prod",
    gatewayName: "生产网关",
    host: "mcp.example.com",
    path: "/research",
    accessMode: "API Key",
    match: "Streamable HTTP · tools/call",
    published: ["web_search", "fetch_page"],
    targets: [
      {
        serviceID: "svc-search-mcp",
        serviceName: "搜索工具服务",
        publishedCapability: "web_search、fetch_page",
        detail: "2 个开放工具",
        role: "主线路",
      },
    ],
    forwarding: {
      strategy: "单线路",
      timeout: "30 秒",
      retries: 1,
      pathHandling: "保持原路径",
      hostRewrite: "使用服务地址",
    },
    requests: "8.2K",
    successRate: "99.90%",
    latency: "P95 238 ms",
    state: "healthy",
  },
];

export const initialCertificates: Certificate[] = [
  {
    id: "cert-wildcard",
    name: "example.com 泛域名证书",
    identities: ["*.example.com", "example.com"],
    issuer: "Let's Encrypt",
    usage: "服务器证书",
    expiresAt: "2026-10-18",
    remainingDays: 67,
    state: "healthy",
  },
  {
    id: "cert-internal",
    name: "内部服务证书",
    identities: ["inside.example.com"],
    issuer: "企业内部 CA",
    usage: "服务器证书",
    expiresAt: "2027-05-20",
    remainingDays: 281,
    state: "healthy",
  },
  {
    id: "cert-upstream-client",
    name: "网关客户端证书",
    identities: ["ingate-client.internal"],
    issuer: "企业内部 CA",
    usage: "客户端证书",
    expiresAt: "2027-02-12",
    remainingDays: 184,
    state: "healthy",
  },
  {
    id: "cert-internal-ca",
    name: "内部服务信任链",
    identities: ["企业内部根 CA"],
    issuer: "企业内部根 CA",
    usage: "信任证书",
    expiresAt: "2032-01-01",
    remainingDays: 1968,
    state: "healthy",
  },
  {
    id: "cert-legacy",
    name: "旧版 API 证书",
    identities: ["legacy.example.com"],
    issuer: "DigiCert",
    usage: "服务器证书",
    expiresAt: "2026-08-29",
    remainingDays: 17,
    state: "warning",
  },
];

export const initialCallers: Caller[] = [
  {
    id: "caller-support",
    name: "客服助手",
    slug: "customer-support",
    owner: "客户体验团队",
    purpose: "客服工作台中的订单查询与 AI 辅助回复",
    enabled: true,
    keys: [
      {
        id: "key-support-prod",
        name: "客服生产环境",
        prefix: "ig_live_8a4f…",
        createdAt: "2026-05-13",
        expiresAt: "2026-11-09",
        lastUsed: "刚刚",
        state: "healthy",
      },
      {
        id: "key-support-test",
        name: "客服联调",
        prefix: "ig_test_71c2…",
        createdAt: "2026-06-01",
        expiresAt: "2026-08-30",
        lastUsed: "3 天前",
        state: "warning",
      },
    ],
    permissions: [
      { routeID: "route-orders", scopes: ["GET /api/orders/{id}"] },
      { routeID: "route-ai-prod", scopes: ["qwen-max"] },
      { routeID: "route-mcp-research", scopes: ["web_search", "fetch_page"] },
    ],
    metrics: [
      { label: "API 请求", value: "26.6K", note: "今天" },
      { label: "AI Token", value: "12.6M", note: "本月" },
      { label: "MCP 工具调用", value: "3.7K", note: "今天" },
    ],
    quotas: [
      {
        routeID: "route-ai-prod",
        used: 12600000,
        limit: 20000000,
        period: "每月",
      },
    ],
    state: "healthy",
    lastActive: "刚刚",
  },
  {
    id: "caller-rd",
    name: "研发知识库",
    slug: "rd-knowledge",
    owner: "研发效能团队",
    purpose: "内部知识检索和代码解释",
    enabled: true,
    keys: [
      {
        id: "key-rd-prod",
        name: "知识库生产环境",
        prefix: "ig_live_3bd9…",
        createdAt: "2026-04-18",
        expiresAt: "2027-04-18",
        lastUsed: "2 分钟前",
        state: "healthy",
      },
      {
        id: "key-rd-ci",
        name: "知识库构建任务",
        prefix: "ig_live_a182…",
        createdAt: "2026-07-06",
        expiresAt: "2026-10-04",
        lastUsed: "1 小时前",
        state: "healthy",
      },
      {
        id: "key-rd-old",
        name: "旧版联调密钥",
        prefix: "ig_test_90d1…",
        createdAt: "2026-01-12",
        expiresAt: "2026-07-12",
        lastUsed: "45 天前",
        state: "disabled",
      },
    ],
    permissions: [
      { routeID: "route-ai-prod", scopes: ["claude-sonnet"] },
      { routeID: "route-mcp-research", scopes: ["web_search"] },
    ],
    metrics: [
      { label: "AI 请求", value: "20.6K", note: "今天" },
      { label: "AI Token", value: "18.2M", note: "本月" },
      { label: "MCP 工具调用", value: "4.5K", note: "今天" },
    ],
    quotas: [
      {
        routeID: "route-ai-prod",
        used: 18200000,
        limit: 30000000,
        period: "每月",
      },
    ],
    state: "healthy",
    lastActive: "2 分钟前",
  },
  {
    id: "caller-automation",
    name: "内部自动化",
    slug: "platform-automation",
    owner: "平台工程团队",
    purpose: "自动化巡检与批量处理",
    enabled: true,
    keys: [
      {
        id: "key-auto-prod",
        name: "自动化生产环境",
        prefix: "ig_live_6fa0…",
        createdAt: "2026-02-20",
        expiresAt: "2027-02-20",
        lastUsed: "18 分钟前",
        state: "healthy",
      },
      {
        id: "key-auto-rotate",
        name: "待轮换密钥",
        prefix: "ig_live_c443…",
        createdAt: "2026-05-27",
        expiresAt: "2026-08-25",
        lastUsed: "昨天",
        state: "warning",
      },
    ],
    permissions: [{ routeID: "route-ai-prod", scopes: ["qwen-max"] }],
    metrics: [
      { label: "AI 请求", value: "12.1K", note: "今天" },
      { label: "AI Token", value: "20.0M", note: "本月" },
      { label: "用量拒绝", value: "124", note: "今天" },
    ],
    quotas: [
      {
        routeID: "route-ai-prod",
        used: 20000000,
        limit: 20000000,
        period: "每月",
      },
    ],
    state: "warning",
    lastActive: "18 分钟前",
  },
  {
    id: "caller-web",
    name: "电商 BFF",
    slug: "commerce-bff",
    owner: "电商应用团队",
    purpose: "代表商城前端访问订单和客户资料 API",
    enabled: true,
    keys: [
      {
        id: "key-web-prod",
        name: "商城生产环境",
        prefix: "ig_live_2e17…",
        createdAt: "2026-03-08",
        expiresAt: "2027-03-08",
        lastUsed: "刚刚",
        state: "healthy",
      },
      {
        id: "key-web-gray",
        name: "商城灰度环境",
        prefix: "ig_live_55ab…",
        createdAt: "2026-07-16",
        expiresAt: "2026-10-14",
        lastUsed: "12 分钟前",
        state: "healthy",
      },
    ],
    permissions: [
      { routeID: "route-orders", scopes: ["GET /api/orders/{id}"] },
      { routeID: "route-customers", scopes: ["GET /api/customers/{id}"] },
    ],
    metrics: [
      { label: "API 请求", value: "47.5K", note: "今天" },
      { label: "响应流量", value: "42.7 GB", note: "今天" },
      { label: "限流拒绝", value: "18", note: "今天" },
    ],
    quotas: [],
    state: "healthy",
    lastActive: "刚刚",
  },
];

export const initialIdentitySources: IdentitySource[] = [
  {
    id: "idp-enterprise",
    name: "企业统一身份",
    provider: "Microsoft Entra ID",
    discoveryURL:
      "https://login.microsoftonline.com/example/v2.0/.well-known/openid-configuration",
    audiences: ["api://ingate-production"],
    state: "healthy",
    lastVerified: "5 分钟前",
  },
  {
    id: "idp-partner",
    name: "合作方身份中心",
    provider: "Keycloak",
    discoveryURL:
      "https://identity.partner.example.com/realms/api/.well-known/openid-configuration",
    audiences: ["partner-api"],
    state: "healthy",
    lastVerified: "昨天",
  },
];

export const initialPolicies: Policy[] = [
  {
    id: "policy-api-rate",
    name: "外部 API 限流",
    type: "请求限流",
    targets: [
      { kind: "路由", id: "route-orders", name: "订单查询 API" },
      { kind: "路由", id: "route-customers", name: "客户资料 API" },
    ],
    rule: "每个调用方每分钟 1,000 次，各路由独立计数",
    effect: "今日拒绝 18 次",
    state: "healthy",
    settings: {
      rateLimit: "1000",
      ratePeriod: "分钟",
      rateDimension: "每个调用方",
    },
  },
  {
    id: "policy-ip",
    name: "办公网访问限制",
    type: "IP 访问限制",
    targets: [{ kind: "网关", id: "gw-prod", name: "生产网关" }],
    rule: "仅允许办公网与 VPN 出口",
    effect: "今日拒绝 42 次",
    state: "healthy",
    settings: {
      ipMode: "仅允许",
      ipRanges: "10.20.0.0/16\n10.21.0.0/16\n172.22.8.14/32",
    },
  },
];

export const initialRequests: RequestRecord[] = [
  {
    id: "req_4Ft7Xs",
    time: "14:32:31",
    type: "API",
    method: "GET",
    host: "api.example.com",
    path: "/api/orders/78421",
    query: "include=items",
    clientIP: "10.20.18.42",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-orders",
    caller: "电商 BFF",
    callerID: "caller-web",
    route: "订单查询 API",
    target: "订单服务",
    result: "成功",
    code: "200",
    latency: "86 ms",
    usage: "12.4 KB",
    cost: "—",
    attempts: [
      {
        serviceID: "svc-orders",
        service: "订单服务",
        endpoint: "10.24.8.17:8080",
        result: "成功",
        code: "200",
        latency: "79 ms",
        state: "healthy",
      },
    ],
    decisions: [
      {
        name: "调用方认证",
        detail: "电商 BFF · 生产密钥",
        state: "healthy",
      },
      {
        name: "访问策略",
        detail: "权限、请求限流和 IP 限制通过",
        state: "healthy",
      },
      {
        name: "选择服务",
        detail: "GET /api/orders/* 匹配成功，请求转发至订单服务",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_7Jv1Kq",
    time: "14:32:18",
    type: "AI",
    method: "POST",
    host: "api.example.com",
    path: "/v1/chat/completions",
    detail: "model: qwen-max",
    clientIP: "10.20.32.16",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-ai-prod",
    caller: "客服助手",
    callerID: "caller-support",
    route: "生产 AI 路由",
    target: "通义千问生产 / qwen-max",
    result: "成功",
    code: "200",
    latency: "TTFT 612 ms · 总计 1.8 s",
    usage: "1,842 Token",
    cost: "¥0.06",
    attempts: [
      {
        serviceID: "svc-qwen",
        service: "通义千问生产",
        endpoint: "dashscope.aliyuncs.com:443",
        provider: "阿里云百炼",
        actualModel: "qwen-max",
        result: "成功",
        code: "200",
        latency: "1.79 s",
        ttft: "612 ms",
        inputTokens: 1246,
        outputTokens: 596,
        cachedTokens: 312,
        cost: "¥0.06",
        state: "healthy",
      },
    ],
    decisions: [
      {
        name: "调用方认证",
        detail: "客服助手 · 生产密钥",
        state: "healthy",
      },
      {
        name: "访问策略",
        detail: "模型权限和 Token 用量上限通过",
        state: "healthy",
      },
      {
        name: "选择模型线路",
        detail: "qwen-max → 通义千问生产 / qwen-max",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_2Pq9Lm",
    time: "14:31:56",
    type: "AI",
    method: "POST",
    host: "api.example.com",
    path: "/v1/chat/completions",
    detail: "model: claude-sonnet",
    clientIP: "10.20.35.91",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-ai-prod",
    caller: "研发知识库",
    callerID: "caller-rd",
    route: "生产 AI 路由",
    target: "Bedrock 灾备 / claude-sonnet-4",
    result: "主备切换",
    code: "200",
    latency: "TTFT 1.9 s · 总计 3.4 s",
    usage: "3,106 Token",
    cost: "¥0.15",
    attempts: [
      {
        serviceID: "svc-anthropic",
        service: "Anthropic 公网",
        endpoint: "api.anthropic.com:443",
        provider: "Anthropic",
        actualModel: "claude-sonnet-4",
        result: "失败",
        code: "UPSTREAM_TIMEOUT",
        latency: "2.0 s",
        ttft: "—",
        error: "连接超时，路由已切换备用线路",
        state: "error",
      },
      {
        serviceID: "svc-bedrock",
        service: "Bedrock 灾备",
        endpoint: "bedrock-runtime.us-east-1.amazonaws.com:443",
        provider: "AWS",
        actualModel: "claude-sonnet-4",
        result: "成功",
        code: "200",
        latency: "1.4 s",
        ttft: "1.9 s",
        inputTokens: 2110,
        outputTokens: 996,
        cachedTokens: 0,
        cost: "¥0.15",
        state: "healthy",
      },
    ],
    decisions: [
      {
        name: "调用方认证",
        detail: "研发知识库 · 生产密钥",
        state: "healthy",
      },
      {
        name: "访问策略",
        detail: "模型权限和 Token 用量上限通过",
        state: "healthy",
      },
      {
        name: "选择模型线路",
        detail: "主线路超时后切换至 Bedrock 灾备",
        state: "warning",
      },
    ],
  },
  {
    id: "req_9Ab4Xe",
    time: "14:31:42",
    type: "AI",
    method: "POST",
    host: "api.example.com",
    path: "/v1/chat/completions",
    detail: "model: qwen-max",
    clientIP: "10.21.12.28",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-ai-prod",
    caller: "内部自动化",
    callerID: "caller-automation",
    route: "生产 AI 路由",
    target: "—",
    result: "策略拒绝",
    code: "429",
    latency: "18 ms",
    usage: "—",
    cost: "—",
    attempts: [],
    decisions: [
      {
        name: "调用方认证",
        detail: "内部自动化 · 生产密钥",
        state: "healthy",
      },
      {
        name: "用量控制",
        detail: "月度 20M Token 已全部使用",
        state: "error",
      },
    ],
  },
  {
    id: "req_3Mc8Ua",
    time: "14:31:19",
    type: "MCP",
    method: "POST",
    host: "mcp.example.com",
    path: "/research",
    detail: "tools/call · web_search",
    clientIP: "10.20.32.16",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-mcp-research",
    caller: "客服助手",
    callerID: "caller-support",
    route: "研究工具 MCP",
    target: "搜索工具服务 / web_search",
    result: "成功",
    code: "200",
    latency: "238 ms",
    usage: "1 次工具调用",
    cost: "—",
    attempts: [
      {
        serviceID: "svc-search-mcp",
        service: "搜索工具服务",
        endpoint: "10.24.22.15:3000",
        result: "成功",
        code: "200",
        latency: "232 ms",
        state: "healthy",
      },
    ],
    decisions: [
      {
        name: "调用方认证",
        detail: "客服助手 · 生产密钥",
        state: "healthy",
      },
      {
        name: "工具权限",
        detail: "允许调用 web_search",
        state: "healthy",
      },
      {
        name: "选择工具服务",
        detail: "tools/call → 搜索工具服务",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_6Ce2Np",
    time: "14:30:07",
    type: "API",
    method: "POST",
    host: "api.example.com",
    path: "/api/files",
    clientIP: "10.20.18.42",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-files",
    caller: "电商 BFF",
    callerID: "caller-web",
    route: "文件上传 API",
    target: "文件服务",
    result: "失败",
    code: "502",
    latency: "1.2 s",
    usage: "0 KB",
    cost: "—",
    attempts: [
      {
        serviceID: "svc-files",
        service: "文件服务",
        endpoint: "10.24.16.21:8080",
        result: "失败",
        code: "UPSTREAM_TIMEOUT",
        latency: "1.19 s",
        error: "上游连接超时",
        state: "error",
      },
    ],
    decisions: [
      {
        name: "调用方认证",
        detail: "电商 BFF · 生产密钥",
        state: "healthy",
      },
      {
        name: "选择服务",
        detail: "POST /api/files → 文件服务",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_8Dn5Rt",
    time: "14:29:44",
    type: "API",
    method: "GET",
    host: "api.example.com",
    path: "/api/orders/90142",
    clientIP: "203.0.113.18",
    gatewayID: "gw-prod",
    gateway: "生产网关",
    routeID: "route-orders",
    caller: "未知调用方",
    route: "订单查询 API",
    target: "—",
    result: "策略拒绝",
    code: "403",
    latency: "3 ms",
    usage: "—",
    cost: "—",
    attempts: [],
    decisions: [
      {
        name: "IP 访问限制",
        detail: "203.0.113.18 不在办公网与 VPN 允许范围内",
        state: "error",
      },
    ],
  },
];

export const initialAuditRecords: AuditRecord[] = [
  {
    id: "audit-route-142",
    time: "14:28:42",
    actor: "林工程师",
    action: "更新路由",
    resourceType: "路由",
    resource: "生产 AI 路由",
    detail: "claude-sonnet 备用线路由 Anthropic 灾备调整为 Bedrock 灾备",
    result: "成功",
  },
  {
    id: "audit-key-support",
    time: "13:56:17",
    actor: "王管理员",
    action: "签发密钥",
    resourceType: "调用方",
    resource: "客服助手",
    detail: "签发“客服生产环境”访问密钥，有效期 90 天",
    result: "成功",
  },
  {
    id: "audit-bedrock",
    time: "09:42:31",
    actor: "赵开发",
    action: "创建服务",
    resourceType: "服务",
    resource: "Bedrock 灾备",
    detail: "接入 AWS Bedrock，发现 claude-sonnet-4",
    result: "成功",
  },
  {
    id: "audit-policy-ip",
    time: "昨天 18:12",
    actor: "王管理员",
    action: "更新策略",
    resourceType: "流量策略",
    resource: "办公网访问限制",
    detail: "新增办公网出口地址 203.0.113.0/24",
    result: "成功",
  },
];
