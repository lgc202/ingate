import {
  AlertTriangle,
  CheckCircle2,
  ChevronRight,
  Clock3,
  CircleDollarSign,
  Download,
  FileClock,
  History,
  Network,
  Server,
  ShieldCheck,
  RefreshCw,
  Wrench,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { AuditRecord, RequestRecord, TrafficType } from "../data";
import {
  Drawer,
  EmptyState,
  FilterSelect,
  FilterTabs,
  Metric,
  PageHeader,
  PrimaryButton,
  SearchField,
  StatusBadge,
  TypeBadge,
} from "../components/ui";
import { usePrototype } from "../prototype-context";

type RequestFilter = "ALL" | TrafficType | "ERROR";
type UsageView = "QUOTA" | "TOKEN" | "COST";
type UsageGroup = "CLIENT_MODEL" | "ACTUAL_MODEL" | "SERVICE";

interface AIUsageFact {
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

interface ModelCostFact {
  callerID: string;
  service: string;
  provider: string;
  actualModel: string;
  requests: number;
  attempts: number;
  cost: number;
  failedCost: number;
}

const aiUsageFacts: AIUsageFact[] = [
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
    service: "Bedrock 灾备",
    requests: 16000,
    inputTokens: 13000000,
    outputTokens: 5200000,
    cacheTokens: 400000,
  },
];

const modelCostFacts: ModelCostFact[] = [
  {
    callerID: "caller-support",
    service: "通义千问生产",
    provider: "阿里云百炼",
    actualModel: "qwen-max",
    requests: 13200,
    attempts: 13200,
    cost: 350.8,
    failedCost: 0,
  },
  {
    callerID: "caller-automation",
    service: "通义千问生产",
    provider: "阿里云百炼",
    actualModel: "qwen-max",
    requests: 15200,
    attempts: 15200,
    cost: 290,
    failedCost: 0,
  },
  {
    callerID: "caller-rd",
    service: "Anthropic 公网",
    provider: "Anthropic",
    actualModel: "claude-sonnet-4",
    requests: 16000,
    attempts: 16000,
    cost: 244.6,
    failedCost: 47.6,
  },
  {
    callerID: "caller-rd",
    service: "Bedrock 灾备",
    provider: "AWS",
    actualModel: "claude-sonnet-4",
    requests: 2100,
    attempts: 2100,
    cost: 444.6,
    failedCost: 0,
  },
];

export function RequestPage() {
  const { requests } = usePrototype();
  const [params] = useSearchParams();
  const initialQuery = params.get("query") ?? "";
  const [query, setQuery] = useState(initialQuery);
  const [filter, setFilter] = useState<RequestFilter>("ALL");
  const [selected, setSelected] = useState<RequestRecord | null>(
    () => requests.find((item) => item.id === initialQuery) ?? null,
  );
  const visible = useMemo(
    () =>
      requests.filter((request) => {
        const matchesType =
          filter === "ALL" ||
          (filter === "ERROR"
            ? request.result === "失败" || request.result === "策略拒绝"
            : request.type === filter);
        return (
          matchesType &&
          `${request.id}${request.caller}${request.route}${request.request}${request.target}`
            .toLowerCase()
            .includes(query.toLowerCase())
        );
      }),
    [filter, query, requests],
  );

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="运行中心"
        title="请求记录"
        description="查找单次请求的路由、策略与服务执行结果"
      />
      <section className="metric-grid four">
        <Metric label="API 请求" value="98.3K" note="成功率 99.91%" />
        <Metric label="AI 请求" value="44.4K" note="19.7M Token" />
        <Metric label="MCP 工具调用" value="8.2K" note="成功率 99.90%" />
        <Metric
          label="异常与拒绝"
          value="257"
          note="服务异常 66 · 策略拒绝 191"
          tone="warning"
        />
      </section>
      <p className="privacy-note">
        <ShieldCheck />
        默认不记录请求正文
      </p>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索请求编号、路由、调用方或服务"
          />
          <FilterTabs
            value={filter}
            onChange={setFilter}
            options={[
              { value: "ALL", label: "全部", count: requests.length },
              { value: "API", label: "API" },
              { value: "AI", label: "AI" },
              { value: "MCP", label: "MCP" },
              { value: "ERROR", label: "异常" },
            ]}
          />
        </header>
        <div className="table-head request-columns">
          <span>时间 / 请求编号</span>
          <span>调用方</span>
          <span>路由 / 请求</span>
          <span>结果</span>
          <span>最终服务</span>
          <span>耗时</span>
          <span />
        </div>
        {visible.length ? (
          visible.map((request) => (
            <button
              key={request.id}
              className="table-row request-columns"
              type="button"
              onClick={() => setSelected(request)}
            >
              <div>
                <strong>{request.time}</strong>
                <code>{request.id}</code>
              </div>
              <strong>{request.caller}</strong>
              <div className="request-route">
                <span>
                  <TypeBadge type={request.type} />
                  <strong>{request.route}</strong>
                </span>
                <small>{request.request}</small>
              </div>
              <StatusBadge
                state={
                  request.result === "成功"
                    ? "healthy"
                    : request.result === "主备切换"
                      ? "warning"
                      : "error"
                }
                label={`${request.code} · ${request.result}`}
              />
              <strong>{request.target}</strong>
              <span>{request.latency}</span>
              <ChevronRight />
            </button>
          ))
        ) : (
          <EmptyState
            title="没有匹配的请求"
            description="请调整搜索或筛选条件。"
          />
        )}
      </section>
      {selected ? (
        <RequestDetail request={selected} onClose={() => setSelected(null)} />
      ) : null}
    </div>
  );
}

