import type { ReactNode } from 'react';

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
