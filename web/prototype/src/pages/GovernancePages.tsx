import {
  Ban,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  KeyRound,
  Pencil,
  Play,
  Plus,
  RotateCw,
  ShieldCheck,
  Trash2,
  UserRound,
  X,
} from "lucide-react";
import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  CompactTagList,
  CopyButton,
  ConfigBadge,
  DeleteConfirm,
  Drawer,
  EmptyState,
  FilterSelect,
  FilterTabs,
  FormActions,
  Metric,
  PageHeader,
  PrimaryButton,
  RouteTypeBadge,
  RowActions,
  SearchField,
  StatusBadge,
  Toast,
  submitForm,
} from "../components/ui";
import type {
  Caller,
  CallerAccessKey,
  CallerPermission,
  CallerQuota,
  GatewayRoute,
  Policy,
  TrafficType,
} from "../data";
import { usePrototype } from "../prototype-context";

type PermissionDraft = Record<string, string[]>;
type CallerSection = "permissions" | "keys" | "quotas";

export function CallerPage() {
  const {
    callers,
    routes,
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
  const [section, setSection] = useState<CallerSection>("permissions");
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
  const visibleCallers = callers.filter((caller) =>
    `${caller.name}${caller.owner}${caller.slug}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
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
  const selectedStatus = selected ? callerAvailability(selected) : null;

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
      <section className="card table-card">
          <header className="table-toolbar">
            <SearchField
              value={query}
              onChange={setQuery}
              placeholder="搜索名称或负责人"
            />
            <span>{visibleCallers.length} 个调用方</span>
          </header>
          <div className="table-head caller-columns">
            <span>调用方</span>
            <span>负责人</span>
            <span>状态</span>
            <span>权限</span>
            <span>密钥</span>
            <span>最近调用</span>
            <span>操作</span>
          </div>
          {visibleCallers.length ? (
            visibleCallers.map((caller) => (
                <div key={caller.id} className="table-row caller-columns">
                  <div className="name-cell">
                    <span><UserRound /></span>
                    <div>
                      <strong>{caller.name}</strong>
                      <small>{caller.slug}</small>
                    </div>
                  </div>
                  <span>{caller.owner}</span>
                  <StatusBadge
                    state={callerAvailability(caller).state}
                    label={callerAvailability(caller).label}
                  />
                  <strong>{caller.permissions.length}</strong>
                  <strong>
                    {caller.keys.filter((key) => key.state !== "disabled").length}
                  </strong>
                  <span>{caller.lastActive}</span>
                  <RowActions
                    onDetail={() => {
                      setSelectedID(caller.id);
                      setSection("permissions");
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
            ))
          ) : (
            <EmptyState
              title="没有匹配的调用方"
              description="请调整搜索条件。"
            />
          )}

        {viewing && selected && selectedStatus ? (
        <Drawer
          title={selected.name}
          description={`${selected.owner} · ${selected.slug}`}
          onClose={() => setViewing(false)}
          width="wide"
        >
          <article className="caller-detail-panel">
            <div className="drawer-actions">
              <button
                className="button button-secondary"
                type="button"
                onClick={() => setEditingCaller(true)}
              >
                <Pencil />
                编辑
              </button>
              <button
                className="button button-danger"
                type="button"
                onClick={() => setDeletingCaller(true)}
              >
                <Trash2 />
                删除
              </button>
            </div>
            <div className="detail-hero">
              <span>
                <UserRound />
              </span>
              <div>
                <StatusBadge
                  state={selectedStatus.state}
                  label={selectedStatus.label}
                />
                <h3>{selected.purpose || "未填写用途"}</h3>
                <p>
                  {selected.owner} · {selected.slug}
                </p>
              </div>
            </div>
            <div className="detail-kpis">
              <Metric
                label="访问权限"
                value={String(selectedRoutes.length)}
                note="可调用的路由"
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
            </div>
            <nav className="caller-tabs" aria-label="调用方管理内容">
              <button
                type="button"
                className={section === "permissions" ? "is-active" : ""}
                onClick={() => setSection("permissions")}
              >
                访问权限 <span>{selectedRoutes.length}</span>
              </button>
              <button
                type="button"
                className={section === "keys" ? "is-active" : ""}
                onClick={() => setSection("keys")}
              >
                访问密钥 <span>{selectedActiveKeys.length}</span>
              </button>
              <button
                type="button"
                className={section === "quotas" ? "is-active" : ""}
                onClick={() => setSection("quotas")}
              >
                用量额度 <span>{selected.quotas.length}</span>
              </button>
            </nav>

          <CallerSectionContent
            section={section}
            caller={selected}
            routes={selectedRoutes}
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
            setSection("permissions");
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

function callerAvailability(caller: Caller) {
  const activeKeys = caller.keys.filter((key) => key.state !== "disabled");
  if (!caller.permissions.length)
    return { state: "disabled" as const, label: "未授权" };
  if (!activeKeys.length)
    return { state: "warning" as const, label: "缺少密钥" };
  if (activeKeys.some((key) => key.state === "warning"))
    return { state: "warning" as const, label: "密钥待处理" };
  return { state: "healthy" as const, label: "可调用" };
}

function EditCaller({
  caller,
  onClose,
  onSave,
}: {
  caller: Caller;
  onClose: () => void;
  onSave: (caller: Caller) => void;
}) {
  const [name, setName] = useState(caller.name);
  const [owner, setOwner] = useState(caller.owner);
  const [purpose, setPurpose] = useState(caller.purpose);
  return (
    <Drawer title="编辑调用方" description={caller.name} onClose={onClose}>
      <form
        onSubmit={(event) =>
          submitForm(event, () => onSave({ ...caller, name, owner, purpose }))
        }
      >
        <div className="form-grid">
          <label className="field-wide">
            <span>调用方名称</span>
            <input required value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="field-wide">
            <span>负责人</span>
            <input required value={owner} onChange={(event) => setOwner(event.target.value)} />
          </label>
          <label className="field-wide">
            <span>用途</span>
            <textarea required value={purpose} onChange={(event) => setPurpose(event.target.value)} />
          </label>
        </div>
        <FormActions submitLabel="保存" onCancel={onClose} />
      </form>
    </Drawer>
  );
}

function CallerSectionContent({
  section,
  caller,
  routes,
  revokeCandidateID,
  onManagePermissions,
  onIssueKey,
  onRotateKey,
  onStartRevoke,
  onCancelRevoke,
  onRevoke,
  onTry,
  onQuota,
}: {
  section: CallerSection;
  caller: Caller;
  routes: Array<{ permission: CallerPermission; route: GatewayRoute }>;
  revokeCandidateID: string;
  onManagePermissions: () => void;
  onIssueKey: () => void;
  onRotateKey: (key: CallerAccessKey) => void;
  onStartRevoke: (keyID: string) => void;
  onCancelRevoke: () => void;
  onRevoke: (keyID: string) => void;
  onTry: (routeID: string) => void;
  onQuota: (routeID: string) => void;
}) {
  if (section === "permissions")
    return (
      <section className="caller-resource-section">
        <header>
          <div>
            <h3>访问权限</h3>
            <span>密钥可调用的 API、模型和工具</span>
          </div>
          <button
            className="button button-secondary"
            type="button"
            onClick={onManagePermissions}
          >
            <Pencil />
            管理权限
          </button>
        </header>
        {routes.length ? (
          <div className="caller-access-list">
            {routes.map(({ permission, route }) => (
              <article className="caller-access-row" key={permission.routeID}>
                <div className="access-capability">
                  <span
                    className={`type-badge type-${route.type.toLowerCase()}`}
                  >
                    {route.type}
                  </span>
                  <strong>{route.name}</strong>
                </div>
                <code className="caller-access-url">
                  {`https://${route.host}${route.path}`}
                </code>
                <CompactTagList items={permission.scopes} limit={3} />
                <button
                  className="section-action"
                  type="button"
                  onClick={() => onTry(route.id)}
                >
                  <Play />
                  调用示例
                </button>
              </article>
            ))}
          </div>
        ) : (
          <EmptyState
            title="尚未授予访问权限"
            description="该调用方当前不能调用受保护能力。"
          />
        )}
      </section>
    );

  if (section === "keys")
    return (
      <section className="caller-resource-section">
        <header>
          <div>
            <h3>访问密钥</h3>
            <span>每个运行环境或任务使用独立密钥</span>
          </div>
          <button
            className="button button-secondary"
            type="button"
            onClick={onIssueKey}
          >
            <KeyRound />
            签发密钥
          </button>
        </header>
        <div className="key-list">
          {caller.keys.length ? (
            caller.keys.map((key) => (
              <div className="key-row" key={key.id}>
                <span>
                  <KeyRound />
                </span>
                <div>
                  <strong>{key.name}</strong>
                  <code>{key.prefix}</code>
                  <small>
                    创建于 {key.createdAt} · 到期 {key.expiresAt} · 最后使用{" "}
                    {key.lastUsed}
                    {key.graceUntil ? ` · 旧密钥宽限至 ${key.graceUntil}` : ""}
                  </small>
                </div>
                <StatusBadge
                  state={key.state}
                  label={
                    key.state === "healthy"
                      ? "有效"
                      : key.state === "warning"
                        ? key.graceUntil
                          ? "轮换宽限期"
                          : "即将到期"
                        : "已停用"
                  }
                />
                {key.state !== "disabled" ? (
                  revokeCandidateID === key.id ? (
                    <div className="key-confirm">
                      <button type="button" onClick={onCancelRevoke}>
                        取消
                      </button>
                      <button
                        className="is-danger"
                        type="button"
                        onClick={() => onRevoke(key.id)}
                      >
                        确认停用
                      </button>
                    </div>
                  ) : (
                    <div className="key-actions">
                      {!key.replacedByID ? (
                        <button type="button" onClick={() => onRotateKey(key)}>
                          <RotateCw />
                          轮换
                        </button>
                      ) : null}
                      <button
                        className="key-revoke"
                        type="button"
                        onClick={() => onStartRevoke(key.id)}
                      >
                        <Ban />
                        停用
                      </button>
                    </div>
                  )
                ) : (
                  <span className="key-disabled-note">不可恢复</span>
                )}
              </div>
            ))
          ) : (
            <EmptyState
              title="尚未签发密钥"
              description="签发后才能使用该调用方身份发起请求。"
            />
          )}
        </div>
      </section>
    );

  return (
    <section className="caller-resource-section">
      <header>
        <div>
          <h3>用量额度</h3>
          <span>每项访问权限独立累计</span>
        </div>
        <Link
          className="button button-secondary"
          to={`/usage?caller=${caller.id}`}
        >
          查看用量明细
          <ChevronRight />
        </Link>
      </header>
      {routes.length ? (
        <div className="caller-quota-list">
          {routes.map(({ route }) => {
            const quota = caller.quotas.find(
              (item) => item.routeID === route.id,
            );
            const percent = quota
              ? Math.min(100, Math.round((quota.used / quota.limit) * 100))
              : 0;
            return (
              <article className="caller-quota-row" key={route.id}>
                <div className="caller-quota-title">
                  <span
                    className={`type-badge type-${route.type.toLowerCase()}`}
                  >
                    {route.type}
                  </span>
                  <strong>{route.name}</strong>
                </div>
                <div className="caller-quota-value">
                  <strong>
                    {quota
                      ? `${formatUsage(quota.used, route.type)} / ${formatUsage(quota.limit, route.type)}`
                      : "未设置上限"}
                  </strong>
                  {quota ? (
                    <>
                      <i>
                        <b style={{ width: `${percent}%` }} />
                      </i>
                      <small>
                        {quota.period}重置 · 已使用 {percent}%
                      </small>
                    </>
                  ) : (
                    <small>仅记录用量，不限制调用</small>
                  )}
                </div>
                <button
                  className="section-action"
                  type="button"
                  onClick={() => onQuota(route.id)}
                >
                  {quota ? (
                    <>
                      <Pencil />
                      修改额度
                    </>
                  ) : (
                    <>
                      <Plus />
                      设置额度
                    </>
                  )}
                </button>
              </article>
            );
          })}
        </div>
      ) : (
        <EmptyState
          title="没有可配置的用量范围"
          description="授予访问权限后可按项设置额度。"
        />
      )}
    </section>
  );
}

