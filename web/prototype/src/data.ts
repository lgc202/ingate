export type TrafficType = "API" | "AI" | "MCP";
export type ServiceType = "HTTP" | "MODEL" | "MCP";
export type HealthState =
  | "healthy"
  | "warning"
  | "error"
  | "disabled"
  | "pending"
  | "unverified";
export type ConfigState = "active" | "publishing" | "failed" | "not-applied";
export type ReleaseState = "发布中" | "已生效" | "发布失败";

export interface ReleaseRecord {
  version: number;
  time: string;
  summary: string;
  resources: string;
  state: ReleaseState;
  syncedInstances: number;
  totalInstances: number;
  changes: string[];
  error?: string;
}

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
    | "流量策略"
    | "配置发布";
  resource: string;
  detail: string;
  result: "成功" | "失败";
}

export interface ProxyInstance {
  id: string;
  address: string;
  zone: string;
  version: string;
  activeConfigVersion: number;
  state: HealthState;
  lastSeen: string;
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
  accessMode: "需要调用方密钥" | "JWT / OIDC" | "公开访问";
  match: string;
  published: string[];
  targets: RouteTarget[];
  forwarding: RouteForwarding;
  conditions?: RouteMatchCondition[];
  rewrite?: RouteRewrite;
  jwt?: { issuer: string; audience: string; jwksURI: string };
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
    { input: number; output: number; unit: "每百万 Token"; updatedAt: string }
  >;
  capabilityChanges?: {
    added: string[];
    removed: string[];
    detectedAt: string;
    reviewed: boolean;
  };
  resilience?: { consecutiveFailures: number; ejectionTime: string };
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
}

export interface Caller {
  id: string;
  name: string;
  slug: string;
  owner: string;
  purpose: string;
  keys: CallerAccessKey[];
  permissions: CallerPermission[];
  metrics: CallerMetric[];
  quotas: CallerQuota[];
  state: HealthState;
  lastActive: string;
}

export interface Policy {
  id: string;
  name: string;
  type: "请求限流" | "IP 访问限制" | "AI 参数约束";
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
    maxTokens?: string;
    maxTemperature?: string;
  };
}

