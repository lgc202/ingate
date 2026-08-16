import {
  Activity,
  ChartNoAxesCombined,
  KeyRound,
  Layers3,
  Route,
  Server,
  ShieldCheck,
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
    key: 'configuration',
    label: '流量配置',
    items: [
      { key: 'gateways', label: '网关', to: '/gateways', icon: Layers3 },
      { key: 'routes', label: '路由', to: '/routes', icon: Route },
      { key: 'services', label: '服务', to: '/services', icon: Server },
      { key: 'certificates', label: '证书', to: '/certificates', icon: KeyRound },
    ],
  },
  {
    key: 'access',
    label: '访问治理',
    items: [{ key: 'policies', label: '流量策略', to: '/policies', icon: ShieldCheck }],
  },
  {
    key: 'operations',
    label: '观测分析',
    items: [
      { key: 'requests', label: '请求记录', to: '/requests', icon: Activity },
      { key: 'analysis', label: '流量分析', to: '/analysis', icon: ChartNoAxesCombined },
    ],
  },
];

export const navigationItems = navigation.flatMap((group) => group.items);
