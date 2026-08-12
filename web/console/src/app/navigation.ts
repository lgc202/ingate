import {
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
    label: '配置管理',
    items: [
      { key: 'gateways', label: '网关', to: '/gateways', icon: Layers3 },
      { key: 'routes', label: '路由', to: '/routes', icon: Route },
      { key: 'services', label: '服务', to: '/services', icon: Server },
      { key: 'certificates', label: '证书', to: '/certificates', icon: KeyRound },
    ],
  },
  {
    key: 'access',
    label: '治理策略',
    items: [{ key: 'policies', label: '流量策略', to: '/policies', icon: ShieldCheck }],
  },
];

export const navigationItems = navigation.flatMap((group) => group.items);
