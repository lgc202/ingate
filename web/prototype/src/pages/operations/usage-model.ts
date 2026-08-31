export type UsageView = "QUOTA" | "TOKEN" | "COST";
export type UsageGroup = "CLIENT_MODEL" | "ACTUAL_MODEL" | "SERVICE";

export interface AIUsageFact {
  callerID: string;
  route: string;
  clientModel: string;
  actualModel: string;
  service: string;
  requests: number;
  inputTokens: number;
  outputTokens: number;
  cacheTokens: number;
}

export interface ModelCostFact {
  callerID: string;
  service: string;
  provider: string;
  actualModel: string;
  requests: number;
  attempts: number;
  costedAttempts: number;
  cost: number;
}

export const aiUsageFacts: AIUsageFact[] = [
  {
    callerID: "caller-support",
    route: "生产 AI 路由",
    clientModel: "qwen-max",
    actualModel: "qwen-max",
    service: "通义千问生产",
    requests: 13200,
    inputTokens: 9100000,
    outputTokens: 3500000,
    cacheTokens: 1100000,
  },
  {
    callerID: "caller-automation",
    route: "生产 AI 路由",
    clientModel: "qwen-max",
    actualModel: "qwen-max",
    service: "通义千问生产",
    requests: 15200,
    inputTokens: 14300000,
    outputTokens: 5700000,
    cacheTokens: 2000000,
  },
  {
    callerID: "caller-rd",
    route: "生产 AI 路由",
    clientModel: "claude-sonnet",
    actualModel: "claude-sonnet-4",
    service: "Anthropic 公网",
    requests: 13900,
    inputTokens: 11300000,
    outputTokens: 4500000,
    cacheTokens: 400000,
  },
  {
    callerID: "caller-rd",
    route: "生产 AI 路由",
    clientModel: "claude-sonnet",
    actualModel: "claude-sonnet-4",
    service: "Bedrock 灾备",
    requests: 2100,
    inputTokens: 1700000,
    outputTokens: 700000,
    cacheTokens: 0,
  },
];

export const modelCostFacts: ModelCostFact[] = [
  {
    callerID: "caller-support",
    service: "通义千问生产",
    provider: "阿里云百炼",
    actualModel: "qwen-max",
    requests: 13200,
    attempts: 13200,
    costedAttempts: 13200,
    cost: 375.5,
  },
  {
    callerID: "caller-automation",
    service: "通义千问生产",
    provider: "阿里云百炼",
    actualModel: "qwen-max",
    requests: 15200,
    attempts: 15200,
    costedAttempts: 15200,
    cost: 598,
  },
  {
    callerID: "caller-rd",
    service: "Anthropic 公网",
    provider: "Anthropic",
    actualModel: "claude-sonnet-4",
    requests: 16000,
    attempts: 16000,
    costedAttempts: 13900,
    cost: 702.24,
  },
  {
    callerID: "caller-rd",
    service: "Bedrock 灾备",
    provider: "AWS",
    actualModel: "claude-sonnet-4",
    requests: 2100,
    attempts: 2100,
    costedAttempts: 2100,
    cost: 109.2,
  },
];

export function aggregateUsageFacts(facts: AIUsageFact[], group: UsageGroup) {
  const grouped = new Map<
    string,
    {
      label: string;
      detail: string;
      requests: number;
      inputTokens: number;
      outputTokens: number;
      cacheTokens: number;
    }
  >();
  facts.forEach((fact) => {
    const label =
      group === "CLIENT_MODEL"
        ? fact.clientModel
        : group === "ACTUAL_MODEL"
          ? fact.actualModel
          : fact.service;
    const detail =
      group === "CLIENT_MODEL"
        ? `${fact.route} · 对外模型名`
        : group === "ACTUAL_MODEL"
          ? `${fact.service} · 实际调用`
          : `${fact.actualModel} · 模型服务`;
    const current = grouped.get(label) ?? {
      label,
      detail,
      requests: 0,
      inputTokens: 0,
      outputTokens: 0,
      cacheTokens: 0,
    };
    current.requests += fact.requests;
    current.inputTokens += fact.inputTokens;
    current.outputTokens += fact.outputTokens;
    current.cacheTokens += fact.cacheTokens;
    grouped.set(label, current);
  });
  return [...grouped.values()].sort(
    (left, right) =>
      right.inputTokens + right.outputTokens -
      (left.inputTokens + left.outputTokens),
  );
}

export function aggregateCostFacts(facts: ModelCostFact[]) {
  const grouped = new Map<string, ModelCostFact>();
  facts.forEach((fact) => {
    const key = `${fact.service}-${fact.actualModel}`;
    const current = grouped.get(key) ?? {
      ...fact,
          requests: 0,
          attempts: 0,
          costedAttempts: 0,
          cost: 0,
        };
    current.requests += fact.requests;
    current.attempts += fact.attempts;
    current.costedAttempts += fact.costedAttempts;
    current.cost += fact.cost;
    grouped.set(key, current);
  });
  return [...grouped.values()].sort((left, right) => right.cost - left.cost);
}

export function formatTokenCount(value: number) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return value.toLocaleString("zh-CN");
}

export function formatCount(value: number) {
  return value >= 10_000
    ? `${(value / 1_000).toFixed(1)}K`
    : value.toLocaleString("zh-CN");
}
