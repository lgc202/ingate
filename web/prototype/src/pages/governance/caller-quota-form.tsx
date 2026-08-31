import {
  ChevronRight,
  Trash2,
} from "lucide-react";
import { useState } from "react";
import {
  Drawer,
  PrimaryButton,
  RouteTypeBadge,
  submitForm,
} from "../../components/ui";
import type {
  Caller,
  CallerQuota,
  GatewayRoute,
  TrafficType,
} from "../../data";

export function ManageQuota({
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

export function quotaUnit(type: TrafficType) {
  return type === "AI" ? "Token" : type === "MCP" ? "工具调用次数" : "请求次数";
}

export function formatUsage(value: number, type: TrafficType) {
  const suffix =
    type === "AI" ? " Token" : type === "MCP" ? " 次工具调用" : " 次请求";
  return `${value.toLocaleString("zh-CN")}${suffix}`;
}
