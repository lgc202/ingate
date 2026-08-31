import {
  ChevronRight,
  Server,
  ShieldCheck,
} from "lucide-react";
import { Link } from "react-router-dom";
import {
  ConfigBadge,
  Drawer,
  EmptyState,
  RouteTypeBadge,
  StatusBadge,
  Topology,
} from "../../components/ui";
import type {
  GatewayRoute,
  Policy,
} from "../../data";
import { hostRewriteLabel } from "./route-model";

export function RouteDetail({
  route,
  policies,
  authorizedCallerCount,
  onClose,
}: {
  route: GatewayRoute;
  policies: Policy[];
  authorizedCallerCount: number;
  onClose: () => void;
}) {
  const accessMode = route.accessMode ?? "API Key";
  const appliedPolicies = policies.filter((policy) =>
    policy.targets.some(
      (target) =>
        (target.kind === "路由" && target.id === route.id) ||
        (target.kind === "网关" && target.id === route.gatewayID),
    ),
  );
  return (
    <Drawer
      title={route.name}
      description={`${route.host}${route.path}`}
      onClose={onClose}
      width="wide"
    >
      <div className="drawer-actions">
        <RouteTypeBadge type={route.type} />
        <ConfigBadge state={route.configState} />
      </div>
      <Topology
        gateway={route.gatewayName}
        route={route.name}
        service={route.targets[0]?.serviceName ?? "未选择"}
        detail={route.targets[0]?.detail}
      />
      {accessMode === "API Key" ? (
        <div className="detail-jump">
          <div>
            <strong>
              {authorizedCallerCount
                ? `${authorizedCallerCount} 个调用方可访问`
                : "还没有调用方可访问"}
            </strong>
            <span>为调用方授权后可签发密钥并复制调用示例</span>
          </div>
          <Link className="button button-secondary" to="/callers">
            管理调用方 <ChevronRight />
          </Link>
        </div>
      ) : null}
      <div className="detail-jump detail-jump-actions">
        <div><strong>路由运行数据</strong><span>查看聚合趋势，或下钻到这个路由匹配的每一次实际请求</span></div>
        <span>
          <Link className="button button-secondary" to={`/requests?route=${route.id}`}>查看请求 <ChevronRight /></Link>
          <Link className="button button-secondary" to={`/analysis?route=${encodeURIComponent(route.name)}`}>查看趋势 <ChevronRight /></Link>
        </span>
      </div>
      <section className="detail-section">
        <header>
          <h3>匹配与能力</h3>
        </header>
        <dl className="definition-list">
          <div>
            <dt>访问方式</dt>
            <dd>{accessMode}</dd>
          </div>
          <div>
            <dt>匹配规则</dt>
            <dd>{route.match}</dd>
          </div>
          <div>
            <dt>
              {route.type === "AI"
                ? "客户端模型名"
                : route.type === "MCP"
                  ? "开放工具"
                  : "对外接口"}
            </dt>
            <dd>
              {route.published.map((item) => (
                <code key={item}>{item}</code>
              ))}
            </dd>
          </div>
        </dl>
      </section>
      <section className="detail-section">
        <header>
          <h3>目标服务</h3>
          <span>{route.forwarding.strategy}</span>
        </header>
        {route.targets.map((target) => (
          <div
            className="detail-line"
            key={`${target.serviceID}-${target.detail}-${target.role}`}
          >
            <span>
              <Server />
            </span>
            <div>
              <strong>{target.serviceName}</strong>
              <small>
                {target.publishedCapability
                  ? `${target.publishedCapability} → ${target.detail}`
                  : target.detail}
              </small>
            </div>
            <span>
              {target.role}
              {target.weight ? ` · ${target.weight}%` : ""}
            </span>
            <StatusBadge state="healthy" />
          </div>
        ))}
      </section>
      <section className="detail-section">
        <header>
          <h3>转发控制</h3>
        </header>
        <dl className="definition-list">
          <div>
            <dt>请求超时</dt>
            <dd>{route.forwarding.timeout}</dd>
          </div>
          <div>
            <dt>失败重试</dt>
            <dd>{route.forwarding.retries} 次</dd>
          </div>
          {route.type === "API" ? (
            <>
              <div>
                <dt>路径处理</dt>
                <dd>{route.forwarding.pathHandling}</dd>
              </div>
              <div>
                <dt>转发主机名</dt>
                <dd>{hostRewriteLabel(route.forwarding)}</dd>
              </div>
            </>
          ) : null}
        </dl>
      </section>
      {route.type === "API" ? (
        <section className="detail-section">
          <header>
            <h3>请求匹配与改写</h3>
          </header>
          <dl className="definition-list">
            {route.conditions?.map((condition) => (
              <div key={`${condition.kind}-${condition.name}`}>
                <dt>{condition.kind} 条件</dt>
                <dd>
                  <code>{condition.name}</code>
                  {condition.mode === "精确匹配"
                    ? ` = ${condition.value}`
                    : " 存在"}
                </dd>
              </div>
            ))}
            <div>
              <dt>路径改写</dt>
              <dd>{route.rewrite?.pathPrefix || "不改写"}</dd>
            </div>
            <div>
              <dt>新增请求头</dt>
              <dd>
                {route.rewrite?.requestHeaders.length
                  ? route.rewrite.requestHeaders.map((header) => (
                      <code key={header.name}>
                        {header.name}: {header.value}
                      </code>
                    ))
                  : "无"}
              </dd>
            </div>
            <div>
              <dt>异常实例</dt>
              <dd>
                连续失败{" "}
                {route.forwarding.circuitBreaker?.consecutiveFailures ?? 5}{" "}
                次后摘除{" "}
                {route.forwarding.circuitBreaker?.ejectionTime ?? "30 秒"}
              </dd>
            </div>
          </dl>
        </section>
      ) : route.forwarding.failoverOn?.length ? (
        <section className="detail-section">
          <header>
            <h3>故障切换条件</h3>
          </header>
          <div className="chip-list">
            {route.forwarding.failoverOn.map((reason) => (
              <span key={reason}>
                <ShieldCheck />
                {reason}
              </span>
            ))}
          </div>
        </section>
      ) : null}
      <section className="detail-section">
        <header>
          <h3>已应用策略</h3>
        </header>
        {appliedPolicies.length ? (
          <div className="chip-list">
            {appliedPolicies.map((policy) => (
              <span key={policy.id}>
                <ShieldCheck />
                {policy.name}
                {policy.targets.some(
                  (target) =>
                    target.kind === "网关" && target.id === route.gatewayID,
                )
                  ? " · 继承自网关"
                  : ""}
              </span>
            ))}
          </div>
        ) : (
          <EmptyState
            title="未应用策略"
            description="当前路由仅执行访问控制与基础转发。"
          />
        )}
      </section>
    </Drawer>
  );
}