function CallExample({
  caller,
  route,
  onClose,
}: {
  caller: Caller;
  route: GatewayRoute;
  onClose: () => void;
}) {
  const scope =
    caller.permissions.find((permission) => permission.routeID === route.id)
      ?.scopes[0] ?? route.published[0];
  const path =
    route.type === "API"
      ? route.published[0]
          ?.replace(/^[A-Z]+\s+/, "")
          .replace("{id}", "78421")
          .replace(/\/\*$/, "/example") ?? route.path
      : route.type === "AI"
        ? `${route.path}/chat/completions`
        : route.path;
  const body =
    route.type === "AI"
      ? ` -d '{"model":"${scope}","messages":[{"role":"user","content":"你好"}]}'`
      : route.type === "MCP"
        ? ` -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"${scope}"}}'`
        : "";
  const matchHeaders = (route.conditions ?? [])
    .filter((condition) => condition.kind === "Header")
    .map(
      (condition) =>
        ` \\\n  -H '${condition.name}: ${condition.mode === "存在" ? "<值>" : condition.value}'`,
    )
    .join("");
  const command = `curl 'https://${route.host}${path}' \\\n  -H 'Authorization: Bearer <${caller.name}密钥>'${matchHeaders}${route.type !== "API" ? ` \\\n  -H 'Content-Type: application/json' \\\n ${body}` : ""}`;
  return (
    <Drawer
      title="调用示例"
      description={caller.name}
      onClose={onClose}
      width="wide"
    >
      <div className="call-example">
        <div className="call-target">
          <div><span>访问入口</span><strong>https://{route.host}</strong></div>
          <div><span>请求地址</span><strong>{path}</strong></div>
          <div><span>可调用能力</span><strong>{scope}</strong></div>
        </div>
        <div className="code-block">
          <code>{command}</code>
          <CopyButton value={command} />
        </div>
        <div className="form-note">
          <ShieldCheck />
          访问密钥仅在签发时展示，复制命令后请替换占位符再执行
        </div>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>完成</PrimaryButton>
        </footer>
      </div>
    </Drawer>
  );
}

