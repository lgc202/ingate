import type { ConsoleRepository } from './contracts';
import type { Gateway, GatewayListView, GatewayMutationPayload, GatewayMutationResult, GatewayRuntimeGroupOption, GatewayValidationReport } from '@/domain/gateway';
import type { HttpMethod, RouteActionResult, RouteComposerPreview, RouteListView, RoutePageView, RoutePolicyCapabilities, RoutePublishPayload, RouteTargetOption, RouteTargetPayload, RouteValidationReport } from '@/domain/route';
import {
  routePolicyCapabilityRetry,
  routePolicyCapabilityTimeout,
} from '@/domain/route';
import type { ServiceListView, ServiceMutationPayload, ServiceMutationResult, ServiceValidationReport } from '@/domain/service';
import { serviceLoadBalancePolicyLabel } from '@/domain/service';

interface GatewayMutationResponse {
  success: boolean;
  id?: string;
}

interface ApiResponse<T> {
  code: number;
  msg: string;
  data: T;
}

type GatewayResource = Omit<Gateway, 'runtimeGroupName'>;

interface GatewayListResponse {
  gateways: GatewayResource[];
}

interface RuntimeGroupListResponse {
  runtimeGroups: RuntimeGroupSummary[];
}

interface RuntimeGroupSummary {
  id: string;
  displayName: string;
  description: string;
  enabled: boolean;
  target: string;
}

interface UpstreamMutationResponse {
  success: boolean;
}

interface RouteMutationResponse {
  success: boolean;
}

const apiBaseUrl = (import.meta.env.VITE_INGATE_API_BASE_URL as string | undefined) ?? '/api/v1';
const defaultRouteTimeoutMillis = 30000;

