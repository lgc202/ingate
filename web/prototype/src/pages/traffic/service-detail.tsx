import {
  Bot,
  Braces,
  CheckCircle2,
  ChevronRight,
  KeyRound,
  RotateCw,
  Server,
  Sparkles,
  Wrench,
} from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import {
  ConfigBadge,
  Drawer,
  EmptyState,
  RouteTypeBadge,
  SearchField,
  ServiceTypeBadge,
  StatusBadge,
} from "../../components/ui";
import type {
  Certificate,
  GatewayRoute,
  Service,
} from "../../data";

export function ServiceDetail({
  service,
  routes,
  certificates,
  onVerify,
  onRotateCredential,
  onClose,
}: {
  service: Service;
  routes: GatewayRoute[];
  certificates: Certificate[];
  onVerify: () => void;
  onRotateCredential: () => void;
  onClose: () => void;
}) {
  const [checked, setChecked] = useState(false);
  const [capabilityQuery, setCapabilityQuery] = useState("");
  const clientCertificate = certificates.find(
    (certificate) => certificate.id === service.clientCertificateID,
  );
  const trustCertificate = certificates.find(
    (certificate) => certificate.id === service.trustCertificateID,
  );
  const visibleCapabilities = service.capabilities.filter((capability) =>
    capability.toLowerCase().includes(capabilityQuery.toLowerCase()),
  );
  return (
    <Drawer
      title={service.name}
      description={`${service.protocol} · ${service.provider}`}
      onClose={onClose}
      width="wide"
    >
      <div className="drawer-actions">
        <ServiceTypeBadge type={service.type} />
        <ConfigBadge state={service.configState} />
        {service.authentication !== "无认证" ? (
          <button
            className="button button-secondary"
            type="button"
            onClick={onRotateCredential}
          >
            <KeyRound />
            更新凭据
          </button>
        ) : null}
        <button
          className="button button-secondary"
          type="button"
          onClick={() => {
            setChecked(true);
            onVerify();
          }}
        >
          {checked ? <CheckCircle2 /> : <RotateCw />}
          {checked ? "校验通过" : "重新校验"}
        </button>
      </div>
      <div className="detail-hero">
        <span>
          {service.type === "HTTP" ? (
            <Braces />
          ) : service.type === "MODEL" ? (
            <Bot />
          ) : (
            <Wrench />
          )}
        </span>
        <div>
          <StatusBadge
            state={
              checked || service.verificationState === "verified"
                ? "healthy"
                : service.verificationState === "failed"
                  ? "error"
                  : "unverified"
            }
            label={
              checked || service.verificationState === "verified"
                ? "配置已校验"
                : service.verificationState === "failed"
                  ? "配置校验失败"
                  : "配置待校验"
            }
          />
          <h3>{service.endpoints[0]?.address}</h3>
          <p>
            {service.authentication} · {service.healthCheck}
          </p>
        </div>
      </div>
      <div className="detail-jump">
        <div>
          <strong>{routes.length ? `${routes.length} 条路由引用此服务` : "此服务尚未对外开放"}</strong>
          <span>{routes.length ? `成功率 ${service.successRate} · 延迟 ${service.latency}` : "创建路由后，外部请求才能转发到此服务"}</span>
        </div>
        {routes.length ? (
          <span className="detail-jump-actions-inline">
            <Link className="button button-secondary" to={`/requests?service=${service.id}`}>查看请求 <ChevronRight /></Link>
            <Link className="button button-secondary" to={`/analysis?service=${encodeURIComponent(service.name)}`}>查看趋势 <ChevronRight /></Link>
          </span>
        ) : (
          <Link className="button button-secondary" to={`/routes?create=1&service=${service.id}`}>创建路由 <ChevronRight /></Link>
        )}
      </div>
      <section className="detail-section">
        <header>
          <h3>服务端点</h3>
          <span>
            {service.endpoints.length > 1 ? service.loadBalancing : "单端点"}
          </span>
        </header>
        {service.endpoints.map((endpoint) => (
          <div className="detail-line" key={endpoint.address}>
            <span>
              <Server />
            </span>
            <div>
              <strong>{endpoint.address}</strong>
              <small>
                {service.endpoints.length > 1
                  ? `权重 ${endpoint.weight}%`
                  : service.protocol}
              </small>
            </div>
            <span className="endpoint-weight">
              {service.endpoints.length > 1 ? `${endpoint.weight}%` : "单端点"}
            </span>
            <StatusBadge
              state={endpoint.state}
              label={endpoint.state === "healthy" ? "健康" : "异常"}
            />
          </div>
        ))}
        <dl className="definition-list">
          <div>
            <dt>认证方式</dt>
            <dd>{service.authentication}</dd>
          </div>
          {service.authentication !== "无认证" ? (
            <div>
              <dt>凭据更新</dt>
              <dd>{service.credentialUpdatedAt ?? "创建服务时"}</dd>
            </div>
          ) : null}
          {clientCertificate ? (
            <div>
              <dt>客户端证书</dt>
              <dd>{clientCertificate.name}</dd>
            </div>
          ) : null}
          <div>
            <dt>连接加密</dt>
            <dd>{service.transportSecurity}</dd>
          </div>
          {service.transportSecurity === "TLS" ? (
            <>
              <div>
                <dt>服务端名称</dt>
                <dd>{service.serverName || "从端点地址推导"}</dd>
              </div>
              <div>
                <dt>证书信任</dt>
                <dd>{trustCertificate?.name ?? "系统信任"}</dd>
              </div>
            </>
          ) : null}
          <div>
            <dt>健康检查</dt>
            <dd>{service.healthCheck}</dd>
          </div>
          {service.endpoints.length > 1 ? (
            <div>
              <dt>负载方式</dt>
              <dd>{service.loadBalancing}</dd>
            </div>
          ) : null}
        </dl>
      </section>
      {service.type === "MODEL" && service.capabilities.length ? (
        <section className="detail-section service-capability-section">
          <header>
            <div>
              <h3>实际模型</h3>
              <small>配置校验返回或人工填写的厂商模型 ID</small>
            </div>
            <div className="service-capability-tools">
              <SearchField
                value={capabilityQuery}
                onChange={setCapabilityQuery}
                placeholder="搜索实际模型"
              />
              <span>
                {visibleCapabilities.length} / {service.capabilities.length}
              </span>
            </div>
          </header>
          <div className="service-capability-list">
            {visibleCapabilities.map((model) => {
              const price = service.modelPrices?.[model];
              return (
                <div className="service-capability-item" key={model}>
                  <span><Sparkles /></span>
                  <div>
                    <strong>{model}</strong>
                    <small>
                      {price
                        ? `输入 ¥${price.input} · 输出 ¥${price.output} / 百万 Token`
                        : "未配置单价，成本将标记为未知"}
                    </small>
                  </div>
                  <StatusBadge state="healthy" label="已配置" />
                </div>
              );
            })}
            {!visibleCapabilities.length ? (
              <EmptyState
                title="没有匹配的模型"
                description="请调整搜索条件。"
              />
            ) : null}
          </div>
        </section>
      ) : service.type === "MCP" ? (
        <section className="detail-section service-capability-section">
          <header>
            <div>
              <h3>已发现工具</h3>
              <small>可在 MCP 路由中选择对外开放</small>
            </div>
            <div className="service-capability-tools">
              <SearchField
                value={capabilityQuery}
                onChange={setCapabilityQuery}
                placeholder="搜索已发现工具"
              />
              <span>
                {visibleCapabilities.length} / {service.capabilities.length}
              </span>
            </div>
          </header>
          <div className="service-capability-list">
            {visibleCapabilities.map((capability) => (
              <div className="detail-line" key={capability}>
                <span>
                  <Wrench />
                </span>
                <div>
                  <strong>{capability}</strong>
                  <small>可在路由中选择对外开放</small>
                </div>
                <StatusBadge state="healthy" label="可用" />
              </div>
            ))}
            {!visibleCapabilities.length ? (
              <EmptyState
                title="没有匹配的工具"
                description="请调整搜索条件。"
              />
            ) : null}
          </div>
        </section>
      ) : null}
      <section className="detail-section">
        <header>
          <h3>引用路由</h3>
        </header>
        {routes.length ? (
          routes.flatMap((route) =>
            route.targets
              .filter((target) => target.serviceID === service.id)
              .map((target) => (
                <div
                  className="detail-line"
                  key={`${route.id}-${target.publishedCapability}-${target.role}`}
                >
                  <RouteTypeBadge type={route.type} />
                  <div>
                    <strong>{route.name}</strong>
                    <small>
                      {target.publishedCapability
                        ? `${target.publishedCapability} · `
                        : ""}
                      {route.host}
                      {route.path}
                    </small>
                  </div>
                  <span>{target.role}</span>
                </div>
              )),
          )
        ) : (
          <EmptyState
            title="尚未被路由使用"
            description="创建路由后可引用该服务。"
          />
        )}
      </section>
    </Drawer>
  );
}
