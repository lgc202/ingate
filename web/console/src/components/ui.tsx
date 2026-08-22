import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { AlertCircle, ChevronDown, CircleCheck, MoreHorizontal, Search, SlidersHorizontal, X } from 'lucide-react';

export function PageFrame({
  title,
  actions,
  children,
}: {
  title: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="content-grid space-y-4">
      <section className="topbar flex items-center justify-between gap-4 pb-3 border-b border-slate-200">
        <h1 className="page-title text-xl font-bold text-slate-900 tracking-tight">{title}</h1>
        {actions ? <div className="toolbar flex items-center gap-2">{actions}</div> : null}
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
    <section className="panel bg-white border border-slate-200/90 rounded-xl shadow-xs overflow-hidden">
      {title || actions ? (
        <header className="panel-header px-5 py-3 border-b border-slate-200/80 flex items-center justify-between gap-3 bg-slate-50/60">
          <div>
            {title ? <h3 className="text-sm font-semibold text-slate-900">{title}</h3> : null}
            {subtitle ? <p className="panel-subtitle text-xs text-slate-500 mt-0.5">{subtitle}</p> : null}
          </div>
          {actions ? <div className="toolbar flex items-center gap-2">{actions}</div> : null}
        </header>
      ) : null}
      <div className={`panel-body ${title || actions ? 'p-5' : 'p-0'}`}>{children}</div>
    </section>
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
    <div className="resource-search">
      <Search aria-hidden="true" />
      <input
        type="search"
        value={value}
        placeholder={placeholder}
        aria-label={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

export function SelectPopover({
  label,
  summary,
  emptyMessage,
  hasOptions,
  children,
}: {
  label: string;
  summary: string;
  emptyMessage: string;
  hasOptions: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const labelID = useId();

  useEffect(() => {
    if (!open) return;

    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  return (
    <div className="field field-wide">
      <label id={labelID}>{label}</label>
      <div ref={rootRef} className={`resource-select ${open ? 'open' : ''}`.trim()}>
        <button
          className="resource-select-trigger"
          type="button"
          aria-labelledby={labelID}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => setOpen((current) => !current)}
        >
          <span>{summary}</span>
          <ChevronDown aria-hidden="true" />
        </button>
        {open ? (
          <div className="resource-select-menu" role="listbox" aria-label={label}>
            {hasOptions ? children : <div className="resource-select-empty">{emptyMessage}</div>}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function ResourceListFilters({
  summary,
  resultLabel,
  children,
  onSearch,
  onReset,
}: {
  summary: string;
  resultLabel: string;
  children: ReactNode;
  onSearch: () => void;
  onReset: () => void;
}) {
  const [expanded, setExpanded] = useState(true);

  return (
    <form
      className={`resource-filter-panel${expanded ? '' : ' is-collapsed'}`}
      onSubmit={(event) => {
        event.preventDefault();
        onSearch();
      }}
    >
      <header className="resource-filter-header">
        <div className="resource-filter-heading">
          <SlidersHorizontal aria-hidden="true" />
          <div>
            <strong>筛选条件</strong>
            <span>{summary}</span>
          </div>
        </div>
        <div className="resource-filter-header-actions">
          <span>{resultLabel}</span>
          <button
            className="resource-filter-toggle"
            type="button"
            aria-expanded={expanded}
            onClick={() => setExpanded((current) => !current)}
          >
            {expanded ? '收起' : '展开'}
            <ChevronDown className={expanded ? 'is-open' : ''} aria-hidden="true" />
          </button>
        </div>
      </header>
      {expanded ? (
        <>
          <div className="resource-filter-fields">{children}</div>
          <footer className="resource-filter-footer">
            <Button type="button" variant="ghost" onClick={onReset}>
              重置
            </Button>
            <Button type="submit">查询</Button>
          </footer>
        </>
      ) : null}
    </form>
  );
}

export function ResourceFilterField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="resource-filter-field">
      <span>{label}</span>
      {children}
    </label>
  );
}

export function RowActions({
  onDetail,
  onEdit,
  editLabel = '编辑',
  onToggle,
  toggleLabel,
  toggleDisabled = false,
  onDelete,
  deleteLabel = '删除',
}: {
  onDetail: () => void;
  onEdit?: () => void;
  editLabel?: string;
  onToggle?: () => void;
  toggleLabel?: string;
  toggleDisabled?: boolean;
  onDelete?: () => void;
  deleteLabel?: string;
}) {
  const moreRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [moreOpen, setMoreOpen] = useState(false);
  const [menuPosition, setMenuPosition] = useState({ top: 0, left: 0 });

  useEffect(() => {
    if (!moreOpen) return;
    const close = () => setMoreOpen(false);
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close();
    };
    const closeOnOutsideClick = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!moreRef.current?.contains(target) && !menuRef.current?.contains(target)) close();
    };
    window.addEventListener('resize', close);
    window.addEventListener('scroll', close, true);
    window.addEventListener('keydown', closeOnEscape);
    document.addEventListener('pointerdown', closeOnOutsideClick);
    return () => {
      window.removeEventListener('resize', close);
      window.removeEventListener('scroll', close, true);
      window.removeEventListener('keydown', closeOnEscape);
      document.removeEventListener('pointerdown', closeOnOutsideClick);
    };
  }, [moreOpen]);

  const toggleMore = () => {
    if (!moreOpen && moreRef.current) {
      const rect = moreRef.current.getBoundingClientRect();
      const menuHeight = 40;
      const top = rect.bottom + menuHeight + 4 <= window.innerHeight
        ? rect.bottom + 4
        : Math.max(8, rect.top - menuHeight - 4);
      setMenuPosition({ top, left: Math.max(8, Math.min(window.innerWidth - 96, rect.right - 88)) });
    }
    setMoreOpen((current) => !current);
  };

  return (
    <div className="row-actions" onClick={(event) => event.stopPropagation()}>
      <button className="link-button" type="button" onClick={onDetail}>详情</button>
      {onEdit ? <button className="link-button" type="button" onClick={onEdit}>{editLabel}</button> : null}
      {onToggle && toggleLabel ? <button className="link-button" type="button" disabled={toggleDisabled} onClick={onToggle}>{toggleLabel}</button> : null}
      {onDelete ? (
        <div className="row-more">
          <button ref={moreRef} className="row-more-trigger" type="button" aria-label="更多操作" aria-expanded={moreOpen} onClick={toggleMore}><MoreHorizontal aria-hidden="true" /></button>
          {moreOpen ? <div ref={menuRef} role="menu" style={menuPosition}><button className="danger" role="menuitem" type="button" onClick={() => { setMoreOpen(false); onDelete(); }}>{deleteLabel}</button></div> : null}
        </div>
      ) : null}
    </div>
  );
}

export function ResourcePagination({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}) {
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="resource-pagination">
      <span>第 {Math.min(page, pageCount)} 页 · 共 {total} 条</span>
      <label><span>每页</span><select value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))}>{[10, 20, 50].map((size) => <option key={size} value={size}>{size}</option>)}</select><span>条</span></label>
      <div>
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>上一页</Button>
        <Button variant="outline" size="sm" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)}>下一页</Button>
      </div>
    </div>
  );
}

