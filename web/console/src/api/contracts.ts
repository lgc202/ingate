import type { GatewayListView, GatewayMutationPayload, GatewayMutationResult, GatewayValidationReport } from '@/domain/gateway';
import type { HomeDashboard } from '@/domain/home';
import type { ObservabilityOverview } from '@/domain/observability';
import type { PluginListView } from '@/domain/plugin';
import type {
  AccessControlPolicyPayload,
  PolicyBindingPayload,
  PolicyMutationResult,
  PolicyWorkspace,
  RateLimitPolicyPayload,
} from '@/domain/policy';
import type { RuntimeStatusView } from '@/domain/runtime';
import type { RouteActionResult, RouteMutationPayload, RoutePageView, RouteValidationReport } from '@/domain/route';
import type { ServiceListView, ServiceMutationPayload, ServiceMutationResult, ServiceValidationReport } from '@/domain/service';
import type { SettingsWorkspace } from '@/domain/settings';

export interface ConsoleRepository {
  getHomeDashboard(): Promise<HomeDashboard>;
  listGateways(): Promise<GatewayListView>;
  saveGatewayDraft(payload: GatewayMutationPayload): Promise<GatewayMutationResult>;
  deleteGateway(id: string): Promise<GatewayMutationResult>;
  setGatewayEnabled(id: string, enabled: boolean): Promise<GatewayMutationResult>;
  validateGatewayDraft(payload: GatewayMutationPayload): Promise<GatewayValidationReport>;
  getRouteWorkspace(): Promise<RoutePageView>;
  saveRouteDraft(payload: RouteMutationPayload): Promise<RouteActionResult>;
  deleteRoute(id: string): Promise<RouteActionResult>;
  setRouteEnabled(id: string, enabled: boolean): Promise<RouteActionResult>;
  validateRouteDraft(payload: RouteMutationPayload): Promise<RouteValidationReport>;
  listServices(): Promise<ServiceListView>;
  saveServiceDraft(payload: ServiceMutationPayload): Promise<ServiceMutationResult>;
  deleteService(id: string): Promise<ServiceMutationResult>;
  validateServiceDraft(payload: ServiceMutationPayload): Promise<ServiceValidationReport>;
  getRuntimeStatus(): Promise<RuntimeStatusView>;
  getPolicyWorkspace(): Promise<PolicyWorkspace>;
  saveRateLimitPolicy(payload: RateLimitPolicyPayload): Promise<PolicyMutationResult>;
  saveAccessControlPolicy(payload: AccessControlPolicyPayload): Promise<PolicyMutationResult>;
  savePolicyBinding(payload: PolicyBindingPayload): Promise<PolicyMutationResult>;
  deleteRateLimitPolicy(id: string): Promise<PolicyMutationResult>;
  deleteAccessControlPolicy(id: string): Promise<PolicyMutationResult>;
  deletePolicyBinding(id: string): Promise<PolicyMutationResult>;
  setRateLimitPolicyEnabled(id: string, enabled: boolean): Promise<PolicyMutationResult>;
  setAccessControlPolicyEnabled(id: string, enabled: boolean): Promise<PolicyMutationResult>;
  setPolicyBindingEnabled(id: string, enabled: boolean): Promise<PolicyMutationResult>;
  listPlugins(): Promise<PluginListView>;
  getObservabilityOverview(): Promise<ObservabilityOverview>;
  getSettingsWorkspace(): Promise<SettingsWorkspace>;
}
