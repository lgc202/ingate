import type { ConsoleRepository } from './contracts';
import type { Gateway, GatewayListView, GatewayMutationPayload, GatewayMutationResult, GatewayRuntimeGroupOption, GatewayValidationReport } from '@/domain/gateway';
import type {
  AccessControlPolicy,
  PolicyBinding,
  PolicyMutationResult,
  PolicyOption,
  PolicyWorkspace,
  RateLimitPolicy,
  RedisStoreOption,
} from '@/domain/policy';
import type { HttpMethod, RouteActionResult, RouteComposerPreview, RouteListView, RoutePageView, RoutePolicyCapabilities, RoutePublishPayload, RouteRule, RouteTargetOption, RouteTargetPayload, RouteValidationReport } from '@/domain/route';
import {
  routePolicyCapabilityRequestHeaderModifier,
  routePolicyCapabilityResponseHeaderModifier,
  routePolicyCapabilityRetry,
  routePolicyCapabilityTimeout,
} from '@/domain/route';
import type { ServiceListView, ServiceMutationPayload, ServiceMutationResult, ServiceResource, ServiceValidationReport } from '@/domain/service';
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
  id?: string;
}

interface PolicyMutationResponse {
  success: boolean;
  id?: string;
}

interface RateLimitPolicyListResponse {
  policies: RateLimitPolicy[];
}

interface AccessControlPolicyListResponse {
  policies: AccessControlPolicy[];
}

interface PolicyBindingListResponse {
  bindings: PolicyBinding[];
}

interface RedisStoreListResponse {
  redisStores: RedisStoreOption[];
}

