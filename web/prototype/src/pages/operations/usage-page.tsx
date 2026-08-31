import {
  ChevronRight,
  CircleDollarSign,
} from "lucide-react";
import { useState } from "react";
import {
  Link,
  useSearchParams,
} from "react-router-dom";
import type { RequestRecord } from "../../data";
import {
  EmptyState,
  FilterSelect,
  FilterTabs,
  Metric,
  PageHeader,
  StatusBadge,
} from "../../components/ui";
import { usePrototype } from "../../prototype-context";
import {
  aggregateCostFacts,
  aggregateUsageFacts,
  aiUsageFacts,
  formatCount,
  formatTokenCount,
  modelCostFacts,
  type AIUsageFact,
  type ModelCostFact,
  type UsageGroup,
  type UsageView,
} from "./usage-model";

export function UsagePage() {
  const { callers, routes, requests, services } = usePrototype();
  const [params] = useSearchParams();
  const [view, setView] = useState<UsageView>("QUOTA");
  const [group, setGroup] = useState<UsageGroup>("CLIENT_MODEL");
  const [selectedCallerID, setSelectedCallerID] = useState(
    params.get("caller") ?? "ALL",
  );
  const selectedCaller = callers.find((caller) => caller.id === selectedCallerID);
  const visibleCallers = selectedCaller ? [selectedCaller] : callers;
  const quotas = visibleCallers.flatMap((caller) =>
    caller.quotas.map((quota) => ({
      caller,
      quota,
      route: routes.find((route) => route.id === quota.routeID),
    })),
  );
  const usageFacts = aiUsageFacts.filter(
    (fact) => !selectedCaller || fact.callerID === selectedCaller.id,
  );
  const costFacts = modelCostFacts.filter(
    (fact) => !selectedCaller || fact.callerID === selectedCaller.id,
  );
  const aiRequests = requests.filter(
    (request) =>
      request.type === "AI" &&
      (!selectedCaller || request.caller === selectedCaller.name),
  );
  const inputTokens = usageFacts.reduce(
    (sum, fact) => sum + fact.inputTokens,
    0,
  );
  const outputTokens = usageFacts.reduce(
    (sum, fact) => sum + fact.outputTokens,
    0,
  );
  const cacheTokens = usageFacts.reduce(
    (sum, fact) => sum + fact.cacheTokens,
    0,
  );
  const requestCount = usageFacts.reduce(
    (sum, fact) => sum + fact.requests,
    0,
  );
  const totalCost = costFacts.reduce((sum, fact) => sum + fact.cost, 0);
  const serviceAttempts = costFacts.reduce(
    (sum, fact) => sum + fact.attempts,
    0,
  );
  const costedAttempts = costFacts.reduce(
    (sum, fact) => sum + fact.costedAttempts,
    0,
  );
  const exhaustedQuotas = quotas.filter(
    (item) => item.quota.used >= item.quota.limit,
  ).length;
  const warningQuotas = quotas.filter(
    (item) =>
      item.quota.used < item.quota.limit &&
      item.quota.used / item.quota.limit >= 0.8,
  ).length;
  const quotaRejections = selectedCaller
    ? selectedCaller.state === "warning"
      ? "124"
      : "0"
    : "124";

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="观测分析"
        title="用量与成本"
        description="分别查看额度归属、AI Token 消耗和模型服务成本"
        actions={
          <Link className="button button-secondary" to="/callers">
            管理调用方与额度 <ChevronRight />
          </Link>
        }
      />
      <div className="usage-view-tabs" role="tablist" aria-label="用量视图">
        <button
          type="button"
          className={view === "QUOTA" ? "is-active" : ""}
          onClick={() => setView("QUOTA")}
        >
          <strong>额度执行</strong>
          <span>调用方 × 路由</span>
        </button>
        <button
          type="button"
          className={view === "TOKEN" ? "is-active" : ""}
          onClick={() => setView("TOKEN")}
        >
          <strong>AI 用量</strong>
          <span>客户端模型与实际模型</span>
        </button>
        <button
          type="button"
          className={view === "COST" ? "is-active" : ""}
          onClick={() => setView("COST")}
        >
          <strong>成本分析</strong>
          <span>包含重试与备用线路调用</span>
        </button>
      </div>
      <div className="usage-filter-bar">
        <div className="usage-period">
          <strong>本月</strong>
          <span>8 月 1 日至今天</span>
        </div>
        <FilterSelect
          label="调用方"
          value={selectedCallerID}
          onChange={setSelectedCallerID}
          options={[
            { value: "ALL", label: "全部调用方", count: callers.length },
            ...callers.map((caller) => ({
              value: caller.id,
              label: caller.name,
            })),
          ]}
        />
      </div>
      {view === "QUOTA" ? (
        <QuotaUsageView
          quotas={quotas}
          exhausted={exhaustedQuotas}
          warning={warningQuotas}
          rejections={quotaRejections}
        />
      ) : null}
      {view === "TOKEN" ? (
        <TokenUsageView
          facts={usageFacts}
          group={group}
          onGroupChange={setGroup}
          requestCount={requestCount}
          inputTokens={inputTokens}
          outputTokens={outputTokens}
          cacheTokens={cacheTokens}
        />
      ) : null}
      {view === "COST" ? (
        <CostUsageView
          facts={costFacts}
          requests={aiRequests}
          services={services}
          totalCost={totalCost}
          gatewayRequests={requestCount}
          serviceAttempts={serviceAttempts}
          costedAttempts={costedAttempts}
        />
      ) : null}
    </div>
  );
}