function IssueCallerKey({
  caller,
  onClose,
  onIssue,
}: {
  caller: Caller;
  onClose: () => void;
  onIssue: (key: CallerAccessKey) => void;
}) {
  const [name, setName] = useState(`${caller.name}访问密钥`);
  const [validityDays, setValidityDays] = useState("90");
  const [secret, setSecret] = useState("");
  const [issuedKey, setIssuedKey] = useState<CallerAccessKey | null>(null);
  const issue = () => {
    const value = `ig_live_${crypto.randomUUID().replaceAll("-", "")}`;
    const created = new Date();
    const expires = new Date(created);
    expires.setDate(expires.getDate() + Number(validityDays));
    const key: CallerAccessKey = {
      id: `key-${crypto.randomUUID()}`,
      name,
      prefix: `${value.slice(0, 16)}…`,
      createdAt: created.toISOString().slice(0, 10),
      expiresAt: expires.toISOString().slice(0, 10),
      lastUsed: "尚未使用",
      state: "healthy",
    };
    onIssue(key);
    setIssuedKey(key);
    setSecret(value);
  };

  if (secret && issuedKey)
    return (
      <Drawer
        title="访问密钥已签发"
        description={caller.name}
        onClose={onClose}
      >
        <div className="secret-result">
          <span>
            <KeyRound />
          </span>
          <div>
            <strong>立即复制并妥善保存</strong>
            <p>
              完整密钥仅显示一次。系统只保留密钥标识和管理信息，无法再次查看明文。
            </p>
          </div>
        </div>
        <div className="secret-value">
          <code>{secret}</code>
          <CopyButton value={secret} />
        </div>
        <dl className="definition-list secret-metadata">
          <div>
            <dt>密钥名称</dt>
            <dd>{issuedKey.name}</dd>
          </div>
          <div>
            <dt>到期时间</dt>
            <dd>{issuedKey.expiresAt}</dd>
          </div>
          <div>
            <dt>保存记录</dt>
            <dd>
              <code>{issuedKey.prefix}</code>
            </dd>
          </div>
        </dl>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>完成</PrimaryButton>
        </footer>
      </Drawer>
    );

  return (
    <Drawer title="签发访问密钥" description={caller.name} onClose={onClose}>
      <form onSubmit={(event) => submitForm(event, issue)}>
        <div className="form-grid">
          <label className="field-wide">
            <span>密钥名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如生产环境、自动化任务"
            />
          </label>
          <label className="field-wide">
            <span>有效期</span>
            <select
              value={validityDays}
              onChange={(event) => setValidityDays(event.target.value)}
            >
              <option value="30">30 天</option>
              <option value="90">90 天</option>
              <option value="180">180 天</option>
              <option value="365">365 天</option>
            </select>
            <small>到期后自动失效，可提前停用</small>
          </label>
        </div>
        <div className="form-note">
          <CalendarDays />
          签发后生成新的访问密钥；完整密钥仅显示一次，之后只能查看名称、标识和使用状态。
        </div>
        <FormActions submitLabel="签发密钥" onCancel={onClose} />
      </form>
    </Drawer>
  );
}

