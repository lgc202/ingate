import { Plus } from "lucide-react";
import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  CompactTagList,
  ConfigBadge,
  DeleteConfirm,
  EmptyState,
  FilterTabs,
  Metric,
  PageHeader,
  PrimaryButton,
  RowActions,
  SearchField,
  ServiceTypeBadge,
  StatusBadge,
  Toast,
} from "../../components/ui";
import type {
  Service,
  ServiceType,
} from "../../data";
import { usePrototype } from "../../prototype-context";
import { RotateServiceCredential } from "./service-credential-form";
import { ServiceDetail } from "./service-detail";
import { CreateService } from "./service-form";

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
  const [params] = useSearchParams();
  const [filter, setFilter] = useState<ServiceFilter>("ALL");
  const [query, setQuery] = useState(params.get("query") ?? "");
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
          label="配置已校验"
          value={`${verifiedServices} / ${services.length}`}
          note="端点、认证与协议校验"
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
          <span>配置校验</span>
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
              <div className="service-signal">
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
                      ? "校验失败"
                      : service.verificationState === "unverified"
                        ? "待校验"
                        : "配置已校验"
                  }
                />
                {service.state === "warning" || service.state === "error" ? (
                  <small>{service.successRate} · {service.latency}</small>
                ) : null}
              </div>
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
            setSelected(service);
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
            setToast("服务修改已保存");
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
            setToast("服务已删除");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
