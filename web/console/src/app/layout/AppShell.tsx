import { useState } from 'react';
import { LogOut } from 'lucide-react';
import { NavLink, Outlet } from 'react-router-dom';
import { primaryNav } from '@/app/navigation';
import { StatusDot } from '@/components/ui';
import { useAuth } from '@/auth/AuthContext';

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const { enabled: authenticationEnabled, principal, signOut } = useAuth();

  return (
    <div className={`h-screen overflow-hidden grid transition-all duration-200 ${collapsed ? 'grid-cols-[64px_1fr]' : 'grid-cols-[220px_1fr]'}`}>
      <aside className="bg-slate-900 text-slate-200 flex flex-col justify-between border-r border-slate-800 select-none h-full overflow-hidden">
        <div>
          {/* Brand Header */}
          <div className="h-14 px-4 flex items-center justify-between border-b border-slate-800/80">
            <div className="flex items-center gap-3 overflow-hidden">
              <BrandMark />
              {!collapsed && (
                <div className="truncate">
                  <div className="text-sm font-semibold text-white tracking-tight leading-none">Ingate</div>
                  <span className="text-[10px] font-mono text-slate-400">API 网关</span>
                </div>
              )}
            </div>
            <button
              type="button"
              onClick={() => setCollapsed(!collapsed)}
              className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
              title={collapsed ? '展开侧边栏' : '折叠侧边栏'}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                {collapsed ? (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 5l7 7-7 7M5 5l7 7-7" />
                ) : (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
                )}
              </svg>
            </button>
          </div>

          {/* Navigation */}
          <nav className="p-2 space-y-1" aria-label="主导航">
            {primaryNav.map((item) => (
              <NavLink
                key={item.key}
                to={item.to}
                className={({ isActive }) =>
                  `flex items-center gap-3 px-3 py-2 text-xs font-medium rounded-lg transition-colors ${
                    isActive
                      ? 'bg-blue-600/90 text-white shadow-xs'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                  } ${collapsed ? 'justify-center px-0' : ''}`
                }
                title={collapsed ? item.label : undefined}
              >
                <item.icon className="w-4 h-4 shrink-0" />
                {!collapsed && <span className="truncate">{item.label}</span>}
              </NavLink>
            ))}
          </nav>
        </div>

        <div>
          {authenticationEnabled && principal && (
            <div className={`border-t border-slate-800 px-3 py-3 ${collapsed ? 'flex justify-center' : 'flex items-center gap-3'}`}>
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-800 text-xs font-semibold text-slate-200">
                {(principal.name || principal.email || principal.subject).slice(0, 1).toUpperCase()}
              </div>
              {!collapsed && (
                <>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-xs font-medium text-slate-200">{principal.name || principal.email || principal.subject}</div>
                    <div className="text-[10px] uppercase tracking-wide text-slate-500">{roleLabel(principal.role)}</div>
                  </div>
                  <button
                    type="button"
                    onClick={() => void signOut()}
                    className="cursor-pointer rounded p-1.5 text-slate-500 transition hover:bg-slate-800 hover:text-slate-200"
                    aria-label="退出登录"
                    title="退出登录"
                  >
                    <LogOut className="h-4 w-4" />
                  </button>
                </>
              )}
            </div>
          )}
          {/* Footer Status Bar */}
          <div className="p-3 border-t border-slate-800/80 bg-slate-950/40 text-[11px] text-slate-400 flex items-center justify-between">
            {!collapsed ? (
              <>
                <div className="flex items-center gap-2">
                  <StatusDot status="healthy" />
                  <span className="font-medium text-slate-300">网关运行中</span>
                </div>
                <span className="font-mono text-slate-500">v0.2</span>
              </>
            ) : (
              <div className="mx-auto" title="网关运行中">
                <StatusDot status="healthy" />
              </div>
            )}
          </div>
        </div>
      </aside>

      {/* Main Content Area: Fixed Height + Native Vertical Scrollbar */}
      <main className="bg-slate-50/60 h-screen overflow-y-auto min-h-0 p-6">
        <Outlet />
      </main>
    </div>
  );
}

function roleLabel(role: string) {
  if (role === 'admin') return '管理员';
  if (role === 'operator') return '操作员';
  return '查看者';
}

function BrandMark() {
  return (
    <svg className="w-7 h-7 shrink-0" viewBox="0 0 40 40" fill="none">
      <defs>
        <linearGradient id="brand-bg" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#3b82f6" />
          <stop offset="1" stopColor="#1d4ed8" />
        </linearGradient>
      </defs>
      <rect width="40" height="40" rx="10" fill="url(#brand-bg)" />
      <path d="M20 9L30 14V26L20 31L10 26V14L20 9Z" stroke="#ffffff" strokeWidth="2.2" strokeLinejoin="round" />
      <circle cx="20" cy="20" r="3.5" fill="#ffffff" />
    </svg>
  );
}
