import type { ConfigState, HealthState, TrafficType } from "./resource-state";

export type ServiceType = "HTTP" | "MODEL" | "MCP";
export type RouteAccessMode =
  | "公开访问"
  | "API Key"
  | "JWT 访问令牌"
  | "浏览器登录"
  | "客户端证书";

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
  hostRewrite: "使用服务地址" | "保持请求主机" | "自定义主机名";
  customHostname?: string;
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
