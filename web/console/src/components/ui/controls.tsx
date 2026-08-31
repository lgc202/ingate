import type { ReactNode } from 'react';

export function Button({
  type = 'button',
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
    <button type={type} className={`${base} ${variants[variant]} ${sizes[size]} ${className}`.trim()} {...props}>
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
