import {
  AlertTriangle,
  CheckCircle2,
  ChevronRight,
  ShieldCheck,
} from "lucide-react";
import {
  useMemo,
  useState,
} from "react";
import {
  Link,
  useSearchParams,
} from "react-router-dom";
import type {
  RequestRecord,
  TrafficType,
} from "../../data";
import {
  Drawer,
  EmptyState,
  FilterTabs,
  PageHeader,
  RouteTypeBadge,
  SearchField,
  StatusBadge,
} from "../../components/ui";
import { usePrototype } from "../../prototype-context";

type RequestFilter = "ALL" | TrafficType | "ERROR";
export function RequestPage() {
  const { requests } = usePrototype();
  const [params] = useSearchParams();
  const initialQuery = params.get("query") ?? "";
  const routeID = params.get("route");
  const serviceID = params.get("service");
  const callerID = params.get("caller");
  const gatewayID = params.get("gateway");
  const requestedResult = params.get("result");
  const requestedType = params.get("type");
  const [query, setQuery] = useState(initialQuery);
  const [filter, setFilter] = useState<RequestFilter>(
    requestedType === "API" || requestedType === "AI" || requestedType === "MCP"
      ? requestedType
      : "ALL",
  );
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
        const matchesContext =
          (!routeID || request.routeID === routeID) &&
          (!serviceID ||
            request.attempts.some((attempt) => attempt.serviceID === serviceID)) &&
          (!callerID || request.callerID === callerID) &&
          (!gatewayID || request.gatewayID === gatewayID) &&
          (!requestedResult || request.result === requestedResult);
        const actualRequest = `${request.method} ${request.host}${request.path}${request.query ? `?${request.query}` : ""}`;
        return (
          matchesType &&
          matchesContext &&
          `${request.id}${actualRequest}${request.detail ?? ""}${request.clientIP}${request.gateway}${request.caller}${request.route}${request.target}${request.result}${request.code}${request.attempts.map((attempt) => `${attempt.service}${attempt.endpoint}${attempt.code}${attempt.error ?? ""}`).join("")}${request.decisions.map((decision) => `${decision.name}${decision.detail}`).join("")}`
            .toLowerCase()
            .includes(query.toLowerCase())
        );
      }),
    [callerID, filter, gatewayID, query, requestedResult, requests, routeID, serviceID],
  );
  const contextLabel = routeID
    ? `路由：${requests.find((request) => request.routeID === routeID)?.route ?? routeID}`
    : serviceID
      ? `服务：${requests.flatMap((request) => request.attempts).find((attempt) => attempt.serviceID === serviceID)?.service ?? serviceID}`
      : callerID
        ? `调用方：${requests.find((request) => request.callerID === callerID)?.caller ?? callerID}`
        : gatewayID
          ? `网关：${requests.find((request) => request.gatewayID === gatewayID)?.gateway ?? gatewayID}`
          : requestedResult
            ? `结果：${requestedResult}`
            : "";

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="观测分析"
        title="请求记录"
        description="逐次查看进入网关的真实请求、匹配过程与服务响应"
      />
      <p className="privacy-note">
        <ShieldCheck />
        不持久化请求正文，仅记录排障所需的请求元数据
      </p>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索请求编号、Host、Path、调用方或服务"
          />
          <FilterTabs
            value={filter}
            onChange={setFilter}
            options={[
              { value: "ALL", label: "全部", count: requests.length },
              { value: "API", label: "API 请求" },
              { value: "AI", label: "AI 请求" },
              { value: "MCP", label: "MCP 调用" },
              { value: "ERROR", label: "异常" },
            ]}
          />
        </header>
        {contextLabel ? (
          <div className="request-context-filter">
            <span>当前范围</span>
            <strong>{contextLabel}</strong>
            <Link to="/requests">清除筛选</Link>
          </div>
        ) : null}
        <div className="table-head request-columns">
          <span>时间 / 请求编号</span>
          <span>实际请求</span>
          <span>响应</span>
          <span>调用身份</span>
          <span>匹配与转发</span>
          <span>总耗时</span>
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
              <div className="request-address">
                <span>
                  <strong className={`method method-${request.method.toLowerCase()}`}>
                    {request.method}
                  </strong>
                  <code>{request.host}</code>
                </span>
                <strong>{request.path}{request.query ? `?${request.query}` : ""}</strong>
                {request.detail ? <small>{request.detail}</small> : null}
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
              <div className="request-identity">
                <strong>{request.caller}</strong>
                <small>{request.clientIP}</small>
              </div>
              <div className="request-route">
                <span>
                  <RouteTypeBadge type={request.type} />
                  <strong>{request.route}</strong>
                </span>
                <small>
                  → {request.attempts.at(-1)?.service ?? "未访问服务"}
                  {request.attempts.at(-1)?.endpoint ? ` · ${request.attempts.at(-1)?.endpoint}` : " · 在网关内结束"}
                </small>
              </div>
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

