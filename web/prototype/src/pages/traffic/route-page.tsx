import { Plus } from "lucide-react";
import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  ConfigBadge,
  DeleteConfirm,
  EmptyState,
  FilterTabs,
  PageHeader,
  PrimaryButton,
  RouteTypeBadge,
  RowActions,
  SearchField,
  Toast,
} from "../../components/ui";
import type {
  GatewayRoute,
  TrafficType,
} from "../../data";
import { usePrototype } from "../../prototype-context";
import { RouteDetail } from "./route-detail";
import { CreateRoute } from "./route-form";

type RouteFilter = "ALL" | TrafficType;
export function RoutePage() {
  const {
    routes,
    gateways,
    services,
    policies,
    callers,
    identitySources,
    addRoute,
    updateRoute,
    deleteRoute,
  } = usePrototype();
  const [params] = useSearchParams();
  const preferredServiceID = params.get("service") ?? undefined;
  const preferredService = services.find(
    (service) => service.id === preferredServiceID,
  );
  const [filter, setFilter] = useState<RouteFilter>(
    preferredService?.type === "MODEL"
      ? "AI"
      : preferredService?.type === "MCP"
        ? "MCP"
        : preferredService
          ? "API"
          : "ALL",
  );
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<GatewayRoute | null>(null);
  const [editing, setEditing] = useState<GatewayRoute | null>(null);
  const [deleting, setDeleting] = useState<GatewayRoute | null>(null);
  const [creating, setCreating] = useState(params.get("create") === "1");
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
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            创建路由
          </PrimaryButton>
        }
      />
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索路由、域名、模型或工具"
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
          authorizedCallerCount={callers.filter((caller) =>
            caller.permissions.some(
              (permission) => permission.routeID === selected.id,
            ),
          ).length}
          onClose={() => setSelected(null)}
        />
      ) : null}
      {creating ? (
        <CreateRoute
          routes={routes}
          gateways={gateways}
          services={services}
          identitySources={identitySources}
          preferredServiceID={preferredServiceID}
          onClose={() => setCreating(false)}
          onSave={(route) => {
            addRoute(route);
            setCreating(false);
            setSelected(route);
            setToast("路由已保存");
          }}
        />
      ) : null}
      {editing ? (
        <CreateRoute
          initial={editing}
          routes={routes}
          gateways={gateways}
          services={services}
          identitySources={identitySources}
          onClose={() => setEditing(null)}
          onSave={(route) => {
            updateRoute(route);
            setEditing(null);
            setToast("路由修改已保存");
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
            setToast("路由已删除");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
