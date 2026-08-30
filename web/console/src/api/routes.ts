import { apiListAllByCursor, apiListPageByCursor, apiRequest, type CursorPage, type CursorPagedResponse } from './client';
import { listGateways } from './gateways';
import { listUpstreams } from './upstreams';
import { normalizeResourceState } from '@/domain/common';
import type {
  AIRoute,
  HeaderModifier,
  HostRewrite,
  RouteAccessMode,
  RouteListView,
  RouteMutationPayload,
  RoutePathMatchType,
  RouteResource,
  RouteWorkspace,
  WeightedService,
} from '@/domain/route';

interface RouteAPIConfig {
  enabled: boolean;
  accessMode: RouteAccessMode;
  gatewayIDs: string[];
  hostnames: string[];
  match: {
    path: { type: RoutePathMatchType; value: string };
    methods: string[];
    headers: RouteResource['match']['headers'];
  };
  forwarding: {
    service?: { targets: WeightedService[] };
    ai?: AIRoute;
  };
  hostRewrite?: HostRewrite;
  requestHeaderModifier?: HeaderModifier;
  responseHeaderModifier?: HeaderModifier;
  timeout?: RouteResource['timeout'];
  retry?: RouteResource['retry'];
}

interface RouteAPIResource {
  id: string;
  name: string;
  config: RouteAPIConfig;
  state: string;
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

interface RouteListResponse extends CursorPagedResponse {
  routes: RouteAPIResource[];
}

export async function listRoutes(): Promise<RouteListView> {
  const routes = await apiListAllByCursor<RouteListResponse, RouteAPIResource>('/routes', (page) => page.routes ?? []);
  return { routes: routes.map(routeFromAPI) };
}

export async function listRoutePage(input: {
  limit: number; cursor: string; query?: string; enabled?: boolean; state?: string; type?: string;
}): Promise<CursorPage<RouteResource>> {
  const page = await apiListPageByCursor<RouteListResponse, RouteAPIResource>('/routes', input, (value) => value.routes ?? []);
  return { ...page, items: page.items.map(routeFromAPI) };
}

export async function getRouteOptions(): Promise<RouteWorkspace> {
  const [gatewayList, serviceList] = await Promise.all([listGateways(), listUpstreams()]);
  return {
    routes: [],
    gateways: gatewayList.gateways.map((gateway) => ({ id: gateway.id, name: gateway.name, listeners: gateway.listeners })),
    services: serviceList.upstreams.map((service) => ({
      id: service.id,
      name: service.name,
      endpoint: service.endpoints.map((endpoint) => `${endpoint.address}:${endpoint.port}`).join('、'),
      type: service.model ? 'MODEL' : 'HTTP',
    })),
  };
}

export async function saveRoute(payload: RouteMutationPayload): Promise<RouteResource> {
  const path = payload.id ? `/routes/${encodeURIComponent(payload.id)}` : '/routes';
  const forwarding = payload.ai
    ? { ai: payload.ai }
    : { service: { targets: payload.services } };
  const route = await apiRequest<RouteAPIResource>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify({
      id: payload.id,
      version: payload.version,
      name: payload.name,
      config: {
        enabled: payload.enabled,
        accessMode: payload.accessMode,
        gatewayIDs: payload.gatewayIDs,
        hostnames: payload.hostnames,
        match: {
          ...payload.match,
          methods: payload.match.methods.map((method) => `HTTP_METHOD_${method}`),
        },
        forwarding,
        hostRewrite: payload.hostRewrite,
        requestHeaderModifier: payload.requestHeaderModifier,
        responseHeaderModifier: payload.responseHeaderModifier,
        timeout: payload.timeout,
        retry: payload.retry,
      },
    }),
  });
  return routeFromAPI(route);
}

export async function deleteRoute(id: string, version: number) {
  await apiRequest<Record<string, never>>(`/routes/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}

function routeFromAPI(route: RouteAPIResource): RouteResource {
  const config = route.config;
  if (!config?.forwarding) {
    throw new Error(`路由 ${route.id} 的转发配置不完整`);
  }
  const services = config.forwarding?.service?.targets;
  const ai = config.forwarding?.ai;
  if (Boolean(services) === Boolean(ai)) {
    throw new Error(`路由 ${route.id} 的转发配置不完整`);
  }
  if (!config.hostRewrite || !config.timeout) {
    throw new Error(`路由 ${route.id} 缺少规范化后的转发配置`);
  }

  return {
    id: route.id,
    name: route.name,
    enabled: config.enabled,
    accessMode: config.accessMode ?? 'ROUTE_ACCESS_MODE_CALLER',
    gatewayIDs: config.gatewayIDs ?? [],
    hostnames: config.hostnames ?? [],
    match: {
      ...config.match,
      methods: (config.match?.methods ?? []).map((method) => method.replace(/^HTTP_METHOD_/, '') as RouteResource['match']['methods'][number]),
      headers: config.match?.headers ?? [],
    },
    services: services ?? [],
    ai,
    hostRewrite: config.hostRewrite,
    requestHeaderModifier: config.requestHeaderModifier,
    responseHeaderModifier: config.responseHeaderModifier,
    timeout: config.timeout,
    retry: config.retry,
    state: normalizeResourceState(route.state),
    message: route.message,
    version: Number(route.version),
    createdAt: route.createdAt,
    updatedAt: route.updatedAt,
  };
}