export const liveConsoleRepository: ConsoleRepository = {
  async getHomeDashboard() {
    return unavailable('首页概览');
  },

  async listGateways() {
    const [gatewayList, runtimeGroups] = await Promise.all([
      request<GatewayListResponse>('/gateways'),
      listRuntimeGroupOptions(),
    ]);
    return gatewayListView(gatewayList, runtimeGroups);
  },

  async listRuntimeGroups() {
    return listRuntimeGroupOptions();
  },

  async saveGatewayDraft(payload) {
    const id = payload.id ?? '';
    const path = payload.id ? `/gateways/${encodeURIComponent(id)}` : '/gateways';
    const method = payload.id ? 'PUT' : 'POST';
    const response = await request<GatewayMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });

    return mutationResult(payload.displayName, response.id);
  },

  async deleteGateway(id) {
    await request<GatewayMutationResponse>(`/gateways/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return {
      message: `网关已删除：${id}`,
    };
  },

  async setGatewayEnabled(id, enabled) {
    await request<GatewayMutationResponse>(`/gateways/${encodeURIComponent(id)}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });
    return {
      message: `网关已${enabled ? '启用' : '停用'}：${id}`,
    };
  },

  async validateGatewayDraft(payload) {
    return validateGatewayPayload(payload);
  },

  async previewGatewayChange() {
    return unavailable('网关变更预览');
  },

  async publishGatewayChange(payload) {
    return this.saveGatewayDraft(payload);
  },

  async getRouteWorkspace() {
    const [routeList, gatewayListResponse, runtimeGroups, serviceList, policyCapabilities] = await Promise.all([
      request<RouteListView>('/routes'),
      request<GatewayListResponse>('/gateways'),
      listRuntimeGroupOptions(),
      request<ServiceListView>('/upstreams'),
      request<RoutePolicyCapabilities>('/route-policy-capabilities'),
    ]);

    const gatewayList = gatewayListView(gatewayListResponse, runtimeGroups);
    return routePageView(routeList, gatewayList, serviceList, policyCapabilities);
  },

  async saveRouteDraft(payload) {
    const name = payload.id ?? routeName(payload.serviceName, payload.methods[0] ?? 'any', payload.path);
    const path = payload.id ? `/routes/${encodeURIComponent(name)}` : '/routes';
    const method = payload.id ? 'PUT' : 'POST';
    await request<RouteMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });

    return routeMutationResult(`${payload.methods.length > 0 ? payload.methods.join('、') : '全部方法'} ${payload.path}`);
  },

  async deleteRoute(id) {
    await request<RouteMutationResponse>(`/routes/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return {
      message: `路由已删除：${id}`,
    };
  },

  async setRouteEnabled(id, enabled) {
    await request<RouteMutationResponse>(`/routes/${encodeURIComponent(id)}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });
    return {
      message: `路由已${enabled ? '启用' : '停用'}：${id}`,
    };
  },

  async validateRouteDraft(payload) {
    return validateRoutePayload(payload);
  },

  async previewRoutePublish() {
    return unavailable('路由变更预览');
  },

  async publishRoute(payload) {
    return this.saveRouteDraft(payload);
  },

  async listServices() {
    return request<ServiceListView>('/upstreams');
  },

  async saveServiceDraft(payload) {
    const name = payload.id ?? payload.name;
    const path = payload.id ? `/upstreams/${encodeURIComponent(name)}` : '/upstreams';
    const method = payload.id ? 'PUT' : 'POST';
    await request<UpstreamMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });

    return serviceMutationResult(payload.name);
  },

  async deleteService(id) {
    await request<UpstreamMutationResponse>(`/upstreams/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return {
      message: `服务已删除：${id}`,
    };
  },

  async validateServiceDraft(payload) {
    return validateServicePayload(payload);
  },

  async previewServiceChange() {
    return unavailable('服务变更预览');
  },

  async publishServiceChange(payload) {
    return this.saveServiceDraft(payload);
  },

  async listPublishSnapshots() {
    return unavailable('发布记录');
  },

  async listPolicies() {
    return unavailable('策略列表');
  },

  async listPlugins() {
    return unavailable('插件列表');
  },

  async getObservabilityOverview() {
    return unavailable('观测数据');
  },

  async getSettingsWorkspace() {
    return unavailable('系统设置');
  },
};

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  });

  const body = await response.json() as ApiResponse<T>;
  if (!response.ok) {
    throw new Error(errorMessage(body, response.status));
  }

  return body.data;
}

function errorMessage(body: { msg?: string; message?: string; error?: string }, status: number) {
  return body.msg || body.message || body.error || `请求失败：${status}`;
}

function mutationResult(gatewayName: string, id?: string): GatewayMutationResult {
  return {
    message: `网关已保存：${gatewayName}`,
    changeId: id,
  };
}

async function listRuntimeGroupOptions(): Promise<GatewayRuntimeGroupOption[]> {
  const response = await request<RuntimeGroupListResponse>('/runtime-groups');
  return response.runtimeGroups.map((runtimeGroup) => ({
    id: runtimeGroup.id,
    name: runtimeGroup.displayName,
  }));
}

function gatewayListView(response: GatewayListResponse, runtimeGroups: GatewayRuntimeGroupOption[]): GatewayListView {
  return {
    gateways: response.gateways.map((gateway) => ({
      ...gateway,
      runtimeGroupName: runtimeGroupName(gateway.runtimeGroup, runtimeGroups),
    })),
  };
}

function runtimeGroupName(id: string, runtimeGroups: GatewayRuntimeGroupOption[]) {
  return runtimeGroups.find((runtimeGroup) => runtimeGroup.id === id)?.name ?? id;
}

function serviceMutationResult(serviceName: string): ServiceMutationResult {
  return {
    message: `服务已保存：${serviceName}`,
  };
}

function routeMutationResult(route: string): RouteActionResult {
  return {
    message: `路由已保存：${route}`,
  };
}

function routePageView(
  routeList: RouteListView,
  gatewayList: GatewayListView,
  serviceList: ServiceListView,
  policyCapabilities: RoutePolicyCapabilities,
): RoutePageView {
  return {
    routes: routeList.routes,
    composer: routeComposer(routeList, gatewayList, serviceList, policyCapabilities),
  };
}

function routeComposer(
  routeList: RouteListView,
  gatewayList: GatewayListView,
  serviceList: ServiceListView,
  policyCapabilities: RoutePolicyCapabilities,
): RouteComposerPreview {
  const gatewayNames = gatewayList.gateways.map((gateway) => gateway.id).sort();
  const targets = routeTargets(routeList, serviceList);

  return {
    methods: [],
    path: '/',
    gatewayNames: gatewayNames[0] ? [gatewayNames[0]] : [],
    hostnames: [],
    serviceName: targets[0]?.name ?? '',
    policyCount: 0,
    rateLimit: '',
    validations: [],
    targets,
    policies: policyCapabilities.policies,
  };
}

function routeTargets(routeList: RouteListView, serviceList: ServiceListView): RouteTargetOption[] {
  return serviceList.services
    .map((service) => ({
      name: service.name || service.id,
      type: service.type,
      endpoint: service.endpoint,
      meta: service.instances,
      healthStatus: service.healthStatus,
      referencedRoutes: referencedRoutes(routeList, service.name || service.id),
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

function referencedRoutes(routeList: RouteListView, serviceName: string): number {
  return routeList.routes.filter((route) => {
    if (route.targets && route.targets.length > 0) {
      return route.targets.some((target) => target.name === serviceName);
    }
    return route.serviceName === serviceName;
  }).length;
}

function routeName(serviceName: string, method: string, path: string) {
  const value = `${serviceName}-${method}-${path}`
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 63)
    .replace(/-+$/g, '');

  return value || 'route';
}

function unavailable(name: string): Promise<never> {
  return Promise.reject(new Error(`${name}后端接口未接入`));
}

function validateGatewayPayload(payload: GatewayMutationPayload): GatewayValidationReport {
  const hostnames = payload.hostBindings.map((binding) => binding.hostname ?? '').filter(Boolean);
  const invalidHostnames = hostnames.filter((hostname) => !isValidHostname(hostname));
  const ports = payload.listeners.map((listener) => String(listener.port)).filter(Boolean);
  const duplicatePorts = ports.filter((port, index) => ports.indexOf(port) !== index);
  const httpsWithoutCertificate = payload.listeners.filter((listener) => listener.protocol === 'HTTPS' && !listener.certificateId);
  const items: GatewayValidationReport['items'] = [
    {
      label: '网关名称',
      status: payload.displayName.trim() ? 'healthy' : 'critical',
      message: payload.displayName.trim() ? payload.displayName.trim() : '请输入网关名称',
    },
    {
      label: '运行组',
      status: payload.runtimeGroup.trim() ? 'healthy' : 'critical',
      message: payload.runtimeGroup.trim() || '请选择运行组',
    },
    {
      label: '监听器',
      status: payload.listeners.length > 0 && payload.listeners.every((listener) => listener.port > 0) && duplicatePorts.length === 0 ? 'healthy' : 'critical',
      message: duplicatePorts.length > 0
        ? `端口重复：${Array.from(new Set(duplicatePorts)).join('、')}`
        : payload.listeners.length > 0 && payload.listeners.every((listener) => listener.port > 0)
          ? payload.listeners.map((listener) => `${listener.protocol}:${listener.port}`).join(' / ')
          : '至少配置一个监听器，并填写端口',
    },
    {
      label: 'HTTPS 证书',
      status: httpsWithoutCertificate.length > 0 ? 'critical' : 'healthy',
      message: httpsWithoutCertificate.length > 0 ? 'HTTPS 监听器必须选择证书' : '证书配置满足要求',
    },
    {
      label: 'Host 策略',
      status: invalidHostnames.length > 0 ? 'critical' : 'healthy',
      message: invalidHostnames.length > 0
        ? `域名格式不正确：${invalidHostnames.join('、')}`
        : hostnames.length > 0
          ? `限制 ${hostnames.length} 个 Host`
          : '不限制 Host',
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '网关配置通过校验，可以保存。' : '网关配置还存在未完成项。',
    items,
  };
}

function validateRoutePayload(payload: RoutePublishPayload): RouteValidationReport {
  const policyValidationMessage = validateRoutePolicyRelationship(payload);
  const targetError = routeTargetValidationMessage(payload);
  const targets = routeMutationTargets(payload);
  const items: RouteValidationReport['items'] = [
    {
      label: '所属网关',
      status: payload.gatewayNames.length > 0 ? 'healthy' : 'critical',
      message: payload.gatewayNames.length > 0 ? payload.gatewayNames.join('、') : '至少选择一个网关',
    },
    {
      label: '匹配路径',
      status: isValidPath(payload.path) ? 'healthy' : 'critical',
      message: isValidPath(payload.path) ? payload.path : '路径必须以 / 开头',
    },
    {
      label: '请求方法',
      status: areValidMethods(payload.methods) ? 'healthy' : 'critical',
      message: payload.methods.length > 0 ? payload.methods.join('、') : '不限制方法',
    },
    {
      label: 'Host 匹配',
      status: payload.hostnames.every(isValidHostname) ? 'healthy' : 'critical',
      message: payload.hostnames.length > 0 ? payload.hostnames.join('、') : '不限制 Host',
    },
    {
      label: '目标服务',
      status: targetError ? 'critical' : 'healthy',
      message: targetError || `已选择 ${targets.length} 个目标服务，总权重 ${routeTargetWeightSum(targets)}`,
    },
    {
      label: '策略参数',
      status: policyValidationMessage ? 'critical' : 'healthy',
      message: policyValidationMessage || (payload.policyBindings.length > 0 ? `已配置 ${payload.policyBindings.length} 个策略` : '未绑定策略'),
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '路由配置通过校验，可以保存。' : '路由配置还存在未完成项。',
    items,
  };
}

function routeMutationTargets(payload: RoutePublishPayload): RouteTargetPayload[] {
  if (payload.targets.length > 0) {
    return payload.targets;
  }
  return payload.serviceName ? [{ name: payload.serviceName, weight: 100 }] : [];
}

function routeTargetValidationMessage(payload: RoutePublishPayload) {
  const targets = routeMutationTargets(payload);
  if (targets.length === 0) {
    return '请选择目标服务';
  }

  const seenNames = new Set<string>();
  for (const target of targets) {
    if (!target.name.trim()) {
      return '目标服务不能为空';
    }
    if (seenNames.has(target.name)) {
      return '目标服务不能重复';
    }
    seenNames.add(target.name);
    if (target.weight < 1 || target.weight > 1000) {
      return '目标权重必须在 1-1000 之间';
    }
  }
  return '';
}

function routeTargetWeightSum(targets: RouteTargetPayload[]) {
  return targets.reduce((sum, target) => sum + target.weight, 0);
}

function validateRoutePolicyRelationship(payload: RoutePublishPayload) {
  if (!payload.policyBindings.every((binding) => Object.values(binding.parameters).some(hasPolicyValue))) {
    return '策略参数未填写完整';
  }

  const retryPolicy = payload.policyBindings.find((binding) => binding.capability === routePolicyCapabilityRetry);
  if (!retryPolicy) {
    return '';
  }

  const timeoutPolicy = payload.policyBindings.find((binding) => binding.capability === routePolicyCapabilityTimeout);
  const totalTimeoutMillis = Number(timeoutPolicy?.parameters.timeoutMillis ?? defaultRouteTimeoutMillis);
  const perTryTimeoutMillis = Number(retryPolicy.parameters.perTryTimeoutMillis ?? 0);
  if (Number.isFinite(perTryTimeoutMillis) && Number.isFinite(totalTimeoutMillis) && perTryTimeoutMillis > totalTimeoutMillis) {
    return `单次尝试超时不能大于请求总超时 ${totalTimeoutMillis}ms`;
  }

  return '';
}

function validateServicePayload(payload: ServiceMutationPayload): ServiceValidationReport {
  const endpointErrors = validateServiceEndpoints(payload.endpoints);
  const enabledEndpointCount = payload.endpoints.filter((endpoint) => endpoint.enabled).length;
  const healthInterval = Number(payload.healthCheckIntervalSeconds);
  const healthTimeout = Number(payload.healthCheckTimeoutSeconds);
  const items: ServiceValidationReport['items'] = [
    {
      label: '服务名称',
      status: payload.name.trim() ? 'healthy' : 'critical',
      message: payload.name.trim() ? payload.name.trim() : '请输入服务名称',
    },
    {
      label: '服务端点',
      status: endpointErrors.length === 0 && enabledEndpointCount > 0 ? 'healthy' : 'critical',
      message: endpointErrors[0] ?? (enabledEndpointCount > 0 ? `已配置 ${payload.endpoints.length} 个端点` : '至少保留一个启用端点'),
    },
    {
      label: '负载均衡',
      status: payload.loadBalancePolicy ? 'healthy' : 'critical',
      message: payload.loadBalancePolicy ? serviceLoadBalancePolicyLabel(payload.loadBalancePolicy) : '请选择负载均衡方式',
    },
    {
      label: '健康检查',
      status: validateServiceHealth(payload, healthInterval, healthTimeout),
      message: serviceHealthMessage(payload, healthInterval, healthTimeout),
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '服务配置通过校验，可以保存。' : '服务配置还存在未完成项。',
    items,
  };
}

function isValidPath(path: string): boolean {
  return path.trim().startsWith('/');
}

function areValidMethods(methods: HttpMethod[]): boolean {
  const allowedMethods: HttpMethod[] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'];

  return methods.every((method) => allowedMethods.includes(method));
}

function hasPolicyValue(value: string | string[]): boolean {
  if (Array.isArray(value)) {
    return value.some((item) => item.trim().length > 0);
  }

  return value.trim().length > 0;
}

function isValidHostname(hostname: string): boolean {
  const normalized = hostname.startsWith('*.') ? hostname.slice(2) : hostname;

  if (!normalized.includes('.') || normalized.length > 253) {
    return false;
  }

  return normalized
    .split('.')
    .every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part));
}

function validateServiceEndpoints(endpoints: ServiceMutationPayload['endpoints']) {
  if (endpoints.length === 0) {
    return ['至少配置一个服务端点'];
  }

  return endpoints.flatMap((endpoint, index) => {
    const messages: string[] = [];
    const port = Number(endpoint.port);
    const weight = Number(endpoint.weight);

    if (!endpoint.address.trim()) {
      messages.push(`第 ${index + 1} 个端点缺少地址`);
    }

    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      messages.push(`第 ${index + 1} 个端点端口不合法`);
    }

    if (!Number.isInteger(weight) || weight < 0 || weight > 1000) {
      messages.push(`第 ${index + 1} 个端点权重需要在 0-1000 之间`);
    }

    return messages;
  });
}

function validateServiceHealth(payload: ServiceMutationPayload, interval: number, timeout: number) {
  if (!payload.healthCheckEnabled) {
    return 'healthy';
  }

  if (!payload.healthCheckPath.startsWith('/')) {
    return 'critical';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return 'critical';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return 'critical';
  }

  return 'healthy';
}

function serviceHealthMessage(payload: ServiceMutationPayload, interval: number, timeout: number) {
  if (!payload.healthCheckEnabled) {
    return '未启用健康检查';
  }

  if (!payload.healthCheckPath.startsWith('/')) {
    return '探活路径必须以 / 开头';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return '检查间隔需要在 1-300 秒之间';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return '超时时间需要在 1-60 秒之间，并且小于检查间隔';
  }

  return `${payload.healthCheckPath} / ${interval}s / ${timeout}s`;
}
