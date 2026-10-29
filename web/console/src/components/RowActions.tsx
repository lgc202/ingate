import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { MoreHorizontal } from 'lucide-react';

export function RowActions({
  onDetail,
  onEdit,
  onDelete,
  disabled = false,
}: {
  onDetail: () => void;
  onEdit: () => void;
  onDelete: () => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const menuItem = useRef<HTMLButtonElement>(null);

  useLayoutEffect(() => {
    if (!open) return;
    const element = menu.current!;
    const bounds = trigger.current!.getBoundingClientRect();
    element.style.left = Math.max(8, Math.min(innerWidth - 112, bounds.right - 104)) + 'px';
    element.style.top =
      (bounds.bottom + 48 < innerHeight ? bounds.bottom + 4 : bounds.top - 46) + 'px';
    element.showPopover();
    menuItem.current!.focus({ preventScroll: true });

    return () => {
      if (element.matches(':popover-open')) element.hidePopover();
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const close = () => setOpen(false);
    const outside = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!trigger.current?.contains(target) && !menu.current?.contains(target)) close();
    };
    const key = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' && event.key !== 'Tab') return;
      if (event.key === 'Escape') event.preventDefault();
      trigger.current?.focus();
      close();
    };
    document.addEventListener('pointerdown', outside);
    document.addEventListener('keydown', key);
    window.addEventListener('resize', close);
    window.addEventListener('scroll', close, true);

    return () => {
      document.removeEventListener('pointerdown', outside);
      document.removeEventListener('keydown', key);
      window.removeEventListener('resize', close);
      window.removeEventListener('scroll', close, true);
    };
  }, [open]);

  return (
    <div className="row-actions">
      <button className="text-button" disabled={disabled} onClick={onDetail}>
        详情
      </button>
      <button className="text-button" disabled={disabled} onClick={onEdit}>
        编辑
      </button>
      <button
        ref={trigger}
        disabled={disabled}
        className="row-more"
        aria-label="更多操作"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown') {
            event.preventDefault();
            setOpen(true);
          }
        }}
      >
        <MoreHorizontal />
      </button>
      {open && (
        <div ref={menu} className="row-menu" popover="manual" role="menu" aria-label="更多操作">
          <button
            ref={menuItem}
            role="menuitem"
            onClick={() => {
              setOpen(false);
              onDelete();
            }}
          >
            删除
          </button>
        </div>
      )}
    </div>
  );
}