function RequestDetail({
  request,
  onClose,
}: {
  request: RequestRecord;
  onClose: () => void;
}) {
  const failed = request.result === "失败" || request.result === "策略拒绝";
  const usageLabel =
    request.type === "AI"
      ? "Token 用量"
      : request.type === "MCP"
        ? "工具调用"
        : "响应流量";
  const facts = [
    ["调用方", request.caller],
    ["路由", request.route],
    ["请求", request.request],
    ["最终结果", request.target],
    ["总耗时", request.latency],
    [
      request.type === "AI" ? "Token / 成本" : usageLabel,
      request.type === "AI" ? `${request.usage} · ${request.cost}` : request.usage,
    ],
  ];
  const serviceNames = new Set(
    request.attempts.map((attempt) => attempt.service),
  );
  const gatewaySteps = request.steps.filter(
    (step) => !serviceNames.has(step.name),
  );

  return (
    <Drawer
      title={`请求 ${request.id}`}
      description={`${request.time} · ${request.caller} · ${request.type}`}
      onClose={onClose}
      width="wide"
    >
      <div
        className={`request-verdict ${failed ? "is-error" : request.result === "主备切换" ? "is-warning" : ""}`}
      >
        <span>{failed ? <AlertTriangle /> : <CheckCircle2 />}</span>
        <div>
          <StatusBadge
            state={
              failed
                ? "error"
                : request.result === "主备切换"
                  ? "warning"
                  : "healthy"
            }
            label={`${request.code} · ${request.result}`}
          />
          <h3>
            {request.result === "主备切换"
              ? "主服务失败后由备用服务成功响应"
              : request.result === "策略拒绝"
                ? "请求在到达服务之前被策略拒绝"
                : request.result === "失败"
                  ? "目标服务没有成功响应"
                  : "请求已成功完成"}
          </h3>
        </div>
      </div>
      <div className="request-facts">
        {facts.map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      <section className="service-attempts">
        <header>
          <div>
            <span className="eyebrow">服务调用</span>
            <h3>服务尝试</h3>
          </div>
          <span>
            {request.attempts.length
              ? `${request.attempts.length} 次尝试`
              : "未到达服务"}
          </span>
        </header>
        {request.attempts.length ? (
          request.attempts.map((attempt, index) => (
            <article
              className={`attempt-card state-${attempt.state}`}
              key={`${attempt.service}-${index}`}
            >
              <span className="attempt-index">{index + 1}</span>
              <div className="attempt-main">
                <header>
                  <div>
                    <strong>{attempt.service}</strong>
                    <span>
                      {[attempt.provider, attempt.actualModel]
                        .filter(Boolean)
                        .join(" · ") || "目标服务"}
                    </span>
                  </div>
                  <StatusBadge
                    state={attempt.state}
                    label={`${attempt.code} · ${attempt.result}`}
                  />
                </header>
                <div className="attempt-facts">
                  <span>
                    <small>耗时</small>
                    <strong>{attempt.latency}</strong>
                  </span>
                  {attempt.ttft ? (
                    <span>
                      <small>首个 Token</small>
                      <strong>{attempt.ttft}</strong>
                    </span>
                  ) : null}
                  {attempt.inputTokens !== undefined ? (
                    <span>
                      <small>输入 Token</small>
                      <strong>{attempt.inputTokens.toLocaleString("zh-CN")}</strong>
                    </span>
                  ) : null}
                  {attempt.outputTokens !== undefined ? (
                    <span>
                      <small>输出 Token</small>
                      <strong>{attempt.outputTokens.toLocaleString("zh-CN")}</strong>
                    </span>
                  ) : null}
                  {attempt.cachedTokens !== undefined ? (
                    <span>
                      <small>缓存 Token</small>
                      <strong>{attempt.cachedTokens.toLocaleString("zh-CN")}</strong>
                    </span>
                  ) : null}
                  {attempt.cost ? (
                    <span>
                      <small>本次成本</small>
                      <strong>{attempt.cost}</strong>
                    </span>
                  ) : null}
                </div>
                {attempt.error ? (
                  <p>
                    <AlertTriangle />
                    {attempt.error}
                  </p>
                ) : null}
              </div>
            </article>
          ))
        ) : (
          <div className="attempt-empty">
            <ShieldCheck />
            <div>
              <strong>请求在网关内结束</strong>
              <span>没有向任何服务发送流量，也没有产生服务侧 Token 或成本</span>
            </div>
          </div>
        )}
      </section>
      <section className="execution-timeline">
        <header>
          <span className="eyebrow">网关处理</span>
          <h3>认证、策略与路由</h3>
        </header>
        {gatewaySteps.map((step, index) => (
          <div className="timeline-step" key={`${step.name}-${index}`}>
            <span className={`timeline-dot state-${step.state}`}>
              {index + 1}
            </span>
            <div>
              <strong>{step.name}</strong>
              <p>{step.detail}</p>
            </div>
            <small>{step.duration}</small>
          </div>
        ))}
      </section>
      <p className="privacy-note">
        <ShieldCheck />
        未记录请求正文
      </p>
    </Drawer>
  );
}

type AnalysisFilter = TrafficType;

const analysisViews = {
  API: {
    title: "API 流量",
    metrics: [
      ["API 请求", "98.3K", "+5.4%"],
      ["成功率", "99.91%", "5xx 62 次"],
      ["P95 延迟", "124 ms", "P99 680 ms"],
      ["响应流量", "51.1 GB", "文件上传占 75%"],
    ],
    bars: [43, 52, 49, 64, 60, 69, 76, 72, 85, 81, 91, 83],
    unit: "API 请求",
    composition: {
      title: "响应结果",
      total: "98.3K",
      label: "API 请求",
      gradient:
        "var(--teal) 0 99.4%, var(--amber) 99.4% 99.91%, var(--red) 99.91% 100%",
      segments: [
        ["成功", "99.40%", "api"],
        ["客户端错误", "0.51%", "ai"],
        ["服务端错误", "0.09%", "mcp"],
      ],
    },
    headers: [
      "路由",
      "类型",
      "目标服务",
      "请求",
      "成功率",
      "P95 延迟",
      "响应流量",
    ],
    rows: [
      ["订单查询 API", "API", "订单服务", "58.4K", "99.96%", "86 ms", "8.4 GB"],
      [
        "客户资料 API",
        "API",
        "客户中心",
        "33.8K",
        "99.91%",
        "124 ms",
        "4.1 GB",
      ],
      [
        "文件上传 API",
        "API",
        "文件服务",
        "6.1K",
        "99.42%",
        "680 ms",
        "38.6 GB",
      ],
    ],
  },
  AI: {
    title: "AI 流量",
    metrics: [
      ["AI 请求", "44.4K", "+9.1%"],
      ["首个 Token P95", "612 ms", "Anthropic 2.8 s"],
      ["成功率", "99.72%", "主备切换 41 次"],
      ["服务错误", "124", "429 58 · 5xx 66"],
    ],
    bars: [28, 39, 35, 43, 49, 52, 58, 63, 69, 75, 82, 79],
    unit: "AI 请求",
    composition: {
      title: "响应结果",
      total: "44.4K",
      label: "AI 请求",
      gradient: "var(--teal) 0 99.72%, var(--amber) 99.72% 99.9%, var(--red) 99.9% 100%",
      segments: [
        ["成功", "99.72%", "mcp"],
        ["主备切换", "0.18%", "ai"],
        ["失败", "0.10%", "api"],
      ],
    },
    headers: [
      "路由",
      "类型",
      "模型线路",
      "请求",
      "成功率",
      "首个 Token",
      "主备切换",
    ],
    rows: [
      [
        "生产 AI 路由",
        "AI",
        "3 个模型服务",
        "28.4K",
        "99.72%",
        "612 ms",
        "41",
      ],
      [
        "内部 AI 路由",
        "AI",
        "通义千问生产",
        "16.0K",
        "99.99%",
        "112 ms",
        "0",
      ],
    ],
  },
  MCP: {
    title: "MCP 流量",
    metrics: [
      ["工具调用", "8.2K", "+12.4%"],
      ["成功率", "99.90%", "失败 8 次"],
      ["P95 耗时", "238 ms", "P99 640 ms"],
      ["权限拒绝", "17", "主要为未授权工具"],
    ],
    bars: [20, 25, 28, 35, 31, 42, 45, 54, 61, 66, 74, 80],
    unit: "工具调用",
    composition: {
      title: "工具分布",
      total: "8.2K",
      label: "今日调用",
      gradient: "var(--blue) 0 65.9%, var(--orange) 65.9% 100%",
      segments: [
        ["web_search", "5.4K", "api"],
        ["fetch_page", "2.8K", "ai"],
      ],
    },
    headers: [
      "工具",
      "类型",
      "MCP 服务",
      "调用",
      "成功率",
      "P95 耗时",
      "权限拒绝",
    ],
    rows: [
      ["web_search", "MCP", "搜索工具服务", "5.4K", "99.94%", "201 ms", "11"],
      ["fetch_page", "MCP", "搜索工具服务", "2.8K", "99.86%", "298 ms", "6"],
    ],
  },
} as const;

export function AnalysisPage() {
  const { routes } = usePrototype();
  const [params] = useSearchParams();
  const requestedRoute = params.get("query");
  const requestedType = routes.find(
    (route) => route.name === requestedRoute,
  )?.type;
  const [filter, setFilter] = useState<AnalysisFilter>(
    requestedType === "AI" || requestedType === "MCP" ? requestedType : "API",
  );
  const view = analysisViews[filter];
  const visibleRows = requestedRoute
    ? view.rows.filter((row) => row[0] === requestedRoute)
    : view.rows;
  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="运行中心" title="流量分析" description={requestedRoute ? `已定位到路由“${requestedRoute}”的运行数据` : "按 API、AI 与 MCP 查看吞吐、成功率和延迟"} />
      <FilterTabs
        value={filter}
        onChange={setFilter}
        options={[
          { value: "API", label: "API" },
          { value: "AI", label: "AI" },
          { value: "MCP", label: "MCP" },
        ]}
      />
      <section className="metric-grid four">
        {view.metrics.map(([label, value, note]) => (
          <Metric key={label} label={label} value={value} note={note} />
        ))}
      </section>
      <section className="analysis-layout">
        <article className="card chart-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">过去 12 小时</span>
              <h3>{view.title}趋势</h3>
            </div>
            <span>{view.unit}</span>
          </header>
          <div className="bar-chart">
            {view.bars.map((height, index) => (
              <i key={index}>
                <b style={{ height: `${height}%` }} />
                <span>{index % 2 === 0 ? `${index + 3}:00` : ""}</span>
              </i>
            ))}
          </div>
        </article>
        <article className="card composition-card">
          <header>
            <span className="eyebrow">构成</span>
            <h3>{view.composition.title}</h3>
          </header>
          <div
            className="donut"
            style={{
              background: `radial-gradient(circle,#fffef9 0 52%,transparent 53%), conic-gradient(${view.composition.gradient})`,
            }}
          >
            <div>
              <strong>{view.composition.total}</strong>
              <span>{view.composition.label}</span>
            </div>
          </div>
          {view.composition.segments.map(([label, value, tone]) => (
            <p key={label}>
              <i className={tone} />
              {label}
              <strong>{value}</strong>
            </p>
          ))}
        </article>
      </section>
      <section className="card ranking-card">
        <header className="card-header">
          <div>
            <span className="eyebrow">明细排名</span>
            <h3>{view.title}</h3>
          </div>
          <Link
            to={`/requests?query=${encodeURIComponent(visibleRows[0]?.[0] ?? filter)}`}
          >
            查看请求 <ChevronRight />
          </Link>
        </header>
        <div className="ranking-row ranking-head">
          {view.headers.map((header) => (
            <span key={header}>{header}</span>
          ))}
        </div>
        {visibleRows.map((row) => (
          <Link
            className="ranking-row"
            to={`/requests?query=${encodeURIComponent(row[0])}`}
            key={row[0]}
          >
            {row.map((cell, index) =>
              index === 1 ? (
                <TypeBadge
                  key={`${row[0]}-${cell}`}
                  type={cell as TrafficType}
                />
              ) : (
                <span key={`${row[0]}-${cell}-${index}`}>{cell}</span>
              ),
            )}
          </Link>
        ))}
      </section>
    </div>
  );
}

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
  const failedAttemptCost = costFacts.reduce(
    (sum, fact) => sum + fact.failedCost,
    0,
  );
  const quotaUsed = quotas.reduce((sum, item) => sum + item.quota.used, 0);
  const quotaLimit = quotas.reduce((sum, item) => sum + item.quota.limit, 0);
  const quotaRejections = selectedCaller
    ? selectedCaller.state === "warning"
      ? "124"
      : "0"
    : "124";

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="运行中心"
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
          <span>所有模型服务尝试</span>
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
          used={quotaUsed}
          limit={quotaLimit}
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
          failedAttemptCost={failedAttemptCost}
        />
      ) : null}
    </div>
  );
}