function RotateCallerKey({
  caller,
  currentKey,
  onClose,
  onRotate,
}: {
  caller: Caller;
  currentKey: CallerAccessKey;
  onClose: () => void;
  onRotate: (key: CallerAccessKey, graceUntil: string) => void;
}) {
  const [graceDays, setGraceDays] = useState("7");
  const [secret, setSecret] = useState("");
  const [newKey, setNewKey] = useState<CallerAccessKey | null>(null);
  const [graceUntil, setGraceUntil] = useState("");
  const rotate = () => {
    const value = `ig_live_${crypto.randomUUID().replaceAll("-", "")}`;
    const created = new Date();
    const expires = new Date(created);
    expires.setDate(expires.getDate() + 90);
    const graceEnd = new Date(created);
    graceEnd.setDate(graceEnd.getDate() + Number(graceDays));
    const createdKey: CallerAccessKey = {
      id: `key-${crypto.randomUUID()}`,
      name: `${currentKey.name} · 新密钥`,
      prefix: `${value.slice(0, 16)}…`,
      createdAt: created.toISOString().slice(0, 10),
      expiresAt: expires.toISOString().slice(0, 10),
      lastUsed: "尚未使用",
      state: "healthy",
    };
    const graceDate = graceEnd.toISOString().slice(0, 10);
    onRotate(createdKey, graceDate);
    setNewKey(createdKey);
    setGraceUntil(graceDate);
    setSecret(value);
  };

  if (secret && newKey)
    return (
      <Drawer
        title="密钥轮换已开始"
        description={`${caller.name} · ${currentKey.name}`}
        onClose={onClose}
      >
        <div className="secret-result">
          <span>
            <RotateCw />
          </span>
          <div>
            <strong>先部署新密钥，再停用旧密钥</strong>
            <p>
              新旧密钥同时有效至 {graceUntil}。完整新密钥仅显示一次。
            </p>
          </div>
        </div>
        <div className="secret-value">
          <code>{secret}</code>
          <CopyButton value={secret} />
        </div>
        <dl className="definition-list secret-metadata">
          <div>
            <dt>新密钥</dt>
            <dd>{newKey.name}</dd>
          </div>
          <div>
            <dt>旧密钥宽限期</dt>
            <dd>截至 {graceUntil}</dd>
          </div>
          <div>
            <dt>下一步</dt>
            <dd>确认调用已切换后，可提前停用旧密钥</dd>
          </div>
        </dl>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>我已保存新密钥</PrimaryButton>
        </footer>
      </Drawer>
    );

  return (
    <Drawer
      title="轮换访问密钥"
      description={`${caller.name} · ${currentKey.name}`}
      onClose={onClose}
    >
      <form onSubmit={(event) => submitForm(event, rotate)}>
        <div className="rotation-path">
          <div>
            <span>1</span>
            <strong>生成新密钥</strong>
          </div>
          <i />
          <div>
            <span>2</span>
            <strong>迁移调用方</strong>
          </div>
          <i />
          <div>
            <span>3</span>
            <strong>停用旧密钥</strong>
          </div>
        </div>
        <div className="form-grid">
          <label className="field-wide">
            <span>旧密钥宽限期</span>
            <select
              value={graceDays}
              onChange={(event) => setGraceDays(event.target.value)}
            >
              <option value="1">1 天</option>
              <option value="7">7 天</option>
              <option value="14">14 天</option>
              <option value="30">30 天</option>
            </select>
            <small>宽限期内新旧密钥同时有效</small>
          </label>
        </div>
        <div className="form-note">
          <ShieldCheck />
          访问权限和用量归属保持不变；宽限期结束后旧密钥自动失效。
        </div>
        <FormActions submitLabel="生成新密钥" onCancel={onClose} />
      </form>
    </Drawer>
  );
}

function CreateCaller({
  routes,
  onClose,
  onSave,
}: {
  routes: GatewayRoute[];
  onClose: () => void;
  onSave: (caller: Caller) => void;
}) {
  const [name, setName] = useState("");
  const [owner, setOwner] = useState("");
  const [purpose, setPurpose] = useState("");
  const [permissions, setPermissions] = useState<PermissionDraft>({});
  const [keyName, setKeyName] = useState("");
  const [validityDays, setValidityDays] = useState("90");
  const [createdKey, setCreatedKey] = useState<CallerAccessKey | null>(null);
  const [secret, setSecret] = useState("");
  const save = () => {
    const granted = permissionRecords(permissions);
    const grantedTypes = new Set(
      granted.map(
        (permission) =>
          routes.find((route) => route.id === permission.routeID)?.type,
      ),
    );
    const metrics = [
      grantedTypes.has("API")
        ? { label: "API 请求", value: "0", note: "今天" }
        : null,
      grantedTypes.has("AI")
        ? { label: "AI Token", value: "0", note: "本月" }
        : null,
      grantedTypes.has("MCP")
        ? { label: "MCP 工具调用", value: "0", note: "今天" }
        : null,
    ].filter((item): item is { label: string; value: string; note: string } =>
      Boolean(item),
    );
    while (metrics.length < 3)
      metrics.push({
        label: metrics.length ? "策略拒绝" : "请求",
        value: "0",
        note: "今天",
      });
    const value = `ig_live_${crypto.randomUUID().replaceAll("-", "")}`;
    const createdAt = new Date();
    const expiresAt = new Date(createdAt);
    expiresAt.setDate(expiresAt.getDate() + Number(validityDays));
    const key: CallerAccessKey = {
      id: `key-${crypto.randomUUID()}`,
      name: keyName || `${name}访问密钥`,
      prefix: `${value.slice(0, 16)}…`,
      createdAt: createdAt.toISOString().slice(0, 10),
      expiresAt: expiresAt.toISOString().slice(0, 10),
      lastUsed: "尚未使用",
      state: "healthy",
    };
    onSave({
      id: `caller-${Date.now()}`,
      name,
      slug: `caller-${crypto.randomUUID().slice(0, 8)}`,
      owner,
      purpose,
      keys: [key],
      permissions: granted,
      metrics,
      quotas: [],
      state: "healthy",
      lastActive: "从未调用",
    });
    setCreatedKey(key);
    setSecret(value);
  };

  if (createdKey)
    return (
      <Drawer title="调用方已创建" description={name} onClose={onClose}>
        <div className="secret-result">
          <span><KeyRound /></span>
          <div>
            <strong>复制访问密钥</strong>
            <p>完整密钥仅显示一次，关闭后无法再次查看。</p>
          </div>
        </div>
        <div className="secret-value">
          <code>{secret}</code>
          <CopyButton value={secret} />
        </div>
        <dl className="definition-list secret-metadata">
          <div><dt>调用方</dt><dd>{name}</dd></div>
          <div><dt>访问权限</dt><dd>{permissionRecords(permissions).length} 项</dd></div>
          <div><dt>密钥到期</dt><dd>{createdKey.expiresAt}</dd></div>
        </dl>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>我已保存密钥</PrimaryButton>
        </footer>
      </Drawer>
    );

  return (
    <Drawer
      title="创建调用方"
      description="身份、访问权限与首个密钥"
      onClose={onClose}
      width="wide"
    >
      <form
        className="caller-create-form"
        onSubmit={(event) => submitForm(event, save)}
      >
        <section className="form-section">
          <header>
            <span>1</span>
            <div>
              <strong>基本信息</strong>
              <small>用于识别实际发起请求的应用或服务</small>
            </div>
          </header>
          <div className="form-grid">
            <label>
              <span>调用方名称</span>
              <input
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="例如客服助手"
              />
            </label>
            <label>
              <span>负责人</span>
              <input
                required
                value={owner}
                onChange={(event) => setOwner(event.target.value)}
                placeholder="团队或负责人"
              />
            </label>
            <label className="field-wide">
              <span>用途</span>
              <textarea
                required
                value={purpose}
                onChange={(event) => setPurpose(event.target.value)}
                placeholder="例如客服工作台查询订单并生成辅助回复"
              />
            </label>
          </div>
        </section>

        <PermissionSelector
          sectionNumber="2"
          routes={routes}
          value={permissions}
          onChange={setPermissions}
        />

        <section className="form-section">
          <header>
            <span>3</span>
            <div>
              <strong>访问密钥</strong>
              <small>创建后完整密钥仅显示一次</small>
            </div>
          </header>
          <div className="form-grid">
            <label>
              <span>密钥名称</span>
              <input
                value={keyName}
                onChange={(event) => setKeyName(event.target.value)}
                placeholder={`${name || "调用方"}生产环境`}
              />
            </label>
            <label>
              <span>有效期</span>
              <select
                value={validityDays}
                onChange={(event) => setValidityDays(event.target.value)}
              >
                <option value="30">30 天</option>
                <option value="90">90 天</option>
                <option value="180">180 天</option>
                <option value="365">365 天</option>
              </select>
            </label>
          </div>
        </section>

        <FormActions submitLabel="创建并签发密钥" onCancel={onClose} />
      </form>
    </Drawer>
  );
}

