import { useEffect, useRef, type ReactNode } from 'react';
import { CircleCheck, X } from 'lucide-react';

export function PageFrame({
  title,
  subtitle,
  actions,
  children,
}: {
  title: string;
  subtitle: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="content-grid">
      <section className="topbar" style={{ marginBottom: 0 }}>
        <div>
          <h1 className="page-title">{title}</h1>
          <p className="page-subtitle">{subtitle}</p>
        </div>
        <div className="toolbar">{actions}</div>
      </section>
      {children}
    </div>
  );
}

export function Panel({
  title,
  subtitle,
  actions,
  children,
}: {
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="panel">
      {(title || subtitle || actions) && (
        <div className="panel-header">
          <div>
            {title ? <h2 className="panel-title">{title}</h2> : null}
            {subtitle ? <p className="panel-subtitle">{subtitle}</p> : null}
          </div>
          {actions ? <div className="toolbar">{actions}</div> : null}
        </div>
      )}
      {children}
    </section>
  );
}

export function StatCard({ label, value, meta, footer }: { label: string; value: string; meta: string; footer: string }) {
  return (
    <article className="stat-card">
      <div className="stat-card-top">
        <div className="stat-label">{label}</div>
        <span className="badge">{footer}</span>
      </div>
      <div className="stat-card-main">
        <div className="stat-value">{value}</div>
        <div className="stat-meta">{meta}</div>
      </div>
    </article>
  );
}

export function Badge({ tone = 'neutral', children }: { tone?: 'green' | 'amber' | 'red' | 'neutral'; children: ReactNode }) {
  return <span className={`badge ${tone !== 'neutral' ? tone : ''}`.trim()}>{children}</span>;
}

export function Button({
  variant = 'default',
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'default' | 'primary' | 'soft' | 'ghost' }) {
  return (
    <button className={`button ${variant}`.trim()} {...props}>
      {children}
    </button>
  );
}

export function Tabs({
  tabs,
  active,
  onChange,
}: {
  tabs: { key: string; label: string }[];
  active: string;
  onChange: (key: string) => void;
}) {
  return (
    <div className="tabs">
      {tabs.map((tab) => (
        <button key={tab.key} type="button" className={`tab-button ${tab.key === active ? 'active' : ''}`} onClick={() => onChange(tab.key)}>
          {tab.label}
        </button>
      ))}
    </div>
  );
}

export function Toast({
  message,
  onClose,
  duration = 3200,
}: {
  message: string | null;
  onClose: () => void;
  duration?: number;
}) {
  const onCloseRef = useRef(onClose);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!message) {
      return;
    }

    const timer = window.setTimeout(() => onCloseRef.current(), duration);
    return () => window.clearTimeout(timer);
  }, [duration, message]);

  if (!message) {
    return null;
  }

  return (
    <div className="toast" role="status">
      <CircleCheck size={17} strokeWidth={2.3} aria-hidden="true" />
      <span>{message}</span>
      <button type="button" onClick={onClose} aria-label="关闭提示">
        <X size={15} strokeWidth={2.4} aria-hidden="true" />
      </button>
    </div>
  );
}

export function ResourceStatePanel({ title, message }: { title: string; message: string }) {
  return (
    <Panel title={title}>
      <div className="mini-card">
        <div className="mini-card-meta">{message}</div>
      </div>
    </Panel>
  );
}

export function EmptyState({ title, message }: { title: string; message: string }) {
  return (
    <div className="detail-card">
      <h4>{title}</h4>
      <div className="mini-card-meta">{message}</div>
    </div>
  );
}