function QuotaUsageView({
  quotas,
  used,
  limit,
  rejections,
}: {
  quotas: Array<{
    caller: ReturnType<typeof usePrototype>["callers"][number];
    quota: ReturnType<typeof usePrototype>["callers"][number]["quotas"][number];
    route: ReturnType<typeof usePrototype>["routes"][number] | undefined;
  }>;
  used: number;
  limit: number;
  rejections: string;
}) {
  return (
    <>
      <section className="metric-grid four">
        <Metric
          label="已消耗额度"
          value={formatTokenCount(used)}
          note="客户端成功使用的 Token"
        />
        <Metric
          label="额度上限"
          value={formatTokenCount(limit)}
          note="当前筛选范围合计"
        />
        <Metric
          label="剩余额度"
          value={formatTokenCount(Math.max(limit - used, 0))}
          note={limit ? `${Math.round(((limit - used) / limit) * 100)}% 可用` : "未配置额度"}
        />
        <Metric
          label="额度拒绝"
          value={rejections}
          note="本月累计"
          tone={rejections === "0" ? "good" : "warning"}
        />
      </section>
      <section className="usage-explain">
        <span>额度归属口径</span>
        <strong>调用方</strong>
        <i>+</i>
        <strong>路由</strong>
        <i>+</i>
        <strong>统计周期</strong>
        <p>底层模型发生重试或主备切换，不会重复扣减调用方额度</p>
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
          }) : <EmptyState title="没有配置额度" description="当前调用方不限制累计用量。" />}
      </section>
    </>
  );
}

