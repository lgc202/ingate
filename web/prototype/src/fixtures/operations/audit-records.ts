import type { AuditRecord } from "../../models/operations";

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