const apiBaseUrl = (import.meta.env.VITE_INGATE_API_BASE_URL as string | undefined) ?? '/api/v1';
const defaultRouteTimeoutMillis = 30000;
const routePolicyCapabilities: RoutePolicyCapabilities = {
  policies: [
    {
      capability: routePolicyCapabilityRequestHeaderModifier,
      displayName: '请求 Header 改写',
      meta: '在转发到上游前设置、追加或删除请求 Header，常用于租户标识、灰度标记和上游兼容',
      enabled: false,
      params: [
        { key: 'setHeadersOn', label: '写入 Header 名称', inputType: 'text', defaultValue: '', placeholder: '多个名称用逗号分隔', required: true },
        { key: 'value', label: 'Header 值', inputType: 'text', defaultValue: '', placeholder: '请输入要写入的 Header 值', required: true },
        { key: 'removeHeadersOn', label: '删除 Header 名称', inputType: 'text', defaultValue: '', placeholder: '多个名称用逗号分隔' },
      ],
    },
    {
      capability: routePolicyCapabilityResponseHeaderModifier,
      displayName: '响应 Header 改写',
      meta: '在返回给客户端前设置、追加或删除响应 Header，常用于跨域、安全响应头和兼容旧客户端',
      enabled: false,
      params: [
        { key: 'setHeadersOn', label: '写入 Header 名称', inputType: 'text', defaultValue: '', placeholder: '多个名称用逗号分隔', required: true },
        { key: 'value', label: 'Header 值', inputType: 'text', defaultValue: '', placeholder: '请输入要写入的 Header 值', required: true },
        { key: 'removeHeadersOn', label: '删除 Header 名称', inputType: 'text', defaultValue: '', placeholder: '多个名称用逗号分隔' },
      ],
    },
    {
      capability: routePolicyCapabilityTimeout,
      displayName: '超时控制',
      meta: '设置当前路由从进入网关到返回响应的最长时间，包含失败重试过程',
      enabled: false,
      params: [
        { key: 'timeoutMillis', label: '请求总超时', inputType: 'number', defaultValue: '30000', required: true, unit: 'ms', min: 100, max: 300000 },
      ],
    },
    {
      capability: routePolicyCapabilityRetry,
      displayName: '失败重试',
      meta: '针对 5xx、连接失败等上游异常进行有限重试；单次尝试超时不能超过请求总超时',
      enabled: false,
      params: [
        { key: 'attempts', label: '重试次数', inputType: 'number', defaultValue: '2', required: true, unit: '次', min: 1, max: 5 },
        { key: 'perTryTimeoutMillis', label: '单次尝试超时', inputType: 'number', defaultValue: '1000', required: true, unit: 'ms', min: 100, max: 60000 },
      ],
    },
  ],
};

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

    return mutationResult(payload.name, response.id);
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
    const [routeListResponse, gatewayListResponse, runtimeGroups, serviceList] = await Promise.all([
      request<RouteListView>('/routes'),
      request<GatewayListResponse>('/gateways'),
      listRuntimeGroupOptions(),
      request<ServiceListView>('/upstreams'),
    ]);

    const routeList = normalizeRouteListView(routeListResponse);
    const gatewayList = gatewayListView(gatewayListResponse, runtimeGroups);
    return routePageView(routeList, gatewayList, serviceList, routePolicyCapabilities);
  },

  async saveRouteDraft(payload) {
    const path = payload.id ? `/routes/${encodeURIComponent(payload.id)}` : '/routes';
    const method = payload.id ? 'PUT' : 'POST';
    const response = await request<RouteMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });

    return routeMutationResult(routePayloadSummary(payload), response.id ?? payload.id);
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

  async getPolicyWorkspace() {
    const [rateLimitPolicies, accessControlPolicies, policyBindings, redisStores, gatewayListResponse, routeListResponse] = await Promise.all([
      request<RateLimitPolicyListResponse>('/rate-limit-policies'),
      request<AccessControlPolicyListResponse>('/access-control-policies'),
      request<PolicyBindingListResponse>('/policy-bindings'),
      request<RedisStoreListResponse>('/redis-stores'),
      request<GatewayListResponse>('/gateways'),
      request<RouteListView>('/routes'),
    ]);

    return policyWorkspace(
      rateLimitPolicies,
      accessControlPolicies,
      policyBindings,
      redisStores,
      gatewayListResponse,
      normalizeRouteListView(routeListResponse),
    );
  },

  async saveRateLimitPolicy(payload) {
    const path = payload.id ? `/rate-limit-policies/${encodeURIComponent(payload.id)}` : '/rate-limit-policies';
    const method = payload.id ? 'PUT' : 'POST';
    const response = await request<PolicyMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });
    return policyMutationResult('限流策略', payload.name, response.id ?? payload.id);
  },

  async deleteRateLimitPolicy(id) {
    await request<PolicyMutationResponse>(`/rate-limit-policies/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return { message: `限流策略已删除：${id}` };
  },

  async setRateLimitPolicyEnabled(id, enabled) {
    await request<PolicyMutationResponse>(`/rate-limit-policies/${encodeURIComponent(id)}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });
    return { message: `限流策略已${enabled ? '启用' : '停用'}：${id}` };
  },

  async saveAccessControlPolicy(payload) {
    const path = payload.id ? `/access-control-policies/${encodeURIComponent(payload.id)}` : '/access-control-policies';
    const method = payload.id ? 'PUT' : 'POST';
    const response = await request<PolicyMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });
    return policyMutationResult('访问控制策略', payload.name, response.id ?? payload.id);
  },

  async deleteAccessControlPolicy(id) {
    await request<PolicyMutationResponse>(`/access-control-policies/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return { message: `访问控制策略已删除：${id}` };
  },

  async setAccessControlPolicyEnabled(id, enabled) {
    await request<PolicyMutationResponse>(`/access-control-policies/${encodeURIComponent(id)}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });
    return { message: `访问控制策略已${enabled ? '启用' : '停用'}：${id}` };
  },

  async savePolicyBinding(payload) {
    const path = payload.id ? `/policy-bindings/${encodeURIComponent(payload.id)}` : '/policy-bindings';
    const method = payload.id ? 'PUT' : 'POST';
    const response = await request<PolicyMutationResponse>(path, {
      method,
      body: JSON.stringify(payload),
    });
    return policyMutationResult('策略绑定', payload.name, response.id ?? payload.id);
  },

  async deletePolicyBinding(id) {
    await request<PolicyMutationResponse>(`/policy-bindings/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return { message: `策略绑定已删除：${id}` };
  },

  async setPolicyBindingEnabled(id, enabled) {
    await request<PolicyMutationResponse>(`/policy-bindings/${encodeURIComponent(id)}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });
    return { message: `策略绑定已${enabled ? '启用' : '停用'}：${id}` };
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

function routeMutationResult(route: string, changeId?: string): RouteActionResult {
  return {
    message: `路由已保存：${route}`,
    changeId,
  };
}

function routePayloadSummary(payload: RoutePublishPayload) {
  if (payload.name.trim()) {
    return payload.name.trim();
  }
  const rule = primaryRouteRule(payload);
  if (!rule) {
    return payload.id ?? '未配置规则';
  }
  const methods = rule.methods ?? [];
  return `${methods.length > 0 ? methods.join('、') : '全部方法'} ${rule.pathPrefix}`;
}

