import {
  Activity,
  Key,
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

export const primaryNav: NavItem[] = [
  { key: 'gateways', label: '网关', to: '/gateways', icon: Layers3 },
  { key: 'routes', label: '路由', to: '/routes', icon: Route },
  { key: 'services', label: '服务', to: '/services', icon: Server },
  { key: 'certificates', label: '证书', to: '/certificates', icon: KeyRound },
  { key: 'access-keys', label: '访问密钥', to: '/access-keys', icon: Key },
  { key: 'policies', label: '策略', to: '/policies', icon: ShieldCheck },
  { key: 'status', label: '配置状态', to: '/status', icon: Activity },
];
