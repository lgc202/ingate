import { NavLink, Outlet } from 'react-router-dom';
import { primaryNav } from '@/app/navigation';

export function AppShell() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <BrandMark />
          <div className="brand-copy">
            <div className="brand-name">Ingate</div>
            <span>API 与 AI 网关</span>
          </div>
        </div>

        <nav className="nav-group" aria-label="主导航">
          {primaryNav.map((item) => (
            <NavLink key={item.key} to={item.to} className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
              <item.icon className="nav-icon" />
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
      </aside>

      <main className="main">
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
          <stop offset="0" stopColor="#9fc2ff" />
          <stop offset="0.48" stopColor="#4b7ee8" />
          <stop offset="1" stopColor="#244a9f" />
        </linearGradient>
        <linearGradient id="brand-mark-flow" x1="8" y1="12" x2="32" y2="28" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#f1f6ff" />
          <stop offset="1" stopColor="#b9d5ff" />
        </linearGradient>
      </defs>
      <rect x="1.5" y="1.5" width="37" height="37" rx="10" fill="url(#brand-mark-bg)" />
      <path className="brand-mark-boundary" d="M20 8.5L28 12.2V18.8C28 24.1 25.2 28.3 20 31.2C14.8 28.3 12 24.1 12 18.8V12.2L20 8.5Z" />
      <path className="brand-mark-flow" d="M8.7 12.5H13.6L18.5 18.4" />
      <path className="brand-mark-flow" d="M8.7 20H18.2" />
      <path className="brand-mark-flow" d="M8.7 27.5H13.6L18.5 21.6" />
      <path className="brand-mark-flow" d="M21.8 20H31.3" />
      <circle cx="20" cy="20" r="4.2" fill="#111c35" opacity="0.82" />
      <circle cx="20" cy="20" r="2" fill="#edf4ff" />
      <circle cx="8.7" cy="12.5" r="1.4" fill="#f1f6ff" />
      <circle cx="8.7" cy="20" r="1.4" fill="#f1f6ff" />
      <circle cx="8.7" cy="27.5" r="1.4" fill="#f1f6ff" />
      <circle cx="31.3" cy="20" r="1.4" fill="#c8dcff" />
    </svg>
  );
}