function TokenUsageView({
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
        <Metric label="AI 请求" value={formatCount(requestCount)} note="一次客户端调用计一次" />
        <Metric label="输入 Token" value={formatTokenCount(inputTokens)} note="包含缓存命中部分" />
        <Metric label="输出 Token" value={formatTokenCount(outputTokens)} note="模型实际生成" />
        <Metric label="缓存命中 Token" value={formatTokenCount(cacheTokens)} note="输入 Token 的子集" tone="good" />
      </section>
      <section className="usage-explain">
        <span>用量归属链路</span>
        <strong>调用方</strong>
        <i>→</i>
        <strong>路由</strong>
        <i>→</i>
        <strong>客户端模型</strong>
        <i>→</i>
        <strong>实际模型</strong>
        <p>缓存 Token 单独展示，但不与输入 Token 重复相加</p>
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
              { value: "SERVICE", label: "模型服务" },
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
        )) : <EmptyState title="没有 AI 用量" description="当前调用方在所选周期内没有 AI 请求。" />}
      </section>
    </>
  );
}

function CostUsageView({
  facts,
  requests,
  services,
  totalCost,
  gatewayRequests,
  serviceAttempts,
  failedAttemptCost,
}: {
  facts: ModelCostFact[];
  requests: RequestRecord[];
  services: ReturnType<typeof usePrototype>["services"];
  totalCost: number;
  gatewayRequests: number;
  serviceAttempts: number;
  failedAttemptCost: number;
}) {
  const groupedFacts = aggregateCostFacts(facts);
  return (
    <>
      <section className="metric-grid four">
        <Metric label="估算成本" value={`¥${totalCost.toFixed(2)}`} note="所有模型服务尝试合计" />
        <Metric label="网关请求" value={formatCount(gatewayRequests)} note="客户端发起的 AI 请求" />
        <Metric label="服务尝试" value={formatCount(serviceAttempts)} note={`${Math.max(serviceAttempts - gatewayRequests, 0).toLocaleString("zh-CN")} 次重试或切换`} />
        <Metric label="失败尝试成本" value={`¥${failedAttemptCost.toFixed(2)}`} note="失败线路仍可能产生费用" tone={failedAttemptCost ? "warning" : "good"} />
      </section>
      <section className="usage-explain cost-explain">
        <span>成本计算口径</span>
        <strong>输入 Token × 单价</strong>
        <i>+</i>
        <strong>输出 Token × 单价</strong>
        <i>+</i>
        <strong>其它厂商计费项</strong>
        <p>全部服务尝试累加；没有配置单价时显示成本未知，不按零元计算</p>
      </section>
      <section className="usage-layout">
        <article className="card usage-breakdown-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">服务成本</span>
              <h3>模型服务尝试</h3>
            </div>
          </header>
          <div className="cost-breakdown-head">
            <span>服务与实际模型</span>
            <span>网关请求</span>
            <span>服务尝试</span>
            <span>估算成本</span>
            <span>失败成本</span>
          </div>
          {groupedFacts.length ? groupedFacts.map((fact) => (
            <div className="cost-breakdown-row" key={`${fact.service}-${fact.actualModel}`}>
              <div><strong>{fact.service}</strong><small>{fact.provider} · {fact.actualModel}</small></div>
              <span>{formatCount(fact.requests)}</span>
              <span>{formatCount(fact.attempts)}</span>
              <strong>¥{fact.cost.toFixed(2)}</strong>
              <span className={fact.failedCost ? "is-warning" : ""}>¥{fact.failedCost.toFixed(2)}</span>
            </div>
          )) : <EmptyState title="没有成本数据" description="当前调用方在所选周期内没有模型服务调用。" />}
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
                      <br />
                      输出 ¥{price.output}
                    </p>
                  </div>
                ),
              ),
            )}
          <footer>人民币 / 百万 Token · 实际账单以模型厂商为准</footer>
        </aside>
      </section>
      <section className="card ranking-card">
        <header className="card-header">
          <div>
            <span className="eyebrow">请求下钻</span>
            <h3>网关请求与服务尝试</h3>
          </div>
          <Link to="/requests?query=AI">
            查看请求 <ChevronRight />
          </Link>
        </header>
        <div className="usage-request-head">
          <span>请求编号</span>
          <span>调用方</span>
          <span>客户端模型与实际线路</span>
          <span>服务尝试</span>
          <span>Token</span>
          <span>估算成本</span>
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
              {request.request} → {request.target}
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

