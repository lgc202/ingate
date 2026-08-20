import { apiListAllByCursor, apiRequest, type CursorPagedResponse } from './client';
import { listGateways } from './gateways';
import { listUpstreams } from './upstreams';
import { normalizeResourceState } from '@/domain/common';
import type { RouteListView, RouteMutationPayload, RouteResource, RouteWorkspace } from '@/domain/route';

interface RouteListResponse extends RouteListView, CursorPagedResponse {}

export async function listRoutes(): Promise<RouteListView> {
  const routes = await apiListAllByCursor<RouteListResponse, RouteResource>('/routes', (page) => page.routes ?? []);
  return { routes: routes.map(routeFromAPI) };
}

export async function getRouteWorkspace(): Promise<RouteWorkspace> {
  const [routeList, gatewayList, upstreamList] = await Promise.all([listRoutes(), listGateways(), listUpstreams()]);
  return {
    routes: routeList.routes,
    gateways: gatewayList.gateways.map((gateway) => ({ id: gateway.id, name: gateway.name })),
    upstreams: upstreamList.upstreams.map((upstream) => ({
      id: upstream.id,
      name: upstream.name,
      endpoint: upstream.endpoints.map((endpoint) => `${endpoint.address}:${endpoint.port}`).join('、'),
      type: upstream.model ? 'MODEL' : 'HTTP',
    })),
  };
}

export async function saveRoute(payload: RouteMutationPayload): Promise<RouteResource> {
  const path = payload.id ? `/routes/${encodeURIComponent(payload.id)}` : '/routes';
  const route = await apiRequest<RouteResource>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify({
      ...payload,
      match: {
        ...payload.match,
        methods: payload.match.methods.map((method) => `HTTP_METHOD_${method}`),
      },
    }),
  });
  return routeFromAPI(route);
}

export async function deleteRoute(id: string, version: number) {
  await apiRequest<Record<string, never>>(`/routes/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}

function routeFromAPI(route: RouteResource): RouteResource {
  return {
    ...route,
    version: Number(route.version),
    state: normalizeResourceState(route.state),
    accessMode: route.accessMode ?? 'ROUTE_ACCESS_CALLER',
    gatewayIDs: route.gatewayIDs ?? [],
    hostnames: route.hostnames ?? [],
    match: {
      ...route.match,
      methods: (route.match?.methods ?? []).map((method) => method.replace(/^HTTP_METHOD_/, '') as RouteResource['match']['methods'][number]),
      headers: route.match?.headers ?? [],
    },
    upstreams: route.upstreams ?? [],
    ai: route.ai,
    hostRewrite: route.hostRewrite ?? { mode: 'HOST_REWRITE_MODE_PRESERVE' },
  };
}
