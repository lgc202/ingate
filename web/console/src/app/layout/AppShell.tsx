import { ChevronRight, LogOut, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import { useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { navigation, navigationItems } from '@/app/navigation';
import { useAuth } from '@/auth/AuthContext';

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const location = useLocation();
  const currentPage = navigationItems.find((item) => location.pathname.startsWith(item.to));
  const currentGroup = navigation.find((group) => group.items.some((item) => item.key === currentPage?.key));
  const currentPageLabel = currentPage?.label ?? '网关';
  const currentGroupLabel = currentGroup?.label ?? 'Ingate';
  const { enabled: authenticationEnabled, principal, signOut } = useAuth();

  return (
    <div className={`console-shell ${collapsed ? 'is-collapsed' : ''}`}>
      <aside className="console-sidebar">
        <div className="console-brand">
          <BrandMark />
          {!collapsed ? (
            <div className="console-brand-copy">
              <strong>Ingate</strong>
              <span>API 与 AI 网关</span>
            </div>
          ) : null}
          <button
            type="button"
            onClick={() => setCollapsed((value) => !value)}
            className="sidebar-collapse"
            aria-label={collapsed ? '展开导航' : '收起导航'}
          >
            {collapsed ? <PanelLeftOpen /> : <PanelLeftClose />}
          </button>
        </div>

        <nav className="console-navigation" aria-label="主导航">
          {navigation.map((group) => (
            <section key={group.key} className="nav-group">
              {!collapsed ? <div className="nav-group-label">{group.label}</div> : null}
              {group.items.map((item) => (
                <NavLink
                  key={item.key}
                  to={item.to}
                  className={({ isActive }) => `nav-item ${isActive ? 'is-active' : ''}`}
                  title={collapsed ? item.label : undefined}
                >
                  <item.icon />
                  {!collapsed ? <span>{item.label}</span> : null}
                </NavLink>
              ))}
            </section>
          ))}
        </nav>

        <div className="sidebar-footer">
          {authenticationEnabled && principal ? (
            <div className="principal-card">
              <span className="principal-avatar">{(principal.name || principal.email || principal.subject).slice(0, 1).toUpperCase()}</span>
              {!collapsed ? (
                <>
                  <div>
                    <strong>{principal.name || principal.email || principal.subject}</strong>
                    <span>{roleLabel(principal.role)}</span>
                  </div>
                  <button type="button" onClick={() => void signOut()} aria-label="退出登录"><LogOut /></button>
                </>
              ) : null}
            </div>
          ) : (
            <div className="system-version" title={collapsed ? 'Ingate 0.2.0' : undefined}>
              <span className="version-monogram">I</span>
              {!collapsed ? <span>Ingate 0.2.0</span> : null}
            </div>
          )}
        </div>
      </aside>

      <div className="console-workspace">
        <header className="workspace-header">
          <div className="breadcrumb">
            <span>{currentGroupLabel}</span>
            <ChevronRight />
            <strong>{currentPageLabel}</strong>
          </div>
        </header>

        <main className="workspace-content">
          <Outlet />
        </main>
      </div>
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
    <svg className="brand-mark" viewBox="0 0 40 40" fill="none" aria-hidden="true">
      <rect width="40" height="40" rx="11" fill="#e9edff" />
      <path d="M12 12.5h16v5.3H17.4v4.4H28v5.3H12v-15Z" fill="#3047c7" />
      <path d="m28 12.5-5.3 5.3H28v-5.3Z" fill="#ef8058" />
    </svg>
  );
}
