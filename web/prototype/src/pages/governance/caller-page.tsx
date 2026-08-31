import {
  AlertTriangle,
  ChevronRight,
  Plus,
  UserRound,
} from "lucide-react";
import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  DeleteConfirm,
  Drawer,
  EmptyState,
  FilterSelect,
  Metric,
  PageHeader,
  PrimaryButton,
  RowActions,
  SearchField,
  StatusBadge,
  Toast,
} from "../../components/ui";
import type {
  Caller,
  CallerAccessKey,
  CallerPermission,
  GatewayRoute,
} from "../../data";
import { usePrototype } from "../../prototype-context";
import {
  CallerSectionContent,
  CallExample,
} from "./caller-detail";
import type {
  CallerAttention,
  CallerAttentionFilter,
  CallerSection,
  CallerStatusFilter,
} from "./caller-types";
import { CreateCaller } from "./create-caller-form";
import { EditCaller } from "./edit-caller-form";
import { IssueCallerKey } from "./issue-caller-key-form";
import { ManagePermissions } from "./caller-permission-form";
import {
  formatUsage,
  ManageQuota,
} from "./caller-quota-form";
import { RotateCallerKey } from "./rotate-caller-key-form";

export function CallerPage() {
  const {
    callers,
    routes,
    requests,
    addCaller,
    updateCaller,
    deleteCaller,
    updateCallerPermissions,
    setCallerQuota,
    removeCallerQuota,
    issueCallerKey,
    rotateCallerKey,
    revokeCallerKey,
  } = usePrototype();
  const [searchParams] = useSearchParams();
  const [selectedID, setSelectedID] = useState(
    searchParams.get("selected") ?? callers[0]?.id ?? "",
  );
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] =
    useState<CallerStatusFilter>("ALL");
  const [attentionFilter, setAttentionFilter] =
    useState<CallerAttentionFilter>("ALL");
  const [section, setSection] = useState<CallerSection>("identity");
  const [viewing, setViewing] = useState(false);
  const [creating, setCreating] = useState(false);
  const [editingCaller, setEditingCaller] = useState(false);
  const [deletingCaller, setDeletingCaller] = useState(false);
  const [editingPermissions, setEditingPermissions] = useState(false);
  const [quotaRouteID, setQuotaRouteID] = useState("");
  const [issuingKey, setIssuingKey] = useState(false);
  const [rotatingKey, setRotatingKey] = useState<CallerAccessKey | null>(null);
  const [tryingRouteID, setTryingRouteID] = useState("");
  const [toast, setToast] = useState("");
  const [revokeCandidateID, setRevokeCandidateID] = useState("");
  const selected =
    callers.find((caller) => caller.id === selectedID) ?? callers[0] ?? null;
  const visibleCallers = callers.filter((caller) => {
    const attentions = callerAttentions(caller, routes);
    const matchesStatus =
      statusFilter === "ALL" ||
      (statusFilter === "ENABLED" && caller.enabled) ||
      (statusFilter === "DISABLED" && !caller.enabled);
    const matchesAttention =
      attentionFilter === "ALL" || attentions.length > 0;
    return (
      matchesStatus &&
      matchesAttention &&
      `${caller.name}${caller.owner}${caller.slug}${attentions.map((item) => item.label).join("")}`
        .toLowerCase()
        .includes(query.toLowerCase())
    );
  });
  const selectedRoutes = selected
    ? selected.permissions
        .map((permission) => ({
          permission,
          route: routes.find((route) => route.id === permission.routeID),
        }))
        .filter(
          (
            item,
          ): item is {
            permission: CallerPermission;
            route: GatewayRoute;
          } => Boolean(item.route),
        )
    : [];
  const quotaRoute = routes.find((route) => route.id === quotaRouteID);
  const tryingRoute = routes.find((route) => route.id === tryingRouteID);
  const selectedActiveKeys =
    selected?.keys.filter((key) => key.state !== "disabled") ?? [];
  const selectedRequests = selected
    ? requests.filter((request) => request.caller === selected.name)
    : [];
  const selectedAttentions = selected ? callerAttentions(selected, routes) : [];

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="访问治理"
        title="调用方"
        description="调用网关的应用和服务"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            创建调用方
          </PrimaryButton>
        }
      />
      <section className="card table-card caller-table-card">
          <header className="table-toolbar">
            <SearchField
              value={query}
              onChange={setQuery}
              placeholder="搜索名称、负责人或待处理事项"
            />
            <FilterSelect
              label="账号状态"
              value={statusFilter}
              onChange={setStatusFilter}
              options={[
                { value: "ALL", label: "全部状态", count: callers.length },
                {
                  value: "ENABLED",
                  label: "已启用",
                  count: callers.filter((caller) => caller.enabled).length,
                },
                {
                  value: "DISABLED",
                  label: "已停用",
                  count: callers.filter((caller) => !caller.enabled).length,
                },
              ]}
            />
            <FilterSelect
              label="处理提醒"
              value={attentionFilter}
              onChange={setAttentionFilter}
              options={[
                { value: "ALL", label: "全部调用方" },
                {
                  value: "NEEDS_ATTENTION",
                  label: "需要处理",
                  count: callers.filter((caller) => callerAttentions(caller, routes).length).length,
                },
              ]}
            />
            <span>{visibleCallers.length} 个调用方</span>
          </header>
          <div className="table-head caller-columns">
            <span>调用方</span>
            <span>账号状态</span>
            <span>身份方式</span>
            <span>授权范围</span>
            <span>需要处理</span>
            <span>最近调用</span>
            <span>操作</span>
          </div>
          {visibleCallers.length ? (
            visibleCallers.map((caller) => {
              const attentions = callerAttentions(caller, routes);
              return (
                <div key={caller.id} className="table-row caller-columns">
                  <div className="name-cell">
                    <span><UserRound /></span>
                    <div>
                      <strong>{caller.name}</strong>
                      <small>{caller.owner} · {caller.slug}</small>
                    </div>
                  </div>
                  <StatusBadge
                    state={caller.enabled ? "healthy" : "disabled"}
                    label={caller.enabled ? "已启用" : "已停用"}
                  />
                  <div className="caller-identity-mode">
                    <strong>API Key</strong>
                    <small>{caller.keys.filter((key) => key.state !== "disabled").length} 个有效凭据</small>
                  </div>
                  <div className="caller-permission-count">
                    <strong>{caller.permissions.length} 条路由</strong>
                    <small>{callerScopeCount(caller)} 项能力</small>
                  </div>
                  {attentions.length ? (
                    <button
                      className="caller-attention-link"
                      type="button"
                      onClick={() => {
                        setSelectedID(caller.id);
                        setSection(attentions[0].section);
                        setViewing(true);
                      }}
                    >
                      <AlertTriangle />
                      <span>
                        <strong>{attentions[0].label}</strong>
                        <small>
                          {attentions.length > 1
                            ? `另有 ${attentions.length - 1} 项需要处理`
                            : attentions[0].detail}
                        </small>
                      </span>
                    </button>
                  ) : (
                    <span className="caller-attention-empty">无</span>
                  )}
                  <span>{caller.lastActive}</span>
                  <RowActions
                    onDetail={() => {
                      setSelectedID(caller.id);
                      setSection("identity");
                      setViewing(true);
                      setRevokeCandidateID("");
                    }}
                    onEdit={() => {
                      setSelectedID(caller.id);
                      setEditingCaller(true);
                    }}
                    onDelete={() => {
                      setSelectedID(caller.id);
                      setDeletingCaller(true);
                    }}
                  />
                </div>
              );
            })
          ) : (
            <EmptyState
              title="没有匹配的调用方"
              description="请调整搜索条件。"
            />
          )}

        {viewing && selected ? (
        <Drawer
          title={selected.name}
          description={`${selected.owner} · ${selected.slug}`}
          onClose={() => setViewing(false)}
          width="wide"
        >
          <article className="caller-detail-panel">
            <div className="detail-hero">
              <span>
                <UserRound />
              </span>
              <div>
                <StatusBadge
                  state={selected.enabled ? "healthy" : "disabled"}
                  label={selected.enabled ? "已启用" : "已停用"}
                />
                <span className="caller-access-reason">
                  API Key 客户端 · {selectedActiveKeys.length} 个有效凭据
                </span>
                <h3>{selected.purpose || "未填写用途"}</h3>
                <p>
                  {selected.owner} · {selected.slug}
                </p>
              </div>
            </div>
            {selectedAttentions.length ? (
              <section className="caller-attention-panel">
                <header>
                  <span><AlertTriangle /></span>
                  <div>
                    <strong>{selectedAttentions.length} 项需要处理</strong>
                    <small>
                      处理结果仅影响对应凭据或授权范围
                    </small>
                  </div>
                </header>
                <div>
                  {selectedAttentions.map((attention) => (
                    <button
                      type="button"
                      key={`${attention.section}-${attention.label}`}
                      onClick={() => setSection(attention.section)}
                    >
                      <span>
                        <strong>{attention.label}</strong>
                        <small>{attention.detail}</small>
                      </span>
                      <ChevronRight />
                    </button>
                  ))}
                </div>
              </section>
            ) : null}
            <div className="detail-kpis">
              <Metric
                label="授权路由"
                value={String(selectedRoutes.length)}
                note={`${callerScopeCount(selected)} 项接口、模型或工具`}
              />
              <Metric
                label="有效密钥"
                value={String(selectedActiveKeys.length)}
                note={`最近调用 ${selected.lastActive}`}
              />
              <Metric
                label="额度规则"
                value={String(selected.quotas.length)}
                note="未设置则仅记录用量"
              />
              <Metric
                label="最近请求"
                value={String(selectedRequests.length)}
                note={`${selectedRequests.filter((request) => request.result !== "成功").length} 条异常或拒绝`}
              />
            </div>
            <nav className="caller-tabs" aria-label="调用方管理内容">
              <button
                type="button"
                className={section === "identity" ? "is-active" : ""}
                onClick={() => setSection("identity")}
              >
                身份与凭据 <span>{selectedActiveKeys.length}</span>
              </button>
              <button
                type="button"
                className={section === "permissions" ? "is-active" : ""}
                onClick={() => setSection("permissions")}
              >
                访问授权 <span>{selectedRoutes.length}</span>
              </button>
              <button
                type="button"
                className={section === "quotas" ? "is-active" : ""}
                onClick={() => setSection("quotas")}
              >
                用量额度 <span>{selected.quotas.length}</span>
              </button>
              <button
                type="button"
                className={section === "activity" ? "is-active" : ""}
                onClick={() => setSection("activity")}
              >
                最近请求 <span>{selectedRequests.length}</span>
              </button>
            </nav>

          <CallerSectionContent
            section={section}
            caller={selected}
            routes={selectedRoutes}
            requests={selectedRequests}
            revokeCandidateID={revokeCandidateID}
            onManagePermissions={() => setEditingPermissions(true)}
            onIssueKey={() => setIssuingKey(true)}
            onRotateKey={setRotatingKey}
            onStartRevoke={setRevokeCandidateID}
            onCancelRevoke={() => setRevokeCandidateID("")}
            onRevoke={(keyID) => {
              revokeCallerKey(selected.id, keyID);
              setRevokeCandidateID("");
              setToast("访问密钥已停用");
            }}
            onTry={setTryingRouteID}
            onQuota={setQuotaRouteID}
            />
          </article>
        </Drawer>
        ) : null}
      </section>

      {creating ? (
        <CreateCaller
          routes={routes}
          onClose={() => setCreating(false)}
          onSave={(caller) => {
            addCaller(caller);
            setSelectedID(caller.id);
            setSection("identity");
            setViewing(true);
            setToast("调用方已创建");
          }}
        />
      ) : null}
      {editingCaller && selected ? (
        <EditCaller
          caller={selected}
          onClose={() => setEditingCaller(false)}
          onSave={(caller) => {
            updateCaller(caller);
            setEditingCaller(false);
            setToast("调用方信息已更新");
          }}
        />
      ) : null}
      {deletingCaller && selected ? (
        <DeleteConfirm
          resourceType="调用方及其全部密钥、权限和额度"
          resourceName={selected.name}
          onCancel={() => setDeletingCaller(false)}
          onConfirm={() => {
            const nextCaller = callers.find((caller) => caller.id !== selected.id);
            deleteCaller(selected.id);
            setDeletingCaller(false);
            setSelectedID(nextCaller?.id ?? "");
            setViewing(false);
            setToast("调用方已删除");
          }}
        />
      ) : null}
      {editingPermissions && selected ? (
        <ManagePermissions
          caller={selected}
          routes={routes}
          onClose={() => setEditingPermissions(false)}
          onSave={(permissions) => {
            updateCallerPermissions(selected.id, permissions);
            setEditingPermissions(false);
            setToast("访问权限已更新");
          }}
        />
      ) : null}
      {quotaRoute && selected ? (
        <ManageQuota
          caller={selected}
          route={quotaRoute}
          onClose={() => setQuotaRouteID("")}
          onSave={(quota) => {
            setCallerQuota(selected.id, quota);
            setQuotaRouteID("");
            setToast("用量上限已更新");
          }}
          onRemove={() => {
            removeCallerQuota(selected.id, quotaRoute.id);
            setQuotaRouteID("");
            setToast("用量上限已移除");
          }}
        />
      ) : null}
      {issuingKey && selected ? (
        <IssueCallerKey
          caller={selected}
          onClose={() => setIssuingKey(false)}
          onIssue={(key) => issueCallerKey(selected.id, key)}
        />
      ) : null}
      {rotatingKey && selected ? (
        <RotateCallerKey
          caller={selected}
          currentKey={rotatingKey}
          onClose={() => setRotatingKey(null)}
          onRotate={(key, graceUntil) => {
            rotateCallerKey(selected.id, rotatingKey.id, key, graceUntil);
            setToast("新密钥已生成，旧密钥进入宽限期");
          }}
        />
      ) : null}
      {tryingRoute && selected ? (
        <CallExample
          caller={selected}
          route={tryingRoute}
          onClose={() => setTryingRouteID("")}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}

export function callerAttentions(caller: Caller, routes: GatewayRoute[]) {
  const activeKeys = caller.keys.filter((key) => key.state !== "disabled");
  const accessAttentions: CallerAttention[] = [];
  if (!caller.permissions.length)
    accessAttentions.push({
      section: "permissions",
      label: "尚未授予访问权限",
      detail: "授权路由后才能调用",
    });
  if (!activeKeys.length)
    accessAttentions.push({
      section: "identity",
      label: "没有可用的访问密钥",
      detail: "签发新密钥后才能调用",
    });
  const quotaAttentions: CallerAttention[] = caller.quotas
    .filter((quota) => quota.used >= quota.limit)
    .map((quota) => {
      const route = routes.find((item) => item.id === quota.routeID);
      return {
        section: "quotas",
        label: `${route?.name ?? "未知路由"}额度已用尽`,
        detail: `${formatUsage(quota.used, route?.type ?? "API")} / ${formatUsage(quota.limit, route?.type ?? "API")} · ${quota.period}重置`,
      };
    });
  const keyAttentions: CallerAttention[] = caller.keys
    .filter((key) => key.state === "warning")
    .map((key) => ({
      section: "identity",
      label: key.graceUntil
        ? `${key.name}正在轮换`
        : `${key.name}即将到期`,
      detail: key.graceUntil
        ? `旧密钥宽限至 ${key.graceUntil}`
        : `${key.expiresAt} 到期`,
    }));
  return [...accessAttentions, ...quotaAttentions, ...keyAttentions];
}

export function callerScopeCount(caller: Caller) {
  return caller.permissions.reduce(
    (count, permission) => count + permission.scopes.length,
    0,
  );
}
