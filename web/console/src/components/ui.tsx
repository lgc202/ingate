import { useEffect, useId, useRef, type ReactNode } from 'react';
import { X } from 'lucide-react';
import { Tooltip } from './Tooltip';

export function Field({
  label,
  help,
  children,
}: {
  label: string;
  help?: string;
  children: ReactNode;
}) {
  const id = useId();

  return (
    <div className="field">
      <div className="field-label">
        <span id={id}>{label}</span>
        {help && <Tooltip label={label}>{help}</Tooltip>}
      </div>
      <div className="field-control" role="group" aria-labelledby={id}>
        {children}
      </div>
    </div>
  );
}

export function Section({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="form-section">
      <header className="section-title">
        <h3>{title}</h3>
        {action}
      </header>
      {children}
    </section>
  );
}

export function Drawer({
  title,
  namespace,
  onClose,
  footer,
  children,
}: {
  title: string;
  namespace: string;
  onClose: () => void;
  footer?: ReactNode;
  children: ReactNode;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleID = useId();

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();

    return () => element.close();
  }, []);

  return (
    <dialog
      ref={dialog}
      className="drawer"
      aria-labelledby={titleID}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <div className="drawer-layout">
        <header className="drawer-heading">
          <div>
            <h2 id={titleID}>{title}</h2>
            <span className="mono muted">{namespace}</span>
          </div>
          <button className="icon-button" aria-label="关闭抽屉" onClick={onClose}>
            <X />
          </button>
        </header>
        <div className="drawer-body">{children}</div>
        {footer && <footer className="drawer-footer">{footer}</footer>}
      </div>
    </dialog>
  );
}

export function Confirm({
  title,
  children,
  busy,
  error,
  onClose,
  onConfirm,
}: {
  title: string;
  children: ReactNode;
  busy: boolean;
  error?: string;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const id = useId();

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();
    return () => element.close();
  }, []);

  return (
    <dialog
      ref={dialog}
      className="confirm-dialog"
      aria-labelledby={id}
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onClose();
      }}
    >
      <header>
        <h2 id={id}>{title}</h2>
      </header>
      <div className="confirm-body">
        {children}
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
      </div>
      <footer>
        <button className="button" disabled={busy} onClick={onClose} autoFocus>
          取消
        </button>
        <button className="button danger" disabled={busy} onClick={onConfirm}>
          {busy ? '删除中…' : '确认删除'}
        </button>
      </footer>
    </dialog>
  );
}

export function StatePanel({
  title,
  message,
  onRetry,
}: {
  title: string;
  message?: string;
  onRetry?: () => void;
}) {
  return (
    <div className="state-panel" role="status">
      <h3>{title}</h3>
      {message && <p>{message}</p>}
      {onRetry && (
        <button className="button" onClick={onRetry}>
          重新加载
        </button>
      )}
    </div>
  );
}