function ManagePermissions({
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
          移除访问项会同时删除对应额度；现有密钥保持有效。
        </div>
        <FormActions submitLabel="保存权限" onCancel={onClose} />
      </form>
    </Drawer>
  );
}

function PermissionSelector({
  sectionNumber,
  routes,
  value,
  onChange,
}: {
  sectionNumber?: string;
  routes: GatewayRoute[];
  value: PermissionDraft;
  onChange: (value: PermissionDraft) => void;
}) {
  const [query, setQuery] = useState("");
  const [type, setType] = useState<"ALL" | TrafficType>("ALL");
  const protectedRoutes = routes.filter(
    (route) => route.accessMode === "需要调用方密钥",
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
    <section className="form-section permission-editor">
      <header>
        <span>
          {sectionNumber ?? <ShieldCheck />}
        </span>
        <div>
          <strong>访问权限</strong>
          <small>选择可调用能力</small>
        </div>
        <b>{selectedRoutes.length} 项已授权</b>
      </header>
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
    </section>
  );
}

function permissionRecords(draft: PermissionDraft): CallerPermission[] {
  return Object.entries(draft)
    .filter(([, scopes]) => scopes.length)
    .map(([routeID, scopes]) => ({ routeID, scopes }));
}

function ManageQuota({
  caller,
  route,
  onClose,
  onSave,
  onRemove,
}: {
  caller: Caller;
  route: GatewayRoute;
  onClose: () => void;
  onSave: (quota: CallerQuota) => void;
  onRemove: () => void;
}) {
  const current = caller.quotas.find((quota) => quota.routeID === route.id);
  const [period, setPeriod] = useState<"每日" | "每月">(
    current?.period ?? "每月",
  );
  const [limit, setLimit] = useState(
    String(
      current?.limit ??
        (route.type === "AI"
          ? 20000000
          : route.type === "MCP"
            ? 100000
            : 1000000),
    ),
  );
  const numericLimit = Number(limit);
  return (
    <Drawer
      title={current ? "修改用量上限" : "设置用量上限"}
      description={`${caller.name} · ${route.name}`}
      onClose={onClose}
    >
      <form
        onSubmit={(event) =>
          submitForm(event, () =>
            onSave({
              routeID: route.id,
              used: current?.used ?? 0,
              limit: numericLimit,
              period,
            }),
          )
        }
      >
        <div className="quota-preview">
          <RouteTypeBadge type={route.type} />
          <span>{caller.name}</span>
          <ChevronRight />
          <span>{route.name}</span>
        </div>
        <div className="form-grid">
          <label>
            <span>重置周期</span>
            <select
              value={period}
              onChange={(event) =>
                setPeriod(event.target.value as "每日" | "每月")
              }
            >
              <option>每日</option>
              <option>每月</option>
            </select>
          </label>
          <label>
            <span>{quotaUnit(route.type)}上限</span>
            <input
              required
              type="number"
              min="1"
              step="1"
              value={limit}
              onChange={(event) => setLimit(event.target.value)}
            />
            <small>
              请输入完整数值，保存后显示为{" "}
              {Number.isFinite(numericLimit)
                ? numericLimit.toLocaleString("zh-CN")
                : "—"}
            </small>
          </label>
        </div>
        <dl className="definition-list quota-definition">
          <div>
            <dt>计量范围</dt>
            <dd>调用方在该路由下全部已授权能力的合计值</dd>
          </div>
          <div>
            <dt>超过上限</dt>
            <dd>拒绝新请求并返回 429</dd>
          </div>
          <div>
            <dt>其他路由</dt>
            <dd>独立计量，不受此上限影响</dd>
          </div>
        </dl>
        <footer className="form-actions">
          {current ? (
            <button
              className="button button-danger"
              type="button"
              onClick={onRemove}
            >
              <Trash2 />
              移除上限
            </button>
          ) : (
            <span />
          )}
          <div>
            <button
              className="button button-secondary"
              type="button"
              onClick={onClose}
            >
              取消
            </button>
            <PrimaryButton
              type="submit"
              disabled={!Number.isInteger(numericLimit) || numericLimit < 1}
            >
              保存上限
            </PrimaryButton>
          </div>
        </footer>
      </form>
    </Drawer>
  );
}

function quotaUnit(type: TrafficType) {
  return type === "AI" ? "Token" : type === "MCP" ? "工具调用次数" : "请求次数";
}

function formatUsage(value: number, type: TrafficType) {
  const suffix =
    type === "AI" ? " Token" : type === "MCP" ? " 次工具调用" : " 次请求";
  return `${value.toLocaleString("zh-CN")}${suffix}`;
}

type PolicyGroup = "ALL" | "访问控制" | "流量控制";

type TargetTrafficFilter = "ALL" | TrafficType;

interface PolicyTargetCandidate {
  key: string;
  kind: "网关" | "路由";
  id: string;
  name: string;
  gatewayID?: string;
  gatewayName?: string;
  trafficType?: TrafficType;
  detail: string;
}

const targetPageSize = 10;

function policyGroup(type: Policy["type"]): Exclude<PolicyGroup, "ALL"> {
  if (type === "IP 访问限制") return "访问控制";
  return "流量控制";
}

