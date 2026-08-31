import { useState } from "react";
import type {
  Gateway,
  GatewayRoute,
  IdentitySource,
  RouteForwarding,
  Service,
  TrafficType,
} from "../../data";
import {
  gatewayDomains,
  routesConflict,
} from "./gateway-model";
import {
  modelMappingsFromRoute,
  newModelMapping,
  normalizeModelMapping,
  type ModelMappingDraft,
  validHostname,
} from "./route-model";

export interface RouteFormProps {
  initial?: GatewayRoute;
  routes: GatewayRoute[];
  gateways: Gateway[];
  services: Service[];
  identitySources: IdentitySource[];
  preferredServiceID?: string;
  onClose: () => void;
  onSave: (route: GatewayRoute) => void;
}

export type RouteFormState = ReturnType<typeof useRouteForm>;

export function useRouteForm({
  initial,
  routes,
  gateways,
  services,
  identitySources,
  preferredServiceID,
  onSave,
}: RouteFormProps) {
  const preferredService = services.find(
    (service) => service.id === preferredServiceID,
  );
  const preferredType: TrafficType =
    preferredService?.type === "MODEL"
      ? "AI"
      : preferredService?.type === "MCP"
        ? "MCP"
        : "API";
  const [type, setType] = useState<TrafficType>(initial?.type ?? preferredType);
  const [name, setName] = useState(initial?.name ?? "新路由");
  const [gatewayID, setGatewayID] = useState(
    initial?.gatewayID ?? gateways[0]?.id ?? "",
  );
  const [host, setHost] = useState(
    initial?.host ??
      (gateways[0] ? (gatewayDomains(gateways[0])[0] ?? "") : ""),
  );
  const [path, setPath] = useState(initial?.path ?? "/new");
  const [method, setMethod] = useState(
    initial?.type === "API" ? (initial.match.split(" ")[0] ?? "ANY") : "ANY",
  );
  const [matchMode, setMatchMode] = useState<"精确匹配" | "前缀匹配">(
    initial?.type === "API" && !initial.match.endsWith("/*")
      ? "精确匹配"
      : "前缀匹配",
  );
  const [accessMode, setAccessMode] = useState<GatewayRoute["accessMode"]>(
    initial?.accessMode ?? "API Key",
  );
  const [identitySourceID, setIdentitySourceID] = useState(
    initial?.identitySourceID ?? identitySources[0]?.id ?? "",
  );
  const [serviceID, setServiceID] = useState(
    initial?.targets[0]?.serviceID ??
      (preferredService?.type === "HTTP" || preferredService?.type === "MCP"
        ? preferredService.id
        : ""),
  );
  const [selectedTools, setSelectedTools] = useState<string[]>(
    initial?.type === "MCP" ? initial.published : [],
  );
  const [toolQuery, setToolQuery] = useState("");
  const [secondLineEnabled, setSecondLineEnabled] = useState(
    Boolean(initial && initial.type !== "AI" && initial.targets.length > 1),
  );
  const [secondServiceID, setSecondServiceID] = useState(
    initial?.targets[1]?.serviceID ?? "",
  );
  const [strategy, setStrategy] = useState<"主备切换" | "权重分流">(
    initial?.forwarding.strategy === "权重分流" ? "权重分流" : "主备切换",
  );
  const [primaryWeight, setPrimaryWeight] = useState(
    initial?.targets[0]?.weight ?? 80,
  );
  const [modelMappings, setModelMappings] = useState<ModelMappingDraft[]>(
    initial?.type === "AI"
      ? modelMappingsFromRoute(initial)
      : [newModelMapping(services, preferredServiceID)],
  );
  const [timeout, setTimeout] = useState(
    initial?.forwarding.timeout.replace(/\D/g, "") || "30",
  );
  const [retries, setRetries] = useState(
    String(initial?.forwarding.retries ?? 0),
  );
  const [pathHandling, setPathHandling] = useState(
    initial?.forwarding.pathHandling ?? "保持原路径",
  );
  const [hostRewrite, setHostRewrite] = useState<RouteForwarding["hostRewrite"]>(
    initial?.forwarding.hostRewrite ?? "使用服务地址",
  );
  const [customHostname, setCustomHostname] = useState(
    initial?.forwarding.customHostname ?? "",
  );
  const [conditionKind, setConditionKind] = useState<"Header" | "Query">(
    initial?.conditions?.[0]?.kind ?? "Header",
  );
  const [conditionName, setConditionName] = useState(
    initial?.conditions?.[0]?.name ?? "",
  );
  const [conditionValue, setConditionValue] = useState(
    initial?.conditions?.[0]?.value ?? "",
  );
  const [conditionMode, setConditionMode] = useState<"精确匹配" | "存在">(
    initial?.conditions?.[0]?.mode ?? "精确匹配",
  );
  const [rewritePath, setRewritePath] = useState(
    initial?.rewrite?.pathPrefix ?? "",
  );
  const [addHeaderName, setAddHeaderName] = useState(
    initial?.rewrite?.requestHeaders[0]?.name ?? "",
  );
  const [addHeaderValue, setAddHeaderValue] = useState(
    initial?.rewrite?.requestHeaders[0]?.value ?? "",
  );
  const [removeHeaders, setRemoveHeaders] = useState(
    initial?.rewrite?.removeHeaders.join(", ") ?? "",
  );
  const [failoverOn, setFailoverOn] = useState<string[]>(
    initial?.forwarding.failoverOn ?? ["连接失败", "超时", "HTTP 5xx"],
  );
  const gateway = gateways.find((item) => item.id === gatewayID) ?? gateways[0];
  const effectiveHost =
    gateway && gatewayDomains(gateway).includes(host)
      ? host
      : gateway
        ? (gatewayDomains(gateway)[0] ?? "")
        : "";
  const availableServices = services.filter(
    (service) => service.state === "healthy" || service.state === "warning",
  );
  const compatible = availableServices.filter(
    (service) => service.type === (type === "API" ? "HTTP" : "MCP"),
  );
  const effectiveServiceID = compatible.some(
    (service) => service.id === serviceID,
  )
    ? serviceID
    : (compatible[0]?.id ?? "");
  const primaryService = compatible.find(
    (service) => service.id === effectiveServiceID,
  );
  const effectiveTools = selectedTools.filter((tool) =>
    primaryService?.capabilities.includes(tool),
  );
  const visibleTools = (primaryService?.capabilities ?? []).filter((tool) =>
    tool.toLowerCase().includes(toolQuery.toLowerCase()),
  );
  const secondCandidates = compatible.filter(
    (service) => service.id !== effectiveServiceID,
  );
  const effectiveSecondServiceID = secondCandidates.some(
    (service) => service.id === secondServiceID,
  )
    ? secondServiceID
    : (secondCandidates[0]?.id ?? "");
  const secondService = secondCandidates.find(
    (service) => service.id === effectiveSecondServiceID,
  );
  const normalizedMappings = modelMappings.map((mapping) =>
    normalizeModelMapping(mapping, availableServices),
  );
  const duplicateModelNames =
    new Set(normalizedMappings.map((mapping) => mapping.published.trim()))
      .size !== normalizedMappings.length;
  const modelMappingsComplete =
    normalizedMappings.length > 0 &&
    normalizedMappings.every(
      (mapping) =>
        mapping.published.trim() &&
        mapping.primaryServiceID &&
        mapping.primaryModel &&
        (!mapping.backupEnabled ||
          (mapping.backupServiceID && mapping.backupModel)),
    );
  const routeConflict =
    type === "API"
      ? routesConflict(
          routes,
          initial?.id,
          gateway?.id ?? "",
          effectiveHost,
          path,
          method,
          matchMode,
        )
      : undefined;
  const routeReady =
    Boolean(gateway) &&
    !routeConflict &&
    (type !== "API" ||
      hostRewrite !== "自定义主机名" ||
      validHostname(customHostname)) &&
    (type === "AI"
      ? modelMappingsComplete && !duplicateModelNames
      : Boolean(primaryService) &&
        (type !== "MCP" || effectiveTools.length > 0));
  const changeType = (next: TrafficType) => {
    setType(next);
    setServiceID("");
    setSelectedTools([]);
    setSecondLineEnabled(false);
    setSecondServiceID("");
    setPath(next === "API" ? "/new" : next === "AI" ? "/v1" : "/mcp");
    setTimeout(next === "AI" ? "120" : "30");
    setRetries("0");
    if (next === "AI") setModelMappings([newModelMapping(services)]);
  };
  const updateMapping = (id: string, changes: Partial<ModelMappingDraft>) =>
    setModelMappings((items) =>
      items.map((item) =>
        item.id === id
          ? {
              ...item,
              ...changes,
            }
          : item,
      ),
    );
  const save = () => {
    if (!routeReady || !gateway) return;
    const apiCapability = `${method} ${path}${matchMode === "前缀匹配" ? "/*" : ""}`;
    const published =
      type === "API"
        ? [apiCapability]
        : type === "MCP"
          ? effectiveTools
          : normalizedMappings.map((mapping) => mapping.published.trim());
    const targets: GatewayRoute["targets"] = [];
    if (type === "AI") {
      normalizedMappings.forEach((mapping) => {
        const primary = services.find(
          (service) => service.id === mapping.primaryServiceID,
        )!;
        targets.push({
          serviceID: primary.id,
          serviceName: primary.name,
          publishedCapability: mapping.published.trim(),
          detail: mapping.primaryModel,
          role: "主线路",
        });
        if (mapping.backupEnabled) {
          const backup = services.find(
            (service) => service.id === mapping.backupServiceID,
          )!;
          targets.push({
            serviceID: backup.id,
            serviceName: backup.name,
            publishedCapability: mapping.published.trim(),
            detail: mapping.backupModel,
            role: "备用线路",
          });
        }
      });
    } else if (primaryService) {
      targets.push({
        serviceID: primaryService.id,
        serviceName: primaryService.name,
        publishedCapability: published.join("、"),
        detail:
          type === "MCP"
            ? `${effectiveTools.length} 个开放工具`
            : `${primaryService.endpoints.length} 个端点`,
        role:
          secondLineEnabled && strategy === "权重分流" ? "加权线路" : "主线路",
        weight:
          secondLineEnabled && strategy === "权重分流"
            ? primaryWeight
            : undefined,
      });
      if (secondLineEnabled && secondService)
        targets.push({
          serviceID: secondService.id,
          serviceName: secondService.name,
          publishedCapability: published.join("、"),
          detail: `${secondService.endpoints.length} 个端点`,
          role: strategy === "权重分流" ? "加权线路" : "备用线路",
          weight: strategy === "权重分流" ? 100 - primaryWeight : undefined,
        });
    }
    const forwardingStrategy =
      type === "AI"
        ? normalizedMappings.some((mapping) => mapping.backupEnabled)
          ? "主备切换"
          : "单线路"
        : secondLineEnabled
          ? strategy
          : "单线路";
    onSave({
      id: initial?.id ?? `route-${Date.now()}`,
      name,
      type,
      gatewayID: gateway.id,
      gatewayName: gateway.name,
      host: effectiveHost,
      path,
      accessMode,
      identitySourceID:
        accessMode === "JWT 访问令牌" || accessMode === "浏览器登录"
          ? identitySourceID
          : undefined,
      match:
        type === "API"
          ? apiCapability
          : type === "AI"
            ? "OpenAI API · 请求体 model"
            : "Streamable HTTP · tools/call",
      published,
      targets,
      conditions:
        type === "API" && conditionName.trim()
          ? [
              {
                kind: conditionKind,
                name: conditionName.trim(),
                value: conditionValue.trim(),
                mode: conditionMode,
              },
            ]
          : undefined,
      rewrite:
        type === "API"
          ? {
              pathPrefix: rewritePath.trim() || undefined,
              requestHeaders:
                addHeaderName.trim() && addHeaderValue.trim()
                  ? [
                      {
                        name: addHeaderName.trim(),
                        value: addHeaderValue.trim(),
                      },
                    ]
                  : [],
              removeHeaders: removeHeaders
                .split(",")
                .map((header) => header.trim())
                .filter(Boolean),
            }
          : undefined,
      forwarding: {
        strategy: forwardingStrategy,
        timeout: `${timeout} 秒`,
        retries: Number(retries),
        pathHandling,
        hostRewrite,
        customHostname:
          type === "API" && hostRewrite === "自定义主机名"
            ? customHostname.trim().toLowerCase()
            : undefined,
        failoverOn:
          type === "AI" && forwardingStrategy === "主备切换"
            ? failoverOn
            : undefined,
        circuitBreaker:
          type === "API"
            ? {
                consecutiveFailures: 5,
                ejectionTime: "30 秒",
              }
            : undefined,
      },
      requests: initial?.requests ?? "0",
      successRate: initial?.successRate ?? "—",
      latency: initial?.latency ?? "—",
      state: initial?.state ?? "healthy",
    });
  };
  return {
    gateways,
    services,
    identitySources,
    type,
    setType,
    name,
    setName,
    gatewayID,
    setGatewayID,
    host,
    setHost,
    path,
    setPath,
    method,
    setMethod,
    matchMode,
    setMatchMode,
    accessMode,
    setAccessMode,
    identitySourceID,
    setIdentitySourceID,
    serviceID,
    setServiceID,
    selectedTools,
    setSelectedTools,
    toolQuery,
    setToolQuery,
    secondLineEnabled,
    setSecondLineEnabled,
    secondServiceID,
    setSecondServiceID,
    strategy,
    setStrategy,
    primaryWeight,
    setPrimaryWeight,
    modelMappings,
    setModelMappings,
    timeout,
    setTimeout,
    retries,
    setRetries,
    pathHandling,
    setPathHandling,
    hostRewrite,
    setHostRewrite,
    customHostname,
    setCustomHostname,
    conditionKind,
    setConditionKind,
    conditionName,
    setConditionName,
    conditionValue,
    setConditionValue,
    conditionMode,
    setConditionMode,
    rewritePath,
    setRewritePath,
    addHeaderName,
    setAddHeaderName,
    addHeaderValue,
    setAddHeaderValue,
    removeHeaders,
    setRemoveHeaders,
    failoverOn,
    setFailoverOn,
    gateway,
    effectiveHost,
    availableServices,
    compatible,
    effectiveServiceID,
    primaryService,
    effectiveTools,
    visibleTools,
    secondCandidates,
    effectiveSecondServiceID,
    secondService,
    normalizedMappings,
    duplicateModelNames,
    modelMappingsComplete,
    routeConflict,
    routeReady,
    changeType,
    updateMapping,
    save,
  };
 }
