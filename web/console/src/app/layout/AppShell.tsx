import { ChevronLeft, CircleCheck, LogOut, PanelLeftClose, PanelLeftOpen, Search } from 'lucide-react';
import { useState, type FormEvent } from 'react';
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { navigation, navigationItems } from '@/app/navigation';
import { useAuth } from '@/auth/AuthContext';

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const [query, setQuery] = useState('');
  const location = useLocation();
  const navigate = useNavigate();
  const currentPage = navigationItems.find((item) => location.pathname.startsWith(item.to));
  const currentGroup = navigation.find((group) => group.items.some((item) => item.key === currentPage?.key));
  const isRouteDebugger = location.pathname.startsWith('/playground');
  const currentPageLabel = currentPage?.label ?? (isRouteDebugger ? '调试请求' : '概览');
  const currentGroupLabel = currentGroup?.label ?? (isRouteDebugger ? '流量管理' : 'Ingate');
  const { enabled: authenticationEnabled, principal, signOut } = useAuth();

  const search = (event: FormEvent) => {
    event.preventDefault();
    const value = query.trim();
    if (!value) return;
    navigate(`/requests?query=${encodeURIComponent(value)}`);
  };

  return (
    <div className={`console-shell ${collapsed ? 'is-collapsed' : ''}`}>
      <aside className="console-sidebar">
        <div className="console-brand">
          <BrandMark />
          {!collapsed ? (
            <div className="console-brand-copy">
              <strong>Ingate</strong>
              <span>API & AI Gateway</span>
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
            <ChevronLeft />
            <strong>{currentPageLabel}</strong>
          </div>
          <div className="workspace-actions">
            <form className="command-search" onSubmit={search}>
              <Search />
              <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索请求、路由、调用方或服务" aria-label="全局搜索" />
            </form>
            <Link to="/health" className="delivery-status" title="查看健康与发布状态">
              <CircleCheck />
              <span>运行正常</span>
            </Link>
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