export function QuotaUsageView({
  quotas,
  exhausted,
  warning,
  rejections,
}: {
  quotas: Array<{
    caller: ReturnType<typeof usePrototype>["callers"][number];
    quota: ReturnType<typeof usePrototype>["callers"][number]["quotas"][number];
    route: ReturnType<typeof usePrototype>["routes"][number] | undefined;
  }>;
  exhausted: number;
  warning: number;
  rejections: string;
}) {
  return (
    <>
      <section className="metric-grid four">
        <Metric
          label="额度规则"
          value={String(quotas.length)}
          note="按调用方与路由独立统计"
        />
        <Metric
          label="已用尽"
          value={String(exhausted)}
          note="后续请求将被拒绝"
          tone={exhausted ? "warning" : "good"}
        />
        <Metric
          label="接近上限"
          value={String(warning)}
          note="使用率达到 80%"
          tone={warning ? "warning" : "good"}
        />
        <Metric
          label="额度拒绝"
          value={rejections}
          note="本月累计"
          tone={rejections === "0" ? "good" : "warning"}
        />
      </section>
      <section className="usage-explain">
        <span>额度维度</span>
        <strong>调用方</strong>
        <i>+</i>
        <strong>路由</strong>
        <i>+</i>
        <strong>统计周期</strong>
        <p>服务重试或线路切换不重复扣减额度</p>
      </section>
      <section className="card usage-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">额度执行</span>
              <h3>调用方与路由额度</h3>
            </div>
            <span>{quotas.length} 条额度规则</span>
          </header>
          {quotas.length ? quotas.map(({ caller, quota, route }) => {
            const percent = Math.round((quota.used / quota.limit) * 100);
            return (
              <div
                className="usage-row"
                key={`${caller.id}-${quota.routeID}`}
              >
                <span>
                  <CircleDollarSign />
                </span>
                <div>
                  <strong>{caller.name}</strong>
                  <small>
                    {route?.name ?? quota.routeID} · {quota.period}重置
                  </small>
                </div>
                <p>
                  <strong>{quota.used.toLocaleString("zh-CN")}</strong>
                  <span>
                    / {quota.limit.toLocaleString("zh-CN")} {route?.type === "AI" ? "Token" : "次"}
                  </span>
                </p>
                <i>
                  <b
                    className={percent >= 100 ? "is-exhausted" : ""}
                    style={{ width: `${Math.min(percent, 100)}%` }}
                  />
                </i>
                <StatusBadge
                  state={
                    percent >= 100
                      ? "error"
                      : percent >= 80
                        ? "warning"
                        : "healthy"
                  }
                  label={`${percent}%`}
                />
              </div>
            );
          }) : <EmptyState title="没有配置额度" description="所选调用方未设置累计用量上限。" />}
      </section>
    </>
  );
}

