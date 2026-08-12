import {
  createContext,
  useContext,
  useState,
  type ReactNode,
} from "react";
import {
  initialAuditRecords,
  initialCallers,
  initialCertificates,
  initialGateways,
  initialPolicies,
  initialRequests,
  initialRoutes,
  initialServices,
  type AuditRecord,
  type Caller,
  type CallerAccessKey,
  type CallerPermission,
  type CallerQuota,
  type Certificate,
  type Gateway,
  type GatewayRoute,
  type Policy,
  type RequestRecord,
  type Service,
} from "./data";

interface PrototypeState {
  gateways: Gateway[];
  routes: GatewayRoute[];
  services: Service[];
  certificates: Certificate[];
  callers: Caller[];
  policies: Policy[];
  requests: RequestRecord[];
  auditRecords: AuditRecord[];
  addGateway: (gateway: Gateway) => void;
  updateGateway: (gateway: Gateway) => void;
  deleteGateway: (gatewayID: string) => void;
  addRoute: (route: GatewayRoute) => void;
  updateRoute: (route: GatewayRoute) => void;
  deleteRoute: (routeID: string) => void;
  addService: (service: Service) => void;
  updateService: (service: Service) => void;
  deleteService: (serviceID: string) => void;
  verifyService: (serviceID: string) => void;
  updateServiceCredential: (
    serviceID: string,
    clientCertificateID?: string,
  ) => void;
  addCertificate: (certificate: Certificate) => void;
  updateCertificate: (certificate: Certificate) => void;
  deleteCertificate: (certificateID: string) => void;
  addCaller: (caller: Caller) => void;
  updateCallerPermissions: (
    callerID: string,
    permissions: CallerPermission[],
  ) => void;
  setCallerQuota: (callerID: string, quota: CallerQuota) => void;
  removeCallerQuota: (callerID: string, routeID: string) => void;
  issueCallerKey: (callerID: string, key: CallerAccessKey) => void;
  rotateCallerKey: (
    callerID: string,
    keyID: string,
    key: CallerAccessKey,
    graceUntil: string,
  ) => void;
  revokeCallerKey: (callerID: string, keyID: string) => void;
  addPolicy: (policy: Policy) => void;
  updatePolicy: (policy: Policy) => void;
  deletePolicy: (policyID: string) => void;
  resetDemo: () => void;
}

const PrototypeContext = createContext<PrototypeState | null>(null);