function normalizeRouteListView(response: RouteListView): RouteListView {
  return {
    routes: (response.routes ?? []).map((route) => ({
      ...route,
      gatewayIDs: route.gatewayIDs ?? [],
      hostnames: route.hostnames ?? [],
      rules: (route.rules ?? []).map((rule) => ({
        ...rule,
        methods: rule.methods ?? [],
        headers: rule.headers ?? [],
        targets: rule.targets ?? [],
        requestHeaderModifier: rule.requestHeaderModifier
          ? {
            ...rule.requestHeaderModifier,
            set: rule.requestHeaderModifier.set ?? [],
            remove: rule.requestHeaderModifier.remove ?? [],
          }
          : undefined,
        responseHeaderModifier: rule.responseHeaderModifier
          ? {
            ...rule.responseHeaderModifier,
            set: rule.responseHeaderModifier.set ?? [],
            remove: rule.responseHeaderModifier.remove ?? [],
          }
          : undefined,
      })),
    })),
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
    composer: routeComposer(gatewayList, serviceList, policyCapabilities),
  };
}

function routeComposer(
  gatewayList: GatewayListView,
  serviceList: ServiceListView,
  policyCapabilities: RoutePolicyCapabilities,
): RouteComposerPreview {
  const gateways = gatewayList.gateways
    .map((gateway) => ({ id: gateway.id, name: gateway.name || gateway.id }))
    .sort((a, b) => a.name.localeCompare(b.name));
  const targets = routeTargets(serviceList);

  return {
    name: '',
    methods: [],
    path: '/',
    gatewayIDs: gateways[0] ? [gateways[0].id] : [],
    gateways,
    hostnames: [],
    validations: [],
    targets,
    policies: policyCapabilities.policies,
  };
}