export function TokenUsageView({
  facts,
  group,
  onGroupChange,
  requestCount,
  inputTokens,
  outputTokens,
  cacheTokens,
}: {
  facts: AIUsageFact[];
  group: UsageGroup;
  onGroupChange: (group: UsageGroup) => void;
  requestCount: number;
  inputTokens: number;
  outputTokens: number;
  cacheTokens: number;
}) {
  const groupedFacts = aggregateUsageFacts(facts, group);
  return (
    <>
      <section className="metric-grid four">
        <Metric label="AI 请求" value={formatCount(requestCount)} note="按客户端请求计数" />
        <Metric label="输入 Token" value={formatTokenCount(inputTokens)} note="包含缓存命中部分" />
        <Metric label="输出 Token" value={formatTokenCount(outputTokens)} note="模型实际生成" />
        <Metric label="缓存命中 Token" value={formatTokenCount(cacheTokens)} note="输入 Token 的子集" tone="good" />
      </section>
      <section className="usage-explain">
        <span>用量归属</span>
        <strong>调用方</strong>
        <i>→</i>
        <strong>路由</strong>
        <i>→</i>
        <strong>客户端模型</strong>
        <i>→</i>
        <strong>实际模型</strong>
        <p>缓存命中 Token 已包含在输入 Token 中</p>
      </section>
      <section className="card usage-breakdown-card">
        <header className="card-header">
          <div>
            <span className="eyebrow">AI 用量构成</span>
            <h3>Token 消耗明细</h3>
          </div>
          <FilterTabs
            value={group}
            onChange={onGroupChange}
            options={[
              { value: "CLIENT_MODEL", label: "客户端模型" },
              { value: "ACTUAL_MODEL", label: "实际模型" },
              { value: "SERVICE", label: "大模型服务" },
            ]}
          />
        </header>
        <div className="usage-breakdown-head">
          <span>统计维度</span>
          <span>AI 请求</span>
          <span>输入 Token</span>
          <span>输出 Token</span>
          <span>缓存命中</span>
          <span>Token 合计</span>
        </div>
        {groupedFacts.length ? groupedFacts.map((fact) => (
          <div className="usage-breakdown-row" key={fact.label}>
            <div>
              <strong>{fact.label}</strong>
              <small>{fact.detail}</small>
            </div>
            <span>{formatCount(fact.requests)}</span>
            <span>{formatTokenCount(fact.inputTokens)}</span>
            <span>{formatTokenCount(fact.outputTokens)}</span>
            <span>{formatTokenCount(fact.cacheTokens)}</span>
            <strong>{formatTokenCount(fact.inputTokens + fact.outputTokens)}</strong>
          </div>
        )) : <EmptyState title="没有 AI 用量" description="所选调用方在该周期内没有 AI 请求。" />}
      </section>
    </>
  );
}

