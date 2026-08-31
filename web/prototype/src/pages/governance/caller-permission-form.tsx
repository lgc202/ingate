import {
  ChevronRight,
  ShieldCheck,
  X,
} from "lucide-react";
import { useState } from "react";
import {
  Drawer,
  EmptyState,
  FilterSelect,
  FormActions,
  RouteTypeBadge,
  SearchField,
  submitForm,
} from "../../components/ui";
import type {
  Caller,
  CallerPermission,
  GatewayRoute,
  TrafficType,
} from "../../data";
import type { PermissionDraft } from "./caller-types";

export function ManagePermissions({
  caller,
  routes,
  onClose,
  onSave,
}: {
  caller: Caller;
  routes: GatewayRoute[];
  onClose: () => void;
  onSave: (permissions: CallerPermission[]) => void;
}) {
  const [permissions, setPermissions] = useState<PermissionDraft>(() =>
    Object.fromEntries(
      caller.permissions.map((permission) => [
        permission.routeID,
        permission.scopes,
      ]),
    ),
  );
  return (
    <Drawer
      title="管理访问权限"
      description={caller.name}
      onClose={onClose}
      width="wide"
    >
      <form
        onSubmit={(event) =>
          submitForm(event, () => onSave(permissionRecords(permissions)))
        }
      >
        <PermissionSelector
          routes={routes}
          value={permissions}
          onChange={setPermissions}
        />
        <div className="form-note">
          <ShieldCheck />
          移除访问项会同时删除对应额度；密钥本身保持有效，但后续请求立即按最新权限校验。
        </div>
        <FormActions submitLabel="保存权限" onCancel={onClose} />
      </form>
    </Drawer>
  );
}

export function PermissionSelector({
  sectionNumber,
  routes,
  value,
  onChange,
  expanded,
  onToggle,
}: {
  sectionNumber?: string;
  routes: GatewayRoute[];
  value: PermissionDraft;
  onChange: (value: PermissionDraft) => void;
  expanded?: boolean;
  onToggle?: () => void;
}) {
  const [query, setQuery] = useState("");
  const [type, setType] = useState<"ALL" | TrafficType>("ALL");
  const protectedRoutes = routes.filter(
    (route) => route.accessMode === "API Key",
  );
  const selectedRoutes = protectedRoutes.filter((route) => value[route.id]);
  const visibleRoutes = protectedRoutes.filter(
    (route) =>
      (type === "ALL" || route.type === type) &&
      `${route.name}${route.host}${route.path}${route.published.join("")}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const toggleRoute = (route: GatewayRoute) => {
    const next = { ...value };
    if (next[route.id]) delete next[route.id];
    else next[route.id] = [...route.published];
    onChange(next);
  };
  const toggleScope = (route: GatewayRoute, scope: string) => {
    const current = value[route.id] ?? [];
    if (current.includes(scope) && current.length === 1) return;
    onChange({
      ...value,
      [route.id]: current.includes(scope)
        ? current.filter((item) => item !== scope)
        : [...current, scope],
    });
  };
  return (
    <section className={`form-section permission-editor ${onToggle ? "caller-create-section" : ""}`}>
      {onToggle ? (
        <button
          className="caller-create-section-toggle"
          type="button"
          onClick={onToggle}
          aria-expanded={expanded}
        >
          <span>{sectionNumber}</span>
          <div>
            <strong>访问权限</strong>
            <small>
              {selectedRoutes.length
                ? `${selectedRoutes.length} 条路由 · ${permissionScopeCount(value)} 项能力`
                : "选择可调用的 API、模型和工具"}
            </small>
          </div>
          <ChevronRight />
        </button>
      ) : (
        <header>
          <span><ShieldCheck /></span>
          <div>
            <strong>访问权限</strong>
            <small>选择可调用能力</small>
          </div>
          <b>
            {selectedRoutes.length} 条路由 · {permissionScopeCount(value)} 项能力
          </b>
        </header>
      )}
      <div className="permission-editor-body" hidden={expanded === false}>
      {selectedRoutes.length ? (
        <div className="permission-selected">
          {selectedRoutes.map((route) => (
            <span key={route.id}>
              <RouteTypeBadge type={route.type} />
              <strong>{route.name}</strong>
              <small>{value[route.id]?.length ?? 0} 项能力</small>
              <button
                type="button"
                aria-label={`移除${route.name}`}
                onClick={() => toggleRoute(route)}
              >
                <X />
              </button>
            </span>
          ))}
        </div>
      ) : (
        <div className="permission-selected-empty">
          尚未选择可调用能力
        </div>
      )}
      <div className="permission-toolbar">
        <SearchField
          value={query}
          onChange={setQuery}
          placeholder="搜索能力或访问地址"
        />
        <FilterSelect
          label="流量类型"
          value={type}
          onChange={setType}
          options={[
            { value: "ALL", label: "全部", count: protectedRoutes.length },
            {
              value: "API",
              label: "API 路由",
              count: protectedRoutes.filter((route) => route.type === "API")
                .length,
            },
            {
              value: "AI",
              label: "AI 路由",
              count: protectedRoutes.filter((route) => route.type === "AI")
                .length,
            },
            {
              value: "MCP",
              label: "MCP 路由",
              count: protectedRoutes.filter((route) => route.type === "MCP")
                .length,
            },
          ]}
        />
      </div>
      <div className="permission-options">
        {visibleRoutes.map((route) => {
          const selected = Boolean(value[route.id]);
          return (
            <article key={route.id} className={selected ? "is-selected" : ""}>
              <label>
                <input
                  type="checkbox"
                  checked={selected}
                  onChange={() => toggleRoute(route)}
                />
                <RouteTypeBadge type={route.type} />
                <span>
                  <strong>{route.name}</strong>
                  <small>{`https://${route.host}${route.path}`}</small>
                </span>
              </label>
              {selected && route.published.length > 1 ? (
                <fieldset>
                  <legend>
                    {route.type === "API"
                      ? "接口"
                      : route.type === "AI"
                        ? "客户端模型"
                        : "工具"}
                  </legend>
                  {route.published.map((scope) => (
                    <label key={scope}>
                      <input
                        type="checkbox"
                        checked={value[route.id]?.includes(scope) ?? false}
                        onChange={() => toggleScope(route, scope)}
                      />
                      <code>{scope}</code>
                    </label>
                  ))}
                </fieldset>
              ) : null}
            </article>
          );
        })}
        {!visibleRoutes.length ? (
          <EmptyState
            title="没有匹配的可调用能力"
            description="请调整搜索或类型筛选。"
          />
        ) : null}
      </div>
      </div>
    </section>
  );
}

export function permissionRecords(draft: PermissionDraft): CallerPermission[] {
  return Object.entries(draft)
    .filter(([, scopes]) => scopes.length)
    .map(([routeID, scopes]) => ({ routeID, scopes }));
}

export function permissionScopeCount(draft: PermissionDraft) {
  return Object.values(draft).reduce((count, scopes) => count + scopes.length, 0);
}
