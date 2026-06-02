import {
  AlertTriangle,
  Boxes,
  ClipboardCheck,
  FileClock,
  Layers3,
  LayoutDashboard,
  Plug,
  Route,
  Server,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

export interface NavItem {
  key: string;
  label: string;
  to: string;
  icon: LucideIcon;
}

export interface NavGroup {
  key: string;
  label: string;
  icon: LucideIcon;
  to?: string;
  children?: NavItem[];
}

export const primaryNav: NavGroup[] = [
  { key: 'home', label: '首页', to: '/', icon: LayoutDashboard },
  {
    key: 'traffic',
    label: '流量',
    icon: Layers3,
    children: [
      { key: 'gateways', label: '网关', to: '/traffic/gateways', icon: Layers3 },
      { key: 'routes', label: '路由', to: '/traffic/routes', icon: Route },
      { key: 'services', label: '服务', to: '/traffic/services', icon: Server },
      { key: 'runtime', label: '运行状态', to: '/traffic/runtime', icon: ClipboardCheck },
      { key: 'traffic-policies', label: '流量策略', to: '/traffic/policies', icon: ShieldCheck },
      { key: 'plugins', label: '插件', to: '/traffic/plugins', icon: Plug },
    ],
  },
  { key: 'assets', label: '资产', to: '/assets', icon: Boxes },
  { key: 'risks', label: '风险', to: '/risks', icon: AlertTriangle },
  { key: 'governance', label: '治理', to: '/governance', icon: SlidersHorizontal },
  { key: 'system', label: '系统', to: '/system', icon: Settings },
];

export const systemNav: NavItem[] = [
  { key: 'users', label: '用户权限', to: '/system/users', icon: LayoutDashboard },
  { key: 'certificates', label: '证书', to: '/system/certificates', icon: ShieldCheck },
  { key: 'audit', label: '审计日志', to: '/system/audit', icon: FileClock },
  { key: 'notifications', label: '通知', to: '/system/notifications', icon: ClipboardCheck },
  { key: 'params', label: '系统参数', to: '/system/params', icon: Settings },
];