export function CostUsageView({
  facts,
  requests,
  services,
  totalCost,
  gatewayRequests,
  serviceAttempts,
  costedAttempts,
}: {
  facts: ModelCostFact[];
  requests: RequestRecord[];
  services: ReturnType<typeof usePrototype>["services"];
  totalCost: number;
  gatewayRequests: number;
  serviceAttempts: number;
  costedAttempts: number;
}) {
  const groupedFacts = aggregateCostFacts(facts);
  return (
    <>
      <section className="metric-grid four">
        <Metric label="预估成本" value={`¥${totalCost.toFixed(2)}`} note="仅统计用量与单价完整的调用" />
        <Metric label="网关请求" value={formatCount(gatewayRequests)} note="客户端发起的 AI 请求" />
        <Metric label="服务调用" value={formatCount(serviceAttempts)} note={`${Math.max(serviceAttempts - gatewayRequests, 0).toLocaleString("zh-CN")} 次重试或切换`} />
        <Metric label="成本未知" value={formatCount(Math.max(serviceAttempts - costedAttempts, 0))} note="缺少 Token 用量或服务单价" tone={serviceAttempts === costedAttempts ? "good" : "warning"} />
      </section>
      <section className="usage-explain cost-explain">
        <span>成本计算</span>
        <strong>非缓存输入 × 单价</strong>
        <i>+</i>
        <strong>输出 Token × 单价</strong>
        <i>+</i>
        <strong>缓存输入 × 缓存单价</strong>
        <p>按调用发生时记录的单价快照估算；缺少用量或单价的调用不计入，最终费用以模型厂商账单为准</p>
      </section>
      <section className="usage-layout">
        <article className="card usage-breakdown-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">服务成本</span>
              <h3>模型服务调用</h3>
            </div>
          </header>
          <div className="cost-breakdown-head">
            <span>服务与实际模型</span>
            <span>涉及请求</span>
            <span>服务调用</span>
            <span>已计价</span>
            <span>预估成本</span>
          </div>
          {groupedFacts.length ? groupedFacts.map((fact) => (
            <div className="cost-breakdown-row" key={`${fact.service}-${fact.actualModel}`}>
              <div><strong>{fact.service}</strong><small>{fact.provider} · {fact.actualModel}</small></div>
              <span>{formatCount(fact.requests)}</span>
              <span>{formatCount(fact.attempts)}</span>
              <span>{formatCount(fact.costedAttempts)}</span>
              <strong>¥{fact.cost.toFixed(2)}</strong>
            </div>
          )) : <EmptyState title="没有成本数据" description="所选调用方在该周期内没有大模型服务调用。" />}
        </article>
        <aside className="card cost-card">
          <header>
            <span className="eyebrow">计价依据</span>
            <h3>模型服务单价</h3>
          </header>
          {services
            .filter((service) => service.type === "MODEL")
            .flatMap((service) =>
              Object.entries(service.modelPrices ?? {}).map(
                ([model, price]) => (
                  <div key={`${service.id}-${model}`}>
                    <span>
                      <strong>{model}</strong>
                      <small>
                        {service.name} · 更新于 {price.updatedAt}
                      </small>
                    </span>
                    <p>
                      输入 ¥{price.input}
                      {price.cachedInput !== undefined ? (
                        <>
                          <br />
                          缓存输入 ¥{price.cachedInput}
                        </>
                      ) : null}
                      <br />
                      输出 ¥{price.output}
                    </p>
                  </div>
                ),
              ),
            )}
          <footer>人民币 / 百万 Token · 调用时记录单价快照，未配置单价时不计算成本</footer>
        </aside>
      </section>
      <section className="card ranking-card">
        <header className="card-header">
          <div>
            <span className="eyebrow">请求下钻</span>
            <h3>网关请求与服务调用</h3>
          </div>
          <Link to="/requests?type=AI">
            查看请求 <ChevronRight />
          </Link>
        </header>
        <div className="usage-request-head">
          <span>请求编号</span>
          <span>调用方</span>
          <span>客户端模型与实际线路</span>
          <span>服务调用</span>
          <span>Token</span>
          <span>预估成本</span>
        </div>
        {requests.map((request) => (
          <Link
            className="usage-request-row"
            to={`/requests?query=${request.id}`}
            key={request.id}
          >
            <code>{request.id}</code>
            <strong>{request.caller}</strong>
            <span>
              {request.detail ?? `${request.method} ${request.path}`} → {request.target}
            </span>
            <span>{request.result === "主备切换" ? "2 次" : request.result === "策略拒绝" ? "0 次" : "1 次"}</span>
            <span>{request.usage}</span>
            <strong>{request.cost}</strong>
          </Link>
        ))}
      </section>
    </>
  );
}
