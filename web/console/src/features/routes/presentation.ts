import type { RouteAccessMode, RouteResource } from '@/domain/route';

export function resourceNames(ids: string[], options: Array<{ id: string; name: string }>): string {
  return ids.map((id) => resourceName(id, options)).join('、') || '—';
}

export function resourceName(id: string, options: Array<{ id: string; name: string }>): string {
  return options.find((option) => option.id === id)?.name ?? id;
}

export function routeServiceIDs(route: RouteResource): string[] {
  if (!route.ai) return route.services.map((target) => target.serviceID);
  return [...new Set(route.ai.models.flatMap((model) => model.targets.map((target) => target.serviceID)))];
}

export function methodLabel(route: RouteResource): string {
  return route.match.methods.length > 0 ? route.match.methods.join('、') : '所有方法';
}

export function pathMatchLabel(route: RouteResource): string {
  return route.match.path.type === 'ROUTE_PATH_MATCH_TYPE_EXACT' ? '精确' : '前缀';
}

export function hostRewriteLabel(route: RouteResource): string {
  switch (route.hostRewrite.mode) {
    case 'HOST_REWRITE_MODE_SERVICE_HOST': return '使用服务端点主机名';
    case 'HOST_REWRITE_MODE_CUSTOM': return route.hostRewrite.hostname || '未填写';
    default: return '保持请求主机';
  }
}

export function accessModeLabel(mode: RouteAccessMode): string {
  return mode === 'ROUTE_ACCESS_MODE_PUBLIC' ? '公开访问' : '调用方密钥';
}
