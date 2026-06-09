import type { GatewayListView, GatewayMutationPayload, GatewayMutationPreview, GatewayMutationResult, GatewayRuntimeGroupOption, GatewayValidationReport } from '@/domain/gateway';
import type { HomeDashboard } from '@/domain/home';
import type { ObservabilityOverview } from '@/domain/observability';
import type { PluginListView } from '@/domain/plugin';
import type {
  AccessControlPolicy,
  PolicyBinding,
  PolicyMutationResult,
  PolicyWorkspace,
  RateLimitPolicy,
} from '@/domain/policy';
import type { PublishListView } from '@/domain/publish';
import type { RouteActionResult, RoutePageView, RoutePublishPayload, RoutePublishPreview, RouteValidationReport } from '@/domain/route';
import type { ServiceListView, ServiceMutationPayload, ServiceMutationPreview, ServiceMutationResult, ServiceValidationReport } from '@/domain/service';
import type { SettingsWorkspace } from '@/domain/settings';

export interface ConsoleRepository {
  getHomeDashboard(): Promise<HomeDashboard>;
  listGateways(): Promise<GatewayListView>;
  listRuntimeGroups(): Promise<GatewayRuntimeGroupOption[]>;
  saveGatewayDraft(payload: GatewayMutationPayload): Promise<GatewayMutationResult>;
  deleteGateway(id: string): Promise<GatewayMutationResult>;
  setGatewayEnabled(id: string, enabled: boolean): Promise<GatewayMutationResult>;
  validateGatewayDraft(payload: GatewayMutationPayload): Promise<GatewayValidationReport>;
  previewGatewayChange(payload: GatewayMutationPayload): Promise<GatewayMutationPreview>;
  publishGatewayChange(payload: GatewayMutationPayload): Promise<GatewayMutationResult>;
  getRouteWorkspace(): Promise<RoutePageView>;
  saveRouteDraft(payload: RoutePublishPayload): Promise<RouteActionResult>;
  deleteRoute(id: string): Promise<RouteActionResult>;
  setRouteEnabled(id: string, enabled: boolean): Promise<RouteActionResult>;
  validateRouteDraft(payload: RoutePublishPayload): Promise<RouteValidationReport>;
  previewRoutePublish(payload: RoutePublishPayload): Promise<RoutePublishPreview>;
  publishRoute(payload: RoutePublishPayload): Promise<RouteActionResult>;
  listServices(): Promise<ServiceListView>;
  saveServiceDraft(payload: ServiceMutationPayload): Promise<ServiceMutationResult>;
  deleteService(id: string): Promise<ServiceMutationResult>;
  validateServiceDraft(payload: ServiceMutationPayload): Promise<ServiceValidationReport>;
  previewServiceChange(payload: ServiceMutationPayload): Promise<ServiceMutationPreview>;
  publishServiceChange(payload: ServiceMutationPayload): Promise<ServiceMutationResult>;
  listPublishSnapshots(): Promise<PublishListView>;
  getPolicyWorkspace(): Promise<PolicyWorkspace>;
  saveRateLimitPolicy(payload: RateLimitPolicy): Promise<PolicyMutationResult>;
  deleteRateLimitPolicy(id: string): Promise<PolicyMutationResult>;
  setRateLimitPolicyEnabled(id: string, enabled: boolean): Promise<PolicyMutationResult>;
  saveAccessControlPolicy(payload: AccessControlPolicy): Promise<PolicyMutationResult>;
  deleteAccessControlPolicy(id: string): Promise<PolicyMutationResult>;
  setAccessControlPolicyEnabled(id: string, enabled: boolean): Promise<PolicyMutationResult>;
  savePolicyBinding(payload: PolicyBinding): Promise<PolicyMutationResult>;
  deletePolicyBinding(id: string): Promise<PolicyMutationResult>;
  setPolicyBindingEnabled(id: string, enabled: boolean): Promise<PolicyMutationResult>;
  listPlugins(): Promise<PluginListView>;
  getObservabilityOverview(): Promise<ObservabilityOverview>;
  getSettingsWorkspace(): Promise<SettingsWorkspace>;
}
