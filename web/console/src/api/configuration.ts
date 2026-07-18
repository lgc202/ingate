import { listCertificates } from './certificates';
import { listGateways } from './gateways';
import {
  listAccessControlPolicies,
  listPolicyBindings,
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
  const [gateways, routes, upstreams, certificates, rateLimitPolicies, accessControlPolicies, policyBindings] = await Promise.all([
    listGateways(),
    listRoutes(),
    listUpstreams(),
    listCertificates(),
    listRateLimitPolicies(),
    listAccessControlPolicies(),
    listPolicyBindings(),
  ]);

  const items = [
    ...gateways.gateways.map((resource) => configurationItem(resource, 'Gateway', '/gateways')),
    ...routes.routes.map((resource) => configurationItem(resource, 'Route', '/routes')),
    ...upstreams.upstreams.map((resource) => configurationItem(resource, 'Upstream', '/services')),
    ...certificates.certificates.map((resource) => configurationItem(resource, 'Certificate', '/certificates')),
    ...rateLimitPolicies.map((resource) => configurationItem(resource, 'RateLimitPolicy', '/policies')),
    ...accessControlPolicies.map((resource) => configurationItem(resource, 'AccessControlPolicy', '/policies')),
    ...policyBindings.map((resource) => configurationItem(resource, 'PolicyBinding', '/policies?tab=bindings')),
  ];

  return { items: items.sort(compareConfigurationItems) };
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
  const priority = { Error: 0, Pending: 1, Ready: 2, Disabled: 3 };
  return priority[left.status.state] - priority[right.status.state]
    || left.kind.localeCompare(right.kind)
    || left.name.localeCompare(right.name);
}
