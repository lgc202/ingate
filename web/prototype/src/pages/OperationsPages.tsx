import {
  AlertTriangle,
  CheckCircle2,
  ChevronRight,
  Clock3,
  CircleDollarSign,
  Download,
  FileClock,
  ShieldCheck,
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
  RouteTypeBadge,
  SearchField,
  StatusBadge,
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
  costedAttempts: number;
  cost: number;
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

const modelCostFacts: ModelCostFact[] = [
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
          `${request.id}${request.caller}${request.route}${request.request}${request.target}${request.result}${request.code}${request.attempts.map((attempt) => `${attempt.service}${attempt.code}${attempt.error ?? ""}`).join("")}${request.decisions.map((decision) => `${decision.name}${decision.detail}`).join("")}`
            .toLowerCase()
            .includes(query.toLowerCase())
        );
      }),
    [filter, query, requests],
  );

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="观测分析"
        title="请求记录"
        description="查找单次请求的路由、策略与服务执行结果"
      />
      <section className="metric-grid four">
        <Metric label="API 请求" value="98.3K" note="成功率 99.91%" />
        <Metric label="AI 请求" value="44.4K" note="50.8M Token" />
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
        请求正文记录：关闭
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
              { value: "API", label: "API 路由" },
              { value: "AI", label: "AI 路由" },
              { value: "MCP", label: "MCP 路由" },
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
                  <RouteTypeBadge type={request.type} />
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
  const relatedService = request.attempts[0]?.service;
  const quotaRejected = request.decisions.some((decision) =>
    decision.name.includes("用量"),
  );
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
      {quotaRejected ? (
        <div className="detail-jump">
          <div>
            <strong>该调用方的用量上限已触发</strong>
            <span>查看本期使用量，必要时调整调用方上限</span>
          </div>
          <Link
            className="button button-secondary"
            to={`/usage?caller=${request.caller === "内部自动化" ? "caller-automation" : "ALL"}`}
          >
            查看调用方用量 <ChevronRight />
          </Link>
        </div>
      ) : relatedService ? (
        <div className="detail-jump">
          <div>
            <strong>{relatedService}</strong>
            <span>查看服务端点与同类流量异常</span>
          </div>
          <Link
            className="button button-secondary"
            to={`/services?query=${encodeURIComponent(relatedService)}`}
          >
            查看服务 <ChevronRight />
          </Link>
        </div>
      ) : null}
      <section className="service-attempts">
        <header>
          <div>
            <span className="eyebrow">服务调用</span>
            <h3>服务调用</h3>
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
          <span className="eyebrow">网关判定</span>
          <h3>认证、策略与路由结果</h3>
        </header>
        {request.decisions.map((step, index) => (
          <div className="timeline-step" key={`${step.name}-${index}`}>
            <span className={`timeline-dot state-${step.state}`}>
              {index + 1}
            </span>
            <div>
              <strong>{step.name}</strong>
              <p>{step.detail}</p>
            </div>
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
  const { routes, services } = usePrototype();
  const [params] = useSearchParams();
  const requestedRoute = params.get("route");
  const requestedService = params.get("service");
  const service = services.find((item) => item.name === requestedService);
  const requestedType =
    routes.find((route) => route.name === requestedRoute)?.type ??
    (service?.type === "MODEL"
      ? "AI"
      : service?.type === "MCP"
        ? "MCP"
        : service
          ? "API"
          : undefined);
  const [filter, setFilter] = useState<AnalysisFilter>(
    requestedType === "AI" || requestedType === "MCP" ? requestedType : "API",
  );
  const view = analysisViews[filter];
  const relatedRouteNames = new Set(
    service
      ? routes
          .filter((route) =>
            route.targets.some((target) => target.serviceID === service.id),
          )
          .map((route) => route.name)
      : [],
  );
  const visibleRows = requestedRoute
    ? view.rows.filter((row) => row[0] === requestedRoute)
    : service
      ? view.rows.filter(
          (row) =>
            relatedRouteNames.has(row[0]) ||
            row[2] === service.name ||
            (filter === "MCP" && service.capabilities.includes(row[0])),
        )
      : view.rows;
  const serviceMetrics = service
    ? [
        ["关联路由", String(relatedRouteNames.size), "当前服务"],
        ["成功率", service.successRate, "过去 12 小时"],
        [service.type === "MODEL" ? "首个 Token" : "P95 延迟", service.latency, "过去 12 小时"],
        [
          "异常端点",
          String(service.endpoints.filter((endpoint) => endpoint.state !== "healthy").length),
          `${service.endpoints.filter((endpoint) => endpoint.state === "healthy").length}/${service.endpoints.length} 健康`,
        ],
      ]
    : view.metrics;
  const focusDescription = requestedRoute
    ? `已定位到路由“${requestedRoute}”的运行数据`
    : service
      ? `已定位到服务“${service.name}”承载的流量`
      : "按 API、AI 与 MCP 查看吞吐、成功率和延迟";
  return (
    <div className="page-stack page-enter">
      <PageHeader eyebrow="观测分析" title="流量分析" description={focusDescription} />
      <FilterTabs
        value={filter}
        onChange={setFilter}
        options={[
          { value: "API", label: "API 路由" },
          { value: "AI", label: "AI 路由" },
          { value: "MCP", label: "MCP 路由" },
        ]}
      />
      <section className="metric-grid four">
        {serviceMetrics.map(([label, value, note]) => (
          <Metric key={label} label={label} value={value} note={note} />
        ))}
      </section>
      <section className="analysis-layout">
        <article className="card chart-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">过去 12 小时</span>
              <h3>{service ? `${service.name}请求趋势` : `${view.title}趋势`}</h3>
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
            <span className="eyebrow">{service ? "服务范围" : "构成"}</span>
            <h3>{service ? "端点状态" : view.composition.title}</h3>
          </header>
          <div
            className="donut"
            style={{
              background: `radial-gradient(circle,#fffef9 0 52%,transparent 53%), conic-gradient(${service ? `var(--teal) 0 ${(service.endpoints.filter((endpoint) => endpoint.state === "healthy").length / service.endpoints.length) * 100}%, var(--red) 0 100%` : view.composition.gradient})`,
            }}
          >
            <div>
              <strong>{service ? service.endpoints.length : view.composition.total}</strong>
              <span>{service ? "服务端点" : view.composition.label}</span>
            </div>
          </div>
          {(service
            ? [
                ["健康", String(service.endpoints.filter((endpoint) => endpoint.state === "healthy").length), "api"],
                ["异常", String(service.endpoints.filter((endpoint) => endpoint.state !== "healthy").length), "mcp"],
              ]
            : view.composition.segments
          ).map(([label, value, tone]) => (
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
            to={`/requests?query=${encodeURIComponent(service?.name ?? visibleRows[0]?.[0] ?? filter)}`}
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
            to={`/requests?query=${encodeURIComponent(service?.name ?? row[0])}`}
            key={row[0]}
          >
            {row.map((cell, index) =>
              index === 1 ? (
                <RouteTypeBadge
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

function QuotaUsageView({
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

function CostUsageView({
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
          <Link to="/requests?query=AI">
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

export function AuditPage() {
  const { auditRecords } = usePrototype();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<AuditRecord | null>(null);
  const [result, setResult] = useState<"ALL" | AuditRecord["result"]>("ALL");
  const [resourceType, setResourceType] = useState<
    "ALL" | AuditRecord["resourceType"]
  >("ALL");
  const visible = auditRecords.filter(
    (record) =>
      (resourceType === "ALL" || record.resourceType === resourceType) &&
      (result === "ALL" || record.result === result) &&
      `${record.actor}${record.action}${record.resource}${record.detail}`.includes(
        query,
      ),
  );
  const exportRecords = () => {
    const rows = [
      ["记录编号", "时间", "操作者", "动作", "资源", "结果", "详情"],
      ...visible.map((record) => [
        record.id,
        record.time,
        record.actor,
        record.action,
        record.resource,
        record.result,
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
        eyebrow="系统管理"
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
          <FilterSelect
            label="操作结果"
            value={result}
            onChange={setResult}
            options={[
              { value: "ALL", label: "全部结果" },
              { value: "成功", label: "成功" },
              { value: "失败", label: "失败" },
            ]}
          />
        </header>
        <div className="audit-date">
          <span>当前记录 · {visible.length} 项</span>
          <i />
        </div>
        {visible.length ? (
          visible.map((record, index) => (
            <button
              className="audit-row"
              type="button"
              onClick={() => setSelected(record)}
              key={record.id}
            >
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
            </button>
          ))
        ) : (
          <EmptyState
            title="没有匹配的审计记录"
            description="请调整搜索或资源类型。"
          />
        )}
      </section>
      {selected ? (
        <Drawer
          title={selected.action}
          description={`${selected.actor} · ${selected.time}`}
          onClose={() => setSelected(null)}
        >
          <div className="detail-hero">
            <span><FileClock /></span>
            <div>
              <StatusBadge state={selected.result === "成功" ? "healthy" : "error"} label={selected.result} />
              <h3>{selected.resource}</h3>
              <p>{selected.resourceType} · 记录编号 {selected.id}</p>
            </div>
          </div>
          <section className="detail-section">
            <header><h3>变更摘要</h3></header>
            <div className="detail-line">
              <span><FileClock /></span>
              <div><strong>{selected.detail}</strong><small>敏感凭据不会写入审计内容</small></div>
              <span>{selected.time}</span>
            </div>
          </section>
        </Drawer>
      ) : null}
    </div>
  );
}