function aggregateUsageFacts(facts: AIUsageFact[], group: UsageGroup) {
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

function aggregateCostFacts(facts: ModelCostFact[]) {
  const grouped = new Map<string, ModelCostFact>();
  facts.forEach((fact) => {
    const key = `${fact.service}-${fact.actualModel}`;
    const current = grouped.get(key) ?? {
      ...fact,
      requests: 0,
      attempts: 0,
      cost: 0,
      failedCost: 0,
    };
    current.requests += fact.requests;
    current.attempts += fact.attempts;
    current.cost += fact.cost;
    current.failedCost += fact.failedCost;
    grouped.set(key, current);
  });
  return [...grouped.values()].sort((left, right) => right.cost - left.cost);
}

function formatTokenCount(value: number) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return value.toLocaleString("zh-CN");
}

function formatCount(value: number) {
  return value >= 10_000
    ? `${(value / 1_000).toFixed(1)}K`
    : value.toLocaleString("zh-CN");
}

function recoveryLabel(state: "ready" | "ejected" | "cooling" | "probing") {
  return {
    ready: "正常承载",
    ejected: "已摘除",
    cooling: "冷却中",
    probing: "试探恢复",
  }[state];
}

export function HealthPage() {
  const { gateways, services, proxyInstances } = usePrototype();
  const serviceWarnings = services.filter(
    (service) =>
      service.state === "warning" || service.state === "error",
  );
  const offlineInstances = proxyInstances.filter(
    (instance) => instance.state !== "healthy",
  );
  const recoveringServices = services.filter(
    (service) => service.recovery && service.recovery.state !== "ready",
  );

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="运行中心"
        title="运行健康"
        description="代理实例、网关入口与服务连接的实时健康状态"
        actions={
          <Link className="button button-secondary" to="/releases">
            查看配置发布 <ChevronRight />
          </Link>
        }
      />
      <section className="health-hero">
        <span>
          <AlertTriangle />
        </span>
        <div>
          <small>生产环境</small>
          <h2>
            {serviceWarnings.length + offlineInstances.length}{" "}
            项运行问题需要关注
          </h2>
          <p>配置交付状态已独立到“配置发布”，这里仅展示真实流量与连接健康</p>
        </div>
        <StatusBadge
          state={
            serviceWarnings.length || offlineInstances.length
              ? "warning"
              : "healthy"
          }
          label={
            serviceWarnings.length || offlineInstances.length
              ? "需要关注"
              : "运行正常"
          }
        />
      </section>
      <section className="metric-grid four">
        <Metric
          label="在线代理"
          value={`${proxyInstances.length - offlineInstances.length} / ${proxyInstances.length}`}
          note={
            offlineInstances.length
              ? `${offlineInstances.length} 个实例离线`
              : "全部在线"
          }
          tone={offlineInstances.length ? "warning" : "good"}
        />
        <Metric
          label="健康服务"
          value={`${services.length - serviceWarnings.length} / ${services.length}`}
          note={`${serviceWarnings.length} 个需要关注`}
        />
        <Metric
          label="健康网关"
          value={`${gateways.filter((gateway) => gateway.state === "healthy").length} / ${gateways.length}`}
          note="入口可用性 99.99%"
          tone="good"
        />
        <Metric
          label="自动恢复"
          value={String(recoveringServices.length)}
          note={
            recoveringServices.length
              ? `${recoveringServices.map((service) => service.name).join("、")} 正在恢复`
              : "所有服务正常承载"
          }
          tone="warning"
        />
      </section>
      <section className="health-layout">
        <article className="card component-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">服务与入口</span>
              <h3>流量路径健康</h3>
            </div>
          </header>
          {gateways.map((gateway) => (
            <div className="component-row" key={gateway.id}>
              <span>
                <Network />
              </span>
              <div>
                <strong>{gateway.name}</strong>
                <small>逻辑网关 · {gateway.listeners.length} 个监听入口</small>
              </div>
              <strong>99.99%</strong>
              <span>P95 4 ms</span>
              <StatusBadge state={gateway.state} />
            </div>
          ))}
          {services.map((service) => {
            const detail =
              service.type === "HTTP"
                ? `${service.endpoints.length} 个端点`
                : (service.capabilities[0] ?? "尚未发现能力");
            return (
              <div className="component-row" key={service.id}>
                <span>{service.type === "MCP" ? <Wrench /> : <Server />}</span>
                <div>
                  <strong>{service.name}</strong>
                  <small>
                    {service.type === "MODEL"
                      ? "模型服务"
                      : service.type === "MCP"
                        ? "MCP 服务"
                        : "HTTP 服务"}{" "}
                    · {detail}
                  </small>
                </div>
                <strong>{service.successRate}</strong>
                <span>
                  {service.recovery && service.recovery.state !== "ready"
                    ? recoveryLabel(service.recovery.state)
                    : service.latency}
                </span>
                <StatusBadge state={service.state} />
              </div>
            );
          })}
        </article>
        <aside className="card alert-card">
          <header>
            <span className="eyebrow">运行告警</span>
            <h3>需要处理</h3>
          </header>
          {serviceWarnings.map((service, index) => (
            <div key={service.id}>
              <AlertTriangle />
              <p>
                <strong>
                  {service.name}
                  {service.type === "MODEL"
                    ? "响应变慢"
                    : "错误率升高"}
                </strong>
                <span>
                  {service.type === "MODEL"
                    ? service.recovery && service.recovery.state !== "ready"
                      ? `${recoveryLabel(service.recovery.state)}，${service.recovery.retryAt ? `${service.recovery.retryAt} 发起恢复探测` : "等待恢复探测"}`
                      : `首个 Token ${service.latency}，路由会按条件切换备用线路`
                    : `成功率 ${service.successRate}，健康端点继续承载流量`}
                </span>
                <small>
                  <Clock3 />
                  持续 {8 + index * 3} 分钟
                </small>
              </p>
            </div>
          ))}
          <footer>
            <CheckCircle2 />
            <span>
              <strong>今天已恢复 3 项</strong>
              <small>最近恢复：搜索工具服务连接</small>
            </span>
          </footer>
        </aside>
      </section>
      <section className="card proxy-card">
        <header className="card-header">
          <div>
            <span className="eyebrow">代理实例</span>
            <h3>在线状态</h3>
          </div>
        </header>
        {proxyInstances.map((instance) => (
          <div className="proxy-row" key={instance.id}>
            <span>
              <Network />
            </span>
            <div>
              <strong>{instance.id}</strong>
              <small>
                {instance.address} · {instance.zone}
              </small>
            </div>
            <code>{instance.version}</code>
            <span>配置 v{instance.activeConfigVersion}</span>
            <small>{instance.lastSeen}</small>
            <StatusBadge
              state={instance.state}
              label={instance.state === "healthy" ? "在线" : "离线"}
            />
          </div>
        ))}
      </section>
    </div>
  );
}

