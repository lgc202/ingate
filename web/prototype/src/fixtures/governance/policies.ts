import type { Policy } from "../../models/governance";

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
