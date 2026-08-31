import {
  Activity,
  Ban,
  ChevronRight,
  KeyRound,
  Pencil,
  Play,
  Plus,
  RotateCw,
  ShieldCheck,
} from "lucide-react";
import { Link } from "react-router-dom";
import {
  CompactTagList,
  CopyButton,
  Drawer,
  EmptyState,
  PrimaryButton,
  StatusBadge,
} from "../../components/ui";
import type {
  Caller,
  CallerAccessKey,
  CallerPermission,
  GatewayRoute,
  RequestRecord,
} from "../../data";
import type { CallerSection } from "./caller-types";
import { formatUsage } from "./caller-quota-form";

export function CallerSectionContent({
  section,
  caller,
  routes,
  requests,
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
  requests: RequestRecord[];
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
  if (section === "activity")
    return (
      <section className="caller-resource-section">
        <header>
          <div>
            <h3>最近请求</h3>
            <span>验证权限、策略和目标服务的实际执行结果</span>
          </div>
          <Link
            className="button button-secondary"
            to={`/requests?caller=${caller.id}`}
          >
            查看全部请求
            <ChevronRight />
          </Link>
        </header>
        {requests.length ? (
          <div className="caller-request-list">
            {requests.slice(0, 6).map((request) => (
              <Link
                className="caller-request-row"
                to={`/requests?query=${request.id}`}
                key={request.id}
              >
                <span><Activity /></span>
                <div>
                  <strong>{request.method} {request.path}</strong>
                  <small>{request.route}{request.detail ? ` · ${request.detail}` : ""}</small>
                </div>
                <code>{request.id}</code>
                <StatusBadge
                  state={
                    request.result === "成功"
                      ? "healthy"
                      : request.result === "主备切换"
                        ? "warning"
                        : "error"
                  }
                  label={`${request.code} · ${request.result}`}
                />
                <span>{request.time} · {request.latency}</span>
                <ChevronRight />
              </Link>
            ))}
          </div>
        ) : (
          <EmptyState
            title="尚无请求记录"
            description="使用访问密钥发起请求后，可在这里查看执行结果。"
          />
        )}
      </section>
    );

  if (section === "permissions")
    return (
      <section className="caller-resource-section">
        <header>
          <div>
            <h3>访问授权</h3>
            <span>允许该客户端访问的路由；AI 与 MCP 可继续限制模型和工具</span>
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

  if (section === "identity")
    return (
      <section className="caller-resource-section">
        <header>
          <div>
            <h3>身份与凭据</h3>
            <span>当前通过 API Key 识别该客户端；每个运行环境使用独立凭据</span>
          </div>
          <button
            className="button button-secondary"
            type="button"
            onClick={onIssueKey}
          >
            <KeyRound />
            签发 API Key
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
            const exhausted = Boolean(quota && quota.used >= quota.limit);
            const percent = quota
              ? Math.min(100, Math.round((quota.used / quota.limit) * 100))
              : 0;
            return (
              <article
                className={`caller-quota-row ${exhausted ? "is-exhausted" : ""}`}
                key={route.id}
              >
                <div className="caller-quota-title">
                  <span
                    className={`type-badge type-${route.type.toLowerCase()}`}
                  >
                    {route.type}
                  </span>
                  <strong>{route.name}</strong>
                  {exhausted ? (
                    <StatusBadge state="error" label="已用尽" />
                  ) : null}
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
                        {exhausted
                          ? `该路由的新请求将被拒绝，其他路由不受影响 · ${quota.period}重置`
                          : `${quota.period}重置 · 已使用 ${percent}%`}
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

export function CallExample({
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