export function ReleasePage() {
  const {
    gateways,
    routes,
    services,
    certificates,
    policies,
    proxyInstances,
    currentVersion,
    candidateVersion,
    releaseHistory,
    retryRelease,
    simulateReleaseFailure,
    setSimulateReleaseFailure,
  } = usePrototype();
  const [selectedVersion, setSelectedVersion] = useState(
    releaseHistory[0]?.version ?? currentVersion,
  );
  const selected =
    releaseHistory.find((release) => release.version === selectedVersion) ??
    releaseHistory[0];
  const failedCount = releaseHistory.filter(
    (release) => release.state === "发布失败",
  ).length;
  const proxyConfigVersions = new Set(
    proxyInstances.map((instance) => instance.activeConfigVersion),
  ).size;
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="变更管理"
        title="配置发布"
        description="从声明变更到全部代理实例确认的完整交付记录"
        actions={
          <Link className="button button-secondary" to="/health">
            查看运行健康 <ChevronRight />
          </Link>
        }
      />
      <section
        className={`release-hero ${candidateVersion ? "is-publishing" : ""}`}
      >
        <span>{candidateVersion ? <RefreshCw /> : <CheckCircle2 />}</span>
        <div>
          <small>最近完整生效版本</small>
          <h2>
            v{currentVersion}
            {candidateVersion
              ? ` · v${candidateVersion} 正在确认`
              : " · 全部实例已确认"}
          </h2>
          <p>
            {candidateVersion
              ? "代理实例会逐个应用候选配置，全部确认后更新完整生效版本"
              : `${proxyInstances.length} 个代理实例配置一致`}
          </p>
        </div>
        <StatusBadge
          state={candidateVersion ? "pending" : "healthy"}
          label={candidateVersion ? "发布中" : "已生效"}
        />
      </section>
      <section className="metric-grid four">
        <Metric
          label="完整生效版本"
          value={`v${currentVersion}`}
          note="全部代理实例都已确认"
          tone="good"
        />
        <Metric
          label="待确认版本"
          value={candidateVersion ? `v${candidateVersion}` : "—"}
          note={candidateVersion ? "实例正在逐个确认" : "没有发布任务"}
        />
        <Metric
          label="代理一致性"
          value={proxyConfigVersions === 1 ? "一致" : "不一致"}
          note={`${proxyConfigVersions} 个配置版本`}
          tone={proxyConfigVersions === 1 ? "good" : "warning"}
        />
        <Metric
          label="发布失败"
          value={String(failedCount)}
          note="历史记录，可重试"
          tone={failedCount ? "warning" : "good"}
        />
      </section>
      <section className="release-layout">
        <article className="card release-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">版本记录</span>
              <h3>配置交付</h3>
            </div>
            <label className="demo-switch">
              <input
                type="checkbox"
                checked={simulateReleaseFailure}
                onChange={(event) =>
                  setSimulateReleaseFailure(event.target.checked)
                }
              />
              下次发布模拟失败
            </label>
          </header>
          <div className="release-history">
            {releaseHistory.map((release) => (
              <button
                type="button"
                className={
                  selected.version === release.version ? "is-selected" : ""
                }
                key={release.version}
                onClick={() => setSelectedVersion(release.version)}
              >
                <span>
                  {release.state === "发布中" ? (
                    <Clock3 />
                  ) : release.state === "发布失败" ? (
                    <AlertTriangle />
                  ) : (
                    <CheckCircle2 />
                  )}
                </span>
                <p>
                  <strong>版本 {release.version}</strong>
                  <small>
                    {release.summary} · {release.resources}
                  </small>
                </p>
                <time>{release.time}</time>
                <StatusBadge
                  state={
                    release.state === "发布中"
                      ? "pending"
                      : release.state === "发布失败"
                        ? "error"
                        : "healthy"
                  }
                  label={release.state}
                />
              </button>
            ))}
          </div>
        </article>
        <aside className="card release-detail">
          <header>
            <span className="eyebrow">版本 v{selected.version}</span>
            <h3>{selected.summary}</h3>
            <StatusBadge
              state={
                selected.state === "发布中"
                  ? "pending"
                  : selected.state === "发布失败"
                    ? "error"
                    : "healthy"
              }
              label={selected.state}
            />
          </header>
          <div className="sync-progress">
            <span>
              <strong>
                {selected.syncedInstances} / {selected.totalInstances}
              </strong>{" "}
              个实例已确认
            </span>
            <i>
              <b
                style={{
                  width: `${(selected.syncedInstances / selected.totalInstances) * 100}%`,
                }}
              />
            </i>
          </div>
          {selected.error ? (
            <div className="release-error">
              <AlertTriangle />
              <div>
                <strong>未形成完整生效版本</strong>
                <p>{selected.error}</p>
              </div>
            </div>
          ) : null}
          <section>
            <h4>变更影响</h4>
            {selected.changes.map((change) => (
              <p key={change}>
                <History />
                {change}
              </p>
            ))}
          </section>
          <dl className="definition-list">
            <div>
              <dt>生效策略</dt>
              <dd>实例逐个应用，全部确认后标记发布成功</dd>
            </div>
            <div>
              <dt>失败处理</dt>
              <dd>展示版本差异，可重试使实例重新一致</dd>
            </div>
            <div>
              <dt>声明资源</dt>
              <dd>
                {gateways.length +
                  routes.length +
                  services.length +
                  certificates.length +
                  policies.length}{" "}
                项
              </dd>
            </div>
          </dl>
          {selected.state === "发布失败" ? (
            <PrimaryButton onClick={() => retryRelease(selected.version)}>
              <RefreshCw />
              重试发布
            </PrimaryButton>
          ) : null}
        </aside>
      </section>
    </div>
  );
}

