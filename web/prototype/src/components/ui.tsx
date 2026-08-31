import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  CircleOff,
  Copy,
  Eye,
  Pencil,
  RotateCcw,
  Search,
  Trash2,
  X,
  XCircle,
} from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import type {
  ConfigState,
  HealthState,
  ServiceType,
  TrafficType,
} from "../data";

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <header className="page-header">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        {description ? <p>{description}</p> : null}
      </div>
      {actions ? <div className="page-actions">{actions}</div> : null}
    </header>
  );
}

export function PrimaryButton({
  children,
  onClick,
  type = "button",
  disabled = false,
}: {
  children: ReactNode;
  onClick?: () => void;
  type?: "button" | "submit";
  disabled?: boolean;
}) {
  return (
    <button
      className="button button-primary"
      type={type}
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  );
}

export function SecondaryButton({
  children,
  onClick,
  type = "button",
}: {
  children: ReactNode;
  onClick?: () => void;
  type?: "button" | "submit";
}) {
  return (
    <button className="button button-secondary" type={type} onClick={onClick}>
      {children}
    </button>
  );
}

export function StatusBadge({
  state,
  label,
}: {
  state: HealthState;
  label?: string;
}) {
  const icon =
    state === "healthy" ? (
      <CheckCircle2 />
    ) : state === "warning" || state === "pending" ? (
      <AlertTriangle />
    ) : state === "error" ? (
      <XCircle />
    ) : (
      <CircleOff />
    );
  const text =
    label ??
    (
      {
        healthy: "正常",
        warning: "需关注",
        error: "异常",
        disabled: "未应用",
        pending: "生效中",
        unverified: "待验证",
      } as const
    )[state];
  return (
    <span className={`status status-${state}`}>
      {icon}
      {text}
    </span>
  );
}

export function ConfigBadge({ state = "active" }: { state?: ConfigState }) {
  const labels: Record<ConfigState, string> = {
    active: "配置正常",
    failed: "未生效",
    "not-applied": "未应用",
  };
  const health: Record<ConfigState, HealthState> = {
    active: "healthy",
    failed: "error",
    "not-applied": "disabled",
  };
  return <StatusBadge state={health[state]} label={labels[state]} />;
}

function TypeBadge({
  type,
  label,
}: {
  type: TrafficType | ServiceType;
  label: string;
}) {
  return (
    <span className={`type-badge type-${type.toLowerCase()}`}>
      {label}
    </span>
  );
}

export function RouteTypeBadge({ type }: { type: TrafficType }) {
  const labels: Record<TrafficType, string> = {
    API: "API 路由",
    AI: "AI 路由",
    MCP: "MCP 路由",
  };
  return <TypeBadge type={type} label={labels[type]} />;
}

export function ServiceTypeBadge({ type }: { type: ServiceType }) {
  const labels: Record<ServiceType, string> = {
    HTTP: "HTTP 服务",
    MODEL: "大模型服务",
    MCP: "MCP 服务",
  };
  return <TypeBadge type={type} label={labels[type]} />;
}

