import { ChevronRight, LogOut, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import { Suspense, useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { errorMessage } from '@/api/errors';
import { getHealth } from '@/api/health';
import { navigation, navigationItems } from '@/app/navigation';
import { Toast } from '@/components/ui';
import { useSession } from '@/features/auth/SessionProvider';

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const [version, setVersion] = useState('');
  const [signingOut, setSigningOut] = useState(false);
  const [signOutError, setSignOutError] = useState('');
  const { session, signOut } = useSession();
  const location = useLocation();
  const currentPage = navigationItems.find((item) => location.pathname.startsWith(item.to));
  const currentGroup = navigation.find((group) => group.items.some((item) => item.key === currentPage?.key));
  const currentPageLabel = currentPage?.label ?? '网关';
  const currentGroupLabel = currentGroup?.label ?? 'Ingate';

  useEffect(() => {
    void getHealth()
      .then((health) => setVersion(health.version))
      .catch(() => setVersion(''));
  }, []);

  const handleSignOut = async () => {
    if (signingOut) return;
    setSigningOut(true);
    setSignOutError('');
    try {
      await signOut();
    } catch (cause) {
      setSignOutError(errorMessage(cause, '退出登录失败'));
      setSigningOut(false);
    }
  };

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
          {!collapsed && version ? <div className="system-version">Ingate {version}</div> : null}
          <div className="session-user" title={collapsed ? session.username : undefined}>
            <span>{session.username.slice(0, 1).toUpperCase()}</span>
            {!collapsed ? <strong>{session.username}</strong> : null}
            <button type="button" aria-label="退出登录" disabled={signingOut} onClick={() => void handleSignOut()}>
              <LogOut aria-hidden="true" />
            </button>
          </div>
        </div>
      </aside>

      <div className={`console-workspace ${currentPage?.key === 'assistant' ? 'is-assistant' : ''}`}>
        <header className="workspace-header">
          <div className="breadcrumb">
            <span>{currentGroupLabel}</span>
            <ChevronRight />
            <strong>{currentPageLabel}</strong>
          </div>
        </header>

        <main className="workspace-content">
          <Suspense fallback={<div className="resource-observability-state">正在加载页面...</div>}>
            <Outlet />
          </Suspense>
        </main>
      </div>
      <Toast message={signOutError} tone="error" onClose={() => setSignOutError('')} />
    </div>
  );
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
