import {
  AlertTriangle,
  Bot,
  Braces,
  CheckCircle2,
  ChevronRight,
  CircleGauge,
  Clock3,
  FileUp,
  Globe2,
  KeyRound,
  LockKeyhole,
  Network,
  Plus,
  RotateCw,
  Server,
  ShieldCheck,
  Sparkles,
  Trash2,
  Wrench,
  X,
} from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import {
  CompactTagList,
  Drawer,
  DeleteConfirm,
  EmptyState,
  FilterTabs,
  FormActions,
  ConfigBadge,
  Metric,
  PageHeader,
  PrimaryButton,
  RouteTypeBadge,
  RowActions,
  SearchField,
  StatusBadge,
  ServiceTypeBadge,
  Toast,
  Topology,
  submitForm,
} from "../components/ui";
import type {
  Certificate,
  Gateway,
  GatewayListener,
  GatewayRoute,
  Policy,
  Service,
  ServiceEndpoint,
  ServiceType,
  TrafficType,
} from "../data";
import { usePrototype } from "../prototype-context";
export function GatewayPage() {
  const {
    gateways,
    routes,
    certificates,
    policies,
    addGateway,
    updateGateway,
    deleteGateway,
  } = usePrototype();
  const [selected, setSelected] = useState<Gateway | null>(null);
  const [editing, setEditing] = useState<Gateway | null>(null);
  const [deleting, setDeleting] = useState<Gateway | null>(null);
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState("");
  const [toast, setToast] = useState("");
  const visible = gateways.filter((gateway) =>
    `${gateway.name}${gatewayDomains(gateway).join("")}${gateway.listeners.map(listenerLabel).join("")}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="流量配置"
        title="网关"
        description="监听入口、域名与证书"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            创建网关
          </PrimaryButton>
        }
      />
      <section className="metric-grid four">
        <Metric
          label="网关"
          value={String(gateways.length)}
          note={`${gateways.filter((gateway) => (gateway.configState ?? "active") === "active").length} 个配置正常`}
        />
        <Metric label="配置异常" value={String(gateways.filter((gateway) => gateway.configState === "failed").length)} note="尚未应用到全部代理实例" tone={gateways.some((gateway) => gateway.configState === "failed") ? "warning" : "good"} />
        <Metric
          label="监听入口"
          value={String(
            gateways.reduce(
              (sum, gateway) => sum + gateway.listeners.length,
              0,
            ),
          )}
          note={`${gateways.flatMap((gateway) => gateway.listeners).filter((listener) => listener.protocol === "HTTPS").length} 个 HTTPS`}
        />
        <Metric
          label="已发布路由"
          value={String(
            routes.filter(
              (route) => (route.configState ?? "active") === "active",
            ).length,
          )}
          note="最近完整生效版本"
        />
      </section>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索网关、域名或监听入口"
          />
          <span>{visible.length} 个网关</span>
        </header>
        <div className="table-head gateway-columns">
          <span>网关</span>
          <span>监听入口</span>
          <span>域名</span>
          <span>证书</span>
          <span>路由</span>
          <span>配置状态</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((gateway) => (
            <div key={gateway.id} className="table-row gateway-columns">
              <div className="name-cell">
                <span>
                  <Network />
                </span>
                <div>
                  <strong>{gateway.name}</strong>
                  <small>{gatewayDomains(gateway)[0]}</small>
                </div>
              </div>
              <span>
                {gateway.listeners.slice(0, 2).map(listenerLabel).join(" · ")}
                {gateway.listeners.length > 2
                  ? ` 等 ${gateway.listeners.length} 个`
                  : ""}
              </span>
              <CompactTagList items={gatewayDomains(gateway)} />
              <CompactTagList
                items={gatewayCertificateNames(gateway, certificates)}
                empty="—"
              />
              <strong>
                {
                  routes.filter((route) => route.gatewayID === gateway.id)
                    .length
                }
              </strong>
              <ConfigBadge state={gateway.configState} />
              <RowActions
                onDetail={() => setSelected(gateway)}
                onEdit={() => setEditing(gateway)}
                onDelete={() => setDeleting(gateway)}
              />
            </div>
          ))
        ) : (
          <EmptyState title="没有匹配的网关" description="请调整搜索条件。" />
        )}
      </section>
      {selected ? (
        <GatewayDetail
          gateway={selected}
          routes={routes.filter((route) => route.gatewayID === selected.id)}
          certificates={certificates}
          policies={policies}
          onClose={() => setSelected(null)}
        />
      ) : null}
      {creating ? (
        <CreateGateway
          gateways={gateways}
          certificates={certificates}
          onClose={() => setCreating(false)}
          onSave={(gateway) => {
            addGateway(gateway);
            setCreating(false);
            setToast("网关已保存，正在发布");
          }}
        />
      ) : null}
      {editing ? (
        <CreateGateway
          initial={editing}
          gateways={gateways}
          certificates={certificates}
          onClose={() => setEditing(null)}
          onSave={(gateway) => {
            updateGateway(gateway);
            setEditing(null);
            setToast("网关修改已保存，正在发布");
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="网关"
          resourceName={deleting.name}
          blockedReason={
            routes.some((route) => route.gatewayID === deleting.id)
              ? `还有 ${routes.filter((route) => route.gatewayID === deleting.id).length} 条路由使用该网关。请先迁移或删除这些路由。`
              : policies.some((policy) =>
                    policy.targets.some(
                      (target) =>
                        target.kind === "网关" && target.id === deleting.id,
                    ),
                  )
                ? "仍有流量策略应用到该网关。请先调整策略的生效范围。"
                : undefined
          }
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deleteGateway(deleting.id);
            setDeleting(null);
            setToast("网关已删除，正在发布");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
function GatewayDetail({
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
          <p>{gateway.listeners.length} 个监听入口 · 配置已发布到当前环境</p>
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
function CreateGateway({
  initial,
  gateways,
  certificates,
  onClose,
  onSave,
}: {
  initial?: Gateway;
  gateways: Gateway[];
  certificates: Certificate[];
  onClose: () => void;
  onSave: (gateway: Gateway) => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [listeners, setListeners] = useState<GatewayListener[]>(
    initial?.listeners.map((listener) => ({
      ...listener,
      bindings: listener.bindings.map((binding) => ({
        ...binding,
      })),
    })) ?? [
      {
        id: crypto.randomUUID(),
        protocol: "HTTPS",
        port: 443,
        bindings: [
          {
            domain: "new-api.example.com",
            certificateID: "",
          },
        ],
      },
    ],
  );
  const updateListener = (
    listenerID: string,
    changes: Partial<GatewayListener>,
  ) =>
    setListeners((items) =>
      items.map((listener) =>
        listener.id === listenerID
          ? {
              ...listener,
              ...changes,
            }
          : listener,
      ),
    );
  const updateBinding = (
    listenerID: string,
    index: number,
    domain: string,
    certificateID = "",
  ) =>
    setListeners((items) =>
      items.map((listener) =>
        listener.id === listenerID
          ? {
              ...listener,
              bindings: listener.bindings.map((binding, bindingIndex) =>
                bindingIndex === index
                  ? {
                      domain,
                      certificateID:
                        listener.protocol === "HTTPS"
                          ? certificateID
                          : undefined,
                    }
                  : binding,
              ),
            }
          : listener,
      ),
    );
  const invalidTLS = listeners.some(
    (listener) =>
      listener.protocol === "HTTPS" &&
      listener.bindings.some(
        (binding) =>
          !binding.certificateID ||
          !certificates.some(
            (certificate) =>
              certificate.id === binding.certificateID &&
              certificate.usage === "服务器证书" &&
              certificate.identities.some((identity) =>
                certificateCoversDomain(identity, binding.domain),
              ),
          ),
      ),
  );
  const duplicateSocket =
    new Set(listeners.map((listener) => listener.port)).size !==
    listeners.length;
  const claimedBinding = listeners.some((listener) =>
    listener.bindings.some((binding) =>
      gateways.some(
        (gateway) =>
          gateway.id !== initial?.id &&
          gateway.listeners.some(
            (current) =>
              current.port === listener.port &&
              current.bindings.some(
                (currentBinding) =>
                  currentBinding.domain.toLowerCase() ===
                  binding.domain.trim().toLowerCase(),
              ),
          ),
      ),
    ),
  );
  const duplicateDomain =
    claimedBinding ||
    listeners.some((listener) => {
      const domains = listener.bindings.map((binding) =>
        binding.domain.trim().toLowerCase(),
      );
      return new Set(domains).size !== domains.length;
    });
  const save = () =>
    onSave({
      id: initial?.id ?? `gw-${Date.now()}`,
      name,
      listeners,
      state: initial?.state ?? "healthy",
    });
  return (
    <Drawer
      title={initial ? "编辑网关" : "创建网关"}
      description="一个网关可以包含多个监听入口，每个 HTTPS 域名绑定自己的证书"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="form-grid">
          <label className="field-wide">
            <span>网关名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如生产网关"
            />
          </label>
        </div>
        <div className="listener-editor">
          {listeners.map((listener, listenerIndex) => (
            <section className="form-section" key={listener.id}>
              <header>
                <span>{listenerIndex + 1}</span>
                <div>
                  <strong>监听入口</strong>
                  <small>{listenerLabel(listener)}</small>
                </div>
                {listeners.length > 1 ? (
                  <button
                    type="button"
                    onClick={() =>
                      setListeners((items) =>
                        items.filter((item) => item.id !== listener.id),
                      )
                    }
                  >
                    <Trash2 />
                  </button>
                ) : null}
              </header>
              <div className="form-grid">
                <label>
                  <span>协议</span>
                  <select
                    value={listener.protocol}
                    onChange={(event) => {
                      const protocol = event.target.value as "HTTP" | "HTTPS";
                      updateListener(listener.id, {
                        protocol,
                        port: protocol === "HTTPS" ? 443 : 80,
                        bindings: listener.bindings.map((binding) => ({
                          domain: binding.domain,
                          certificateID: protocol === "HTTPS" ? "" : undefined,
                        })),
                      });
                    }}
                  >
                    <option>HTTPS</option>
                    <option>HTTP</option>
                  </select>
                </label>
                <label>
                  <span>端口</span>
                  <input
                    required
                    type="number"
                    min="1"
                    max="65535"
                    value={listener.port}
                    onChange={(event) =>
                      updateListener(listener.id, {
                        port: Number(event.target.value),
                      })
                    }
                  />
                </label>
              </div>
              <div className="binding-editor">
                {listener.bindings.map((binding, index) => {
                  const matching = certificates.filter(
                    (certificate) =>
                      certificate.usage === "服务器证书" &&
                      certificate.state !== "error" &&
                      certificate.identities.some((identity) =>
                        certificateCoversDomain(identity, binding.domain),
                      ),
                  );
                  const effectiveCertificateID = matching.some(
                    (certificate) => certificate.id === binding.certificateID,
                  )
                    ? (binding.certificateID ?? "")
                    : "";
                  return (
                    <div className="binding-row" key={index}>
                      <label>
                        <span>请求域名</span>
                        <input
                          required
                          value={binding.domain}
                          onChange={(event) =>
                            updateBinding(
                              listener.id,
                              index,
                              event.target.value,
                            )
                          }
                        />
                      </label>
                      {listener.protocol === "HTTPS" ? (
                        <label>
                          <span>TLS 证书</span>
                          <select
                            required
                            value={effectiveCertificateID}
                            onChange={(event) =>
                              updateBinding(
                                listener.id,
                                index,
                                binding.domain,
                                event.target.value,
                              )
                            }
                          >
                            <option value="">选择覆盖该域名的证书</option>
                            {matching.map((certificate) => (
                              <option
                                key={certificate.id}
                                value={certificate.id}
                              >
                                {certificate.name}
                              </option>
                            ))}
                          </select>
                          {binding.domain && !matching.length ? (
                            <small>没有可覆盖该域名的服务器证书</small>
                          ) : null}
                        </label>
                      ) : null}
                      <button
                        type="button"
                        aria-label="删除域名绑定"
                        disabled={listener.bindings.length === 1}
                        onClick={() =>
                          updateListener(listener.id, {
                            bindings: listener.bindings.filter(
                              (_, bindingIndex) => bindingIndex !== index,
                            ),
                          })
                        }
                      >
                        <Trash2 />
                      </button>
                    </div>
                  );
                })}
              </div>
              <button
                className="text-action"
                type="button"
                onClick={() =>
                  updateListener(listener.id, {
                    bindings: [
                      ...listener.bindings,
                      {
                        domain: "",
                        certificateID:
                          listener.protocol === "HTTPS" ? "" : undefined,
                      },
                    ],
                  })
                }
              >
                <Plus />
                添加域名
              </button>
            </section>
          ))}
        </div>
        <button
          className="text-action"
          type="button"
          onClick={() =>
            setListeners((items) => [
              ...items,
              {
                id: crypto.randomUUID(),
                protocol: "HTTP",
                port: 80,
                bindings: [
                  {
                    domain: "",
                    certificateID: undefined,
                  },
                ],
              },
            ])
          }
        >
          <Plus />
          添加监听入口
        </button>
        {duplicateSocket ? (
          <div className="form-note is-error">
            <AlertTriangle />
            同一网关内不能创建两个占用相同端口的监听入口；同一端口的多个域名应放在一个入口下。
          </div>
        ) : null}
        {duplicateDomain ? (
          <div className="form-note is-error">
            <AlertTriangle />
            同一环境中，相同端口不能重复声明同一域名；请使用已有网关，或调整端口和域名。
          </div>
        ) : null}
        <FormActions
          submitLabel={initial ? "保存修改" : "创建网关"}
          submitDisabled={
            invalidTLS ||
            duplicateSocket ||
            duplicateDomain ||
            listeners.some((listener) =>
              listener.bindings.some((binding) => !binding.domain.trim()),
            )
          }
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
function listenerLabel(listener: GatewayListener) {
  return `${listener.protocol} · ${listener.port}`;
}
function gatewayDomains(gateway: Gateway) {
  return [
    ...new Set(
      gateway.listeners.flatMap((listener) =>
        listener.bindings.map((binding) => binding.domain),
      ),
    ),
  ];
}
function gatewayCertificateNames(
  gateway: Gateway,
  certificates: Certificate[],
) {
  const ids = new Set(
    gateway.listeners.flatMap((listener) =>
      listener.bindings.map((binding) => binding.certificateID).filter(Boolean),
    ),
  );
  return certificates
    .filter((certificate) => ids.has(certificate.id))
    .map((certificate) => certificate.name);
}
function routesConflict(
  routes: GatewayRoute[],
  routeID: string | undefined,
  gatewayID: string,
  host: string,
  path: string,
  method: string,
  matchMode: "精确匹配" | "前缀匹配",
) {
  const normalized = path.replace(/\/$/, "") || "/";
  return routes.find((route) => {
    if (
      route.id === routeID ||
      route.type !== "API" ||
      route.gatewayID !== gatewayID ||
      route.host !== host
    )
      return false;
    const currentMethod = route.match.split(" ")[0];
    if (method !== "ANY" && currentMethod !== "ANY" && method !== currentMethod)
      return false;
    const currentPath = route.path.replace(/\/$/, "") || "/";
    const currentPrefix = route.match.endsWith("/*");
    return (
      currentPath === normalized ||
      (currentPrefix && normalized.startsWith(`${currentPath}/`)) ||
      (matchMode === "前缀匹配" && currentPath.startsWith(`${normalized}/`))
    );
  });
}
function certificateCoversDomain(
  certificateDomain: string,
  requestedDomain: string,
) {
  if (!requestedDomain) return false;
  if (certificateDomain === requestedDomain) return true;
  if (!certificateDomain.startsWith("*.")) return false;
  const suffix = certificateDomain.slice(1);
  return (
    requestedDomain.endsWith(suffix) &&
    requestedDomain.slice(0, -suffix.length).length > 0 &&
    !requestedDomain.slice(0, -suffix.length).includes(".")
  );
}
type RouteFilter = "ALL" | TrafficType;
export function RoutePage() {
  const {
    routes,
    gateways,
    services,
    policies,
    callers,
    addRoute,
    addRoutes,
    updateRoute,
    deleteRoute,
  } = usePrototype();
  const [filter, setFilter] = useState<RouteFilter>("ALL");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<GatewayRoute | null>(null);
  const [editing, setEditing] = useState<GatewayRoute | null>(null);
  const [deleting, setDeleting] = useState<GatewayRoute | null>(null);
  const [creating, setCreating] = useState(false);
  const [importing, setImporting] = useState(false);
  const [toast, setToast] = useState("");
  const visible = routes.filter(
    (route) =>
      (filter === "ALL" || route.type === filter) &&
      `${route.name}${route.host}${route.published.join("")}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="流量配置"
        title="路由"
        actions={
          <>
            <button
              className="button button-secondary"
              type="button"
              onClick={() => setImporting(true)}
            >
              <FileUp />
              导入 OpenAPI
            </button>
            <PrimaryButton onClick={() => setCreating(true)}>
              <Plus />
              创建路由
            </PrimaryButton>
          </>
        }
      />
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索路由、域名或发布能力"
          />
          <FilterTabs
            value={filter}
            onChange={setFilter}
            options={[
              {
                value: "ALL",
                label: "全部",
                count: routes.length,
              },
              {
                value: "API",
                label: "API 路由",
                count: routes.filter((item) => item.type === "API").length,
              },
              {
                value: "AI",
                label: "AI 路由",
                count: routes.filter((item) => item.type === "AI").length,
              },
              {
                value: "MCP",
                label: "MCP 路由",
                count: routes.filter((item) => item.type === "MCP").length,
              },
            ]}
          />
        </header>
        <div className="table-head route-columns">
          <span>路由</span>
          <span>入口</span>
          <span>匹配规则</span>
          <span>目标服务</span>
          <span>访问方式</span>
          <span>配置状态</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((route) => (
            <div key={route.id} className="table-row route-columns">
              <div className="name-cell">
                <RouteTypeBadge type={route.type} />
                <div>
                  <strong>{route.name}</strong>
                  <small>
                    {route.published.slice(0, 2).join("、")}
                    {route.published.length > 2
                      ? ` 等 ${route.published.length} 项`
                      : ""}
                  </small>
                </div>
              </div>
              <div>
                <strong>{route.gatewayName}</strong>
                <small>
                  {route.host}
                  {route.path}
                </small>
              </div>
              <span>{route.match}</span>
              <span>
                {route.targets[0]?.serviceName}
                {route.targets.length > 1
                  ? ` 等 ${route.targets.length} 个`
                  : ""}
              </span>
              <span>{route.accessMode}</span>
              <ConfigBadge state={route.configState} />
              <RowActions
                onDetail={() => setSelected(route)}
                onEdit={() => setEditing(route)}
                onDelete={() => setDeleting(route)}
              />
            </div>
          ))
        ) : (
          <EmptyState
            title="没有匹配的路由"
            description="调整筛选条件，或创建一条新路由。"
          />
        )}
      </section>
      {selected ? (
        <RouteDetail
          route={selected}
          policies={policies}
          onClose={() => setSelected(null)}
        />
      ) : null}
      {creating ? (
        <CreateRoute
          routes={routes}
          gateways={gateways}
          services={services}
          onClose={() => setCreating(false)}
          onSave={(route) => {
            addRoute(route);
            setCreating(false);
            setToast("路由已保存，正在发布");
          }}
        />
      ) : null}
      {editing ? (
        <CreateRoute
          initial={editing}
          routes={routes}
          gateways={gateways}
          services={services}
          onClose={() => setEditing(null)}
          onSave={(route) => {
            updateRoute(route);
            setEditing(null);
            setToast("路由修改已保存，正在发布");
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="路由"
          resourceName={deleting.name}
          blockedReason={
            callers.some((caller) =>
              caller.permissions.some(
                (permission) => permission.routeID === deleting.id,
              ),
            )
              ? "仍有调用方被授权访问该路由。请先移除相关权限。"
              : policies.some((policy) =>
                    policy.targets.some(
                      (target) =>
                        target.kind === "路由" && target.id === deleting.id,
                    ),
                  )
                ? "仍有流量策略应用到该路由。请先调整策略的生效范围。"
                : undefined
          }
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deleteRoute(deleting.id);
            setDeleting(null);
            setToast("路由已删除，正在发布");
          }}
        />
      ) : null}
      {importing ? (
        <ImportOpenAPI
          gateways={gateways}
          services={services}
          onClose={() => setImporting(false)}
          onImport={(newRoutes) => {
            addRoutes(newRoutes);
            setImporting(false);
            setToast(`已创建 ${newRoutes.length} 条 API 路由，正在发布`);
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
function ImportOpenAPI({
  gateways,
  services,
  onClose,
  onImport,
}: {
  gateways: Gateway[];
  services: Service[];
  onClose: () => void;
  onImport: (routes: GatewayRoute[]) => void;
}) {
  const [fileName, setFileName] = useState("");
  const [parsed, setParsed] = useState(false);
  const [gatewayID, setGatewayID] = useState(gateways[0]?.id ?? "");
  const [serviceID, setServiceID] = useState(
    services.find((service) => service.type === "HTTP")?.id ?? "",
  );
  const gateway = gateways.find((item) => item.id === gatewayID);
  const service = services.find((item) => item.id === serviceID);
  const operations = [
    {
      name: "查询订单",
      method: "GET",
      path: "/api/orders/{id}",
    },
    {
      name: "创建订单",
      method: "POST",
      path: "/api/orders",
    },
    {
      name: "取消订单",
      method: "DELETE",
      path: "/api/orders/{id}",
    },
  ];
  const submit = () => {
    if (!gateway || !service) return;
    onImport(
      operations.map((operation) => ({
        id: `route-${crypto.randomUUID()}`,
        name: operation.name,
        type: "API",
        gatewayID: gateway.id,
        gatewayName: gateway.name,
        host: gatewayDomains(gateway)[0] ?? "",
        path: operation.path.replace("/{id}", ""),
        accessMode: "需要调用方密钥",
        match: `${operation.method} ${operation.path}`,
        published: [`${operation.method} ${operation.path}`],
        targets: [
          {
            serviceID: service.id,
            serviceName: service.name,
            publishedCapability: `${operation.method} ${operation.path}`,
            detail: `${service.endpoints.length} 个端点`,
            role: "主线路",
          },
        ],
        forwarding: {
          strategy: "单线路",
          timeout: "30 秒",
          retries: 1,
          pathHandling: "保持原路径",
          hostRewrite: "使用服务地址",
          circuitBreaker: {
            consecutiveFailures: 5,
            ejectionTime: "30 秒",
          },
        },
        requests: "0",
        successRate: "—",
        latency: "—",
        state: "healthy",
      })),
    );
  };
  return (
    <Drawer
      title="导入 OpenAPI"
      description="从 API 契约批量创建基础路由，导入后仍可逐条编辑"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, submit)}>
        <div className="form-grid">
          <label className="field-wide">
            <span>OpenAPI 文件</span>
            <input
              type="file"
              accept=".json,.yaml,.yml"
              required
              onChange={(event) => {
                setFileName(event.target.files?.[0]?.name ?? "");
                setParsed(false);
              }}
            />
            <small>支持 OpenAPI 3.0 JSON 或 YAML</small>
          </label>
          <label>
            <span>生效网关</span>
            <select
              value={gatewayID}
              onChange={(event) => setGatewayID(event.target.value)}
            >
              {gateways.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>目标 HTTP 服务</span>
            <select
              value={serviceID}
              onChange={(event) => setServiceID(event.target.value)}
            >
              {services
                .filter((item) => item.type === "HTTP")
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
            </select>
          </label>
        </div>
        <button
          className={`connection-test ${parsed ? "is-success" : ""}`}
          type="button"
          disabled={!fileName}
          onClick={() => setParsed(true)}
        >
          {parsed ? <CheckCircle2 /> : <FileUp />}
          <div>
            <strong>{parsed ? "契约解析完成" : "解析并检查契约"}</strong>
            <span>
              {parsed
                ? `发现 ${operations.length} 个操作，无路由冲突`
                : "检查格式、操作标识和目标网关中的路由冲突"}
            </span>
          </div>
        </button>
        {parsed ? (
          <section className="import-preview">
            <header>
              <strong>即将创建 {operations.length} 条路由</strong>
              <span>{fileName}</span>
            </header>
            {operations.map((operation) => (
              <div key={`${operation.method}-${operation.path}`}>
                <code>{operation.method}</code>
                <strong>{operation.path}</strong>
                <span>{operation.name}</span>
              </div>
            ))}
          </section>
        ) : null}
        <FormActions
          submitLabel="创建并发布路由"
          submitDisabled={!parsed || !gateway || !service}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
function RouteDetail({
  route,
  policies,
  onClose,
}: {
  route: GatewayRoute;
  policies: Policy[];
  onClose: () => void;
}) {
  const accessMode = route.accessMode ?? "需要调用方密钥";
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
      <div className="detail-jump"><div><strong>路由运行数据</strong><span>请求量、成功率、延迟和失败原因</span></div><Link className="button button-secondary" to={`/analysis?query=${encodeURIComponent(route.name)}`}>查看运行数据 <ChevronRight /></Link></div>
      <section className="detail-section">
        <header>
          <h3>匹配与发布</h3>
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
                <dt>主机头</dt>
                <dd>{route.forwarding.hostRewrite}</dd>
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
interface ModelMappingDraft {
  id: string;
  published: string;
  primaryServiceID: string;
  primaryModel: string;
  backupEnabled: boolean;
  backupServiceID: string;
  backupModel: string;
}
function CreateRoute({
  initial,
  routes,
  gateways,
  services,
  onClose,
  onSave,
}: {
  initial?: GatewayRoute;
  routes: GatewayRoute[];
  gateways: Gateway[];
  services: Service[];
  onClose: () => void;
  onSave: (route: GatewayRoute) => void;
}) {
  const [type, setType] = useState<TrafficType>(initial?.type ?? "API");
  const [name, setName] = useState(initial?.name ?? "新路由");
  const [gatewayID, setGatewayID] = useState(
    initial?.gatewayID ?? gateways[0]?.id ?? "",
  );
  const [host, setHost] = useState(
    initial?.host ??
      (gateways[0] ? (gatewayDomains(gateways[0])[0] ?? "") : ""),
  );
  const [path, setPath] = useState(initial?.path ?? "/new");
  const [method, setMethod] = useState(
    initial?.type === "API" ? (initial.match.split(" ")[0] ?? "ANY") : "ANY",
  );
  const [matchMode, setMatchMode] = useState<"精确匹配" | "前缀匹配">(
    initial?.type === "API" && !initial.match.endsWith("/*")
      ? "精确匹配"
      : "前缀匹配",
  );
  const [accessMode, setAccessMode] = useState<GatewayRoute["accessMode"]>(
    initial?.accessMode ?? "需要调用方密钥",
  );
  const [jwtIssuer, setJWTIssuer] = useState(initial?.jwt?.issuer ?? "");
  const [jwtAudience, setJWTAudience] = useState(initial?.jwt?.audience ?? "");
  const [jwksURI, setJWKSURI] = useState(initial?.jwt?.jwksURI ?? "");
  const [serviceID, setServiceID] = useState(
    initial?.targets[0]?.serviceID ?? "",
  );
  const [selectedTools, setSelectedTools] = useState<string[]>(
    initial?.type === "MCP" ? initial.published : [],
  );
  const [toolQuery, setToolQuery] = useState("");
  const [secondLineEnabled, setSecondLineEnabled] = useState(
    Boolean(initial && initial.type !== "AI" && initial.targets.length > 1),
  );
  const [secondServiceID, setSecondServiceID] = useState(
    initial?.targets[1]?.serviceID ?? "",
  );
  const [strategy, setStrategy] = useState<"主备切换" | "权重分流">(
    initial?.forwarding.strategy === "权重分流" ? "权重分流" : "主备切换",
  );
  const [primaryWeight, setPrimaryWeight] = useState(
    initial?.targets[0]?.weight ?? 80,
  );
  const [modelMappings, setModelMappings] = useState<ModelMappingDraft[]>(
    initial?.type === "AI"
      ? modelMappingsFromRoute(initial)
      : [newModelMapping(services)],
  );
  const [timeout, setTimeout] = useState(
    initial?.forwarding.timeout.replace(/\D/g, "") || "30",
  );
  const [retries, setRetries] = useState(
    String(initial?.forwarding.retries ?? 0),
  );
  const [pathHandling, setPathHandling] = useState(
    initial?.forwarding.pathHandling ?? "保持原路径",
  );
  const [hostRewrite, setHostRewrite] = useState(
    initial?.forwarding.hostRewrite ?? "使用服务地址",
  );
  const [conditionKind, setConditionKind] = useState<"Header" | "Query">(
    initial?.conditions?.[0]?.kind ?? "Header",
  );
  const [conditionName, setConditionName] = useState(
    initial?.conditions?.[0]?.name ?? "",
  );
  const [conditionValue, setConditionValue] = useState(
    initial?.conditions?.[0]?.value ?? "",
  );
  const [conditionMode, setConditionMode] = useState<"精确匹配" | "存在">(
    initial?.conditions?.[0]?.mode ?? "精确匹配",
  );
  const [rewritePath, setRewritePath] = useState(
    initial?.rewrite?.pathPrefix ?? "",
  );
  const [addHeaderName, setAddHeaderName] = useState(
    initial?.rewrite?.requestHeaders[0]?.name ?? "",
  );
  const [addHeaderValue, setAddHeaderValue] = useState(
    initial?.rewrite?.requestHeaders[0]?.value ?? "",
  );
  const [removeHeaders, setRemoveHeaders] = useState(
    initial?.rewrite?.removeHeaders.join(", ") ?? "",
  );
  const [failoverOn, setFailoverOn] = useState<string[]>(
    initial?.forwarding.failoverOn ?? ["连接失败", "超时", "HTTP 5xx"],
  );
  const gateway = gateways.find((item) => item.id === gatewayID) ?? gateways[0];
  const effectiveHost =
    gateway && gatewayDomains(gateway).includes(host)
      ? host
      : gateway
        ? (gatewayDomains(gateway)[0] ?? "")
        : "";
  const availableServices = services.filter(
    (service) => service.state === "healthy" || service.state === "warning",
  );
  const compatible = availableServices.filter(
    (service) => service.type === (type === "API" ? "HTTP" : "MCP"),
  );
  const effectiveServiceID = compatible.some(
    (service) => service.id === serviceID,
  )
    ? serviceID
    : (compatible[0]?.id ?? "");
  const primaryService = compatible.find(
    (service) => service.id === effectiveServiceID,
  );
  const effectiveTools = selectedTools.filter((tool) =>
    primaryService?.capabilities.includes(tool),
  );
  const visibleTools = (primaryService?.capabilities ?? []).filter((tool) =>
    tool.toLowerCase().includes(toolQuery.toLowerCase()),
  );
  const secondCandidates = compatible.filter(
    (service) => service.id !== effectiveServiceID,
  );
  const effectiveSecondServiceID = secondCandidates.some(
    (service) => service.id === secondServiceID,
  )
    ? secondServiceID
    : (secondCandidates[0]?.id ?? "");
  const secondService = secondCandidates.find(
    (service) => service.id === effectiveSecondServiceID,
  );
  const normalizedMappings = modelMappings.map((mapping) =>
    normalizeModelMapping(mapping, availableServices),
  );
  const duplicateModelNames =
    new Set(normalizedMappings.map((mapping) => mapping.published.trim()))
      .size !== normalizedMappings.length;
  const modelMappingsComplete =
    normalizedMappings.length > 0 &&
    normalizedMappings.every(
      (mapping) =>
        mapping.published.trim() &&
        mapping.primaryServiceID &&
        mapping.primaryModel &&
        (!mapping.backupEnabled ||
          (mapping.backupServiceID && mapping.backupModel)),
    );
  const routeConflict =
    type === "API"
      ? routesConflict(
          routes,
          initial?.id,
          gateway?.id ?? "",
          effectiveHost,
          path,
          method,
          matchMode,
        )
      : undefined;
  const routeReady =
    Boolean(gateway) &&
    !routeConflict &&
    (type === "AI"
      ? modelMappingsComplete && !duplicateModelNames
      : Boolean(primaryService) &&
        (type !== "MCP" || effectiveTools.length > 0));
  const changeType = (next: TrafficType) => {
    setType(next);
    setServiceID("");
    setSelectedTools([]);
    setSecondLineEnabled(false);
    setSecondServiceID("");
    setPath(next === "API" ? "/new" : next === "AI" ? "/v1" : "/mcp");
    setTimeout(next === "AI" ? "120" : "30");
    setRetries("0");
    if (next === "AI") setModelMappings([newModelMapping(services)]);
  };
  const updateMapping = (id: string, changes: Partial<ModelMappingDraft>) =>
    setModelMappings((items) =>
      items.map((item) =>
        item.id === id
          ? {
              ...item,
              ...changes,
            }
          : item,
      ),
    );
  const save = () => {
    if (!routeReady || !gateway) return;
    const apiCapability = `${method} ${path}${matchMode === "前缀匹配" ? "/*" : ""}`;
    const published =
      type === "API"
        ? [apiCapability]
        : type === "MCP"
          ? effectiveTools
          : normalizedMappings.map((mapping) => mapping.published.trim());
    const targets: GatewayRoute["targets"] = [];
    if (type === "AI") {
      normalizedMappings.forEach((mapping) => {
        const primary = services.find(
          (service) => service.id === mapping.primaryServiceID,
        )!;
        targets.push({
          serviceID: primary.id,
          serviceName: primary.name,
          publishedCapability: mapping.published.trim(),
          detail: mapping.primaryModel,
          role: "主线路",
        });
        if (mapping.backupEnabled) {
          const backup = services.find(
            (service) => service.id === mapping.backupServiceID,
          )!;
          targets.push({
            serviceID: backup.id,
            serviceName: backup.name,
            publishedCapability: mapping.published.trim(),
            detail: mapping.backupModel,
            role: "备用线路",
          });
        }
      });
    } else if (primaryService) {
      targets.push({
        serviceID: primaryService.id,
        serviceName: primaryService.name,
        publishedCapability: published.join("、"),
        detail:
          type === "MCP"
            ? `${effectiveTools.length} 个开放工具`
            : `${primaryService.endpoints.length} 个端点`,
        role:
          secondLineEnabled && strategy === "权重分流" ? "加权线路" : "主线路",
        weight:
          secondLineEnabled && strategy === "权重分流"
            ? primaryWeight
            : undefined,
      });
      if (secondLineEnabled && secondService)
        targets.push({
          serviceID: secondService.id,
          serviceName: secondService.name,
          publishedCapability: published.join("、"),
          detail: `${secondService.endpoints.length} 个端点`,
          role: strategy === "权重分流" ? "加权线路" : "备用线路",
          weight: strategy === "权重分流" ? 100 - primaryWeight : undefined,
        });
    }
    const forwardingStrategy =
      type === "AI"
        ? normalizedMappings.some((mapping) => mapping.backupEnabled)
          ? "主备切换"
          : "单线路"
        : secondLineEnabled
          ? strategy
          : "单线路";
    onSave({
      id: initial?.id ?? `route-${Date.now()}`,
      name,
      type,
      gatewayID: gateway.id,
      gatewayName: gateway.name,
      host: effectiveHost,
      path,
      accessMode,
      match:
        type === "API"
          ? apiCapability
          : type === "AI"
            ? "OpenAI API · 请求体 model"
            : "Streamable HTTP · tools/call",
      published,
      targets,
      jwt:
        accessMode === "JWT / OIDC"
          ? {
              issuer: jwtIssuer,
              audience: jwtAudience,
              jwksURI,
            }
          : undefined,
      conditions:
        type === "API" && conditionName.trim()
          ? [
              {
                kind: conditionKind,
                name: conditionName.trim(),
                value: conditionValue.trim(),
                mode: conditionMode,
              },
            ]
          : undefined,
      rewrite:
        type === "API"
          ? {
              pathPrefix: rewritePath.trim() || undefined,
              requestHeaders:
                addHeaderName.trim() && addHeaderValue.trim()
                  ? [
                      {
                        name: addHeaderName.trim(),
                        value: addHeaderValue.trim(),
                      },
                    ]
                  : [],
              removeHeaders: removeHeaders
                .split(",")
                .map((header) => header.trim())
                .filter(Boolean),
            }
          : undefined,
      forwarding: {
        strategy: forwardingStrategy,
        timeout: `${timeout} 秒`,
        retries: Number(retries),
        pathHandling,
        hostRewrite,
        failoverOn:
          type === "AI" && forwardingStrategy === "主备切换"
            ? failoverOn
            : undefined,
        circuitBreaker:
          type === "API"
            ? {
                consecutiveFailures: 5,
                ejectionTime: "30 秒",
              }
            : undefined,
      },
      requests: initial?.requests ?? "0",
      successRate: initial?.successRate ?? "—",
      latency: initial?.latency ?? "—",
      state: initial?.state ?? "healthy",
    });
  };
  return (
    <Drawer
      title={initial ? "编辑路由" : "创建路由"}
      description="定义外部请求如何匹配并转发到服务"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="type-selector">
          {(["API", "AI", "MCP"] as const).map((item) => (
            <button
              key={item}
              type="button"
              className={type === item ? "is-selected" : ""}
              onClick={() => changeType(item)}
            >
              <RouteTypeBadge type={item} />
              <strong>
                {item === "API"
                  ? "普通 HTTP API"
                  : item === "AI"
                    ? "统一大模型接口"
                    : "远程工具调用"}
              </strong>
              <small>
                {item === "API"
                  ? "一个方法与路径匹配"
                  : item === "AI"
                    ? "发布一个或多个客户端模型名"
                    : "发布一个服务中的多个工具"}
              </small>
            </button>
          ))}
        </div>
        <section className="form-section">
          <header>
            <span>1</span>
            <div>
              <strong>请求入口</strong>
              <small>请求按域名和路径匹配唯一一条路由</small>
            </div>
          </header>
          <div className="form-grid">
            <label>
              <span>路由名称</span>
              <input
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </label>
            <label>
              <span>生效网关</span>
              <select
                value={gatewayID}
                onChange={(event) => {
                  setGatewayID(event.target.value);
                  setHost("");
                }}
                required
              >
                {gateways.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span>请求域名</span>
              <select
                value={effectiveHost}
                onChange={(event) => setHost(event.target.value)}
                required
              >
                {gateway
                  ? gatewayDomains(gateway).map((domain) => (
                      <option key={domain}>{domain}</option>
                    ))
                  : null}
              </select>
            </label>
            <label>
              <span>请求路径</span>
              <input
                required
                value={path}
                onChange={(event) => setPath(event.target.value)}
              />
            </label>
            <label className="field-wide">
              <span>访问方式</span>
              <select
                value={accessMode}
                onChange={(event) =>
                  setAccessMode(
                    event.target.value as GatewayRoute["accessMode"],
                  )
                }
              >
                <option>需要调用方密钥</option>
                {type === "API" ? <option>JWT / OIDC</option> : null}
                <option>公开访问</option>
              </select>
              <small>
                {accessMode === "需要调用方密钥"
                  ? "校验密钥及该调用方在本路由下的接口、模型或工具权限"
                  : accessMode === "JWT / OIDC"
                    ? "校验标准 JWT 的签发方、受众和签名"
                    : "不识别调用方，调用方权限与用量上限不适用"}
              </small>
            </label>
            {accessMode === "JWT / OIDC" ? (
              <>
                <label>
                  <span>Issuer</span>
                  <input
                    required
                    value={jwtIssuer}
                    onChange={(event) => setJWTIssuer(event.target.value)}
                    placeholder="https://login.example.com/"
                  />
                </label>
                <label>
                  <span>Audience</span>
                  <input
                    required
                    value={jwtAudience}
                    onChange={(event) => setJWTAudience(event.target.value)}
                    placeholder="ingate-api"
                  />
                </label>
                <label className="field-wide">
                  <span>JWKS 地址</span>
                  <input
                    required
                    value={jwksURI}
                    onChange={(event) => setJWKSURI(event.target.value)}
                    placeholder="https://login.example.com/.well-known/jwks.json"
                  />
                </label>
              </>
            ) : null}
            {type === "API" ? (
              <>
                <label>
                  <span>请求方法</span>
                  <select
                    value={method}
                    onChange={(event) => setMethod(event.target.value)}
                  >
                    <option value="ANY">全部方法</option>
                    <option>GET</option>
                    <option>POST</option>
                    <option>PUT</option>
                    <option>PATCH</option>
                    <option>DELETE</option>
                  </select>
                </label>
                <label>
                  <span>路径方式</span>
                  <select
                    value={matchMode}
                    onChange={(event) =>
                      setMatchMode(
                        event.target.value as "精确匹配" | "前缀匹配",
                      )
                    }
                  >
                    <option>精确匹配</option>
                    <option>前缀匹配</option>
                  </select>
                </label>
              </>
            ) : null}
          </div>
          {routeConflict ? (
            <div className="form-note is-error">
              <AlertTriangle />
              与“{routeConflict.name}
              ”冲突：同一网关、域名下的方法和路径匹配范围重叠。
            </div>
          ) : null}
          {type === "API" ? (
            <details className="advanced-config">
              <summary>Header / Query 匹配（可选）</summary>
              <div className="form-grid">
                <label>
                  <span>条件类型</span>
                  <select
                    value={conditionKind}
                    onChange={(event) =>
                      setConditionKind(event.target.value as "Header" | "Query")
                    }
                  >
                    <option>Header</option>
                    <option>Query</option>
                  </select>
                </label>
                <label>
                  <span>匹配方式</span>
                  <select
                    value={conditionMode}
                    onChange={(event) =>
                      setConditionMode(
                        event.target.value as "精确匹配" | "存在",
                      )
                    }
                  >
                    <option>精确匹配</option>
                    <option>存在</option>
                  </select>
                </label>
                <label>
                  <span>{conditionKind} 名称</span>
                  <input
                    value={conditionName}
                    onChange={(event) => setConditionName(event.target.value)}
                    placeholder={
                      conditionKind === "Header"
                        ? "例如 x-api-version"
                        : "例如 tenant"
                    }
                  />
                </label>
                {conditionMode === "精确匹配" ? (
                  <label>
                    <span>匹配值</span>
                    <input
                      value={conditionValue}
                      onChange={(event) =>
                        setConditionValue(event.target.value)
                      }
                      placeholder="例如 v2"
                    />
                  </label>
                ) : null}
              </div>
            </details>
          ) : null}
        </section>
        <section className="form-section">
          <header>
            <span>2</span>
            <div>
              <strong>{type === "AI" ? "模型映射" : "目标服务"}</strong>
              <small>
                {type === "AI"
                  ? "每个客户端模型名拥有独立的主线路和可选备用线路"
                  : type === "MCP"
                    ? "开放已从 MCP 服务发现的工具"
                    : "配置主线路及可选的第二线路"}
              </small>
            </div>
          </header>
          {type === "AI" ? (
            <div className="model-mapping-editor">
              {normalizedMappings.map((mapping, index) => {
                const primary = availableServices.find(
                  (service) => service.id === mapping.primaryServiceID,
                );
                const backupCandidates = availableServices.filter(
                  (service) =>
                    service.type === "MODEL" &&
                    service.id !== mapping.primaryServiceID,
                );
                const backup = backupCandidates.find(
                  (service) => service.id === mapping.backupServiceID,
                );
                return (
                  <article key={mapping.id}>
                    <header>
                      <strong>模型映射 {index + 1}</strong>
                      {normalizedMappings.length > 1 ? (
                        <button
                          type="button"
                          onClick={() =>
                            setModelMappings((items) =>
                              items.filter((item) => item.id !== mapping.id),
                            )
                          }
                        >
                          <Trash2 />
                          移除
                        </button>
                      ) : null}
                    </header>
                    <div className="form-grid">
                      <label>
                        <span>客户端模型名</span>
                        <input
                          required
                          value={mapping.published}
                          onChange={(event) =>
                            updateMapping(mapping.id, {
                              published: event.target.value,
                            })
                          }
                          placeholder="例如 reasoning-pro"
                        />
                      </label>
                      <label>
                        <span>主线路服务</span>
                        <select
                          value={mapping.primaryServiceID}
                          onChange={(event) =>
                            updateMapping(mapping.id, {
                              primaryServiceID: event.target.value,
                              primaryModel: "",
                              backupServiceID: "",
                              backupModel: "",
                            })
                          }
                        >
                          {availableServices
                            .filter((service) => service.type === "MODEL")
                            .map((service) => (
                              <option key={service.id} value={service.id}>
                                {service.name}
                              </option>
                            ))}
                        </select>
                      </label>
                      <label className="field-wide">
                        <span>主线路真实模型</span>
                        <select
                          value={mapping.primaryModel}
                          onChange={(event) =>
                            updateMapping(mapping.id, {
                              primaryModel: event.target.value,
                            })
                          }
                        >
                          {primary?.capabilities.map((model) => (
                            <option key={model}>{model}</option>
                          ))}
                        </select>
                      </label>
                    </div>
                    <button
                      className="text-action"
                      type="button"
                      onClick={() =>
                        updateMapping(mapping.id, {
                          backupEnabled: !mapping.backupEnabled,
                          backupServiceID: "",
                          backupModel: "",
                        })
                      }
                    >
                      {mapping.backupEnabled
                        ? "移除备用线路"
                        : "+ 添加备用线路"}
                    </button>
                    {mapping.backupEnabled ? (
                      <div className="form-grid">
                        <label>
                          <span>备用线路服务</span>
                          <select
                            value={mapping.backupServiceID}
                            onChange={(event) =>
                              updateMapping(mapping.id, {
                                backupServiceID: event.target.value,
                                backupModel: "",
                              })
                            }
                          >
                            <option value="">选择备用服务</option>
                            {backupCandidates.map((service) => (
                              <option key={service.id} value={service.id}>
                                {service.name}
                              </option>
                            ))}
                          </select>
                        </label>
                        <label>
                          <span>备用真实模型</span>
                          <select
                            value={mapping.backupModel}
                            onChange={(event) =>
                              updateMapping(mapping.id, {
                                backupModel: event.target.value,
                              })
                            }
                          >
                            <option value="">选择真实模型</option>
                            {backup?.capabilities.map((model) => (
                              <option key={model}>{model}</option>
                            ))}
                          </select>
                        </label>
                      </div>
                    ) : null}
                  </article>
                );
              })}
              <button
                className="text-action"
                type="button"
                onClick={() =>
                  setModelMappings((items) => [
                    ...items,
                    newModelMapping(availableServices),
                  ])
                }
              >
                <Plus />
                添加模型映射
              </button>
              {duplicateModelNames ? (
                <div className="form-note is-error">
                  <AlertTriangle />
                  客户端模型名不能重复
                </div>
              ) : null}
            </div>
          ) : (
            <>
              <div className="form-grid">
                <label className="field-wide">
                  <span>{type === "API" ? "HTTP 服务" : "MCP 服务"}</span>
                  <select
                    value={effectiveServiceID}
                    onChange={(event) => {
                      setServiceID(event.target.value);
                      setSelectedTools([]);
                      setToolQuery("");
                      setSecondServiceID("");
                    }}
                    required
                  >
                    {compatible.map((service) => (
                      <option key={service.id} value={service.id}>
                        {service.name} ·{" "}
                        {service.type === "HTTP"
                          ? `${service.endpoints.length} 个端点`
                          : service.provider}
                      </option>
                    ))}
                  </select>
                </label>
                {type === "MCP" ? (
                  <fieldset className="field-wide tool-picker">
                    <legend>开放工具</legend>
                    {effectiveTools.length ? (
                      <div className="tool-picker-selected">
                        {effectiveTools.map((tool) => (
                          <span key={tool}>
                            <code>{tool}</code>
                            <button
                              type="button"
                              aria-label={`移除${tool}`}
                              onClick={() =>
                                setSelectedTools((items) =>
                                  items.filter((item) => item !== tool),
                                )
                              }
                            >
                              <X />
                            </button>
                          </span>
                        ))}
                      </div>
                    ) : (
                      <div className="tool-picker-empty">
                        未开放工具，MCP 请求将被拒绝
                      </div>
                    )}
                    <div className="tool-picker-toolbar">
                      <SearchField
                        value={toolQuery}
                        onChange={setToolQuery}
                        placeholder="搜索已发现工具"
                      />
                      <span>
                        已选 {effectiveTools.length} / {primaryService?.capabilities.length ?? 0}
                      </span>
                    </div>
                    <div className="tool-picker-list">
                      {visibleTools.map((tool) => (
                        <label key={tool}>
                          <input
                            type="checkbox"
                            checked={effectiveTools.includes(tool)}
                            onChange={() =>
                              setSelectedTools((items) =>
                                items.includes(tool)
                                  ? items.filter((item) => item !== tool)
                                  : [...items, tool],
                              )
                            }
                          />
                          <span>
                            <code>{tool}</code>
                          </span>
                        </label>
                      ))}
                      {!visibleTools.length ? (
                        <small>没有匹配的工具</small>
                      ) : null}
                    </div>
                  </fieldset>
                ) : null}
              </div>
              {type === "API" && secondCandidates.length ? (
                <div className="optional-target">
                  <button
                    type="button"
                    onClick={() => setSecondLineEnabled((value) => !value)}
                  >
                    {secondLineEnabled ? "移除第二线路" : "+ 添加第二线路"}
                  </button>
                  {secondLineEnabled ? (
                    <div className="form-grid">
                      <label>
                        <span>第二线路服务</span>
                        <select
                          value={effectiveSecondServiceID}
                          onChange={(event) =>
                            setSecondServiceID(event.target.value)
                          }
                        >
                          {secondCandidates.map((service) => (
                            <option key={service.id} value={service.id}>
                              {service.name}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        <span>转发方式</span>
                        <select
                          value={strategy}
                          onChange={(event) =>
                            setStrategy(
                              event.target.value as "主备切换" | "权重分流",
                            )
                          }
                        >
                          <option>主备切换</option>
                          <option>权重分流</option>
                        </select>
                      </label>
                      {strategy === "权重分流" ? (
                        <label>
                          <span>主线路权重（%）</span>
                          <input
                            type="number"
                            min="1"
                            max="99"
                            value={primaryWeight}
                            onChange={(event) =>
                              setPrimaryWeight(Number(event.target.value))
                            }
                          />
                          <small>第二线路权重为 {100 - primaryWeight}%</small>
                        </label>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              ) : null}
            </>
          )}
        </section>
        <details className="advanced-config">
          <summary>高级转发设置</summary>
          <div className="form-grid">
            <label>
              <span>请求超时（秒）</span>
              <input
                type="number"
                min="1"
                value={timeout}
                onChange={(event) => setTimeout(event.target.value)}
              />
            </label>
            <label>
              <span>失败重试次数</span>
              <select
                value={retries}
                onChange={(event) => setRetries(event.target.value)}
              >
                <option value="0">不重试</option>
                <option value="1">1 次</option>
                <option value="2">2 次</option>
                <option value="3">3 次</option>
              </select>
            </label>
            {type === "API" ? (
              <>
                <label>
                  <span>路径处理</span>
                  <select
                    value={pathHandling}
                    onChange={(event) => setPathHandling(event.target.value)}
                  >
                    <option>保持原路径</option>
                    <option>移除匹配前缀</option>
                  </select>
                </label>
                <label>
                  <span>主机头</span>
                  <select
                    value={hostRewrite}
                    onChange={(event) => setHostRewrite(event.target.value)}
                  >
                    <option>使用服务地址</option>
                    <option>保持请求主机</option>
                  </select>
                </label>
                <label>
                  <span>改写路径前缀</span>
                  <input
                    value={rewritePath}
                    onChange={(event) => setRewritePath(event.target.value)}
                    placeholder="例如 /internal/orders"
                  />
                </label>
                <label>
                  <span>删除请求头</span>
                  <input
                    value={removeHeaders}
                    onChange={(event) => setRemoveHeaders(event.target.value)}
                    placeholder="多个名称用逗号分隔"
                  />
                </label>
                <label>
                  <span>新增请求头</span>
                  <input
                    value={addHeaderName}
                    onChange={(event) => setAddHeaderName(event.target.value)}
                    placeholder="例如 x-route-source"
                  />
                </label>
                <label>
                  <span>请求头值</span>
                  <input
                    value={addHeaderValue}
                    onChange={(event) => setAddHeaderValue(event.target.value)}
                    placeholder="例如 ingate"
                  />
                </label>
              </>
            ) : null}
          </div>
          {type === "API" ? (
            <div className="form-note">
              <ShieldCheck />
              服务连续失败 5 次后摘除 30 秒，其余健康端点继续承载流量。
            </div>
          ) : null}
          {type === "AI" &&
          normalizedMappings.some((mapping) => mapping.backupEnabled) ? (
            <fieldset className="failover-picker">
              <legend>切换到备用线路的条件</legend>
              {["连接失败", "超时", "HTTP 429", "HTTP 5xx"].map(
                (reason) => (
                  <label key={reason}>
                    <input
                      type="checkbox"
                      checked={failoverOn.includes(reason)}
                      onChange={() =>
                        setFailoverOn((items) =>
                          items.includes(reason)
                            ? items.filter((item) => item !== reason)
                            : [...items, reason],
                        )
                      }
                    />
                    {reason}
                  </label>
                ),
              )}
            </fieldset>
          ) : null}
        </details>
        <FormActions
          submitLabel={initial ? "保存修改" : "创建路由"}
          submitDisabled={!routeReady}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
function newModelMapping(services: Service[]): ModelMappingDraft {
  const primary = services.find((service) => service.type === "MODEL");
  return {
    id: crypto.randomUUID(),
    published: "",
    primaryServiceID: primary?.id ?? "",
    primaryModel: primary?.capabilities[0] ?? "",
    backupEnabled: false,
    backupServiceID: "",
    backupModel: "",
  };
}
function modelMappingsFromRoute(route: GatewayRoute): ModelMappingDraft[] {
  return route.published.map((published) => {
    const targets = route.targets.filter(
      (target) => target.publishedCapability === published,
    );
    const primary =
      targets.find((target) => target.role === "主线路") ?? targets[0];
    const backup = targets.find((target) => target.role === "备用线路");
    return {
      id: crypto.randomUUID(),
      published,
      primaryServiceID: primary?.serviceID ?? "",
      primaryModel: primary?.detail ?? "",
      backupEnabled: Boolean(backup),
      backupServiceID: backup?.serviceID ?? "",
      backupModel: backup?.detail ?? "",
    };
  });
}
function normalizeModelMapping(
  mapping: ModelMappingDraft,
  services: Service[],
) {
  const modelServices = services.filter(
    (service) =>
      service.type === "MODEL" &&
      (service.state === "healthy" || service.state === "warning"),
  );
  const primary =
    modelServices.find((service) => service.id === mapping.primaryServiceID) ??
    modelServices[0];
  const primaryModel = primary?.capabilities.includes(mapping.primaryModel)
    ? mapping.primaryModel
    : (primary?.capabilities[0] ?? "");
  const backupCandidates = modelServices.filter(
    (service) => service.id !== primary?.id,
  );
  const backup = backupCandidates.find(
    (service) => service.id === mapping.backupServiceID,
  );
  const backupModel = backup?.capabilities.includes(mapping.backupModel)
    ? mapping.backupModel
    : (backup?.capabilities[0] ?? "");
  return {
    ...mapping,
    primaryServiceID: primary?.id ?? "",
    primaryModel,
    backupServiceID: mapping.backupEnabled ? (backup?.id ?? "") : "",
    backupModel: mapping.backupEnabled ? backupModel : "",
  };
}
type ServiceFilter = "ALL" | ServiceType;
export function ServicePage() {
  const {
    services,
    routes,
    certificates,
    addService,
    updateService,
    deleteService,
    verifyService,
    updateServiceCredential,
  } = usePrototype();
  const [filter, setFilter] = useState<ServiceFilter>("ALL");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Service | null>(null);
  const [editing, setEditing] = useState<Service | null>(null);
  const [deleting, setDeleting] = useState<Service | null>(null);
  const [creating, setCreating] = useState(false);
  const [rotatingCredential, setRotatingCredential] = useState<Service | null>(
    null,
  );
  const [toast, setToast] = useState("");
  const visible = services.filter(
    (service) =>
      (filter === "ALL" || service.type === filter) &&
      `${service.name}${service.provider}${service.endpoints.map((item) => item.address).join("")}${service.capabilities.join("")}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const verifiedServices = services.filter(
    (item) => (item.verificationState ?? "verified") === "verified",
  ).length;
  const modelCount = new Set(
    services
      .filter((item) => item.type === "MODEL")
      .flatMap((item) => item.capabilities),
  ).size;
  const toolCount = services
    .filter((item) => item.type === "MCP")
    .reduce((sum, item) => sum + item.capabilities.length, 0);
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="流量配置"
        title="服务"
        description="HTTP、大模型与 MCP 服务连接"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            创建服务
          </PrimaryButton>
        }
      />
      <section className="metric-grid four">
        <Metric
          label="HTTP 服务"
          value={String(services.filter((item) => item.type === "HTTP").length)}
          note="普通业务与开放 API"
        />
        <Metric
          label="大模型服务"
          value={String(
            services.filter((item) => item.type === "MODEL").length,
          )}
          note={`${modelCount} 个实际模型`}
        />
        <Metric
          label="MCP 服务"
          value={String(services.filter((item) => item.type === "MCP").length)}
          note={`${toolCount} 个已发现工具`}
        />
        <Metric
          label="已验证连接"
          value={`${verifiedServices} / ${services.length}`}
          note="端点、认证与协议验证"
          tone={verifiedServices === services.length ? "good" : "warning"}
        />
      </section>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索服务、地址或能力"
          />
          <FilterTabs
            value={filter}
            onChange={setFilter}
            options={[
              {
                value: "ALL",
                label: "全部",
                count: services.length,
              },
              {
                value: "HTTP",
                label: "HTTP 服务",
                count: services.filter((item) => item.type === "HTTP").length,
              },
              {
                value: "MODEL",
                label: "大模型服务",
                count: services.filter((item) => item.type === "MODEL").length,
              },
              {
                value: "MCP",
                label: "MCP 服务",
                count: services.filter((item) => item.type === "MCP").length,
              },
            ]}
          />
        </header>
        <div className="table-head service-columns">
          <span>服务</span>
          <span>协议与地址</span>
          <span>端点 / 模型 / 工具</span>
          <span>引用路由</span>
          <span>连接验证</span>
          <span>配置状态</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((service) => (
            <div key={service.id} className="table-row service-columns">
              <div className="name-cell">
                <ServiceTypeBadge type={service.type} />
                <div>
                  <strong>{service.name}</strong>
                  <small>{service.provider}</small>
                </div>
              </div>
              <div>
                <strong>{service.protocol}</strong>
                <small>
                  {service.endpoints[0]?.address}
                  {service.endpoints.length > 1
                    ? ` 等 ${service.endpoints.length} 个`
                    : ""}
                </small>
              </div>
              <CompactTagList
                items={
                  service.type === "HTTP"
                    ? service.endpoints.map((item) => item.address)
                    : service.capabilities
                }
                limit={3}
              />
              <strong>
                {
                  routes.filter((route) =>
                    route.targets.some(
                      (target) => target.serviceID === service.id,
                    ),
                  ).length
                }{" "}
                条
              </strong>
              <StatusBadge
                state={
                  service.verificationState === "failed"
                    ? "error"
                    : service.verificationState === "unverified"
                      ? "unverified"
                      : "healthy"
                }
                label={
                  service.verificationState === "failed"
                    ? "验证失败"
                    : service.verificationState === "unverified"
                      ? "待验证"
                      : "已验证"
                }
              />
              <ConfigBadge state={service.configState} />
              <RowActions
                onDetail={() => setSelected(service)}
                onEdit={() => setEditing(service)}
                onDelete={() => setDeleting(service)}
              />
            </div>
          ))
        ) : (
          <EmptyState
            title="没有匹配的服务"
            description="请调整搜索或筛选条件。"
          />
        )}
      </section>
      {selected ? (
        <ServiceDetail
          service={
            services.find((service) => service.id === selected.id) ?? selected
          }
          routes={routes.filter((route) =>
            route.targets.some((target) => target.serviceID === selected.id),
          )}
          certificates={certificates}
          onVerify={() => verifyService(selected.id)}
          onRotateCredential={() => {
            setRotatingCredential(
              services.find((service) => service.id === selected.id) ??
                selected,
            );
            setSelected(null);
          }}
          onClose={() => setSelected(null)}
        />
      ) : null}
      {creating ? (
        <CreateService
          certificates={certificates}
          onClose={() => setCreating(false)}
          onSave={(service) => {
            addService(service);
            setCreating(false);
            setToast("服务已创建");
          }}
        />
      ) : null}
      {editing ? (
        <CreateService
          initial={editing}
          certificates={certificates}
          onClose={() => setEditing(null)}
          onSave={(service) => {
            updateService(service);
            setEditing(null);
            setToast("服务修改已保存，正在发布");
          }}
        />
      ) : null}
      {rotatingCredential ? (
        <RotateServiceCredential
          service={rotatingCredential}
          certificates={certificates}
          onClose={() => setRotatingCredential(null)}
          onSave={(clientCertificateID) => {
            updateServiceCredential(rotatingCredential.id, clientCertificateID);
            setRotatingCredential(null);
            setToast("服务凭据已更新");
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="服务"
          resourceName={deleting.name}
          blockedReason={
            routes.some((route) =>
              route.targets.some((target) => target.serviceID === deleting.id),
            )
              ? `还有 ${routes.filter((route) => route.targets.some((target) => target.serviceID === deleting.id)).length} 条路由使用该服务。请先修改这些路由的目标服务。`
              : undefined
          }
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deleteService(deleting.id);
            setDeleting(null);
            setToast("服务已删除，正在发布");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
function ServiceDetail({
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
          {checked ? "检测通过" : "重新检测"}
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
                ? "连接已验证"
                : service.verificationState === "failed"
                  ? "连接验证失败"
                  : "连接待验证"
            }
          />
          <h3>{service.endpoints[0]?.address}</h3>
          <p>
            {service.authentication} · {service.healthCheck}
          </p>
        </div>
      </div>
      <div className="detail-jump"><div><strong>{routes.length} 条路由引用此服务</strong><span>成功率、延迟和端点异常</span></div><Link className="button button-secondary" to="/health">查看运行健康 <ChevronRight /></Link></div>
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
            <span className="endpoint-weight">{service.endpoints.length > 1 ? `${endpoint.weight}%` : "已配置"}</span>
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
              <small>连接验证返回或人工填写的厂商模型 ID</small>
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

function RotateServiceCredential({
  service,
  certificates,
  onClose,
  onSave,
}: {
  service: Service;
  certificates: Certificate[];
  onClose: () => void;
  onSave: (clientCertificateID?: string) => void;
}) {
  const isMTLS = service.authentication.startsWith("mTLS");
  const isAWS = service.authentication === "AWS 签名";
  const isBasic = service.authentication === "Basic";
  const [identity, setIdentity] = useState("");
  const [secret, setSecret] = useState("");
  const [region, setRegion] = useState("");
  const [clientCertificateID, setClientCertificateID] = useState(
    service.clientCertificateID ?? "",
  );
  const [tested, setTested] = useState(false);
  const clientCertificates = certificates.filter(
    (certificate) =>
      certificate.usage === "客户端证书" && certificate.state !== "error",
  );
  const complete = isMTLS
    ? Boolean(clientCertificateID)
    : isAWS
      ? Boolean(identity && secret && region)
      : isBasic
        ? Boolean(identity && secret)
        : Boolean(secret);
  return (
    <Drawer
      title="更新服务凭据"
      description={`${service.name} · ${service.authentication}`}
      onClose={onClose}
    >
      <form
        onSubmit={(event) =>
          submitForm(event, () =>
            onSave(isMTLS ? clientCertificateID : undefined),
          )
        }
      >
        <div className="form-grid">
          {isMTLS ? (
            <label className="field-wide">
              <span>客户端证书</span>
              <select
                required
                value={clientCertificateID}
                onChange={(event) => {
                  setClientCertificateID(event.target.value);
                  setTested(false);
                }}
              >
                <option value="">选择客户端证书</option>
                {clientCertificates.map((certificate) => (
                  <option key={certificate.id} value={certificate.id}>
                    {certificate.name}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <>
              {isAWS || isBasic ? (
                <label>
                  <span>{isAWS ? "Access Key ID" : "用户名"}</span>
                  <input
                    required
                    value={identity}
                    onChange={(event) => {
                      setIdentity(event.target.value);
                      setTested(false);
                    }}
                  />
                </label>
              ) : null}
              <label className={isAWS || isBasic ? "" : "field-wide"}>
                <span>新凭据</span>
                <input
                  required
                  type="password"
                  value={secret}
                  onChange={(event) => {
                    setSecret(event.target.value);
                    setTested(false);
                  }}
                  placeholder="保存后不再显示明文"
                />
              </label>
              {isAWS ? (
                <label className="field-wide">
                  <span>AWS 区域</span>
                  <input
                    required
                    value={region}
                    onChange={(event) => {
                      setRegion(event.target.value);
                      setTested(false);
                    }}
                  />
                </label>
              ) : null}
            </>
          )}
        </div>
        <button
          className={`connection-test ${tested ? "is-success" : ""}`}
          type="button"
          disabled={!complete}
          onClick={() => setTested(true)}
        >
          {tested ? <CheckCircle2 /> : <CircleGauge />}
          <div>
            <strong>{tested ? "新凭据验证通过" : "验证新凭据"}</strong>
            <span>新凭据验证并保存前继续使用旧凭据</span>
          </div>
        </button>
        <div className="form-note">
          <KeyRound />
          保存后立即切换凭据；敏感值仅提交一次，且不写入操作日志。
        </div>
        <FormActions
          submitLabel="保存并切换"
          submitDisabled={!complete || !tested}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
function CreateService({
  initial,
  certificates,
  onClose,
  onSave,
}: {
  initial?: Service;
  certificates: Certificate[];
  onClose: () => void;
  onSave: (service: Service) => void;
}) {
  const initialAuthentication = normalizedAuthentication(
    initial?.authentication,
  );
  const [type, setType] = useState<ServiceType>(initial?.type ?? "HTTP");
  const [name, setName] = useState(initial?.name ?? "");
  const [provider, setProvider] = useState(initial?.provider ?? "内部服务");
  const [protocol, setProtocol] = useState(initial?.protocol ?? "HTTP/1.1");
  const [authentication, setAuthentication] = useState(initialAuthentication);
  const [credential, setCredential] = useState("");
  const [credentialName, setCredentialName] = useState("");
  const [credentialRegion, setCredentialRegion] = useState("");
  const [clientCertificateID, setClientCertificateID] = useState(
    initial?.clientCertificateID ?? "",
  );
  const [transportSecurity, setTransportSecurity] = useState<
    Service["transportSecurity"]
  >(initial?.transportSecurity ?? "明文连接");
  const [serverName, setServerName] = useState(initial?.serverName ?? "");
  const [trustMode, setTrustMode] = useState<"系统信任" | "指定信任证书">(
    initial?.trustCertificateID ? "指定信任证书" : "系统信任",
  );
  const [trustCertificateID, setTrustCertificateID] = useState(
    initial?.trustCertificateID ?? "",
  );
  const [endpoints, setEndpoints] = useState<
    Array<{
      address: string;
      weight: number;
    }>
  >(
    initial?.endpoints.map(({ address, weight }) => ({
      address,
      weight,
    })) ?? [
      {
        address: "service.internal:8080",
        weight: 100,
      },
    ],
  );
  const [loadBalancing, setLoadBalancing] = useState<Service["loadBalancing"]>(
    initial?.loadBalancing ?? "轮询",
  );
  const [healthMode, setHealthMode] = useState(
    initial?.healthCheck.startsWith("被动")
      ? "被动检查"
      : initial?.healthCheck.startsWith("TCP")
        ? "TCP"
        : "HTTP",
  );
  const [healthPath, setHealthPath] = useState(
    initial?.healthCheck.match(/HTTP GET (\S+)/)?.[1] ?? "/health",
  );
  const [healthInterval, setHealthInterval] = useState(
    initial?.healthCheck.match(/(\d+) 秒/)?.[1] ?? "10",
  );
  const [models, setModels] = useState(
    initial?.type === "MODEL" ? initial.capabilities.join(", ") : "",
  );
  const [modelPrices, setModelPrices] = useState<
    Record<
      string,
      {
        input: number;
        cachedInput?: number;
        output: number;
      }
    >
  >(
    Object.fromEntries(
      Object.entries(initial?.modelPrices ?? {}).map(([model, price]) => [
        model,
        {
          input: price.input,
          cachedInput: price.cachedInput,
          output: price.output,
        },
      ]),
    ),
  );
  const [modelPriceQuery, setModelPriceQuery] = useState("");
  const [discoveredTools, setDiscoveredTools] = useState<string[]>(
    initial?.type === "MCP" ? initial.capabilities : [],
  );
  const [tested, setTested] = useState(
    Boolean(initial && initial.state !== "unverified"),
  );
  const protocolOptions =
    type === "HTTP"
      ? ["HTTP/1.1", "HTTP/2"]
      : type === "MODEL"
        ? ["OpenAI 兼容 API", "Anthropic Messages", "AWS Bedrock"]
        : ["Streamable HTTP"];
  const authenticationOptions =
    type === "HTTP"
      ? ["无认证", "Bearer Token", "Basic", "mTLS"]
      : type === "MODEL"
        ? ["API Key", "Bearer Token", "自定义请求头", "AWS 签名"]
        : ["无认证", "Bearer Token", "mTLS"];
  const clientCertificates = certificates.filter(
    (item) => item.usage === "客户端证书" && item.state !== "error",
  );
  const trustCertificates = certificates.filter(
    (item) => item.usage === "信任证书" && item.state !== "error",
  );
  const effectiveClientCertificateID = clientCertificates.some(
    (item) => item.id === clientCertificateID,
  )
    ? clientCertificateID
    : "";
  const effectiveTrustCertificateID = trustCertificates.some(
    (item) => item.id === trustCertificateID,
  )
    ? trustCertificateID
    : "";
  const capabilities =
    type === "MODEL"
      ? models
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean)
      : type === "MCP"
        ? discoveredTools
        : [];
  const visiblePriceModels = capabilities.filter((model) =>
    model.toLowerCase().includes(modelPriceQuery.toLowerCase()),
  );
  const balanceWeights = (
    items: Array<{
      address: string;
      weight: number;
    }>,
  ) => {
    const baseWeight = Math.floor(100 / items.length);
    const remainder = 100 - baseWeight * items.length;
    return items.map((item, index) => ({
      ...item,
      weight: baseWeight + (index < remainder ? 1 : 0),
    }));
  };
  const updateEndpoint = (
    index: number,
    changes: Partial<{
      address: string;
      weight: number;
    }>,
  ) => {
    setEndpoints((items) =>
      items.map((item, itemIndex) =>
        itemIndex === index
          ? {
              ...item,
              ...changes,
            }
          : item,
      ),
    );
    setTested(false);
  };
  const changeType = (next: ServiceType) => {
    setType(next);
    setProvider(
      next === "HTTP"
        ? "内部服务"
        : next === "MODEL"
          ? "模型服务商"
          : "MCP 服务",
    );
    setProtocol(
      next === "HTTP"
        ? "HTTP/1.1"
        : next === "MODEL"
          ? "OpenAI 兼容 API"
          : "Streamable HTTP",
    );
    setAuthentication(
      next === "HTTP" ? "无认证" : next === "MODEL" ? "Bearer Token" : "无认证",
    );
    setEndpoints([
      {
        address:
          next === "MCP"
            ? "tools.internal:443/mcp"
            : next === "MODEL"
              ? "api.provider.com:443"
              : "service.internal:8080",
        weight: 100,
      },
    ]);
    setTransportSecurity(next === "HTTP" ? "明文连接" : "TLS");
    setCredential("");
    setCredentialName("");
    setCredentialRegion("");
    setClientCertificateID("");
    setServerName("");
    setTrustMode("系统信任");
    setTrustCertificateID("");
    setModels("");
    setModelPriceQuery("");
    setDiscoveredTools([]);
    setHealthMode(next === "MODEL" ? "被动检查" : "HTTP");
    setTested(false);
  };
  const testConnection = () => {
    if (
      type === "MODEL" &&
      protocol === "OpenAI 兼容 API" &&
      !models.trim()
    ) {
      const discoveredModels = ["qwen-max", "qwen-plus"];
      setModels(discoveredModels.join(", "));
      setModelPrices((prices) =>
        Object.fromEntries(
          discoveredModels.map((model) => [
            model,
            prices[model] ?? {
              input: 0,
              output: 0,
            },
          ]),
        ),
      );
    }
    if (type === "MCP")
      setDiscoveredTools(["web_search", "fetch_page", "extract_text"]);
    setTested(true);
  };
  const needsCredential = authentication !== "无认证";
  const canReuseStoredCredential = Boolean(
    initial &&
      authentication === initialAuthentication &&
      authentication !== "无认证",
  );
  const credentialComplete =
    !needsCredential ||
    (authentication === "mTLS"
      ? Boolean(effectiveClientCertificateID)
      : canReuseStoredCredential ||
        (authentication === "Basic" || authentication === "自定义请求头"
          ? Boolean(credentialName.trim() && credential.trim())
          : authentication === "AWS 签名"
            ? Boolean(
                credentialName.trim() &&
                  credential.trim() &&
                  credentialRegion.trim(),
              )
            : Boolean(credential.trim())));
  const tlsComplete =
    transportSecurity === "明文连接" ||
    trustMode === "系统信任" ||
    Boolean(effectiveTrustCertificateID);
  const canTest =
    endpoints.every((item) => item.address.trim()) &&
    credentialComplete &&
    tlsComplete;
  const canSave = type !== "MODEL" || capabilities.length > 0;
  const endpointRecords: ServiceEndpoint[] = endpoints.map((item) => ({
    ...item,
    weight: endpoints.length === 1 ? 100 : item.weight,
    state: "healthy",
  }));
  const healthCheck =
    healthMode === "被动检查"
      ? "被动健康检查"
      : healthMode === "TCP"
        ? `TCP 连接 · ${healthInterval} 秒`
        : `HTTP GET ${healthPath} · ${healthInterval} 秒`;
  const savedAuthentication =
    authentication === "mTLS"
      ? `mTLS · ${clientCertificates.find((item) => item.id === effectiveClientCertificateID)?.name}`
      : authentication;
  const save = () =>
    onSave({
      id: initial?.id ?? `svc-${Date.now()}`,
      name,
      type,
      endpoints: endpointRecords,
      provider,
      protocol,
      authentication: savedAuthentication,
      clientCertificateID:
        authentication === "mTLS" ? effectiveClientCertificateID : undefined,
      transportSecurity,
      serverName:
        transportSecurity === "TLS" ? serverName || undefined : undefined,
      trustCertificateID:
        transportSecurity === "TLS" && trustMode === "指定信任证书"
          ? effectiveTrustCertificateID
          : undefined,
      loadBalancing,
      healthCheck,
      capabilities,
      modelPrices:
        type === "MODEL"
          ? Object.fromEntries(
              capabilities.map((model) => [
                model,
                {
                  ...(modelPrices[model] ?? {
                    input: 0,
                    output: 0,
                  }),
                  unit: "每百万 Token" as const,
                  updatedAt: new Date().toISOString().slice(0, 10),
                },
              ]),
            )
          : undefined,
      successRate: initial?.successRate ?? "—",
      latency: initial?.latency ?? "—",
      state: tested ? "healthy" : "unverified",
      verificationState: tested ? "verified" : "unverified",
    });
  return (
    <Drawer
      title={initial ? "编辑服务" : "创建服务"}
      description="连接、认证、TLS、端点与健康检查"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="type-selector">
          {(["HTTP", "MODEL", "MCP"] as const).map((item) => (
            <button
              key={item}
              type="button"
              className={type === item ? "is-selected" : ""}
              onClick={() => changeType(item)}
            >
              <ServiceTypeBadge type={item} />
              <strong>
                {item === "HTTP"
                  ? "普通业务服务"
                  : item === "MODEL"
                    ? "模型厂商或推理服务"
                    : "远程工具服务"}
              </strong>
              <small>
                {item === "HTTP"
                  ? "普通业务接口"
                  : item === "MODEL"
                    ? "模型厂商或自托管推理服务"
                    : "远程 MCP 服务"}
              </small>
            </button>
          ))}
        </div>
        <section className="form-section">
          <header>
            <span>1</span>
            <div>
              <strong>连接信息</strong>
              <small>服务协议和上游身份认证</small>
            </div>
          </header>
          <div className="form-grid">
            <label>
              <span>服务名称</span>
              <input
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </label>
            <label>
              <span>服务提供方</span>
              <input
                required
                value={provider}
                onChange={(event) => setProvider(event.target.value)}
              />
            </label>
            <label>
              <span>{type === "MCP" ? "传输方式" : "请求协议"}</span>
              <select
                value={protocol}
                onChange={(event) => {
                  setProtocol(event.target.value);
                  setTested(false);
                }}
              >
                {protocolOptions.map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </select>
            </label>
            <label>
              <span>认证方式</span>
              <select
                value={authentication}
                onChange={(event) => {
                  setAuthentication(event.target.value);
                  setCredential("");
                  setCredentialName("");
                  setCredentialRegion("");
                  setClientCertificateID("");
                  setTested(false);
                }}
              >
                {authenticationOptions.map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </select>
            </label>
            {authentication === "mTLS" ? (
              <label className="field-wide">
                <span>客户端证书</span>
                <select
                  required
                  value={effectiveClientCertificateID}
                  onChange={(event) => {
                    setClientCertificateID(event.target.value);
                    setTested(false);
                  }}
                >
                  <option value="">选择客户端证书</option>
                  {clientCertificates.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} · {item.issuer}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            {authentication === "Basic" ? (
              <>
                <label>
                  <span>用户名</span>
                  <input
                    required={!canReuseStoredCredential}
                    value={credentialName}
                    onChange={(event) => {
                      setCredentialName(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的用户名"
                        : ""
                    }
                  />
                </label>
                <label>
                  <span>密码</span>
                  <input
                    required={!canReuseStoredCredential}
                    type="password"
                    value={credential}
                    onChange={(event) => {
                      setCredential(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的密码"
                        : "保存后不再显示"
                    }
                  />
                </label>
              </>
            ) : null}
            {authentication === "自定义请求头" ? (
              <>
                <label>
                  <span>请求头名称</span>
                  <input
                    required={!canReuseStoredCredential}
                    value={credentialName}
                    onChange={(event) => {
                      setCredentialName(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的名称"
                        : "例如 x-api-key"
                    }
                  />
                </label>
                <label>
                  <span>请求头值</span>
                  <input
                    required={!canReuseStoredCredential}
                    type="password"
                    value={credential}
                    onChange={(event) => {
                      setCredential(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的值"
                        : "保存后不再显示"
                    }
                  />
                </label>
              </>
            ) : null}
            {authentication === "AWS 签名" ? (
              <>
                <label>
                  <span>Access Key ID</span>
                  <input
                    required={!canReuseStoredCredential}
                    value={credentialName}
                    onChange={(event) => {
                      setCredentialName(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的 ID"
                        : ""
                    }
                  />
                </label>
                <label>
                  <span>Secret Access Key</span>
                  <input
                    required={!canReuseStoredCredential}
                    type="password"
                    value={credential}
                    onChange={(event) => {
                      setCredential(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的密钥"
                        : ""
                    }
                  />
                </label>
                <label className="field-wide">
                  <span>AWS 区域</span>
                  <input
                    required={!canReuseStoredCredential}
                    value={credentialRegion}
                    onChange={(event) => {
                      setCredentialRegion(event.target.value);
                      setTested(false);
                    }}
                    placeholder={
                      canReuseStoredCredential
                        ? "留空表示继续使用已保存的区域"
                        : "例如 us-east-1"
                    }
                  />
                </label>
              </>
            ) : null}
            {authentication === "Bearer Token" ||
            authentication === "API Key" ? (
              <label className="field-wide">
                <span>{authentication}</span>
                <input
                  required={!canReuseStoredCredential}
                  type="password"
                  value={credential}
                  onChange={(event) => {
                    setCredential(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的凭据"
                      : "保存后不再显示明文"
                  }
                />
              </label>
            ) : null}
            {type === "MODEL" ? (
              <label className="field-wide">
                <span>实际模型 ID（可选）</span>
                <input
                  value={models}
                  onChange={(event) => {
                    setModels(event.target.value);
                    setTested(false);
                  }}
                  placeholder="多个模型用逗号分隔；也可在验证连接时读取"
                />
                <small>
                  部分厂商不提供模型列表接口，此时需要手动填写
                </small>
              </label>
            ) : null}
          </div>
        </section>
        <section className="form-section">
          <header>
            <span>2</span>
            <div>
              <strong>服务端点</strong>
              <small>同一服务的多个实例共享协议、认证和 TLS 配置</small>
            </div>
          </header>
          <div className="endpoint-editor">
            {endpoints.map((endpoint, index) => (
              <div className="endpoint-row" key={index}>
                <label>
                  <span>端点地址</span>
                  <input
                    required
                    value={endpoint.address}
                    onChange={(event) =>
                      updateEndpoint(index, {
                        address: event.target.value,
                      })
                    }
                  />
                </label>
                {endpoints.length > 1 ? (
                  <label>
                    <span>权重（%）</span>
                    <input
                      type="number"
                      min="1"
                      max="100"
                      value={endpoint.weight}
                      onChange={(event) =>
                        updateEndpoint(index, {
                          weight: Number(event.target.value),
                        })
                      }
                    />
                  </label>
                ) : null}
                <button
                  type="button"
                  aria-label="删除端点"
                  disabled={endpoints.length === 1}
                  onClick={() => {
                    setEndpoints((items) =>
                      balanceWeights(
                        items.filter((_, itemIndex) => itemIndex !== index),
                      ),
                    );
                    setTested(false);
                  }}
                >
                  <Trash2 />
                </button>
              </div>
            ))}
          </div>
          <button
            className="text-action"
            type="button"
            onClick={() => {
              setEndpoints((items) =>
                balanceWeights([
                  ...items,
                  {
                    address: "",
                    weight: 0,
                  },
                ]),
              );
              setTested(false);
            }}
          >
            <Plus />
            添加端点
          </button>
          {endpoints.length > 1 ? (
            <div className="form-grid load-balance-field">
              <label>
                <span>负载方式</span>
                <select
                  value={loadBalancing}
                  onChange={(event) =>
                    setLoadBalancing(
                      event.target.value as Service["loadBalancing"],
                    )
                  }
                >
                  <option>轮询</option>
                  <option>最少请求</option>
                  <option>随机</option>
                </select>
              </label>
              <div className="weight-summary">
                <span>当前权重合计</span>
                <strong
                  className={
                    endpoints.reduce((sum, item) => sum + item.weight, 0) ===
                    100
                      ? ""
                      : "is-warning"
                  }
                >
                  {endpoints.reduce((sum, item) => sum + item.weight, 0)}%
                </strong>
              </div>
            </div>
          ) : null}
        </section>
        <section className="form-section">
          <header>
            <span>3</span>
            <div>
              <strong>传输安全</strong>
              <small>TLS 服务端校验与可选的客户端证书是两件事</small>
            </div>
          </header>
          <div className="form-grid">
            <label>
              <span>连接加密</span>
              <select
                value={transportSecurity}
                onChange={(event) => {
                  setTransportSecurity(
                    event.target.value as Service["transportSecurity"],
                  );
                  setTested(false);
                }}
              >
                <option>明文连接</option>
                <option>TLS</option>
              </select>
            </label>
            {transportSecurity === "TLS" ? (
              <>
                <label>
                  <span>服务端名称</span>
                  <input
                    value={serverName}
                    onChange={(event) => {
                      setServerName(event.target.value);
                      setTested(false);
                    }}
                    placeholder="默认从端点地址推导"
                  />
                </label>
                <label>
                  <span>证书信任</span>
                  <select
                    value={trustMode}
                    onChange={(event) => {
                      setTrustMode(
                        event.target.value as "系统信任" | "指定信任证书",
                      );
                      setTrustCertificateID("");
                      setTested(false);
                    }}
                  >
                    <option>系统信任</option>
                    <option>指定信任证书</option>
                  </select>
                </label>
                {trustMode === "指定信任证书" ? (
                  <label>
                    <span>信任证书</span>
                    <select
                      required
                      value={effectiveTrustCertificateID}
                      onChange={(event) => {
                        setTrustCertificateID(event.target.value);
                        setTested(false);
                      }}
                    >
                      <option value="">选择 CA 证书</option>
                      {trustCertificates.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : null}
              </>
            ) : null}
          </div>
        </section>
        <details className="advanced-config">
          <summary>健康检查</summary>
          <div className="form-grid">
            <label>
              <span>检查方式</span>
              <select
                value={healthMode}
                onChange={(event) => setHealthMode(event.target.value)}
              >
                <option>HTTP</option>
                <option>TCP</option>
                <option>被动检查</option>
              </select>
            </label>
            {healthMode !== "被动检查" ? (
              <label>
                <span>检查周期（秒）</span>
                <input
                  type="number"
                  min="2"
                  value={healthInterval}
                  onChange={(event) => setHealthInterval(event.target.value)}
                />
              </label>
            ) : null}
            {healthMode === "HTTP" ? (
              <label className="field-wide">
                <span>检查路径</span>
                <input
                  value={healthPath}
                  onChange={(event) => setHealthPath(event.target.value)}
                />
              </label>
            ) : null}
          </div>
        </details>
        <button
          className={`connection-test ${tested ? "is-success" : ""}`}
          type="button"
          disabled={!canTest}
          onClick={testConnection}
        >
          {tested ? <CheckCircle2 /> : <CircleGauge />}
          <div>
            <strong>
              {tested
                ? "连接验证通过"
                : type === "MCP"
                  ? "测试连接并发现工具"
                  : type === "MODEL"
                    ? "测试连接并读取模型 ID"
                    : "测试连接"}
            </strong>
            <span>
              {tested
                ? type === "MCP"
                  ? `已发现 ${discoveredTools.length} 个工具`
                  : type === "MODEL"
                    ? capabilities.length
                      ? `已确认 ${capabilities.length} 个实际模型 ID`
                      : "连接通过，请手动填写实际模型 ID"
                    : `已验证 ${endpoints.length} 个端点、认证和 TLS`
                : "验证端点、认证、TLS 和协议兼容性"}
            </span>
          </div>
        </button>
        {tested && type !== "HTTP" ? (
          <div className="discovery-result">
            <strong>{type === "MODEL" ? "实际模型 ID" : "已发现工具"}</strong>
            <CompactTagList items={capabilities} limit={5} />
          </div>
        ) : null}
        {tested && type === "MODEL" ? (
          <section className="model-price-editor">
            <header>
              <div>
                <strong>模型单价</strong>
                <small>
                  仅对返回 Token 用量的调用预估成本
                </small>
              </div>
              <span>人民币 / 百万 Token</span>
            </header>
            <div className="model-price-toolbar">
              <SearchField
                value={modelPriceQuery}
                onChange={setModelPriceQuery}
                placeholder="搜索实际模型"
              />
              <span>
                显示 {visiblePriceModels.length} / {capabilities.length}
              </span>
            </div>
            <div className="model-price-list">
              {visiblePriceModels.map((model) => (
                <div key={model}>
                  <code>{model}</code>
                  <label>
                    <span>输入</span>
                    <input
                      type="number"
                      min="0"
                      step="0.01"
                      value={modelPrices[model]?.input ?? 0}
                      onChange={(event) =>
                        setModelPrices((prices) => ({
                          ...prices,
                          [model]: {
                            input: Number(event.target.value),
                            cachedInput: prices[model]?.cachedInput,
                            output: prices[model]?.output ?? 0,
                          },
                        }))
                      }
                    />
                  </label>
                  <label>
                    <span>缓存输入</span>
                    <input
                      type="number"
                      min="0"
                      step="0.01"
                      value={modelPrices[model]?.cachedInput ?? 0}
                      onChange={(event) =>
                        setModelPrices((prices) => ({
                          ...prices,
                          [model]: {
                            input: prices[model]?.input ?? 0,
                            cachedInput: Number(event.target.value),
                            output: prices[model]?.output ?? 0,
                          },
                        }))
                      }
                    />
                  </label>
                  <label>
                    <span>输出</span>
                    <input
                      type="number"
                      min="0"
                      step="0.01"
                      value={modelPrices[model]?.output ?? 0}
                      onChange={(event) =>
                        setModelPrices((prices) => ({
                          ...prices,
                          [model]: {
                            input: prices[model]?.input ?? 0,
                            cachedInput: prices[model]?.cachedInput,
                            output: Number(event.target.value),
                          },
                        }))
                      }
                    />
                  </label>
                </div>
              ))}
            </div>
          </section>
        ) : null}
        {!tested ? (
          <div className="form-note">
            <CircleGauge />
            未验证的服务可保存为“待验证”，但不能被新路由引用。
          </div>
        ) : null}
        <FormActions
          submitLabel={
            initial ? "保存修改" : tested ? "保存服务" : "保存为待验证"
          }
          submitDisabled={
            !canTest || !canSave ||
            (endpoints.length > 1 &&
              endpoints.reduce((sum, item) => sum + item.weight, 0) !== 100)
          }
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
function normalizedAuthentication(authentication?: string) {
  if (authentication?.startsWith("mTLS")) return "mTLS";
  return authentication ?? "无认证";
}

function summarizeItems(items: string[], limit = 3) {
  const visible = items.slice(0, limit).join("、");
  return items.length > limit ? `${visible} 等 ${items.length} 项` : visible;
}

export function CertificatePage() {
  const {
    certificates,
    gateways,
    services,
    addCertificate,
    updateCertificate,
    deleteCertificate,
  } = usePrototype();
  const [selected, setSelected] = useState<
    (typeof certificates)[number] | null
  >(null);
  const [editing, setEditing] = useState<Certificate | null>(null);
  const [deleting, setDeleting] = useState<Certificate | null>(null);
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState("");
  const [toast, setToast] = useState("");
  const visible = certificates.filter((certificate) =>
    `${certificate.name}${certificate.identities.join("")}${certificate.issuer}${certificate.usage}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  const referencesFor = (certificateID: string) => [
    ...gateways
      .filter((gateway) =>
        gateway.listeners.some((listener) =>
          listener.bindings.some(
            (binding) => binding.certificateID === certificateID,
          ),
        ),
      )
      .map((gateway) => ({
        name: gateway.name,
        detail: "网关 HTTPS 域名绑定",
      })),
    ...services
      .filter((service) => service.clientCertificateID === certificateID)
      .map((service) => ({
        name: service.name,
        detail: "上游 mTLS 客户端身份",
      })),
    ...services
      .filter((service) => service.trustCertificateID === certificateID)
      .map((service) => ({
        name: service.name,
        detail: "上游 TLS 服务端信任",
      })),
  ];
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="流量配置"
        title="证书"
        description="网关 HTTPS 与上游 TLS 证书"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            导入证书
          </PrimaryButton>
        }
      />
      <section className="certificate-ribbon">
        <div>
          <LockKeyhole />
          <span>
            <strong>{certificates.length}</strong> 张证书
          </span>
        </div>
        <div>
          <CheckCircle2 />
          <span>
            <strong>
              {certificates.filter((item) => item.state === "healthy").length}
            </strong>{" "}
            张状态正常
          </span>
        </div>
        <div className="warning">
          <Clock3 />
          <span>
            <strong>
              {certificates.filter((item) => item.remainingDays < 30).length}
            </strong>{" "}
            张将在 30 天内过期
          </span>
        </div>
      </section>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索证书、标识或签发机构"
          />
          <span>{visible.length} 张证书</span>
        </header>
        <div className="table-head certificate-columns">
          <span>证书</span>
          <span>证书标识</span>
          <span>签发机构</span>
          <span>到期时间</span>
          <span>资源引用</span>
          <span>有效性 / 配置</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((certificate) => {
            const references = referencesFor(certificate.id);
            return (
              <div
                key={certificate.id}
                className="table-row certificate-columns"
              >
                <div className="name-cell">
                  <span>
                    <KeyRound />
                  </span>
                  <div>
                    <strong>{certificate.name}</strong>
                    <small>{certificate.usage}</small>
                  </div>
                </div>
                <CompactTagList items={certificate.identities} />
                <span>{certificate.issuer}</span>
                <div>
                  <strong>{certificate.expiresAt}</strong>
                  <small>剩余 {certificate.remainingDays} 天</small>
                </div>
                <CompactTagList
                  items={references.map((reference) => reference.name)}
                  empty="未引用"
                />
                <div className="certificate-state"><StatusBadge state={certificate.state} label={certificate.state === "warning" ? "即将到期" : certificate.state === "error" ? "不可用" : "有效"} /><ConfigBadge state={certificate.configState} /></div>
                <RowActions
                  onDetail={() => setSelected(certificate)}
                  onEdit={() => setEditing(certificate)}
                  onDelete={() => setDeleting(certificate)}
                />
              </div>
            );
          })
        ) : (
          <EmptyState title="没有匹配的证书" description="请调整搜索条件。" />
        )}
      </section>
      {selected ? (
        <Drawer
          title={selected.name}
          description={selected.usage}
          onClose={() => setSelected(null)}
        >
          <div className="detail-hero">
            <span>
              <KeyRound />
            </span>
            <div>
              <div className="certificate-state">
                <StatusBadge
                  state={selected.state}
                  label={
                    selected.state === "warning"
                      ? "即将到期"
                      : selected.state === "error"
                        ? "不可用"
                        : "有效"
                  }
                />
                <ConfigBadge state={selected.configState} />
              </div>
              <h3>{summarizeItems(selected.identities, 4)}</h3>
              <p>
                {selected.issuer} · {selected.expiresAt} 到期
              </p>
              <div className="certificate-identities">
                {selected.identities.map((identity) => (
                  <code key={identity}>{identity}</code>
                ))}
              </div>
            </div>
          </div>
          <section className="detail-section">
            <header>
              <h3>引用关系</h3>
            </header>
            {referencesFor(selected.id).length ? (
              referencesFor(selected.id).map((reference) => (
                <div
                  className="detail-line"
                  key={`${reference.name}-${reference.detail}`}
                >
                  <span>
                    <Network />
                  </span>
                  <div>
                    <strong>{reference.name}</strong>
                    <small>{reference.detail}</small>
                  </div>
                  <span>引用中</span>
                </div>
              ))
            ) : (
              <EmptyState
                title="证书未被引用"
                description={
                  selected.usage === "服务器证书"
                    ? "可绑定到 HTTPS 监听入口的域名。"
                    : selected.usage === "客户端证书"
                      ? "可作为服务的 mTLS 客户端身份。"
                      : "可校验上游 TLS 服务端证书。"
                }
              />
            )}
          </section>
          <div className="form-note">
            <LockKeyhole />
            {selected.usage === "信任证书"
              ? "信任证书不包含私钥。"
              : "私钥仅在导入时提交，不再显示且不写入审计详情。"}
          </div>
        </Drawer>
      ) : null}
      {creating ? (
        <CreateCertificate
          onClose={() => setCreating(false)}
          onSave={(certificate) => {
            addCertificate(certificate);
            setCreating(false);
            setToast("证书已导入，正在发布");
          }}
        />
      ) : null}
      {editing ? (
        <CreateCertificate
          initial={editing}
          onClose={() => setEditing(null)}
          onSave={(certificate) => {
            updateCertificate(certificate);
            setEditing(null);
            setToast("证书已更新，正在发布");
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="证书"
          resourceName={deleting.name}
          blockedReason={
            referencesFor(deleting.id).length
              ? `该证书仍被 ${referencesFor(deleting.id)
                  .map((reference) => reference.name)
                  .join("、")} 引用。请先解除引用。`
              : undefined
          }
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deleteCertificate(deleting.id);
            setDeleting(null);
            setToast("证书已删除，正在发布");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
function CreateCertificate({
  initial,
  onClose,
  onSave,
}: {
  initial?: Certificate;
  onClose: () => void;
  onSave: (certificate: Certificate) => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [intendedUsage, setIntendedUsage] = useState<Certificate["usage"]>(
    initial?.usage ?? "服务器证书",
  );
  const [inputMode, setInputMode] = useState<"file" | "paste">("file");
  const [certificatePEM, setCertificatePEM] = useState("");
  const [privateKeyPEM, setPrivateKeyPEM] = useState("");
  const [certificateFileName, setCertificateFileName] = useState("");
  const [privateKeyFileName, setPrivateKeyFileName] = useState("");
  const [parsed, setParsed] = useState(false);
  const requiresPrivateKey = intendedUsage !== "信任证书";
  const validPaste =
    certificatePEM.includes("-----BEGIN CERTIFICATE-----") &&
    certificatePEM.includes("-----END CERTIFICATE-----") &&
    (!requiresPrivateKey ||
      privateKeyPEM.includes("-----BEGIN PRIVATE KEY-----") ||
      privateKeyPEM.includes("-----BEGIN RSA PRIVATE KEY-----"));
  const validFiles =
    Boolean(certificateFileName) &&
    (!requiresPrivateKey || Boolean(privateKeyFileName));
  const canParse = inputMode === "file" ? validFiles : validPaste;
  const identities =
    intendedUsage === "服务器证书"
      ? ["*.new.example.com", "new.example.com"]
      : intendedUsage === "客户端证书"
        ? ["ingate-client.internal"]
        : ["企业内部根 CA"];
  const issuer =
    intendedUsage === "服务器证书" ? "Let's Encrypt R13" : "企业内部根 CA";
  const resetParsing = () => setParsed(false);
  const save = () =>
    onSave({
      id: initial?.id ?? `cert-${Date.now()}`,
      name,
      identities,
      issuer,
      usage: intendedUsage,
      expiresAt: "2027-08-12",
      remainingDays: 365,
      sourceName: inputMode === "file" ? certificateFileName : undefined,
      state: "healthy",
    });
  return (
    <Drawer
      title={initial ? "更新证书" : "导入证书"}
      description={
        initial
          ? "上传新证书内容，资源引用将保持不变"
          : "系统读取用途、标识、签发机构和有效期"
      }
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="form-grid">
          <label>
            <span>证书名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如 example.com 生产证书"
            />
          </label>
          <label>
            <span>用于</span>
            <select
              value={intendedUsage}
              onChange={(event) => {
                setIntendedUsage(event.target.value as Certificate["usage"]);
                resetParsing();
              }}
            >
              <option>服务器证书</option>
              <option>客户端证书</option>
              <option>信任证书</option>
            </select>
            <small>解析时会校验证书是否符合这个用途</small>
          </label>
        </div>
        <div
          className="input-method-selector"
          role="tablist"
          aria-label="证书输入方式"
        >
          <button
            type="button"
            className={inputMode === "file" ? "is-selected" : ""}
            onClick={() => {
              setInputMode("file");
              resetParsing();
            }}
          >
            上传文件
          </button>
          <button
            type="button"
            className={inputMode === "paste" ? "is-selected" : ""}
            onClick={() => {
              setInputMode("paste");
              resetParsing();
            }}
          >
            手动粘贴
          </button>
        </div>
        {inputMode === "file" ? (
          <div className="form-grid certificate-files">
            <label className="field-wide">
              <span>证书链文件</span>
              <input
                required
                type="file"
                accept=".pem,.crt,.cer"
                onChange={(event) => {
                  setCertificateFileName(event.target.files?.[0]?.name ?? "");
                  resetParsing();
                }}
              />
              <small>{certificateFileName || "支持 PEM、CRT 或 CER"}</small>
            </label>
            {requiresPrivateKey ? (
              <label className="field-wide">
                <span>私钥文件</span>
                <input
                  required
                  type="file"
                  accept=".pem,.key"
                  onChange={(event) => {
                    setPrivateKeyFileName(event.target.files?.[0]?.name ?? "");
                    resetParsing();
                  }}
                />
                <small>
                  {privateKeyFileName || "支持 PEM 或 KEY；导入后不再显示内容"}
                </small>
              </label>
            ) : null}
          </div>
        ) : (
          <div className="form-grid">
            <label className="field-wide">
              <span>证书链 PEM</span>
              <textarea
                required
                value={certificatePEM}
                onChange={(event) => {
                  setCertificatePEM(event.target.value);
                  resetParsing();
                }}
                placeholder="-----BEGIN CERTIFICATE-----"
              />
            </label>
            {requiresPrivateKey ? (
              <label className="field-wide">
                <span>私钥 PEM</span>
                <textarea
                  required
                  value={privateKeyPEM}
                  onChange={(event) => {
                    setPrivateKeyPEM(event.target.value);
                    resetParsing();
                  }}
                  placeholder="-----BEGIN PRIVATE KEY-----"
                />
              </label>
            ) : null}
          </div>
        )}
        <button
          className={`connection-test ${parsed ? "is-success" : ""}`}
          type="button"
          disabled={!canParse}
          onClick={() => setParsed(true)}
        >
          {parsed ? <CheckCircle2 /> : <CircleGauge />}
          <div>
            <strong>
              {parsed
                ? requiresPrivateKey
                  ? "证书与私钥匹配"
                  : "信任证书校验通过"
                : "解析并校验证书"}
            </strong>
            <span>
              {parsed
                ? "用途、标识、签发机构和有效期已读取"
                : "保存前检查格式、用途、有效期和私钥匹配关系"}
            </span>
          </div>
        </button>
        {parsed ? (
          <dl className="definition-list certificate-preview">
            <div>
              <dt>用途</dt>
              <dd>{intendedUsage}</dd>
            </div>
            <div>
              <dt>
                {intendedUsage === "服务器证书"
                  ? "覆盖域名"
                  : intendedUsage === "客户端证书"
                    ? "客户端标识"
                    : "证书主体"}
              </dt>
              <dd>
                {identities.map((identity) => (
                  <code key={identity}>{identity}</code>
                ))}
              </dd>
            </div>
            <div>
              <dt>签发机构</dt>
              <dd>{issuer}</dd>
            </div>
            <div>
              <dt>有效期</dt>
              <dd>2026-08-12 至 2027-08-12</dd>
            </div>
            <div>
              <dt>指纹</dt>
              <dd>
                <code>SHA256 · A8:31:7F:••:9C</code>
              </dd>
            </div>
          </dl>
        ) : null}
        <FormActions
          submitLabel={initial ? "保存并替换" : "导入证书"}
          submitDisabled={!parsed || !canParse}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
