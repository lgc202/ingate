import type {
  GatewayRoute,
  RouteForwarding,
  Service,
} from "../../data";

export interface ModelMappingDraft {
  id: string;
  published: string;
  primaryServiceID: string;
  primaryModel: string;
  backupEnabled: boolean;
  backupServiceID: string;
  backupModel: string;
}
export function hostRewriteLabel(forwarding: RouteForwarding) {
  if (forwarding.hostRewrite === "自定义主机名") {
    return forwarding.customHostname || "未填写";
  }
  return forwarding.hostRewrite;
}

export function validHostname(value: string) {
  const hostname = value.trim();
  return (
    hostname.length > 0 &&
    hostname.length <= 253 &&
    hostname
      .split(".")
      .every((label) =>
        /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label),
      )
  );
}

export function newModelMapping(
  services: Service[],
  preferredServiceID?: string,
): ModelMappingDraft {
  const primary =
    services.find(
      (service) =>
        service.id === preferredServiceID && service.type === "MODEL",
    ) ?? services.find((service) => service.type === "MODEL");
  return {
    id: crypto.randomUUID(),
    published: "",
    primaryServiceID: primary?.id ?? "",
    primaryModel: primary?.capabilities[0] ?? "",
    backupEnabled: false,
    backupServiceID: "",
    backupModel: "",
  };
}
export function modelMappingsFromRoute(route: GatewayRoute): ModelMappingDraft[] {
  return route.published.map((published) => {
    const targets = route.targets.filter(
      (target) => target.publishedCapability === published,
    );
    const primary =
      targets.find((target) => target.role === "主线路") ?? targets[0];
    const backup = targets.find((target) => target.role === "备用线路");
    return {
      id: crypto.randomUUID(),
      published,
      primaryServiceID: primary?.serviceID ?? "",
      primaryModel: primary?.detail ?? "",
      backupEnabled: Boolean(backup),
      backupServiceID: backup?.serviceID ?? "",
      backupModel: backup?.detail ?? "",
    };
  });
}
export function normalizeModelMapping(
  mapping: ModelMappingDraft,
  services: Service[],
) {
  const modelServices = services.filter(
    (service) =>
      service.type === "MODEL" &&
      (service.state === "healthy" || service.state === "warning"),
  );
  const primary =
    modelServices.find((service) => service.id === mapping.primaryServiceID) ??
    modelServices[0];
  const primaryModel = primary?.capabilities.includes(mapping.primaryModel)
    ? mapping.primaryModel
    : (primary?.capabilities[0] ?? "");
  const backupCandidates = modelServices.filter(
    (service) => service.id !== primary?.id,
  );
  const backup = backupCandidates.find(
    (service) => service.id === mapping.backupServiceID,
  );
  const backupModel = backup?.capabilities.includes(mapping.backupModel)
    ? mapping.backupModel
    : (backup?.capabilities[0] ?? "");
  return {
    ...mapping,
    primaryServiceID: primary?.id ?? "",
    primaryModel,
    backupServiceID: mapping.backupEnabled ? (backup?.id ?? "") : "",
    backupModel: mapping.backupEnabled ? backupModel : "",
  };
}
