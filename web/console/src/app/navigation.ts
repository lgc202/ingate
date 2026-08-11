import {
  ChartNoAxesCombined,
  CircleGauge,
  FileClock,
  HeartPulse,
  KeyRound,
  Layers3,
  Route,
  Server,
  ShieldCheck,
  UsersRound,
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
  items: NavItem[];
}

export const navigation: NavGroup[] = [
  {
    key: 'workspace',
    label: '总览',
    items: [{ key: 'overview', label: '概览', to: '/overview', icon: CircleGauge }],
  },
  {
    key: 'configuration',
    label: '流量管理',
    items: [
      { key: 'gateways', label: '网关', to: '/gateways', icon: Layers3 },
      { key: 'routes', label: '路由', to: '/routes', icon: Route },
      { key: 'services', label: '服务', to: '/services', icon: Server },
      { key: 'certificates', label: '证书', to: '/certificates', icon: KeyRound },
    ],
  },
  {
    key: 'access',
    label: '安全与治理',
    items: [
      { key: 'callers', label: '调用方', to: '/callers', icon: UsersRound },
      { key: 'policies', label: '流量策略', to: '/policies', icon: ShieldCheck },
    ],
  },
  {
    key: 'observe',
    label: '运行与分析',
    items: [
      { key: 'requests', label: '请求', to: '/requests', icon: Route },
      { key: 'analysis', label: '流量分析', to: '/analysis', icon: ChartNoAxesCombined },
      { key: 'health', label: '健康与发布', to: '/health', icon: HeartPulse },
      { key: 'audit', label: '审计', to: '/audit', icon: FileClock },
    ],
  },
];

export const navigationItems = navigation.flatMap((group) => group.items);
