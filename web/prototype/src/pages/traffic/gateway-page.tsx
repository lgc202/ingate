import {
  Network,
  Plus,
} from "lucide-react";
import { useState } from "react";
import {
  CompactTagList,
  ConfigBadge,
  DeleteConfirm,
  EmptyState,
  Metric,
  PageHeader,
  PrimaryButton,
  RowActions,
  SearchField,
  Toast,
} from "../../components/ui";
import type { Gateway } from "../../data";
import { usePrototype } from "../../prototype-context";
import { GatewayDetail } from "./gateway-detail";
import { CreateGateway } from "./gateway-form";
import {
  gatewayCertificateNames,
  gatewayDomains,
  listenerLabel,
} from "./gateway-model";

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
        <Metric label="配置异常" value={String(gateways.filter((gateway) => gateway.configState === "failed").length)} note="需要检查相关网关配置" tone={gateways.some((gateway) => gateway.configState === "failed") ? "warning" : "good"} />
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
          label="已生效路由"
          value={String(
            routes.filter(
              (route) => (route.configState ?? "active") === "active",
            ).length,
          )}
          note="当前配置已生效"
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
            setToast("网关已保存");
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
            setToast("网关修改已保存");
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
            setToast("网关已删除");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