export function PolicyPage() {
  const { policies, routes, gateways, addPolicy, updatePolicy, deletePolicy } =
    usePrototype();
  const [filter, setFilter] = useState<PolicyGroup>("ALL");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Policy | null>(null);
  const [editing, setEditing] = useState<Policy | null>(null);
  const [deleting, setDeleting] = useState<Policy | null>(null);
  const [creating, setCreating] = useState(false);
  const [toast, setToast] = useState("");
  const visible = policies.filter(
    (policy) =>
      (filter === "ALL" || policyGroup(policy.type) === filter) &&
      `${policy.name}${policy.type}${policy.targets.map((target) => target.name).join("")}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const groups: Array<{ value: PolicyGroup; label: string; count?: number }> = [
    { value: "ALL", label: "全部", count: policies.length },
    ...(["访问控制", "流量控制"] as const).map((value) => ({
      value,
      label: value,
      count: policies.filter((policy) => policyGroup(policy.type) === value)
        .length,
    })),
  ];
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="访问治理"
        title="流量策略"
        description="可复用于网关或路由的访问与流量规则"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            创建流量策略
          </PrimaryButton>
        }
      />
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索策略或生效范围"
          />
          <FilterSelect
            label="策略分类"
            value={filter}
            onChange={setFilter}
            options={groups}
          />
        </header>
        <div className="table-head policy-columns">
          <span>策略</span>
          <span>分类</span>
          <span>生效范围</span>
          <span>规则</span>
          <span>配置状态</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((policy) => (
            <div key={policy.id} className="table-row policy-columns">
              <div className="name-cell">
                <span>
                  <ShieldCheck />
                </span>
                <div>
                  <strong>{policy.name}</strong>
                  <small>{policy.type}</small>
                </div>
              </div>
              <span>{policyGroup(policy.type)}</span>
              <CompactTagList
                items={policy.targets.map((target) => target.name)}
                empty="尚未应用"
              />
              <span>{policy.rule}</span>
              <ConfigBadge
                state={
                  policy.configState ??
                  (policy.targets.length ? "active" : "not-applied")
                }
              />
              <RowActions
                onDetail={() => setSelected(policy)}
                onEdit={() => setEditing(policy)}
                onDelete={() => setDeleting(policy)}
              />
            </div>
          ))
        ) : (
          <EmptyState title="没有匹配的策略" description="请调整筛选条件。" />
        )}
      </section>
      {selected ? (
        <Drawer
          title={selected.name}
          description={`${policyGroup(selected.type)} · ${selected.type}`}
          onClose={() => setSelected(null)}
        >
          <div className="detail-hero">
            <span>
              <ShieldCheck />
            </span>
            <div>
              <ConfigBadge
                state={
                  selected.configState ??
                  (selected.targets.length ? "active" : "not-applied")
                }
              />
              <h3>{selected.rule}</h3>
              <p>执行结果与拒绝明细可在请求记录中查看</p>
            </div>
          </div>
          <section className="detail-section">
            <header>
              <h3>生效范围</h3>
            </header>
            {selected.targets.length ? (
              selected.targets.map((target) => (
                <div
                  className="detail-line"
                  key={`${target.kind}-${target.id}`}
                >
                  <span>
                    <ShieldCheck />
                  </span>
                  <div>
                    <strong>{target.name}</strong>
                    <small>{target.kind} · 同一策略的各目标独立执行</small>
                  </div>
                  <ConfigBadge
                    state={
                      selected.configState ??
                      (selected.targets.length ? "active" : "not-applied")
                    }
                  />
                </div>
              ))
            ) : (
              <EmptyState
                title="尚未应用"
                description="策略已保存，但当前不影响任何流量。"
              />
            )}
          </section>
        </Drawer>
      ) : null}
      {creating ? (
        <CreatePolicy
          gateways={gateways}
          routes={routes}
          onClose={() => setCreating(false)}
          onSave={(policy) => {
            addPolicy(policy);
            setCreating(false);
            setToast(
              policy.targets.length
                ? "流量策略已保存"
                : "流量策略已保存，尚未应用",
            );
          }}
        />
      ) : null}
      {editing ? (
        <CreatePolicy
          initial={editing}
          gateways={gateways}
          routes={routes}
          onClose={() => setEditing(null)}
          onSave={(policy) => {
            updatePolicy(policy);
            setEditing(null);
            setToast(
              policy.targets.length
                ? "策略修改已保存"
                : "策略修改已保存，当前未应用",
            );
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="流量策略"
          resourceName={deleting.name}
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deletePolicy(deleting.id);
            setDeleting(null);
            setToast("流量策略已删除");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}

const defaultPolicyNames: Record<Policy["type"], string> = {
  请求限流: "新请求限流策略",
  "IP 访问限制": "新 IP 访问限制",
};

function CreatePolicy({
  initial,
  gateways,
  routes,
  onClose,
  onSave,
}: {
  initial?: Policy;
  gateways: Array<{ id: string; name: string }>;
  routes: GatewayRoute[];
  onClose: () => void;
  onSave: (policy: Policy) => void;
}) {
  const [type, setType] = useState<Policy["type"]>(initial?.type ?? "请求限流");
  const [name, setName] = useState(
    initial?.name ?? defaultPolicyNames["请求限流"],
  );
  const [selectedTargets, setSelectedTargets] = useState<string[]>(
    initial?.targets.map(
      (target) =>
        `${target.kind === "网关" ? "gateway" : "route"}:${target.id}`,
    ) ?? [],
  );
  const [targetQuery, setTargetQuery] = useState("");
  const [targetKind, setTargetKind] = useState<"ALL" | "网关" | "路由">(
    "ALL",
  );
  const [targetGatewayID, setTargetGatewayID] = useState("ALL");
  const [targetTraffic, setTargetTraffic] =
    useState<TargetTrafficFilter>("ALL");
  const [targetPage, setTargetPage] = useState(1);
  const [rateLimit, setRateLimit] = useState(
    initial?.settings?.rateLimit ??
      initial?.rule.match(/[\d,]+(?= 次)/)?.[0].replaceAll(",", "") ??
      "1000",
  );
  const [ratePeriod, setRatePeriod] = useState(
    initial?.settings?.ratePeriod ??
      (initial?.rule.includes("每小时")
        ? "小时"
        : initial?.rule.includes("每秒")
          ? "秒"
          : "分钟"),
  );
  const [rateDimension, setRateDimension] = useState(
    initial?.settings?.rateDimension ??
      (initial?.rule.startsWith("全部请求")
        ? "全部请求共享"
        : initial?.rule.startsWith("每个客户端 IP")
          ? "每个客户端 IP"
          : "每个调用方"),
  );
  const [ipMode, setIPMode] = useState(
    initial?.settings?.ipMode ??
      (initial?.rule.startsWith("拒绝") ? "拒绝" : "仅允许"),
  );
  const [ipRanges, setIPRanges] = useState(
    initial?.settings?.ipRanges ?? "10.0.0.0/8\n192.168.0.0/16",
  );
  const candidates: PolicyTargetCandidate[] =
    type === "请求限流" && rateDimension === "每个调用方"
        ? routes
            .filter((route) => route.accessMode === "需要调用方密钥")
            .map((route) => ({
              key: `route:${route.id}`,
              kind: "路由" as const,
              id: route.id,
              name: route.name,
              gatewayID: route.gatewayID,
              gatewayName: route.gatewayName,
              trafficType: route.type,
              detail: `${route.host}${route.path}`,
            }))
        : [
            ...gateways.map((gateway) => ({
              key: `gateway:${gateway.id}`,
              kind: "网关" as const,
              id: gateway.id,
              name: gateway.name,
              gatewayID: gateway.id,
              gatewayName: gateway.name,
              detail: "该网关下的全部流量",
            })),
            ...routes.map((route) => ({
              key: `route:${route.id}`,
              kind: "路由" as const,
              id: route.id,
              name: route.name,
              gatewayID: route.gatewayID,
              gatewayName: route.gatewayName,
              trafficType: route.type,
              detail: `${route.host}${route.path}`,
            })),
          ];
  const effectiveTargets = selectedTargets.filter((key) =>
    candidates.some((candidate) => candidate.key === key),
  );
  const selectedCandidates = candidates.filter((candidate) =>
    effectiveTargets.includes(candidate.key),
  );
  const filteredCandidates = candidates.filter(
    (candidate) =>
      (targetKind === "ALL" || candidate.kind === targetKind) &&
      (targetGatewayID === "ALL" ||
        candidate.gatewayID === targetGatewayID) &&
      (targetTraffic === "ALL" ||
        candidate.trafficType === targetTraffic) &&
      `${candidate.name}${candidate.gatewayName}${candidate.detail}${candidate.trafficType ?? ""}`
        .toLowerCase()
        .includes(targetQuery.toLowerCase()),
  );
  const targetPageCount = Math.max(
    1,
    Math.ceil(filteredCandidates.length / targetPageSize),
  );
  const effectiveTargetPage = Math.min(targetPage, targetPageCount);
  const visibleCandidates = filteredCandidates.slice(
    (effectiveTargetPage - 1) * targetPageSize,
    effectiveTargetPage * targetPageSize,
  );
  const candidateKinds = new Set(candidates.map((candidate) => candidate.kind));
  const routeCandidates = candidates.filter(
    (candidate) => candidate.kind === "路由",
  );
  const availableGateways = gateways.filter((gateway) =>
    routeCandidates.some((candidate) => candidate.gatewayID === gateway.id),
  );
  const allVisibleSelected =
    visibleCandidates.length > 0 &&
    visibleCandidates.every((candidate) =>
      effectiveTargets.includes(candidate.key),
    );
  const ipRangeCount = ipRanges
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean).length;
  const rule =
    type === "请求限流"
      ? `${rateDimension}每${ratePeriod} ${Number(rateLimit).toLocaleString("zh-CN")} 次${effectiveTargets.length > 1 ? "，各目标独立计数" : ""}`
      : `${ipMode} ${ipRangeCount} 个网段`;
  const save = () =>
    onSave({
      id: initial?.id ?? `policy-${Date.now()}`,
      name,
      type,
      targets: candidates
        .filter((candidate) => effectiveTargets.includes(candidate.key))
        .map(({ kind, id, name: targetName }) => ({
          kind,
          id,
          name: targetName,
        })),
      rule,
      effect: initial?.effect ?? "暂无执行记录",
      state: effectiveTargets.length ? "healthy" : "disabled",
      settings: {
        rateLimit,
        ratePeriod,
        rateDimension,
        ipMode,
        ipRanges,
      },
    });
  const changeType = (next: Policy["type"]) => {
    setType(next);
    setName(defaultPolicyNames[next]);
    setSelectedTargets([]);
    setTargetQuery("");
    setTargetKind("ALL");
    setTargetGatewayID("ALL");
    setTargetTraffic("ALL");
    setTargetPage(1);
  };
  const toggleTarget = (key: string) =>
    setSelectedTargets((items) =>
      items.includes(key)
        ? items.filter((item) => item !== key)
        : [...items, key],
    );
  const toggleVisibleTargets = () =>
    setSelectedTargets((items) => {
      const visibleKeys = new Set(visibleCandidates.map((candidate) => candidate.key));
      if (allVisibleSelected) return items.filter((item) => !visibleKeys.has(item));
      return [...new Set([...items, ...visibleKeys])];
    });
  return (
    <Drawer
      title={initial ? "编辑流量策略" : "创建流量策略"}
      description={policyGroup(type)}
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="form-grid">
          <label>
            <span>策略名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <label>
            <span>策略类型</span>
            <select
              value={type}
              onChange={(event) =>
                changeType(event.target.value as Policy["type"])
              }
            >
              <option>请求限流</option>
              <option>IP 访问限制</option>
            </select>
          </label>
          {type === "请求限流" ? (
            <>
              <label>
                <span>计数维度</span>
                <select
                  value={rateDimension}
                  onChange={(event) => {
                    setRateDimension(event.target.value);
                    setSelectedTargets([]);
                    setTargetQuery("");
                    setTargetKind("ALL");
                    setTargetGatewayID("ALL");
                    setTargetTraffic("ALL");
                    setTargetPage(1);
                  }}
                >
                  <option>每个调用方</option>
                  <option>全部请求共享</option>
                  <option>每个客户端 IP</option>
                </select>
                <small>
                  {rateDimension === "每个调用方"
                    ? "只可应用到需要密钥的路由"
                    : "可应用到网关或路由"}
                </small>
              </label>
              <label>
                <span>时间窗口</span>
                <select
                  value={ratePeriod}
                  onChange={(event) => setRatePeriod(event.target.value)}
                >
                  <option>秒</option>
                  <option>分钟</option>
                  <option>小时</option>
                </select>
              </label>
              <label className="field-wide">
                <span>最大请求数</span>
                <input
                  required
                  type="number"
                  min="1"
                  step="1"
                  value={rateLimit}
                  onChange={(event) => setRateLimit(event.target.value)}
                />
              </label>
            </>
          ) : null}
          {type === "IP 访问限制" ? (
            <>
              <label>
                <span>处理方式</span>
                <select
                  value={ipMode}
                  onChange={(event) => setIPMode(event.target.value)}
                >
                  <option>仅允许</option>
                  <option>拒绝</option>
                </select>
              </label>
              <label className="field-wide">
                <span>IP 或 CIDR（每行一个）</span>
                <textarea
                  required
                  value={ipRanges}
                  onChange={(event) => setIPRanges(event.target.value)}
                />
              </label>
            </>
          ) : null}
        </div>
        <fieldset className="target-picker">
          <legend>生效范围</legend>
          <header className="target-picker-header">
            <div>
              <strong>
                {effectiveTargets.length
                  ? `已选择 ${effectiveTargets.length} 个目标`
                  : "尚未选择目标"}
              </strong>
              <span>未选择目标时保存为“未应用”</span>
            </div>
            {effectiveTargets.length ? (
              <button
                type="button"
                onClick={() => setSelectedTargets([])}
              >
                清空选择
              </button>
            ) : null}
          </header>
          {selectedCandidates.length ? (
            <div className="selected-targets" aria-label="已选生效目标">
              {selectedCandidates.map((candidate) => (
                <span key={candidate.key}>
                  <small>{candidate.kind}</small>
                  <strong>{candidate.name}</strong>
                  <button
                    type="button"
                    aria-label={`移除${candidate.name}`}
                    onClick={() => toggleTarget(candidate.key)}
                  >
                    <X />
                  </button>
                </span>
              ))}
            </div>
          ) : (
            <div className="selected-targets-empty">
              策略尚未关联流量目标
            </div>
          )}
          <div className="target-picker-toolbar">
            <SearchField
              value={targetQuery}
              onChange={(value) => {
                setTargetQuery(value);
                setTargetPage(1);
              }}
              placeholder="搜索目标名称、域名或路径"
            />
            {candidateKinds.size > 1 ? (
              <FilterSelect
                label="资源类型"
                value={targetKind}
                onChange={(value) => {
                  setTargetKind(value);
                  if (value === "网关") setTargetTraffic("ALL");
                  setTargetPage(1);
                }}
                options={[
                  { value: "ALL", label: "全部类型", count: candidates.length },
                  {
                    value: "网关",
                    label: "网关",
                    count: candidates.filter((item) => item.kind === "网关").length,
                  },
                  {
                    value: "路由",
                    label: "路由",
                    count: candidates.filter((item) => item.kind === "路由").length,
                  },
                ]}
              />
            ) : null}
            {routeCandidates.length ? (
              <FilterSelect
                label="所属网关"
                value={targetGatewayID}
                onChange={(value) => {
                  setTargetGatewayID(value);
                  setTargetPage(1);
                }}
                options={[
                  {
                    value: "ALL",
                    label: "全部网关",
                    count: routeCandidates.length,
                  },
                  ...availableGateways.map((gateway) => ({
                    value: gateway.id,
                    label: gateway.name,
                    count: routeCandidates.filter(
                      (candidate) => candidate.gatewayID === gateway.id,
                    ).length,
                  })),
                ]}
              />
            ) : null}
            {routeCandidates.length ? (
              <FilterSelect
                label="流量类型"
                value={targetTraffic}
                onChange={(value) => {
                  setTargetTraffic(value);
                  setTargetPage(1);
                }}
                options={[
                  {
                    value: "ALL",
                    label: "全部流量",
                    count: routeCandidates.length,
                  },
                  ...(["API", "AI", "MCP"] as const).map((trafficType) => ({
                    value: trafficType,
                    label: `${trafficType} 路由`,
                    count: routeCandidates.filter(
                      (candidate) => candidate.trafficType === trafficType,
                    ).length,
                  })),
                ]}
              />
            ) : null}
          </div>
          <div className="target-candidate-heading">
            <span>
              匹配 {filteredCandidates.length} 项 · 第 {effectiveTargetPage} /{" "}
              {targetPageCount} 页
            </span>
            {visibleCandidates.length ? (
              <button type="button" onClick={toggleVisibleTargets}>
                {allVisibleSelected
                  ? "取消选择本页"
                  : `选择本页 ${visibleCandidates.length} 项`}
              </button>
            ) : null}
          </div>
          <div className="target-candidate-list">
            {visibleCandidates.length ? (
              visibleCandidates.map((candidate) => (
                <label
                  className={
                    effectiveTargets.includes(candidate.key)
                      ? "is-selected"
                      : ""
                  }
                  key={candidate.key}
                >
                  <input
                    type="checkbox"
                    checked={effectiveTargets.includes(candidate.key)}
                    onChange={() => toggleTarget(candidate.key)}
                  />
                  {candidate.trafficType ? (
                    <RouteTypeBadge type={candidate.trafficType} />
                  ) : (
                    <span className="target-gateway-badge">网关</span>
                  )}
                  <span>
                    <strong>{candidate.name}</strong>
                    <small>
                      {candidate.kind === "路由"
                        ? `${candidate.gatewayName} · ${candidate.detail}`
                        : candidate.detail}
                    </small>
                  </span>
                </label>
              ))
            ) : (
              <div className="target-candidate-empty">
                没有匹配的生效目标，请调整搜索条件
              </div>
            )}
          </div>
          {targetPageCount > 1 ? (
            <div className="target-pagination">
              <button
                type="button"
                disabled={effectiveTargetPage === 1}
                onClick={() => setTargetPage((page) => Math.max(1, page - 1))}
              >
                <ChevronLeft />
                上一页
              </button>
              <span>
                本页 {visibleCandidates.length} 项，已选 {effectiveTargets.length} 项
              </span>
              <button
                type="button"
                disabled={effectiveTargetPage === targetPageCount}
                onClick={() =>
                  setTargetPage((page) => Math.min(targetPageCount, page + 1))
                }
              >
                下一页
                <ChevronRight />
              </button>
            </div>
          ) : null}
        </fieldset>
        <div className="form-note">
          <ShieldCheck />
          将生成规则：{rule}
        </div>
        <FormActions
          submitLabel={initial ? "保存修改" : "保存流量策略"}
          submitDisabled={type === "IP 访问限制" && ipRangeCount === 0}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