export function Metric({
  label,
  value,
  note,
  tone = "default",
}: {
  label: string;
  value: string;
  note: string;
  tone?: "default" | "good" | "warning";
}) {
  return (
    <article className={`metric metric-${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{note}</small>
    </article>
  );
}

export function EmptyState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="empty-state">
      <CircleOff />
      <strong>{title}</strong>
      <p>{description}</p>
    </div>
  );
}

export function CompactTagList({
  items,
  limit = 2,
  empty = "暂无",
}: {
  items: string[];
  limit?: number;
  empty?: string;
}) {
  if (!items.length) return <small>{empty}</small>;
  return (
    <div className="tag-cell compact-tag-list">
      {items.slice(0, limit).map((item, index) => (
        <code key={`${item}-${index}`}>{item}</code>
      ))}
      {items.length > limit ? (
        <span className="tag-overflow">+{items.length - limit}</span>
      ) : null}
    </div>
  );
}

export function FilterTabs<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: Array<{ value: T; label: string; count?: number }>;
  onChange: (value: T) => void;
}) {
  return (
    <div className="filter-tabs">
      {options.map((option) => (
        <button
          key={option.value}
          className={value === option.value ? "is-active" : ""}
          type="button"
          aria-pressed={value === option.value}
          onClick={() => onChange(option.value)}
        >
          {option.label}
          {option.count === undefined ? null : <span>{option.count}</span>}
        </button>
      ))}
    </div>
  );
}

export function FilterSelect<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: Array<{ value: T; label: string; count?: number }>;
  onChange: (value: T) => void;
}) {
  return (
    <label className="filter-select">
      <span>{label}</span>
      <div>
        <select
          aria-label={label}
          value={value}
          onChange={(event) => onChange(event.target.value as T)}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
              {option.count === undefined ? "" : `（${option.count}）`}
            </option>
          ))}
        </select>
        <ChevronDown />
      </div>
    </label>
  );
}

export function SearchField({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <label className="search-field">
      <Search />
      <input
        aria-label={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    </label>
  );
}

export function Drawer({
  title,
  description,
  children,
  onClose,
  width = "regular",
}: {
  title: string;
  description?: string;
  children: ReactNode;
  onClose: () => void;
  width?: "regular" | "wide";
}) {
  useEffect(() => {
    const close = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [onClose]);

  return (
    <div
      className="drawer-layer"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        className={`drawer drawer-${width}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header>
          <div>
            <h2>{title}</h2>
            {description ? <p>{description}</p> : null}
          </div>
          <button
            className="icon-button"
            type="button"
            onClick={onClose}
            aria-label="关闭"
          >
            <X />
          </button>
        </header>
        <div className="drawer-body">{children}</div>
      </section>
    </div>
  );
}

export function FormActions({
  submitLabel,
  onCancel,
  submitDisabled = false,
}: {
  submitLabel: string;
  onCancel: () => void;
  submitDisabled?: boolean;
}) {
  return (
    <footer className="form-actions">
      <SecondaryButton onClick={onCancel}>取消</SecondaryButton>
      <PrimaryButton type="submit" disabled={submitDisabled}>
        {submitLabel}
      </PrimaryButton>
    </footer>
  );
}

export function RowActions({
  onDetail,
  onEdit,
  onDelete,
}: {
  onDetail: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="row-actions" aria-label="资源操作">
      <button type="button" onClick={onDetail}>
        <Eye />
        详情
      </button>
      <button type="button" onClick={onEdit}>
        <Pencil />
        编辑
      </button>
      <button className="is-danger" type="button" onClick={onDelete}>
        <Trash2 />
        删除
      </button>
    </div>
  );
}

export function DeleteConfirm({
  resourceType,
  resourceName,
  blockedReason,
  onCancel,
  onConfirm,
}: {
  resourceType: string;
  resourceName: string;
  blockedReason?: string;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <div
      className="confirm-layer"
      role="presentation"
      onMouseDown={(event) =>
        event.target === event.currentTarget && onCancel()
      }
    >
      <section
        className="confirm-dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="delete-confirm-title"
      >
        <span className="confirm-icon">
          <AlertTriangle />
        </span>
        <div className="confirm-copy">
          <span className="eyebrow">删除{resourceType}</span>
          <h2 id="delete-confirm-title">
            {blockedReason ? "暂时无法删除" : `确认删除“${resourceName}”？`}
          </h2>
          <p>
            {blockedReason ??
              `删除后，该${resourceType}将从当前环境移除。此操作不能撤销。`}
          </p>
        </div>
        <footer>
          <SecondaryButton onClick={onCancel}>
            {blockedReason ? "知道了" : "取消"}
          </SecondaryButton>
          {blockedReason ? null : (
            <button
              className="button button-danger"
              type="button"
              onClick={onConfirm}
            >
              <Trash2 />
              确认删除
            </button>
          )}
        </footer>
      </section>
    </div>
  );
}

export function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    void navigator.clipboard?.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };
  return (
    <button className="copy-button" type="button" onClick={copy}>
      {copied ? <CheckCircle2 /> : <Copy />}
      {copied ? "已复制" : "复制"}
    </button>
  );
}

export function Toast({
  message,
  onDone,
}: {
  message: string;
  onDone: () => void;
}) {
  useEffect(() => {
    const timer = window.setTimeout(onDone, 2200);
    return () => window.clearTimeout(timer);
  }, [onDone]);
  return (
    <div className="toast" role="status" aria-live="polite">
      <CheckCircle2 />
      {message}
    </div>
  );
}

export function Topology({
  gateway,
  route,
  service,
  detail,
}: {
  gateway: string;
  route: string;
  service: string;
  detail?: string;
}) {
  return (
    <div className="topology">
      <div>
        <small>网关</small>
        <strong>{gateway}</strong>
      </div>
      <ArrowRight />
      <div className="is-route">
        <small>路由</small>
        <strong>{route}</strong>
      </div>
      <ArrowRight />
      <div>
        <small>服务</small>
        <strong>{service}</strong>
        {detail ? <span>{detail}</span> : null}
      </div>
    </div>
  );
}

export function ResetDemoButton({ onReset }: { onReset: () => void }) {
  return (
    <button className="demo-reset" type="button" onClick={onReset}>
      <RotateCcw />
      重置演示数据
    </button>
  );
}

export function submitForm(event: FormEvent, submit: () => void) {
  event.preventDefault();
  submit();
}