export function RequestDetail({
  request,
  onClose,
}: {
  request: RequestRecord;
  onClose: () => void;
}) {
  const failed = request.result === "失败" || request.result === "策略拒绝";
  const relatedService = request.attempts.at(-1);
  const quotaRejected = request.decisions.some((decision) =>
    decision.name.includes("用量"),
  );
  const usageLabel =
    request.type === "AI"
      ? "Token 用量"
      : request.type === "MCP"
        ? "工具调用"
        : "响应流量";
  const requestURL = `https://${request.host}${request.path}${request.query ? `?${request.query}` : ""}`;
  const facts = [
    ["实际请求", `${request.method} ${request.host}${request.path}${request.query ? `?${request.query}` : ""}`],
    ["来源", `${request.caller} · ${request.clientIP}`],
    ["匹配入口", `${request.gateway} · ${request.route}`],
    ["实际目标", request.target],
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
          <code>{request.method} {requestURL}</code>
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
            <strong>{relatedService.service}</strong>
            <span>{relatedService.endpoint} · 查看端点与同类请求</span>
          </div>
          <Link
            className="button button-secondary"
            to={`/services?query=${encodeURIComponent(relatedService.service)}`}
          >
            查看服务 <ChevronRight />
          </Link>
        </div>
      ) : null}
      <section className="execution-timeline request-flow">
        <header>
          <span className="eyebrow">处理链路</span>
          <h3>这次请求经过了什么</h3>
        </header>
        {[
          {
            name: "接收请求",
            detail: `${request.method} ${request.host}${request.path}${request.query ? `?${request.query}` : ""} · 来源 ${request.clientIP}`,
            state: "healthy" as const,
          },
          {
            name: "匹配入口与路由",
            detail: `${request.gateway} → ${request.route}`,
            state: "healthy" as const,
          },
          ...request.decisions,
          ...request.attempts.map((attempt, index) => ({
            name: `服务调用${request.attempts.length > 1 ? ` ${index + 1}` : ""} · ${attempt.service}`,
            detail: `${attempt.endpoint} · ${attempt.code} · ${attempt.latency}${attempt.error ? ` · ${attempt.error}` : ""}`,
            state: attempt.state,
          })),
          {
            name: "返回响应",
            detail: `${request.code} · ${request.result} · 总耗时 ${request.latency}`,
            state: failed ? ("error" as const) : request.result === "主备切换" ? ("warning" as const) : ("healthy" as const),
          },
        ].map((step, index) => (
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
      {request.attempts.length ? (
        <section className="service-attempts">
          <header>
            <div><span className="eyebrow">服务明细</span><h3>端点尝试</h3></div>
            <span>{request.attempts.length} 次尝试</span>
          </header>
          {request.attempts.map((attempt, index) => (
            <article className={`attempt-card state-${attempt.state}`} key={`${attempt.service}-${index}`}>
              <span className="attempt-index">{index + 1}</span>
              <div className="attempt-main">
                <header>
                  <div><strong>{attempt.service}</strong><span>{attempt.endpoint} · {[attempt.provider, attempt.actualModel].filter(Boolean).join(" · ") || "HTTP 服务"}</span></div>
                  <StatusBadge state={attempt.state} label={`${attempt.code} · ${attempt.result}`} />
                </header>
                <div className="attempt-facts">
                  <span><small>服务耗时</small><strong>{attempt.latency}</strong></span>
                  {attempt.ttft ? <span><small>首个 Token</small><strong>{attempt.ttft}</strong></span> : null}
                  {attempt.inputTokens !== undefined ? <span><small>输入 Token</small><strong>{attempt.inputTokens.toLocaleString("zh-CN")}</strong></span> : null}
                  {attempt.outputTokens !== undefined ? <span><small>输出 Token</small><strong>{attempt.outputTokens.toLocaleString("zh-CN")}</strong></span> : null}
                  {attempt.cachedTokens !== undefined ? <span><small>缓存 Token</small><strong>{attempt.cachedTokens.toLocaleString("zh-CN")}</strong></span> : null}
                  {attempt.cost ? <span><small>本次成本</small><strong>{attempt.cost}</strong></span> : null}
                </div>
              </div>
            </article>
          ))}
        </section>
      ) : (
        <div className="attempt-empty"><ShieldCheck /><div><strong>未访问服务</strong><span>请求在网关内被拒绝，没有向后端发送流量</span></div></div>
      )}
      <p className="privacy-note">
        <ShieldCheck />
        未持久化请求正文；AI 模型名和 MCP 工具名作为结构化元数据保留
      </p>
    </Drawer>
  );
}
