import { apiRequest, setQueryParameter } from './client';
import { listCallers } from './callers';
import { listGateways } from './gateways';
import { listRoutes } from './routes';
import { listServices } from './services';
import type { AIUsageAnalysis, AIUsageFilters, AIUsageWorkspace } from '@/domain/aiUsage';

export async function getAIUsageAnalysis(filters: AIUsageFilters): Promise<AIUsageAnalysis> {
  const query = new URLSearchParams({
    startTime: new Date(filters.startTime).toISOString(),
    endTime: new Date(filters.endTime).toISOString(),
    breakdownDimension: filters.breakdownDimension,
    breakdownOrder: filters.breakdownOrder,
    breakdownLimit: String(filters.breakdownLimit ?? 10),
  });
  setQueryParameter(query, 'gatewayID', filters.gatewayID);
  setQueryParameter(query, 'callerID', filters.callerID);
  setQueryParameter(query, 'routeID', filters.routeID);
  setQueryParameter(query, 'clientModel', filters.clientModel);
  setQueryParameter(query, 'serviceID', filters.serviceID);
  setQueryParameter(query, 'actualModel', filters.actualModel);

  const analysis = await apiRequest<AIUsageAnalysis>(`/ai-usage-analysis?${query}`);
  return {
    ...analysis,
    trend: analysis.trend ?? [],
    breakdown: analysis.breakdown ?? [],
  };
}

export async function getAIUsageWorkspace(): Promise<AIUsageWorkspace> {
  const [gatewayList, routeList, serviceList, callers] = await Promise.all([
    listGateways(),
    listRoutes(),
    listServices(),
    listCallers(),
  ]);
  const routes = routeList.routes.filter((route) => route.ai);
  const services = serviceList.services.filter((service) => service.model);
  const modelServiceIDs = new Set(services.map((service) => service.id));
  const clientModels = new Set<string>();
  const actualModels = new Set<string>();

  for (const route of routes) {
    for (const model of route.ai?.models ?? []) {
      if (model.name) clientModels.add(model.name);
      for (const target of model.targets) {
        if (modelServiceIDs.has(target.serviceID) && target.model) actualModels.add(target.model);
      }
    }
  }

  return {
    gateways: gatewayList.gateways.map(({ id, name }) => ({ id, name })),
    callers: callers.map(({ id, name }) => ({ id, name })),
    routes: routes.map(({ id, name }) => ({ id, name })),
    services: services.map(({ id, name }) => ({ id, name })),
    clientModels: [...clientModels].sort((left, right) => left.localeCompare(right, 'zh-CN')),
    actualModels: [...actualModels].sort((left, right) => left.localeCompare(right, 'zh-CN')),
  };
}