export function PrototypeProvider({ children }: { children: ReactNode }) {
  const [gateways, setGateways] = useState(initialGateways);
  const [routes, setRoutes] = useState(initialRoutes);
  const [services, setServices] = useState(initialServices);
  const [certificates, setCertificates] = useState(initialCertificates);
  const [callers, setCallers] = useState(initialCallers);
  const [policies, setPolicies] = useState(initialPolicies);
  const [requests, setRequests] = useState(initialRequests);
  const [auditRecords, setAuditRecords] = useState(initialAuditRecords);

  const recordAudit = (
    action: string,
    resourceType: AuditRecord["resourceType"],
    resource: string,
    detail: string,
  ) => {
    setAuditRecords((records) => [
      {
        id: crypto.randomUUID(),
        time: currentTime(),
        actor: "林工程师",
        action,
        resourceType,
        resource,
        detail,
        result: "成功",
      },
      ...records,
    ]);
  };

  const value: PrototypeState = {
    gateways,
    routes,
    services,
    certificates,
    callers,
    policies,
    requests,
    auditRecords,
    addGateway: (gateway) => {
      setGateways((items) => [...items, { ...gateway, configState: "active" }]);
      recordAudit(
        `创建网关“${gateway.name}”`,
        "网关",
        gateway.name,
        "网关配置已保存",
      );
    },
    updateGateway: (gateway) => {
      setGateways((items) =>
        items.map((item) =>
          item.id === gateway.id
            ? { ...gateway, configState: "active" }
            : item,
        ),
      );
      setRoutes((items) =>
        items.map((route) =>
          route.gatewayID === gateway.id
            ? { ...route, gatewayName: gateway.name }
            : route,
        ),
      );
      setPolicies((items) =>
        items.map((policy) => ({
          ...policy,
          targets: policy.targets.map((target) =>
            target.kind === "网关" && target.id === gateway.id
              ? { ...target, name: gateway.name }
              : target,
          ),
        })),
      );
      recordAudit(
        `更新网关“${gateway.name}”`,
        "网关",
        gateway.name,
        "网关配置已更新",
      );
    },
    deleteGateway: (gatewayID) => {
      const gateway = gateways.find((item) => item.id === gatewayID);
      if (!gateway) return;
      setGateways((items) => items.filter((item) => item.id !== gatewayID));
      recordAudit(
        `删除网关“${gateway.name}”`,
        "网关",
        gateway.name,
        "网关已从当前环境移除",
      );
    },
    addRoute: (route) => {
      setRoutes((items) => [...items, { ...route, configState: "active" }]);
      recordAudit(
        `创建路由“${route.name}”`,
        "路由",
        route.name,
        `${route.host}${route.path} 已保存`,
      );
    },
    updateRoute: (route) => {
      setRoutes((items) =>
        items.map((item) =>
          item.id === route.id ? { ...route, configState: "active" } : item,
        ),
      );
      setPolicies((items) =>
        items.map((policy) => ({
          ...policy,
          targets: policy.targets.map((target) =>
            target.kind === "路由" && target.id === route.id
              ? { ...target, name: route.name }
              : target,
          ),
        })),
      );
      recordAudit(
        `更新路由“${route.name}”`,
        "路由",
        route.name,
        `${route.host}${route.path} 已更新`,
      );
    },
    deleteRoute: (routeID) => {
      const route = routes.find((item) => item.id === routeID);
      if (!route) return;
      setRoutes((items) => items.filter((item) => item.id !== routeID));
      recordAudit(
        `删除路由“${route.name}”`,
        "路由",
        route.name,
        "路由已从当前环境移除",
      );
    },
    addService: (service) => {
      setServices((items) => [...items, { ...service, configState: "active" }]);
      recordAudit(
        `创建服务“${service.name}”`,
        "服务",
        service.name,
        `${service.type === "MODEL" ? "模型" : service.type} 服务连接配置已保存`,
      );
    },
    updateService: (service) => {
      setServices((items) =>
        items.map((item) =>
          item.id === service.id
            ? { ...service, configState: "active" }
            : item,
        ),
      );
      setRoutes((items) =>
        items.map((route) => ({
          ...route,
          targets: route.targets.map((target) =>
            target.serviceID === service.id
              ? { ...target, serviceName: service.name }
              : target,
          ),
        })),
      );
      recordAudit(
        `更新服务“${service.name}”`,
        "服务",
        service.name,
        "服务连接配置已保存",
      );
    },
    deleteService: (serviceID) => {
      const service = services.find((item) => item.id === serviceID);
      if (!service) return;
      setServices((items) => items.filter((item) => item.id !== serviceID));
      recordAudit(
        `删除服务“${service.name}”`,
        "服务",
        service.name,
        "服务连接配置已移除",
      );
    },
    verifyService: (serviceID) => {
      setServices((items) =>
        items.map((item) =>
          item.id === serviceID
            ? { ...item, verificationState: "verified" }
            : item,
        ),
      );
    },
    updateServiceCredential: (serviceID, clientCertificateID) => {
      const service = services.find((item) => item.id === serviceID);
      if (!service) return;
      setServices((items) =>
        items.map((item) =>
          item.id === serviceID
            ? {
                ...item,
                configState: "active",
                clientCertificateID:
                  clientCertificateID ?? item.clientCertificateID,
                credentialUpdatedAt: "刚刚",
              }
            : item,
        ),
      );
      recordAudit(
        `更新服务“${service.name}”凭据`,
        "服务",
        service.name,
        "新凭据已验证并提交切换",
      );
    },
    addCertificate: (certificate) => {
      setCertificates((items) => [
        ...items,
        { ...certificate, configState: "active" },
      ]);
      recordAudit(
        `导入证书“${certificate.name}”`,
        "证书",
        certificate.name,
        "证书已校验并提交配置",
      );
    },
    updateCertificate: (certificate) => {
      setCertificates((items) =>
        items.map((item) =>
          item.id === certificate.id
            ? { ...certificate, configState: "active" }
            : item,
        ),
      );
      recordAudit(
        `更新证书“${certificate.name}”`,
        "证书",
        certificate.name,
        "新证书已校验并提交替换",
      );
    },
    deleteCertificate: (certificateID) => {
      const certificate = certificates.find(
        (item) => item.id === certificateID,
      );
      if (!certificate) return;
      setCertificates((items) =>
        items.filter((item) => item.id !== certificateID),
      );
      recordAudit(
        `删除证书“${certificate.name}”`,
        "证书",
        certificate.name,
        "证书已从当前环境移除",
      );
    },
    addCaller: (caller) => {
      setCallers((items) => [...items, caller]);
      recordAudit(
        "创建调用方",
        "调用方",
        caller.name,
        caller.permissions.length
          ? `已授予 ${caller.permissions.length} 条路由权限`
          : "尚未授予路由权限",
      );
    },
    updateCallerPermissions: (callerID, permissions) => {
      setCallers((items) =>
        items.map((caller) => {
          if (caller.id !== callerID) return caller;
          const routeIDs = new Set(
            permissions.map((permission) => permission.routeID),
          );
          const quotas = caller.quotas.filter((quota) =>
            routeIDs.has(quota.routeID),
          );
          return { ...caller, permissions, quotas, state: callerState(quotas) };
        }),
      );
      recordAudit(
        `更新“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”权限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "路由访问权限已提交",
      );
    },
    setCallerQuota: (callerID, quota) => {
      setCallers((items) =>
        items.map((caller) => {
          if (caller.id !== callerID) return caller;
          const current = caller.quotas.find(
            (item) => item.routeID === quota.routeID,
          );
          const nextQuota = { ...quota, used: current?.used ?? quota.used };
          const quotas = current
            ? caller.quotas.map((item) =>
                item.routeID === quota.routeID ? nextQuota : item,
              )
            : [...caller.quotas, nextQuota];
          return { ...caller, quotas, state: callerState(quotas) };
        }),
      );
      recordAudit(
        `更新“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”用量上限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "累计用量上限已提交",
      );
    },
    removeCallerQuota: (callerID, routeID) => {
      setCallers((items) =>
        items.map((caller) => {
          if (caller.id !== callerID) return caller;
          const quotas = caller.quotas.filter(
            (quota) => quota.routeID !== routeID,
          );
          return { ...caller, quotas, state: callerState(quotas) };
        }),
      );
      recordAudit(
        `移除“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”用量上限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "累计用量上限已移除",
      );
    },
    issueCallerKey: (callerID, key) => {
      setCallers((items) =>
        items.map((caller) =>
          caller.id === callerID
            ? { ...caller, keys: [...caller.keys, key] }
            : caller,
        ),
      );
      const caller = callers.find((item) => item.id === callerID);
      recordAudit(
        "签发密钥",
        "调用方",
        caller?.name ?? callerID,
        `签发“${key.name}”访问密钥，有效期至 ${key.expiresAt}`,
      );
    },
    rotateCallerKey: (callerID, keyID, key, graceUntil) => {
      const caller = callers.find((item) => item.id === callerID);
      const previousKey = caller?.keys.find((item) => item.id === keyID);
      setCallers((items) =>
        items.map((item) =>
          item.id === callerID
            ? {
                ...item,
                keys: [
                  ...item.keys.map((current) =>
                    current.id === keyID
                      ? {
                          ...current,
                          state: "warning" as const,
                          graceUntil,
                          replacedByID: key.id,
                        }
                      : current,
                  ),
                  { ...key, rotatedFromID: keyID },
                ],
              }
            : item,
        ),
      );
      recordAudit(
        "轮换密钥",
        "调用方",
        caller?.name ?? callerID,
        `以“${key.name}”替换“${previousKey?.name ?? keyID}”，旧密钥可用至 ${graceUntil}`,
      );
    },
    revokeCallerKey: (callerID, keyID) => {
      const caller = callers.find((item) => item.id === callerID);
      const key = caller?.keys.find((item) => item.id === keyID);
      setCallers((items) =>
        items.map((item) =>
          item.id === callerID
            ? {
                ...item,
                keys: item.keys.map((current) =>
                  current.id === keyID
                    ? { ...current, state: "disabled" }
                    : current,
                ),
              }
            : item,
        ),
      );
      recordAudit(
        "停用密钥",
        "调用方",
        caller?.name ?? callerID,
        `停用“${key?.name ?? keyID}”访问密钥`,
      );
    },
    addPolicy: (policy) => {
      setPolicies((items) => [
        ...items,
        {
          ...policy,
          configState: policy.targets.length ? "active" : "not-applied",
        },
      ]);
      if (!policy.targets.length) {
        recordAudit(
          "创建流量策略",
          "流量策略",
          policy.name,
          "策略已保存，尚未选择生效目标",
        );
        return;
      }
      recordAudit(
        `创建流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        `已选择 ${policy.targets.length} 个生效目标`,
      );
    },
    updatePolicy: (policy) => {
      setPolicies((items) =>
        items.map((item) =>
          item.id === policy.id
            ? {
                ...policy,
                configState: policy.targets.length ? "active" : "not-applied",
              }
            : item,
        ),
      );
      if (!policy.targets.length) {
        recordAudit(
          "更新流量策略",
          "流量策略",
          policy.name,
          "策略已保存，尚未选择生效目标",
        );
        return;
      }
      recordAudit(
        `更新流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        `已选择 ${policy.targets.length} 个生效目标`,
      );
    },
    deletePolicy: (policyID) => {
      const policy = policies.find((item) => item.id === policyID);
      if (!policy) return;
      setPolicies((items) => items.filter((item) => item.id !== policyID));
      recordAudit(
        `删除流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        "策略已从当前环境移除",
      );
    },
    resetDemo: () => {
      setGateways(initialGateways);
      setRoutes(initialRoutes);
      setServices(initialServices);
      setCertificates(initialCertificates);
      setCallers(initialCallers);
      setPolicies(initialPolicies);
      setRequests(initialRequests);
      setAuditRecords(initialAuditRecords);
    },
  };

  return (
    <PrototypeContext.Provider value={value}>
      {children}
    </PrototypeContext.Provider>
  );
}

function currentTime() {
  return new Date().toLocaleTimeString("zh-CN", { hour12: false });
}

export function usePrototype() {
  const context = useContext(PrototypeContext);
  if (!context) throw new Error("PrototypeProvider is required");
  return context;
}

function callerState(quotas: CallerQuota[]) {
  return quotas.some((quota) => quota.used >= quota.limit)
    ? ("warning" as const)
    : ("healthy" as const);
}
