import { useEffect, useRef } from 'react';
import { AlertCircle, CircleCheck, X, type LucideIcon } from 'lucide-react';
import { Panel } from './layout';

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
  icon?: LucideIcon;
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
