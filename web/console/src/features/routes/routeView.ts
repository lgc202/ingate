import type { PolicyWorkspace } from '@/domain/policy';
import { governancePolicyKey, policyTargetsResource } from '@/domain/policy';
import type {
  HeaderMatch,
  RouteGatewayOption,
  RouteResource,
  RouteRule,
  UpstreamOption,
  WeightedUpstream,
} from '@/domain/route';
import { formatModelRoutes, formatWeightedUpstreams, upstreamWeightSum } from './composer';

export function primaryRouteRule(route: Pick<RouteResource, 'rules'>): RouteRule | undefined {
  return route.rules[0];
}

export function routeUpstreams(route: Pick<RouteResource, 'rules'>): WeightedUpstream[] {
  return primaryRouteRule(route)?.upstreams ?? [];
}

export function routeUpstreamIDs(route: Pick<RouteResource, 'rules'>): string[] {
  const rule = primaryRouteRule(route);
  const serviceIDs = rule?.upstreams.map((upstream) => upstream.upstreamID) ?? [];
  const modelIDs = rule?.modelRouting?.models.map((model) => model.upstreamID) ?? [];
  return Array.from(new Set([...serviceIDs, ...modelIDs]));
}

export function routeUpstreamLabels(route: Pick<RouteResource, 'rules'>, upstreams: UpstreamOption[]): string[] {
  return routeUpstreamIDs(route).map((upstreamID) => upstreamLabel(upstreamID, upstreams));
}

export function routeUpstreamSummary(route: Pick<RouteResource, 'rules'>, upstreams: UpstreamOption[]): string {
  const rule = primaryRouteRule(route);
  if (rule?.modelRouting) {
    return formatModelRoutes(rule.modelRouting.models, upstreams);
  }
  return formatWeightedUpstreams(rule?.upstreams ?? [], upstreams);
}

export function routeTargetCount(route: Pick<RouteResource, 'rules'>): number {
  const rule = primaryRouteRule(route);
  return rule?.modelRouting?.models.length ?? rule?.upstreams.length ?? 0;
}

export function routeTargetKindLabel(route: Pick<RouteResource, 'rules'>): string {
  return primaryRouteRule(route)?.modelRouting ? '模型' : '目标服务';
}

export function routeModelNames(route: Pick<RouteResource, 'rules'>): string[] {
  const models = primaryRouteRule(route)?.modelRouting?.models ?? [];
  return models.flatMap((model) => [model.model, model.upstreamModel ?? '']);
}

export function routeForwardControlCount(route: Pick<RouteResource, 'rules'>): number {
  const rule = primaryRouteRule(route);
  if (!rule) {
    return 0;
  }
  return [rule.requestHeaderModifier, rule.responseHeaderModifier, rule.timeout, rule.retry].filter(Boolean).length;
}

export function routeGovernancePolicyLabel(
  route: Pick<RouteResource, 'id' | 'rules' | 'gatewayIDs'>,
  policyWorkspace: PolicyWorkspace | null | undefined,
): string {
  if (!policyWorkspace) {
    return '策略未知';
  }

  const direct = policyWorkspace.policies.filter((policy) => policyTargetsResource(policy, 'Route', route.id));
  const inherited = policyWorkspace.policies.filter((policy) => (
    policy.targets.some((target) => target.kind === 'Gateway' && route.gatewayIDs.includes(target.id))
  ));
  const unique = new Set([...direct, ...inherited].map(governancePolicyKey));
  if (unique.size === 0) {
    return '未应用策略';
  }
  return inherited.length > 0 ? `${unique.size} 个策略（含继承）` : `${unique.size} 个策略`;
}

export function formatGatewayIDs(gatewayIDs: string[], gateways: RouteGatewayOption[]): string {
  if (gatewayIDs.length === 0) {
    return '-';
  }
  return gatewayIDs.map((gatewayID) => gatewayLabel(gatewayID, gateways)).join('、');
}

export function formatHostnames(hostnames: string[]): string {
  return hostnames.length > 0 ? hostnames.join('、') : '不限制域名';
}

export function formatHeaderMatches(headers: HeaderMatch[]): string {
  if (headers.length === 0) {
    return '不限制请求头';
  }
  return headers.map((header) => `${header.name}=${header.value}`).join('、');
}

export function formatMethods(methods: string[]): string {
  return methods.length > 0 ? methods.join('、') : '全部方法';
}

export function formatRouteMatch(route: Pick<RouteResource, 'rules'>): string {
  const rule = primaryRouteRule(route);
  return `${formatMethods(rule?.methods ?? [])} ${rule?.pathPrefix ?? '-'}`;
}

export function routeDetailItems(
  route: RouteResource,
  tab: string,
  gateways: RouteGatewayOption[],
  upstreams: UpstreamOption[],
): { label: string; value: string }[] {
  const rule = primaryRouteRule(route);

  if (tab === 'match') {
    return [
      { label: '方法', value: formatMethods(rule?.methods ?? []) },
      { label: '路径', value: rule?.pathPrefix ?? '-' },
      { label: '所属网关', value: formatGatewayIDs(route.gatewayIDs, gateways) },
      { label: '匹配域名', value: formatHostnames(route.hostnames) },
      { label: '请求头条件', value: formatHeaderMatches(rule?.headers ?? []) },
    ];
  }

  if (tab === 'upstream') {
    if (rule?.modelRouting) {
      return [
        { label: '转发方式', value: '模型服务代理' },
        { label: '模型映射', value: formatModelRoutes(rule.modelRouting.models, upstreams) },
        { label: '模型数量', value: `${rule.modelRouting.models.length} 个` },
      ];
    }
    return [
      { label: '转发方式', value: '普通服务转发' },
      { label: '目标服务', value: routeUpstreamSummary(route, upstreams) },
      { label: '目标服务数', value: `${routeUpstreams(route).length} 个` },
      { label: '总权重', value: String(upstreamWeightSum(routeUpstreams(route))) },
    ];
  }

  if (tab === 'controls') {
    return [
      { label: '请求头改写', value: rule?.requestHeaderModifier ? '已配置' : '未配置' },
      { label: '响应头改写', value: rule?.responseHeaderModifier ? '已配置' : '未配置' },
      { label: '请求超时', value: rule?.timeout ? `${rule.timeout.requestMillis}ms` : '默认' },
      { label: '失败重试', value: rule?.retry ? `${rule.retry.attempts} 次 / ${rule.retry.perTryTimeoutMillis}ms` : '未配置' },
    ];
  }

  return [
    { label: '路由名称', value: route.name },
    { label: '匹配请求', value: formatRouteMatch(route) },
    { label: '所属网关', value: formatGatewayIDs(route.gatewayIDs, gateways) },
    { label: rule?.modelRouting ? '模型映射' : '目标服务', value: routeUpstreamSummary(route, upstreams) },
    { label: '转发控制', value: `${routeForwardControlCount(route)} 项` },
    { label: '启用状态', value: route.enabled ? '启用' : '停用' },
  ];
}

export function gatewayLabel(gatewayID: string, gateways: RouteGatewayOption[]): string {
  return gateways.find((gateway) => gateway.id === gatewayID)?.name ?? shortResourceID(gatewayID);
}

export function upstreamLabel(upstreamID: string, upstreams: UpstreamOption[]): string {
  return upstreams.find((upstream) => upstream.id === upstreamID)?.name ?? shortResourceID(upstreamID);
}

function shortResourceID(id: string): string {
  if (id.length <= 12) {
    return id;
  }
  return `${id.slice(0, 8)}...${id.slice(-4)}`;
}
