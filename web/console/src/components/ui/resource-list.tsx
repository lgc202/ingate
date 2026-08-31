import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { ChevronDown, MoreHorizontal, Search, SlidersHorizontal } from 'lucide-react';
import { Button } from './controls';

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
  itemCount,
  hasNext,
  onPageChange,
  onPageSizeChange,
}: {
  page: number;
  pageSize: number;
  total?: number;
  itemCount?: number;
  hasNext?: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}) {
  const pageCount = total === undefined ? undefined : Math.max(1, Math.ceil(total / pageSize));
  const currentPage = pageCount === undefined ? page : Math.min(page, pageCount);
  const nextDisabled = pageCount === undefined ? !hasNext : page >= pageCount;
  return (
    <div className="resource-pagination">
      <span>{total === undefined ? `第 ${page} 页 · 本页 ${itemCount ?? 0} 条` : `第 ${currentPage} 页 · 共 ${total} 条`}</span>
      <label><span>每页</span><select value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))}>{[10, 20, 50].map((size) => <option key={size} value={size}>{size}</option>)}</select><span>条</span></label>
      <div>
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>上一页</Button>
        <Button variant="outline" size="sm" disabled={nextDisabled} onClick={() => onPageChange(page + 1)}>下一页</Button>
      </div>
    </div>
  );
}
