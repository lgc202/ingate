import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { CircleHelp } from 'lucide-react';

export function Tooltip({ label, children }: { label: string; children: string }) {
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const bubble = useRef<HTMLDivElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const id = useId();

  const show = () => {
    clearTimeout(timer.current);
    setOpen(true);
  };
  const leave = () => {
    timer.current = setTimeout(() => {
      if (document.activeElement !== button.current) setOpen(false);
    }, 120);
  };

  useLayoutEffect(() => {
    if (!open || !bubble.current || !button.current) return;
    const element = bubble.current;
    element.showPopover();
    const anchor = button.current.getBoundingClientRect();
    const bounds = element.getBoundingClientRect();
    element.style.left = Math.max(12, Math.min(innerWidth - bounds.width - 12, anchor.left)) + 'px';
    element.style.top =
      Math.max(
        12,
        anchor.bottom + bounds.height + 16 < innerHeight
          ? anchor.bottom + 8
          : anchor.top - bounds.height - 8,
      ) + 'px';

    return () => {
      if (element.matches(':popover-open')) element.hidePopover();
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const dismiss = () => setOpen(false);
    const keyboard = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      event.stopImmediatePropagation();
      dismiss();
    };
    const outside = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!button.current?.contains(target) && !bubble.current?.contains(target)) dismiss();
    };
    document.addEventListener('keydown', keyboard, true);
    document.addEventListener('pointerdown', outside);
    window.addEventListener('resize', dismiss);
    window.addEventListener('scroll', dismiss, true);

    return () => {
      document.removeEventListener('keydown', keyboard, true);
      document.removeEventListener('pointerdown', outside);
      window.removeEventListener('resize', dismiss);
      window.removeEventListener('scroll', dismiss, true);
    };
  }, [open]);

  useEffect(() => () => clearTimeout(timer.current), []);

  return (
    <span className="help-wrap" onPointerEnter={show} onPointerLeave={leave}>
      <button
        ref={button}
        type="button"
        className="help-trigger"
        aria-label={label + '说明'}
        aria-expanded={open}
        aria-describedby={open ? id : undefined}
        onFocus={show}
        onBlur={leave}
        onClick={show}
      >
        <CircleHelp aria-hidden="true" />
      </button>
      {open && (
        <div
          ref={bubble}
          id={id}
          className="field-tooltip"
          role="tooltip"
          popover="manual"
          onPointerEnter={show}
          onPointerLeave={leave}
        >
          {children}
        </div>
      )}
    </span>
  );
}
