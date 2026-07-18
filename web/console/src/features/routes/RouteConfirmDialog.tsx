import { Button } from '@/components/ui';

export function RouteConfirmDialog({
  title,
  message,
  details,
  confirmLabel,
  busy,
  onCancel,
  onConfirm,
}: {
  title: string;
  message: string;
  details: { label: string; value: string }[];
  confirmLabel: string;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const titleID = `route-confirm-${title}`;

  return (
    <div className="confirm-overlay" role="presentation" onMouseDown={() => {
      if (!busy) {
        onCancel();
      }
    }}>
      <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby={titleID} onMouseDown={(event) => event.stopPropagation()}>
        <h3 id={titleID}>{title}</h3>
        <p>{message}</p>
        <div className="confirm-meta">
          {details.flatMap((detail) => [
            <span key={`${detail.label}-label`}>{detail.label}</span>,
            <strong key={`${detail.label}-value`}>{detail.value}</strong>,
          ])}
        </div>
        <div className="confirm-actions">
          <Button variant="ghost" disabled={busy} onClick={onCancel}>取消</Button>
          <Button variant="primary" disabled={busy} onClick={onConfirm}>{busy ? '处理中...' : confirmLabel}</Button>
        </div>
      </div>
    </div>
  );
}
