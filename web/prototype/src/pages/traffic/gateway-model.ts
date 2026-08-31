import type {
  Certificate,
  Gateway,
  GatewayListener,
  GatewayRoute,
} from "../../data";

export function listenerLabel(listener: GatewayListener) {
  return `${listener.protocol} · ${listener.port}`;
}
export function gatewayDomains(gateway: Gateway) {
  return [
    ...new Set(
      gateway.listeners.flatMap((listener) =>
        listener.bindings.map((binding) => binding.domain),
      ),
    ),
  ];
}
export function gatewayCertificateNames(
  gateway: Gateway,
  certificates: Certificate[],
) {
  const ids = new Set(
    gateway.listeners.flatMap((listener) =>
      listener.bindings.map((binding) => binding.certificateID).filter(Boolean),
    ),
  );
  return certificates
    .filter((certificate) => ids.has(certificate.id))
    .map((certificate) => certificate.name);
}
export function routesConflict(
  routes: GatewayRoute[],
  routeID: string | undefined,
  gatewayID: string,
  host: string,
  path: string,
  method: string,
  matchMode: "精确匹配" | "前缀匹配",
) {
  const normalized = path.replace(/\/$/, "") || "/";
  return routes.find((route) => {
    if (
      route.id === routeID ||
      route.type !== "API" ||
      route.gatewayID !== gatewayID ||
      route.host !== host
    )
      return false;
    const currentMethod = route.match.split(" ")[0];
    if (method !== "ANY" && currentMethod !== "ANY" && method !== currentMethod)
      return false;
    const currentPath = route.path.replace(/\/$/, "") || "/";
    const currentPrefix = route.match.endsWith("/*");
    return (
      currentPath === normalized ||
      (currentPrefix && normalized.startsWith(`${currentPath}/`)) ||
      (matchMode === "前缀匹配" && currentPath.startsWith(`${normalized}/`))
    );
  });
}
export function certificateCoversDomain(
  certificateDomain: string,
  requestedDomain: string,
) {
  if (!requestedDomain) return false;
  if (certificateDomain === requestedDomain) return true;
  if (!certificateDomain.startsWith("*.")) return false;
  const suffix = certificateDomain.slice(1);
  return (
    requestedDomain.endsWith(suffix) &&
    requestedDomain.slice(0, -suffix.length).length > 0 &&
    !requestedDomain.slice(0, -suffix.length).includes(".")
  );
}