export function AuditPage() {
  const { auditRecords } = usePrototype();
  const [query, setQuery] = useState("");
  const [resourceType, setResourceType] = useState<
    "ALL" | AuditRecord["resourceType"]
  >("ALL");
  const visible = auditRecords.filter(
    (record) =>
      (resourceType === "ALL" || record.resourceType === resourceType) &&
      `${record.actor}${record.action}${record.resource}${record.detail}`.includes(
        query,
      ),
  );
  const exportRecords = () => {
    const rows = [
      ["时间", "操作者", "动作", "资源", "详情"],
      ...visible.map((record) => [
        record.time,
        record.actor,
        record.action,
        record.resource,
        record.detail,
      ]),
    ];
    const csv = rows
      .map((row) =>
        row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(","),
      )
      .join("\n");
    const url = URL.createObjectURL(
      new Blob([`\ufeff${csv}`], { type: "text/csv;charset=utf-8" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "ingate-audit-2026-08-12.csv";
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="变更管理"
        title="审计日志"
        description="配置、权限与凭据操作"
        actions={
          <button
            className="button button-secondary"
            type="button"
            onClick={exportRecords}
            disabled={!visible.length}
          >
            <Download />
            导出日志
          </button>
        }
      />
      <section className="card audit-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索操作者、动作或资源"
          />
          <FilterSelect
            label="资源类型"
            value={resourceType}
            onChange={setResourceType}
            options={[
              { value: "ALL", label: "全部", count: auditRecords.length },
              { value: "网关", label: "网关" },
              { value: "路由", label: "路由" },
              { value: "服务", label: "服务" },
              { value: "调用方", label: "调用方" },
              { value: "流量策略", label: "策略" },
            ]}
          />
        </header>
        <div className="audit-date">
          <span>最近 180 天 · 当前显示 {visible.length} 项</span>
          <i />
        </div>
        {visible.length ? (
          visible.map((record, index) => (
            <article className="audit-row" key={record.id}>
              <span>
                <FileClock />
              </span>
              <div>
                <div>
                  <strong>{record.action}</strong>
                  <StatusBadge
                    state={record.result === "成功" ? "healthy" : "error"}
                    label={record.result}
                  />
                </div>
                <p>{record.detail}</p>
                <small>
                  {record.resourceType} · {record.resource}
                </small>
              </div>
              <div>
                <strong>{record.actor}</strong>
                <time>{record.time}</time>
              </div>
              {index < visible.length - 1 ? <i /> : null}
            </article>
          ))
        ) : (
          <EmptyState
            title="没有匹配的审计记录"
            description="请调整搜索或资源类型。"
          />
        )}
      </section>
    </div>
  );
}
