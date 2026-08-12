import {
  AlertTriangle,
  ArrowRight,
  Bot,
  Braces,
  CheckCircle2,
  CircleDollarSign,
  Clock3,
  ShieldCheck,
} from "lucide-react";
import { Link } from "react-router-dom";
import {
  Metric,
  PageHeader,
  RouteTypeBadge,
  StatusBadge,
} from "../components/ui";
import { usePrototype } from "../prototype-context";

export function OverviewPage() {
  const {
    services,
    callers,
    requests,
    currentVersion,
    candidateVersion,
    proxyInstances,
  } = usePrototype();
  const runtimeAlerts = services.filter(
    (service) =>
      service.state === "warning" || service.state === "error",
  ).length;
  const quotaAlerts = callers
    .flatMap((caller) => caller.quotas)
    .filter((quota) => quota.used / quota.limit >= 0.9).length;

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="生产环境"
        title="概览"
        actions={
          <Link className="button button-primary" to="/services">
            接入服务 <ArrowRight />
          </Link>
        }
      />

      <section className="overview-hero">
        <div className="hero-copy">
          <span className="hero-kicker">
            <i />
            完整生效配置 v{currentVersion}
            {candidateVersion
              ? ` · v${candidateVersion} 发布中`
              : " · 全部实例已确认"}
          </span>
          <h2>流量稳定，{runtimeAlerts} 项服务需要关注</h2>
          <p>Anthropic 公网首个 Token 响应变慢 · 文件服务错误率升高</p>
          <div className="hero-actions">
            <Link to="/health" className="button button-light">
              查看运行告警
            </Link>
            <Link to="/requests" className="button button-ghost">
              排查请求
            </Link>
          </div>
        </div>
        <div className="traffic-orbit is-health" aria-label="各类流量成功率">
          <div className="orbit-ring ring-one" />
          <div className="orbit-ring ring-two" />
          <div className="orbit-core">
            <CheckCircle2 />
            <strong>99.85%</strong>
            <span>请求成功率</span>
          </div>
          <span className="orbit-label orbit-api">
            <Braces />
            API 99.91%
          </span>
          <span className="orbit-label orbit-ai">
            <Bot />
            AI 99.72%
          </span>
        </div>
      </section>

      <section className="metric-grid four">
        <Metric
          label="API 请求"
          value="98.3K"
          note="成功率 99.91% · P95 124 ms"
        />
        <Metric
          label="AI 请求"
          value="44.4K"
          note="50.8M Token · 预估 ¥1,785"
        />
        <Metric
          label="异常请求"
          value="257"
          note="服务异常 66 · 策略拒绝 191"
          tone="warning"
        />
        <Metric
          label="策略拒绝"
          value="191"
          note="用量上限 124 · 访问 49 · 限流 18"
          tone="warning"
        />
      </section>

      <section className="card quick-start-card">
        <header className="card-header">
          <div>
            <span className="eyebrow">首次接入</span>
            <h3>从接入到验证，一条链路完成</h3>
          </div>
          <span>4 个步骤</span>
        </header>
        <div className="quick-start-steps">
          <Link to="/services">
            <i>1</i>
            <div>
              <strong>接入服务</strong>
              <small>配置 HTTP、模型或 MCP 连接</small>
            </div>
            <span>{services.length} 个</span>
          </Link>
          <Link to="/routes">
            <i>2</i>
            <div>
              <strong>发布路由</strong>
              <small>决定客户端如何访问服务</small>
            </div>
            <ArrowRight />
          </Link>
          <Link to="/callers">
            <i>3</i>
            <div>
              <strong>授权调用方</strong>
              <small>签发密钥并选择可访问能力</small>
            </div>
            <ArrowRight />
          </Link>
          <Link to="/requests">
            <i>4</i>
            <div>
              <strong>发送并排障</strong>
              <small>复制 curl，查看单次执行过程</small>
            </div>
            <ArrowRight />
          </Link>
        </div>
      </section>

      <section className="overview-grid">
        <article className="card flow-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">今日流量</span>
              <h3>高频路由</h3>
            </div>
            <Link to="/analysis">
              查看分析 <ArrowRight />
            </Link>
          </header>
          <div className="flow-lanes">
            <TopRoute
              type="API"
              route="订单查询 API"
              service="订单服务"
              usage="58.4K 请求"
            />
            <TopRoute
              type="AI"
              route="生产 AI 路由"
              service="通义千问生产"
              usage="28.4K 请求"
            />
            <TopRoute
              type="API"
              route="客户资料 API"
              service="客户中心"
              usage="33.8K 请求"
            />
          </div>
        </article>

        <article className="card attention-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">运行告警</span>
              <h3>{runtimeAlerts} 项需要处理</h3>
            </div>
            <Link to="/health">查看全部</Link>
          </header>
          <Link to="/health" className="attention-row">
            <span className="attention-icon warning">
              <Clock3 />
            </span>
            <div>
              <strong>Anthropic 公网响应变慢</strong>
              <p>首个 Token P95 2.8 秒，备用线路可用</p>
            </div>
            <StatusBadge state="warning" />
          </Link>
          <Link to="/health" className="attention-row">
            <span className="attention-icon warning">
              <AlertTriangle />
            </span>
            <div>
              <strong>文件服务错误率升高</strong>
              <p>成功率 99.42%，已持续 11 分钟</p>
            </div>
            <StatusBadge state="warning" />
          </Link>
          <div className="attention-foot">
            <CheckCircle2 />
            {proxyInstances.length} 个代理实例运行正常
          </div>
        </article>
      </section>

      <section className="overview-grid resources-grid">
        <article className="card resource-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">用量与策略</span>
              <h3>{quotaAlerts + 1} 项运行提醒</h3>
            </div>
            <Link to="/usage">查看用量</Link>
          </header>
          <Link
            to="/usage?caller=caller-automation"
            className="attention-row"
          >
            <span className="attention-icon error">
              <CircleDollarSign />
            </span>
            <div>
              <strong>内部自动化 Token 用量上限已用尽</strong>
              <p>20M / 20M，本月后续 AI 请求已拒绝</p>
            </div>
            <StatusBadge state="error" label="已用尽" />
          </Link>
          <Link to="/requests?query=req_9Ab4Xe" className="attention-row">
            <span className="attention-icon warning">
              <ShieldCheck />
            </span>
            <div>
              <strong>办公网访问限制</strong>
              <p>今天拒绝 42 次非可信来源请求</p>
            </div>
            <StatusBadge state="healthy" label="已拦截" />
          </Link>
        </article>
        <article className="card recent-card">
          <header className="card-header">
            <div>
              <span className="eyebrow">实时流量</span>
              <h3>最近请求</h3>
            </div>
            <Link to="/requests">查看全部</Link>
          </header>
          <div className="recent-list">
            {requests.slice(0, 4).map((request) => (
              <Link key={request.id} to={`/requests?query=${request.id}`}>
                <RouteTypeBadge type={request.type} />
                <div>
                  <strong>{request.route}</strong>
                  <span>
                    {request.caller} · {request.request}
                  </span>
                </div>
                <div>
                  <strong>{request.code}</strong>
                  <span>{request.latency}</span>
                </div>
              </Link>
            ))}
          </div>
        </article>
      </section>
    </div>
  );
}

function TopRoute({
  type,
  route,
  service,
  usage,
}: {
  type: "API" | "AI" | "MCP";
  route: string;
  service: string;
  usage: string;
}) {
  return (
    <div className="flow-lane">
      <RouteTypeBadge type={type} />
      <div className="route-node">
        <small>路由</small>
        <strong>{route}</strong>
      </div>
      <div>
        <small>目标服务</small>
        <strong>{service}</strong>
      </div>
      <div className="flow-value">
        <small>今日用量</small>
        <strong>{usage}</strong>
      </div>
    </div>
  );
}