export function Button({
  variant = 'primary',
  size = 'md',
  className = '',
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger' | 'soft';
  size?: 'sm' | 'md' | 'lg';
}) {
  const base = 'inline-flex items-center justify-center font-medium rounded-lg transition-colors focus:outline-hidden disabled:opacity-50 disabled:pointer-events-none cursor-pointer';
  const variants = {
    primary: 'bg-blue-600 hover:bg-blue-700 text-white shadow-xs',
    secondary: 'bg-slate-800 hover:bg-slate-900 text-white shadow-xs',
    outline: 'border border-slate-300 bg-white hover:bg-slate-50 text-slate-700 shadow-2xs',
    ghost: 'hover:bg-slate-100 text-slate-600',
    danger: 'bg-rose-600 hover:bg-rose-700 text-white shadow-xs',
    soft: 'bg-blue-50 hover:bg-blue-100 text-blue-700 font-medium',
  };
  const sizes = {
    sm: 'px-2.5 py-1 text-xs gap-1.5',
    md: 'px-3.5 py-1.5 text-xs gap-2',
    lg: 'px-4 py-2 text-sm gap-2',
  };

  return (
    <button className={`${base} ${variants[variant]} ${sizes[size]} ${className}`.trim()} {...props}>
      {children}
    </button>
  );
}