function routeTargets(serviceList: ServiceListView): RouteTargetOption[] {
  return serviceList.upstreams
    .map((service) => ({
      id: service.id,
      name: service.name || service.id,
      type: service.type,
      endpoint: upstreamEndpointSummary(service),
      meta: upstreamEndpointMeta(service),
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

function policyWorkspace(
  rateLimitPolicies: RateLimitPolicyListResponse,
  accessControlPolicies: AccessControlPolicyListResponse,
  policyBindings: PolicyBindingListResponse,
  redisStores: RedisStoreListResponse,
  gatewayList: GatewayListResponse,
  routeList: RouteListView,
): PolicyWorkspace {
  return {
    rateLimitPolicies: (rateLimitPolicies.policies ?? []).map(normalizeRateLimitPolicy),
    accessControlPolicies: (accessControlPolicies.policies ?? []).map(normalizeAccessControlPolicy),
    bindings: policyBindings.bindings ?? [],
    gateways: gatewayList.gateways
      .map((gateway) => ({ id: gateway.id, name: gateway.name || gateway.id }))
      .sort((a, b) => a.name.localeCompare(b.name)),
    routes: routeList.routes
      .map((route): PolicyOption => ({
        id: route.id,
        name: route.name || route.id,
        rules: route.rules.map((rule) => rule.name),
      }))
      .sort((a, b) => a.name.localeCompare(b.name)),
    redisStores: (redisStores.redisStores ?? [])
      .map((store) => ({ id: store.id, name: store.name || store.id }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  };
}

function normalizeRateLimitPolicy(policy: RateLimitPolicy): RateLimitPolicy {
  return {
    ...policy,
    rules: policy.rules ?? [],
    response: policy.response ?? {},
  };
}

function normalizeAccessControlPolicy(policy: AccessControlPolicy): AccessControlPolicy {
  return {
    ...policy,
    defaultAction: policy.defaultAction ?? 'Allow',
    rules: policy.rules ?? [],
    response: policy.response ?? {},
  };
}

function policyMutationResult(kind: string, name: string, id?: string): PolicyMutationResult {
  return {
    message: `${kind}已保存：${name}`,
    changeId: id,
  };
}

function upstreamEndpointSummary(upstream: ServiceResource) {
  const enabledEndpoints = upstream.endpoints.filter((endpoint) => endpoint.enabled);
  const visibleEndpoints = enabledEndpoints.length > 0 ? enabledEndpoints : upstream.endpoints;
  if (visibleEndpoints.length === 0) {
    return '-';
  }

  const first = visibleEndpoints[0];
  const suffix = visibleEndpoints.length > 1 ? ` 等 ${visibleEndpoints.length} 个端点` : '';
  return `${first.address}:${first.port}${suffix}`;
}

function upstreamEndpointMeta(upstream: ServiceResource) {
  const enabledCount = upstream.endpoints.filter((endpoint) => endpoint.enabled).length;
  return `${enabledCount}/${upstream.endpoints.length} 个端点`;
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
      status: payload.name.trim() ? 'healthy' : 'critical',
      message: payload.name.trim() ? payload.name.trim() : '请输入网关名称',
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
  const rule = primaryRouteRule(payload);
  const policyValidationMessage = rule ? validateRoutePolicyRelationship(rule) : '';
  const targetError = rule ? routeTargetValidationMessage(rule.targets) : '请至少配置一条路由规则';
  const targets = rule?.targets ?? [];
  const items: RouteValidationReport['items'] = [
    {
      label: '路由名称',
      status: payload.name.trim() ? 'healthy' : 'critical',
      message: payload.name.trim() ? payload.name.trim() : '请填写路由名称',
    },
    {
      label: '所属网关',
      status: payload.gatewayIDs.length > 0 ? 'healthy' : 'critical',
      message: payload.gatewayIDs.length > 0 ? payload.gatewayIDs.join('、') : '至少选择一个网关',
    },
    {
      label: '匹配路径',
      status: rule && isValidPath(rule.pathPrefix) ? 'healthy' : 'critical',
      message: rule && isValidPath(rule.pathPrefix) ? rule.pathPrefix : '路径必须以 / 开头',
    },
    {
      label: '请求方法',
      status: rule && areValidMethods(rule.methods ?? []) ? 'healthy' : 'critical',
      message: rule && (rule.methods ?? []).length > 0 ? (rule.methods ?? []).join('、') : '不限制方法',
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
      message: policyValidationMessage || (rule ? `已配置 ${routePolicyCount(rule)} 个策略` : '未绑定策略'),
    },
  ];
  const valid = items.every((item) => item.status === 'healthy');

  return {
    valid,
    summary: valid ? '路由配置通过校验，可以保存。' : '路由配置还存在未完成项。',
    items,
  };
}

function primaryRouteRule(payload: RoutePublishPayload): RouteRule | undefined {
  return payload.rules[0];
}

function routeTargetValidationMessage(targets: RouteTargetPayload[]) {
  if (targets.length === 0) {
    return '请选择目标服务';
  }

  const seenIDs = new Set<string>();
  for (const target of targets) {
    if (!target.upstreamID.trim()) {
      return '目标服务不能为空';
    }
    if (seenIDs.has(target.upstreamID)) {
      return '目标服务不能重复';
    }
    seenIDs.add(target.upstreamID);
    if (target.weight < 1 || target.weight > 100) {
      return '目标权重必须在 1-100 之间';
    }
  }
  return '';
}

function routeTargetWeightSum(targets: RouteTargetPayload[]) {
  return targets.reduce((sum, target) => sum + target.weight, 0);
}

function validateRoutePolicyRelationship(rule: RouteRule) {
  const retry = rule.retry;
  if (!retry) {
    return '';
  }

  const totalTimeoutMillis = rule.timeout?.requestMillis ?? defaultRouteTimeoutMillis;
  const perTryTimeoutMillis = retry.perTryTimeoutMillis;
  if (Number.isFinite(perTryTimeoutMillis) && Number.isFinite(totalTimeoutMillis) && perTryTimeoutMillis > totalTimeoutMillis) {
    return `单次尝试超时不能大于请求总超时 ${totalTimeoutMillis}ms`;
  }

  return '';
}

function routePolicyCount(rule: RouteRule) {
  return [
    rule.requestHeaderModifier,
    rule.responseHeaderModifier,
    rule.timeout,
    rule.retry,
  ].filter(Boolean).length;
}

function validateServicePayload(payload: ServiceMutationPayload): ServiceValidationReport {
  const endpointErrors = validateServiceEndpoints(payload.endpoints);
  const enabledEndpointCount = payload.endpoints.filter((endpoint) => endpoint.enabled).length;
  const healthInterval = payload.healthCheck?.intervalSeconds ?? 0;
  const healthTimeout = payload.healthCheck?.timeoutSeconds ?? 0;
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

    if (!Number.isInteger(weight) || weight < 1 || weight > 100) {
      messages.push(`第 ${index + 1} 个端点权重需要在 1-100 之间`);
    }

    return messages;
  });
}

function validateServiceHealth(payload: ServiceMutationPayload, interval: number, timeout: number) {
  if (!payload.healthCheck?.enabled) {
    return 'healthy';
  }

  if (!payload.healthCheck.path?.startsWith('/')) {
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
  if (!payload.healthCheck?.enabled) {
    return '未启用健康检查';
  }

  if (!payload.healthCheck.path?.startsWith('/')) {
    return '探活路径必须以 / 开头';
  }

  if (!Number.isInteger(interval) || interval < 1 || interval > 300) {
    return '检查间隔需要在 1-300 秒之间';
  }

  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 60 || timeout >= interval) {
    return '超时时间需要在 1-60 秒之间，并且小于检查间隔';
  }

  return `${payload.healthCheck.path} / ${interval}s / ${timeout}s`;
}
