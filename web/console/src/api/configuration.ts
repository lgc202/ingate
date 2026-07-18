import { listCertificates } from './certificates';
import { listGateways } from './gateways';
import {
  listAccessControlPolicies,
  listRateLimitPolicies,
} from './policies';
import { listRoutes } from './routes';
import { listUpstreams } from './upstreams';
import type {
  ConfigurationResourceKind,
  ConfigurationStatusItem,
  ConfigurationStatusView,
} from '@/domain/configuration';
import type { ResourceStatus } from '@/domain/common';

export async function getConfigurationStatus(): Promise<ConfigurationStatusView> {
  const [gateways, routes, upstreams, certificates, rateLimitPolicies, accessControlPolicies] = await Promise.all([
    listGateways(),
    listRoutes(),
    listUpstreams(),
    listCertificates(),
    listRateLimitPolicies(),
    listAccessControlPolicies(),
  ]);

  const items = [
    ...gateways.gateways.map((resource) => configurationItem(resource, 'Gateway', '/gateways')),
    ...routes.routes.map((resource) => configurationItem(resource, 'Route', '/routes')),
    ...upstreams.upstreams.map((resource) => configurationItem(resource, 'Upstream', '/services')),
    ...certificates.certificates.map((resource) => configurationItem(resource, 'Certificate', '/certificates')),
    ...rateLimitPolicies.map((resource) => configurationPolicyItem(resource, 'RateLimitPolicy')),
    ...accessControlPolicies.map((resource) => configurationPolicyItem(resource, 'AccessControlPolicy')),
  ];

  return { items: items.sort(compareConfigurationItems) };
}

function configurationPolicyItem(
  resource: { id: string; name: string; status: ResourceStatus; targets: Array<{ status?: ResourceStatus }> },
  kind: 'RateLimitPolicy' | 'AccessControlPolicy',
): ConfigurationStatusItem {
  const targetStatuses = resource.targets.flatMap((target) => target.status ? [target.status] : []);
  const status = [resource.status, ...targetStatuses].sort(compareResourceStatus)[0];
  return configurationItem({ ...resource, status }, kind, '/policies');
}

function configurationItem(
  resource: { id: string; name: string; status: ResourceStatus },
  kind: ConfigurationResourceKind,
  href: string,
): ConfigurationStatusItem {
  return {
    id: resource.id,
    name: resource.name || resource.id,
    kind,
    status: resource.status,
    href,
  };
}

function compareConfigurationItems(left: ConfigurationStatusItem, right: ConfigurationStatusItem) {
  return compareResourceStatus(left.status, right.status)
    || left.kind.localeCompare(right.kind)
    || left.name.localeCompare(right.name);
}

function compareResourceStatus(left: ResourceStatus, right: ResourceStatus) {
  const priority = { Error: 0, Pending: 1, Ready: 2, Disabled: 3 };
  return priority[left.state] - priority[right.state];
}
