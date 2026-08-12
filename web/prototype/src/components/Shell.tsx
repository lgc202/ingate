import {
  Activity,
  ChartNoAxesCombined,
  ChevronRight,
  FileClock,
  HeartPulse,
  KeyRound,
  LayoutDashboard,
  CircleDollarSign,
  PanelLeftClose,
  PanelLeftOpen,
  Route,
  Server,
  Send,
  ShieldCheck,
  UsersRound,
  Waypoints,
} from "lucide-react";
import { useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { usePrototype } from "../prototype-context";
import { ResetDemoButton } from "./ui";

const navigation = [
  {
    label: "工作台",
    items: [{ label: "概览", to: "/overview", icon: LayoutDashboard }],
  },
  {
    label: "流量配置",
    items: [
      { label: "网关", to: "/gateways", icon: Waypoints },
      { label: "路由", to: "/routes", icon: Route },
      { label: "服务", to: "/services", icon: Server },
      { label: "证书", to: "/certificates", icon: KeyRound },
    ],
  },
  {
    label: "访问治理",
    items: [
      { label: "调用方", to: "/callers", icon: UsersRound },
      { label: "流量策略", to: "/policies", icon: ShieldCheck },
    ],
  },
  {
    label: "运行中心",
    items: [
      { label: "运行健康", to: "/health", icon: HeartPulse },
      { label: "请求记录", to: "/requests", icon: Activity },
      { label: "流量分析", to: "/analysis", icon: ChartNoAxesCombined },
      { label: "用量与成本", to: "/usage", icon: CircleDollarSign },
    ],
  },
  {
    label: "变更管理",
    items: [
      { label: "配置发布", to: "/releases", icon: Send },
      { label: "审计日志", to: "/audit", icon: FileClock },
    ],
  },
];

export function Shell() {
  const [collapsed, setCollapsed] = useState(false);
  const location = useLocation();
  const { currentVersion, candidateVersion, releaseHistory, resetDemo } =
    usePrototype();
  const page = navigation
    .flatMap((group) =>
      group.items.map((item) => ({ ...item, group: group.label })),
    )
    .find((item) => location.pathname.startsWith(item.to));

  return (
    <div className={`app-shell ${collapsed ? "is-collapsed" : ""}`}>
      <aside className="sidebar">
        <div className="brand">
          <BrandMark />
          {!collapsed ? (
            <div>
              <strong>Ingate</strong>
              <span>API 与 AI 网关</span>
            </div>
          ) : null}
          <button
            type="button"
            onClick={() => setCollapsed((value) => !value)}
            aria-label={collapsed ? "展开导航" : "收起导航"}
          >
            {collapsed ? <PanelLeftOpen /> : <PanelLeftClose />}
          </button>
        </div>
        <nav aria-label="主导航">
          {navigation.map((group) => (
            <section key={group.label}>
              {!collapsed ? (
                <span className="nav-label">{group.label}</span>
              ) : null}
              {group.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) => (isActive ? "is-active" : "")}
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
          <span className="avatar">林</span>
          {!collapsed ? (
            <div>
              <strong>林工程师</strong>
              <span>平台管理员</span>
            </div>
          ) : null}
        </div>
      </aside>

      <section className="workspace">
        <div className="demo-bar">
          <span>
            <i />
            产品原型 · 全部内容为演示数据
          </span>
          <ResetDemoButton onReset={resetDemo} />
        </div>
        <header className="workspace-header">
          <div className="breadcrumb">
            <span>{page?.group ?? "Ingate"}</span>
            <ChevronRight />
            <strong>{page?.label ?? "概览"}</strong>
          </div>
          <div className="workspace-actions">
            <NavLink to="/releases" className="active-release">
              <span>
                {candidateVersion
                  ? "发布中"
                  : (releaseHistory.find(
                      (release) => release.version === currentVersion,
                    )?.state ?? "已生效")}
              </span>
              {candidateVersion
                ? `候选 v${candidateVersion} · 完整 v${currentVersion}`
                : `v${currentVersion}`}
            </NavLink>
          </div>
        </header>
        <main>
          <Outlet />
        </main>
      </section>
    </div>
  );
}

function BrandMark() {
  return (
    <svg
      className="brand-mark"
      viewBox="0 0 42 42"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M5 8.5 21 3l16 5.5v12.2c0 9.2-6.2 15-16 18.3C11.2 35.7 5 29.9 5 20.7V8.5Z"
        fill="#F3F0E8"
      />
      <path d="M12 13h18v5H17v4h11v5H12V13Z" fill="#1D2022" />
      <path d="m30 13-5 5h5v-5Z" fill="#F06A3A" />
    </svg>
  );
}
