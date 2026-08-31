import {
  Globe2,
  Network,
  ShieldCheck,
} from "lucide-react";
import {
  ConfigBadge,
  Drawer,
  EmptyState,
  Metric,
  RouteTypeBadge,
} from "../../components/ui";
import type {
  Certificate,
  Gateway,
  GatewayRoute,
  Policy,
} from "../../data";
import {
  gatewayDomains,
  listenerLabel,
} from "./gateway-model";

export function GatewayDetail({
  gateway,
  routes,
  certificates,
  policies,
  onClose,
}: {
  gateway: Gateway;
  routes: GatewayRoute[];
  certificates: Certificate[];
  policies: Policy[];
  onClose: () => void;
}) {
  const appliedPolicies = policies.filter((policy) =>
    policy.targets.some(
      (target) => target.kind === "网关" && target.id === gateway.id,
    ),
  );
  return (
    <Drawer
      title={gateway.name}
      description="流量入口与当前生效配置"
      onClose={onClose}
      width="wide"
    >
      <div className="detail-hero">
        <span>
          <Network />
        </span>
        <div>
          <ConfigBadge state={gateway.configState} />
          <h3>{gatewayDomains(gateway).join(" · ")}</h3>
          <p>{gateway.listeners.length} 个监听入口 · 配置已在当前环境生效</p>
        </div>
      </div>
      <div className="detail-kpis">
        <Metric
          label="引用路由"
          value={String(routes.length)}
          note="通过此入口接收流量"
        />
        <Metric
          label="监听入口"
          value={String(gateway.listeners.length)}
          note={gateway.listeners.map(listenerLabel).join(" · ")}
        />
        <Metric
          label="域名绑定"
          value={String(gatewayDomains(gateway).length)}
          note="跨监听入口去重"
        />
      </div>
      <section className="detail-section">
        <header>
          <h3>监听入口</h3>
        </header>
        {gateway.listeners.map((listener) => (
          <div className="listener-detail" key={listener.id}>
            <div className="detail-line">
              <span>
                <Globe2 />
              </span>
              <div>
                <strong>{listenerLabel(listener)}</strong>
                <small>{listener.bindings.length} 个域名绑定</small>
              </div>
              <span>{listener.protocol}</span>
            </div>
            {listener.bindings.map((binding) => (
              <div className="listener-binding" key={binding.domain}>
                <code>{binding.domain}</code>
                <span>
                  {binding.certificateID
                    ? (certificates.find(
                        (certificate) =>
                          certificate.id === binding.certificateID,
                      )?.name ?? "证书不存在")
                    : "不使用 TLS"}
                </span>
              </div>
            ))}
          </div>
        ))}
      </section>
      <section className="detail-section">
        <header>
          <h3>绑定路由</h3>
        </header>
        {routes.map((route) => (
          <div className="detail-line" key={route.id}>
            <RouteTypeBadge type={route.type} />
            <div>
              <strong>{route.name}</strong>
              <small>
                {route.host}
                {route.path}
              </small>
            </div>
            <span>{route.targets[0]?.serviceName}</span>
          </div>
        ))}
      </section>
      <section className="detail-section">
        <header>
          <h3>入口策略</h3>
        </header>
        {appliedPolicies.length ? (
          appliedPolicies.map((policy) => (
            <div className="detail-line" key={policy.id}>
              <span>
                <ShieldCheck />
              </span>
              <div>
                <strong>{policy.name}</strong>
                <small>{policy.rule}</small>
              </div>
              <ConfigBadge
                state={
                  policy.configState ??
                  (policy.targets.length ? "active" : "not-applied")
                }
              />
            </div>
          ))
        ) : (
          <EmptyState
            title="未应用入口策略"
            description="路由级策略不受网关策略影响。"
          />
        )}
      </section>
    </Drawer>
  );
}