export interface RequestRecord {
  id: string;
  time: string;
  type: TrafficType;
  caller: string;
  route: string;
  request: string;
  target: string;
  result: "成功" | "主备切换" | "策略拒绝" | "失败";
  code: string;
  latency: string;
  usage: string;
  cost: string;
  steps: Array<{
    name: string;
    detail: string;
    duration: string;
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

export const initialProxyInstances: ProxyInstance[] = [
  {
    id: "proxy-a1",
    address: "10.8.1.21:19000",
    zone: "可用区 A",
    version: "1.34.1-ingate.3",
    activeConfigVersion: 142,
    state: "healthy",
    lastSeen: "4 秒前",
  },
  {
    id: "proxy-a2",
    address: "10.8.1.22:19000",
    zone: "可用区 A",
    version: "1.34.1-ingate.3",
    activeConfigVersion: 142,
    state: "healthy",
    lastSeen: "5 秒前",
  },
  {
    id: "proxy-b1",
    address: "10.8.2.21:19000",
    zone: "可用区 B",
    version: "1.34.1-ingate.3",
    activeConfigVersion: 142,
    state: "healthy",
    lastSeen: "3 秒前",
  },
  {
    id: "proxy-b2",
    address: "10.8.2.22:19000",
    zone: "可用区 B",
    version: "1.34.1-ingate.3",
    activeConfigVersion: 142,
    state: "healthy",
    lastSeen: "6 秒前",
  },
  {
    id: "proxy-c1",
    address: "10.8.3.21:19000",
    zone: "可用区 C",
    version: "1.34.1-ingate.3",
    activeConfigVersion: 142,
    state: "healthy",
    lastSeen: "4 秒前",
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
        output: 60,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
      "qwen-plus": {
        input: 4,
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
        output: 105,
        unit: "每百万 Token",
        updatedAt: "2026-08-01",
      },
    },
    resilience: { consecutiveFailures: 5, ejectionTime: "30 秒" },
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
    capabilityChanges: {
      added: ["extract_text"],
      removed: [],
      detectedAt: "今天 10:42",
      reviewed: false,
    },
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
    accessMode: "需要调用方密钥",
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
    accessMode: "需要调用方密钥",
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
    accessMode: "需要调用方密钥",
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
    accessMode: "需要调用方密钥",
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
    accessMode: "需要调用方密钥",
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
    accessMode: "需要调用方密钥",
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
    name: "电商 Web",
    slug: "commerce-web",
    owner: "电商应用团队",
    purpose: "面向消费者的订单和客户资料访问",
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
  {
    id: "policy-params",
    name: "AI 参数基线",
    type: "AI 参数约束",
    targets: [{ kind: "路由", id: "route-ai-prod", name: "生产 AI 路由" }],
    rule: "max_tokens ≤ 8,192，temperature ≤ 1.2",
    effect: "今日拒绝 126 次",
    state: "healthy",
    settings: { maxTokens: "8192", maxTemperature: "1.2" },
  },
];

export const initialRequests: RequestRecord[] = [
  {
    id: "req_4Ft7Xs",
    time: "14:32:31",
    type: "API",
    caller: "电商 Web",
    route: "订单查询 API",
    request: "GET /api/orders/78421",
    target: "订单服务",
    result: "成功",
    code: "200",
    latency: "86 ms",
    usage: "12.4 KB",
    cost: "—",
    steps: [
      {
        name: "调用方认证",
        detail: "电商 Web · 生产密钥",
        duration: "2 ms",
        state: "healthy",
      },
      {
        name: "访问策略",
        detail: "权限、请求限流和 IP 限制通过",
        duration: "4 ms",
        state: "healthy",
      },
      {
        name: "订单查询 API",
        detail: "GET /api/orders/78421 → 订单服务",
        duration: "1 ms",
        state: "healthy",
      },
      {
        name: "订单服务",
        detail: "HTTP 200 · 12.4 KB",
        duration: "79 ms",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_7Jv1Kq",
    time: "14:32:18",
    type: "AI",
    caller: "客服助手",
    route: "生产 AI 路由",
    request: "qwen-max · chat/completions",
    target: "通义千问生产 / qwen-max",
    result: "成功",
    code: "200",
    latency: "TTFT 612 ms · 总计 1.8 s",
    usage: "1,842 Token",
    cost: "¥0.08",
    steps: [
      {
        name: "调用方认证",
        detail: "客服助手 · 生产密钥",
        duration: "2 ms",
        state: "healthy",
      },
      {
        name: "访问策略",
        detail: "模型权限、Token 用量上限和参数约束通过",
        duration: "12 ms",
        state: "healthy",
      },
      {
        name: "生产 AI 路由",
        detail: "qwen-max → 通义千问生产 / qwen-max",
        duration: "1 ms",
        state: "healthy",
      },
      {
        name: "通义千问生产",
        detail: "输入 1,246 · 输出 596 · 缓存 312 Token",
        duration: "1.79 s",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_2Pq9Lm",
    time: "14:31:56",
    type: "AI",
    caller: "研发知识库",
    route: "生产 AI 路由",
    request: "claude-sonnet · chat/completions",
    target: "Bedrock 灾备 / claude-sonnet-4",
    result: "主备切换",
    code: "200",
    latency: "TTFT 1.9 s · 总计 3.4 s",
    usage: "3,106 Token",
    cost: "¥0.32",
    steps: [
      {
        name: "调用方认证",
        detail: "研发知识库 · 生产密钥",
        duration: "2 ms",
        state: "healthy",
      },
      {
        name: "访问策略",
        detail: "模型权限、Token 用量上限和参数约束通过",
        duration: "11 ms",
        state: "healthy",
      },
      {
        name: "Anthropic 公网",
        detail: "主服务连接超时，准备切换",
        duration: "2.0 s",
        state: "warning",
      },
      {
        name: "Bedrock 灾备",
        detail: "备用服务成功响应",
        duration: "1.4 s",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_9Ab4Xe",
    time: "14:31:42",
    type: "AI",
    caller: "内部自动化",
    route: "生产 AI 路由",
    request: "qwen-max · chat/completions",
    target: "—",
    result: "策略拒绝",
    code: "429",
    latency: "18 ms",
    usage: "—",
    cost: "—",
    steps: [
      {
        name: "调用方认证",
        detail: "内部自动化 · 生产密钥",
        duration: "2 ms",
        state: "healthy",
      },
      {
        name: "用量控制",
        detail: "月度 20M Token 已全部使用",
        duration: "5 ms",
        state: "error",
      },
    ],
  },
  {
    id: "req_3Mc8Ua",
    time: "14:31:19",
    type: "MCP",
    caller: "客服助手",
    route: "研究工具 MCP",
    request: "tools/call · web_search",
    target: "搜索工具服务 / web_search",
    result: "成功",
    code: "200",
    latency: "238 ms",
    usage: "1 次工具调用",
    cost: "—",
    steps: [
      {
        name: "调用方认证",
        detail: "客服助手 · 生产密钥",
        duration: "2 ms",
        state: "healthy",
      },
      {
        name: "工具权限",
        detail: "允许调用 web_search",
        duration: "3 ms",
        state: "healthy",
      },
      {
        name: "研究工具 MCP",
        detail: "tools/call → 搜索工具服务",
        duration: "1 ms",
        state: "healthy",
      },
      {
        name: "web_search",
        detail: "返回 8 条搜索结果",
        duration: "232 ms",
        state: "healthy",
      },
    ],
  },
  {
    id: "req_6Ce2Np",
    time: "14:30:07",
    type: "API",
    caller: "电商 Web",
    route: "文件上传 API",
    request: "POST /api/files",
    target: "文件服务",
    result: "失败",
    code: "502",
    latency: "1.2 s",
    usage: "0 KB",
    cost: "—",
    steps: [
      {
        name: "调用方认证",
        detail: "电商 Web · 生产密钥",
        duration: "2 ms",
        state: "healthy",
      },
      {
        name: "文件上传 API",
        detail: "POST /api/files → 文件服务",
        duration: "1 ms",
        state: "healthy",
      },
      {
        name: "文件服务",
        detail: "上游连接超时",
        duration: "1.19 s",
        state: "error",
      },
    ],
  },
];

export const initialReleaseHistory: ReleaseRecord[] = [
  {
    version: 142,
    time: "今天 14:31:08",
    summary: "更新生产 AI 路由的 Claude 备用线路",
    resources: "2 项资源",
    state: "已生效",
    syncedInstances: 5,
    totalInstances: 5,
    changes: [
      "生产 AI 路由：新增 Bedrock 灾备线路",
      "Anthropic 公网：更新故障切换条件",
    ],
  },
  {
    version: 141,
    time: "今天 11:04:09",
    summary: "轮换 example.com 泛域名证书",
    resources: "1 项资源",
    state: "已生效",
    syncedInstances: 5,
    totalInstances: 5,
    changes: ["example.com 泛域名证书：替换证书链和私钥"],
  },
  {
    version: 140,
    time: "今天 09:42:31",
    summary: "更新内部自动化用量上限",
    resources: "1 项资源",
    state: "发布失败",
    syncedInstances: 3,
    totalInstances: 5,
    changes: ["内部自动化：Token 月度上限 10M → 20M"],
    error: "3 个代理实例已应用 v140，2 个实例在 30 秒内未确认",
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
    id: "audit-release-141",
    time: "11:04:09",
    actor: "系统",
    action: "发布配置",
    resourceType: "配置发布",
    resource: "版本 141",
    detail: "证书变更已同步到全部网关实例",
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
