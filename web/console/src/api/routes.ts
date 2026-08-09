import { apiListAll, apiRequest } from './client';
import type { PagedResponse } from './client';
import { listGateways } from './gateways';
import { listUpstreams } from './upstreams';
import type {
  HeaderMatch,
  HeaderModifier,
  RouteActionResult,
  RouteMutationPayload,
  RouteResource,
  RouteRetry,
  RouteRule,
  RouteListView,
  RouteTimeout,
  RouteWorkspace,
  WeightedUpstream,
} from '@/domain/route';
import type { Upstream } from '@/domain/upstream';

interface RouteMutationResponse {
  success: boolean;
  id?: string;
}

interface RouteListResponse extends PagedResponse {
  routes?: RouteResponse[];
}

interface RouteResponse extends Omit<RouteResource, 'gatewayIDs' | 'hostnames' | 'rules'> {
  gatewayIDs?: string[];
  hostnames?: string[];
  rules?: RouteRuleResponse[];
}

interface RouteRuleResponse {
  name: string;
  pathPrefix: string;
  methods?: RouteRule['methods'];
  headers?: HeaderMatch[];
  targets?: WeightedUpstream[];
  requestHeaderModifier?: Partial<HeaderModifier>;
  responseHeaderModifier?: Partial<HeaderModifier>;
  timeout?: RouteTimeout;
  retry?: RouteRetry;
  modelRouting?: RouteRule['modelRouting'];
}

export async function getRouteWorkspace(): Promise<RouteWorkspace> {
  const [routeList, gatewayList, upstreamList] = await Promise.all([
    listRoutes(),
    listGateways(),
    listUpstreams(),
  ]);

  return {
    routes: routeList.routes,
    gateways: gatewayList.gateways
      .map((gateway) => ({ id: gateway.id, name: gateway.name || gateway.id }))
      .sort((a, b) => a.name.localeCompare(b.name)),
    upstreams: upstreamList.upstreams
      .map((upstream) => ({
        id: upstream.id,
        name: upstream.name || upstream.id,
        type: upstream.type,
        protocol: upstream.protocol,
        provider: upstream.model?.provider,
        models: upstream.model?.models ?? [],
        endpoint: upstreamEndpointSummary(upstream),
        meta: upstream.type === 'model'
          ? `${(upstream.model?.models ?? []).filter((model) => model.enabled).length} 个可用模型`
          : upstreamEndpointMeta(upstream),
      }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  };
}

export async function listRoutes(): Promise<RouteListView> {
  const routes = await apiListAll<RouteListResponse, RouteResponse>('/routes', (page) => page.routes ?? []);
  return { routes: routes.map(normalizeRoute) };
}

export async function saveRoute(payload: RouteMutationPayload): Promise<RouteActionResult> {
  const path = payload.id ? `/routes/${encodeURIComponent(payload.id)}` : '/routes';
  const response = await apiRequest<RouteMutationResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });

  return {
    message: `路由已保存：${payload.name}`,
    changeId: response.id ?? payload.id,
  };
}

export async function deleteRoute(id: string) {
  await apiRequest<RouteMutationResponse>(`/routes/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function setRouteEnabled(id: string, enabled: boolean) {
  await apiRequest<RouteMutationResponse>(`/routes/${encodeURIComponent(id)}/enabled`, {
    method: 'PATCH',
    body: JSON.stringify({ enabled }),
  });
}

function normalizeRoute(route: RouteResponse): RouteResource {
  return {
    ...route,
    gatewayIDs: route.gatewayIDs ?? [],
    hostnames: route.hostnames ?? [],
    rules: (route.rules ?? []).map((rule) => ({
      name: rule.name,
      pathPrefix: rule.pathPrefix,
      methods: rule.methods ?? [],
      headers: rule.headers ?? [],
      upstreams: rule.targets ?? [],
      modelRouting: rule.modelRouting ? {
        models: rule.modelRouting.models.map((model) => ({ ...model })),
      } : undefined,
      requestHeaderModifier: normalizeHeaderModifier(rule.requestHeaderModifier),
      responseHeaderModifier: normalizeHeaderModifier(rule.responseHeaderModifier),
      timeout: rule.timeout,
      retry: rule.retry,
    })),
  };
}

function normalizeHeaderModifier(modifier: Partial<HeaderModifier> | undefined): HeaderModifier | undefined {
  if (!modifier) {
    return undefined;
  }
  return {
    set: modifier.set ?? [],
    remove: modifier.remove ?? [],
  };
}

function upstreamEndpointSummary(upstream: Upstream) {
  const enabledEndpoints = upstream.endpoints.filter((endpoint) => endpoint.enabled);
  const visibleEndpoints = enabledEndpoints.length > 0 ? enabledEndpoints : upstream.endpoints;
  if (visibleEndpoints.length === 0) {
    return '-';
  }

  const first = visibleEndpoints[0];
  const suffix = visibleEndpoints.length > 1 ? ` 等 ${visibleEndpoints.length} 个端点` : '';
  return `${first.address}:${first.port}${suffix}`;
}

function upstreamEndpointMeta(upstream: Upstream) {
  const enabledCount = upstream.endpoints.filter((endpoint) => endpoint.enabled).length;
  return `${enabledCount}/${upstream.endpoints.length} 个端点`;
}
