import type {
  Policy,
  TrafficType,
} from "../../data";

export type PolicyGroup = "ALL" | "访问控制" | "流量控制";

export type TargetTrafficFilter = "ALL" | TrafficType;

export interface PolicyTargetCandidate {
  key: string;
  kind: "网关" | "路由";
  id: string;
  name: string;
  gatewayID?: string;
  gatewayName?: string;
  trafficType?: TrafficType;
  detail: string;
}

export const defaultPolicyNames: Record<Policy["type"], string> = {
  请求限流: "新请求限流策略",
  "IP 访问限制": "新 IP 访问限制",
};

export const targetPageSize = 10;

export function policyGroup(
  type: Policy["type"],
): Exclude<PolicyGroup, "ALL"> {
  if (type === "IP 访问限制") return "访问控制";
  return "流量控制";
}
