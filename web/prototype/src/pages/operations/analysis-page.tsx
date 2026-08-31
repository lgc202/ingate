import { ChevronRight } from "lucide-react";
import { useState } from "react";
import {
  Link,
  useSearchParams,
} from "react-router-dom";
import type { TrafficType } from "../../data";
import {
  FilterTabs,
  Metric,
  PageHeader,
  RouteTypeBadge,
} from "../../components/ui";
import { usePrototype } from "../../prototype-context";

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
