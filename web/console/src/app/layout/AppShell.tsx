import { Bell, ExternalLink, Menu, Search } from 'lucide-react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { primaryNav } from '@/app/navigation';
import { apiMode } from '@/api/client';

export function AppShell() {
  const location = useLocation();

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div>
          <div className="brand">
            <BrandMark />
            <div className="brand-name">流量监治平台</div>
          </div>

          <div className="nav-group">
            {primaryNav.map((item) => {
              if (!item.children) {
                return (
                  <NavLink key={item.key} to={item.to ?? '/'} end={item.to === '/'} className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
                    <item.icon className="nav-icon" />
                    <span>{item.label}</span>
                  </NavLink>
                );
              }

              const active = location.pathname.startsWith(`/${item.key}`);

              return (
                <div key={item.key} className={`nav-section ${active ? 'active' : ''}`}>
                  <div className="nav-section-title">
                    <item.icon className="nav-icon" />
                    <span>{item.label}</span>
                  </div>
                  <div className="nav-subgroup">
                    {item.children.map((child) => (
                      <NavLink key={child.key} to={child.to} className={({ isActive }) => `nav-subitem ${isActive ? 'active' : ''}`}>
                        <child.icon className="nav-icon" />
                        <span>{child.label}</span>
                      </NavLink>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="sidebar-section">
          <div className="sidebar-card">
            <div className="sidebar-card-title">运行形态</div>
            <div className="sidebar-footer">
              <div>Docker all-in-one</div>
              <span className="badge green">开发</span>
            </div>
          </div>
          <div className="sidebar-card">
            <div className="sidebar-footer">
              <div>
                <div style={{ fontWeight: 700 }}>本地控制台</div>
                <div style={{ color: '#aab5ae', fontSize: '.82rem' }}>{apiMode === 'live' ? 'admin-api live' : 'admin-api mock'}</div>
              </div>
              <span className="badge green">在线</span>
            </div>
          </div>
        </div>
      </aside>

      <main className="main">
        <section className="global-toolbar">
          <div className="toolbar">
            <span className="chip">
              <span className="status ok" style={{ padding: 0, borderRadius: 0, background: 'transparent', color: 'var(--green)' }} />
              在线
            </span>
            <button className="select" type="button">
              第一阶段
            </button>
            <button className="icon-button" type="button" aria-label="搜索">
              <Search className="nav-icon" />
            </button>
            <button className="icon-button" type="button" aria-label="菜单">
              <Menu className="nav-icon" />
            </button>
            <button className="button soft" type="button">
              <ExternalLink className="nav-icon" />
              契约文档
            </button>
            <button className="icon-button" type="button" aria-label="通知">
              <Bell className="nav-icon" />
            </button>
          </div>
        </section>

        <Outlet />
      </main>
    </div>
  );
}

function BrandMark() {
  return (
    <svg className="brand-mark" viewBox="0 0 40 40" aria-hidden="true" focusable="false">
      <defs>
        <linearGradient id="brand-mark-bg" x1="7" y1="4" x2="34" y2="37" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#b7efbd" />
          <stop offset="0.48" stopColor="#3d9f70" />
          <stop offset="1" stopColor="#174934" />
        </linearGradient>
        <linearGradient id="brand-mark-flow" x1="8" y1="12" x2="32" y2="28" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#e8ffe9" />
          <stop offset="1" stopColor="#9ff3cf" />
        </linearGradient>
      </defs>
      <rect x="1.5" y="1.5" width="37" height="37" rx="10" fill="url(#brand-mark-bg)" />
      <path className="brand-mark-boundary" d="M20 8.5L28 12.2V18.8C28 24.1 25.2 28.3 20 31.2C14.8 28.3 12 24.1 12 18.8V12.2L20 8.5Z" />
      <path className="brand-mark-flow" d="M8.7 12.5H13.6L18.5 18.4" />
      <path className="brand-mark-flow" d="M8.7 20H18.2" />
      <path className="brand-mark-flow" d="M8.7 27.5H13.6L18.5 21.6" />
      <path className="brand-mark-flow" d="M21.8 20H31.3" />
      <circle cx="20" cy="20" r="4.2" fill="#08201d" opacity="0.82" />
      <circle cx="20" cy="20" r="2" fill="#dfffe3" />
      <circle cx="8.7" cy="12.5" r="1.4" fill="#e8ffe9" />
      <circle cx="8.7" cy="20" r="1.4" fill="#e8ffe9" />
      <circle cx="8.7" cy="27.5" r="1.4" fill="#e8ffe9" />
      <circle cx="31.3" cy="20" r="1.4" fill="#bff7dd" />
    </svg>
  );
}
