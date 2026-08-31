import type { HealthState, TrafficType } from "./resource-state";

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