export function Badge({
  tone = 'neutral',
  children,
  className = '',
}: {
  tone?: 'success' | 'warning' | 'error' | 'danger' | 'neutral' | 'accent' | 'purple';
  children: ReactNode;
  className?: string;
}) {
  const toneKey = tone === 'danger' ? 'error' : tone;
  const styles = {
    success: 'bg-emerald-50 text-emerald-700 border-emerald-200/60',
    warning: 'bg-amber-50 text-amber-700 border-amber-200/60',
    error: 'bg-rose-50 text-rose-700 border-rose-200/60',
    neutral: 'bg-slate-100 text-slate-600 border-slate-200',
    accent: 'bg-blue-50 text-blue-700 border-blue-200/60',
    purple: 'bg-purple-50 text-purple-700 border-purple-200/60',
  };

  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded-md border ${styles[toneKey]} ${className}`.trim()}>
      {children}
    </span>
  );
}

export function Drawer({
  title,
  subtitle,
  isOpen,
  onClose,
  children,
}: {
  title: string;
  subtitle?: string;
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  const titleID = useId();
  const dialogRef = useRef<HTMLElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!isOpen) return;
      if (e.key === 'Escape') {
        onCloseRef.current();
        return;
      }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])'));
      if (focusable.length === 0) {
        e.preventDefault();
        dialogRef.current.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    if (isOpen) {
      previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      window.requestAnimationFrame(() => dialogRef.current?.focus());
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      previousFocusRef.current?.focus();
    };
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-hidden bg-slate-900/40 backdrop-blur-xs flex justify-end transition-opacity">
      <div className="fixed inset-0" onClick={onClose} aria-hidden="true" />
      <section ref={dialogRef} role="dialog" aria-modal="true" aria-labelledby={titleID} tabIndex={-1} className="relative w-full max-w-4xl bg-white h-full shadow-2xl flex flex-col border-l border-slate-200 z-10 animate-in slide-in-from-right duration-200 outline-none">
        <header className="px-6 py-4 border-b border-slate-200 flex items-center justify-between bg-slate-50/80">
          <div>
            <h2 id={titleID} className="text-base font-semibold text-slate-900 tracking-tight">{title}</h2>
            {subtitle ? <p className="text-xs text-slate-500 mt-0.5">{subtitle}</p> : null}
          </div>
          <button
            type="button"
            className="rounded-lg p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-200/60 transition-colors"
            onClick={onClose}
            aria-label="关闭抽屉"
          >
            <X className="w-5 h-5" />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto p-6 space-y-6">{children}</div>
      </section>
    </div>
  );
}

export function Modal({
  title,
  isOpen,
  onClose,
  children,
}: {
  title: string;
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  const titleID = useId();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    const currentDialog = dialogRef.current;
    if (!currentDialog) return;

    if (isOpen && !currentDialog.open) {
      currentDialog.showModal();
    } else if (!isOpen && currentDialog.open) {
      currentDialog.close();
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      onCloseRef.current();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={titleID}
      className="modal fixed inset-0 m-auto max-h-[calc(100dvh-3rem)] backdrop:bg-slate-900/40 backdrop:backdrop-blur-xs bg-white rounded-xl shadow-2xl p-0 border border-slate-200 w-[calc(100%-2rem)] max-w-xl overflow-hidden"
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClose={onClose}
    >
      <div className="modal-content">
        <header className="px-6 py-4 border-b border-slate-200 flex items-center justify-between bg-slate-50/80">
          <h3 id={titleID} className="text-base font-semibold text-slate-900">{title}</h3>
          <button
            type="button"
            className="rounded-lg p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-200/60 transition-colors"
            onClick={onClose}
            aria-label="关闭"
          >
            <X className="w-5 h-5" />
          </button>
        </header>
        <div className="p-6">{children}</div>
      </div>
    </dialog>
  );
}

export function StatusDot({ status }: { status: 'healthy' | 'warning' | 'error' | 'inactive' }) {
  const colors = {
    healthy: 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]',
    warning: 'bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.4)]',
    error: 'bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.4)]',
    inactive: 'bg-slate-300',
  };

  return <span className={`inline-block w-2 h-2 rounded-full ${colors[status]}`} />;
}

export function Toast({
  message,
  tone = 'success',
  onClose,
  duration = 3600,
}: {
  message: string | null;
  tone?: 'success' | 'error';
  onClose: () => void;
  duration?: number;
}) {
  const onCloseRef = useRef(onClose);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!message) return;
    const timer = window.setTimeout(() => onCloseRef.current(), duration);
    return () => window.clearTimeout(timer);
  }, [duration, message]);

  if (!message) return null;

  return (
    <div
      className={`fixed bottom-5 right-5 z-50 flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg border text-xs font-medium transition-all ${
        tone === 'error' ? 'bg-rose-900 text-rose-50 border-rose-700' : 'bg-slate-900 text-slate-100 border-slate-700'
      }`}
      role="status"
    >
      {tone === 'error' ? (
        <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
      ) : (
        <CircleCheck className="w-4 h-4 text-emerald-400 shrink-0" />
      )}
      <span>{message}</span>
      <button type="button" className="p-1 hover:opacity-75 transition-opacity" onClick={onClose} aria-label="关闭提示">
        <X className="w-3.5 h-3.5" />
      </button>
    </div>
  );
}

export function ResourceStatePanel({ title, message }: { title: string; message: string }) {
  return (
    <Panel title={title}>
      <div className="p-4 bg-slate-50 border border-slate-200/80 rounded-lg text-xs text-slate-600">{message}</div>
    </Panel>
  );
}

export function EmptyState({ title, message }: { title: string; message: string }) {
  return (
    <div className="p-8 text-center bg-slate-50/50 border border-dashed border-slate-200 rounded-xl">
      <h4 className="text-sm font-medium text-slate-700">{title}</h4>
      <p className="text-xs text-slate-400 mt-1">{message}</p>
    </div>
  );
}

export function StatCard({
  title,
  value,
  subvalue,
  icon: Icon,
  trend,
}: {
  title: string;
  value: string | number;
  subvalue?: string;
  icon?: any;
  trend?: string;
}) {
  return (
    <div className="p-4 bg-white border border-slate-200/80 rounded-xl shadow-2xs flex items-center justify-between">
      <div>
        <p className="text-xs font-medium text-slate-500 uppercase tracking-wider">{title}</p>
        <div className="flex items-baseline gap-2 mt-1">
          <span className="text-2xl font-bold text-slate-900 tracking-tight">{value}</span>
          {trend ? <span className="text-xs font-medium text-emerald-600">{trend}</span> : null}
        </div>
        {subvalue ? <p className="text-xs text-slate-400 mt-0.5">{subvalue}</p> : null}
      </div>
      {Icon ? (
        <div className="p-2.5 bg-slate-100/80 text-slate-600 rounded-lg">
          <Icon className="w-5 h-5" />
        </div>
      ) : null}
    </div>
  );
}
