import {
  createContext,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  initialAuditRecords,
  initialCallers,
  initialCertificates,
  initialGateways,
  initialPolicies,
  initialProxyInstances,
  initialReleaseHistory,
  initialRequests,
  initialRoutes,
  initialServices,
  type AuditRecord,
  type Caller,
  type CallerAccessKey,
  type CallerPermission,
  type CallerQuota,
  type Certificate,
  type ConfigState,
  type Gateway,
  type GatewayRoute,
  type Policy,
  type ProxyInstance,
  type RequestRecord,
  type ReleaseRecord,
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
  proxyInstances: ProxyInstance[];
  currentVersion: number;
  candidateVersion?: number;
  releaseHistory: ReleaseRecord[];
  auditRecords: AuditRecord[];
  addGateway: (gateway: Gateway) => void;
  updateGateway: (gateway: Gateway) => void;
  deleteGateway: (gatewayID: string) => void;
  addRoute: (route: GatewayRoute) => void;
  addRoutes: (routes: GatewayRoute[]) => void;
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
  retryRelease: (version: number) => void;
  simulateReleaseFailure: boolean;
  setSimulateReleaseFailure: (enabled: boolean) => void;
  reviewServiceChanges: (serviceID: string) => void;
  sendDemoRequest: (caller: Caller, route: GatewayRoute) => string;
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
  const [proxyInstances, setProxyInstances] = useState(initialProxyInstances);
  const [currentVersion, setCurrentVersion] = useState(142);
  const [candidateVersion, setCandidateVersion] = useState<number>();
  const [simulateReleaseFailure, setSimulateReleaseFailure] = useState(false);
  const [releaseHistory, setReleaseHistory] = useState(initialReleaseHistory);
  const [auditRecords, setAuditRecords] = useState(initialAuditRecords);
  const nextVersion = useRef(143);

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

  const publish = (
    summary: string,
    resourceType: AuditRecord["resourceType"],
    resource: string,
    detail: string,
    complete: () => void,
  ) => {
    const version = nextVersion.current++;
    setCandidateVersion(version);
    setReleaseHistory((records) => [
      {
        version,
        time: `今天 ${currentTime()}`,
        summary,
        resources: "1 项资源",
        state: "发布中",
        syncedInstances: 0,
        totalInstances: 5,
        changes: [detail],
      },
      ...records,
    ]);
    recordAudit(summary, resourceType, resource, detail);
    window.setTimeout(() => {
      if (simulateReleaseFailure) {
        setCandidateVersion(undefined);
        setGateways((items) => items.map(markPublishingAsFailed));
        setRoutes((items) => items.map(markPublishingAsFailed));
        setServices((items) => items.map(markPublishingAsFailed));
        setCertificates((items) => items.map(markPublishingAsFailed));
        setPolicies((items) => items.map(markPublishingAsFailed));
        setProxyInstances((instances) =>
          instances.map((instance, index) =>
            index < 3
              ? { ...instance, activeConfigVersion: version }
              : instance,
          ),
        );
        setReleaseHistory((records) =>
          records.map((record) =>
            record.version === version
              ? {
                  ...record,
                  state: "发布失败",
                  syncedInstances: 3,
                  error:
                    `3 个代理实例已应用 v${version}，2 个实例在 30 秒内未确认`,
                }
              : record,
          ),
        );
        setAuditRecords((records) => [
          {
            id: crypto.randomUUID(),
            time: currentTime(),
            actor: "系统",
            action: "发布配置",
            resourceType: "配置发布",
            resource: `版本 ${version}`,
            detail: `3 个实例已应用 v${version}，2 个实例仍使用上一版本`,
            result: "失败",
          },
          ...records,
        ]);
        return;
      }
      complete();
      setCurrentVersion(version);
      setCandidateVersion(undefined);
      setReleaseHistory((records) =>
        records.map((record) =>
          record.version === version
            ? {
                ...record,
                state: "已生效",
                syncedInstances: record.totalInstances,
                error: undefined,
              }
            : record,
        ),
      );
      setProxyInstances((instances) =>
        instances.map((instance) => ({
          ...instance,
          activeConfigVersion: version,
        })),
      );
    }, 900);
  };

  const value: PrototypeState = {
    gateways,
    routes,
    services,
    certificates,
    callers,
    policies,
    requests,
    proxyInstances,
    currentVersion,
    candidateVersion,
    releaseHistory,
    auditRecords,
    addGateway: (gateway) => {
      setGateways((items) => [
        ...items,
        { ...gateway, configState: "publishing" },
      ]);
      publish(
        `创建网关“${gateway.name}”`,
        "网关",
        gateway.name,
        "网关配置已提交，正在同步到代理实例",
        () =>
          setGateways((items) =>
            items.map((item) =>
              item.id === gateway.id
                ? { ...item, configState: "active" }
                : item,
            ),
          ),
      );
    },
    updateGateway: (gateway) => {
      setGateways((items) =>
        items.map((item) =>
          item.id === gateway.id
            ? { ...gateway, configState: "publishing" }
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
      publish(
        `更新网关“${gateway.name}”`,
        "网关",
        gateway.name,
        "网关配置已提交，正在同步到代理实例",
        () =>
          setGateways((items) =>
            items.map((item) =>
              item.id === gateway.id
                ? { ...item, configState: "active" }
                : item,
            ),
          ),
      );
    },
    deleteGateway: (gatewayID) => {
      const gateway = gateways.find((item) => item.id === gatewayID);
      if (!gateway) return;
      setGateways((items) => items.filter((item) => item.id !== gatewayID));
      publish(
        `删除网关“${gateway.name}”`,
        "网关",
        gateway.name,
        "网关已从当前环境移除",
        () => undefined,
      );
    },
    addRoute: (route) => {
      setRoutes((items) => [...items, { ...route, configState: "publishing" }]);
      publish(
        `创建路由“${route.name}”`,
        "路由",
        route.name,
        `${route.host}${route.path} 已提交发布`,
        () =>
          setRoutes((items) =>
            items.map((item) =>
              item.id === route.id ? { ...item, configState: "active" } : item,
            ),
          ),
      );
    },
    addRoutes: (newRoutes) => {
      if (!newRoutes.length) return;
      const routeIDs = new Set(newRoutes.map((route) => route.id));
      setRoutes((items) => [
        ...items,
        ...newRoutes.map((route) => ({
          ...route,
          configState: "publishing" as const,
        })),
      ]);
      publish(
        `导入 ${newRoutes.length} 条 API 路由`,
        "路由",
        newRoutes.map((route) => route.name).join("、"),
        `OpenAPI 定义已生成 ${newRoutes.length} 条路由并作为同一版本发布`,
        () =>
          setRoutes((items) =>
            items.map((item) =>
              routeIDs.has(item.id) ? { ...item, configState: "active" } : item,
            ),
          ),
      );
    },
    updateRoute: (route) => {
      setRoutes((items) =>
        items.map((item) =>
          item.id === route.id ? { ...route, configState: "publishing" } : item,
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
      publish(
        `更新路由“${route.name}”`,
        "路由",
        route.name,
        `${route.host}${route.path} 已提交发布`,
        () =>
          setRoutes((items) =>
            items.map((item) =>
              item.id === route.id ? { ...item, configState: "active" } : item,
            ),
          ),
      );
    },
    deleteRoute: (routeID) => {
      const route = routes.find((item) => item.id === routeID);
      if (!route) return;
      setRoutes((items) => items.filter((item) => item.id !== routeID));
      publish(
        `删除路由“${route.name}”`,
        "路由",
        route.name,
        "路由已从当前环境移除",
        () => undefined,
      );
    },
    addService: (service) => {
      setServices((items) => [
        ...items,
        { ...service, configState: "publishing" },
      ]);
      publish(
        `创建服务“${service.name}”`,
        "服务",
        service.name,
        `${service.type === "MODEL" ? "模型" : service.type} 服务连接配置已保存`,
        () =>
          setServices((items) =>
            items.map((item) =>
              item.id === service.id
                ? { ...item, configState: "active" }
                : item,
            ),
          ),
      );
    },
    updateService: (service) => {
      setServices((items) =>
        items.map((item) =>
          item.id === service.id
            ? { ...service, configState: "publishing" }
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
      publish(
        `更新服务“${service.name}”`,
        "服务",
        service.name,
        "服务连接配置已保存",
        () =>
          setServices((items) =>
            items.map((item) =>
              item.id === service.id
                ? { ...item, configState: "active" }
                : item,
            ),
          ),
      );
    },
    deleteService: (serviceID) => {
      const service = services.find((item) => item.id === serviceID);
      if (!service) return;
      setServices((items) => items.filter((item) => item.id !== serviceID));
      publish(
        `删除服务“${service.name}”`,
        "服务",
        service.name,
        "服务连接配置已移除",
        () => undefined,
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
                configState: "publishing",
                clientCertificateID:
                  clientCertificateID ?? item.clientCertificateID,
                credentialUpdatedAt: "刚刚",
              }
            : item,
        ),
      );
      publish(
        `更新服务“${service.name}”凭据`,
        "服务",
        service.name,
        "新凭据已验证并提交切换",
        () =>
          setServices((items) =>
            items.map((item) =>
              item.id === serviceID ? { ...item, configState: "active" } : item,
            ),
          ),
      );
    },
    addCertificate: (certificate) => {
      setCertificates((items) => [
        ...items,
        { ...certificate, configState: "publishing" },
      ]);
      publish(
        `导入证书“${certificate.name}”`,
        "证书",
        certificate.name,
        "证书已校验并提交配置",
        () =>
          setCertificates((items) =>
            items.map((item) =>
              item.id === certificate.id
                ? { ...item, configState: "active" }
                : item,
            ),
          ),
      );
    },
    updateCertificate: (certificate) => {
      setCertificates((items) =>
        items.map((item) =>
          item.id === certificate.id
            ? { ...certificate, configState: "publishing" }
            : item,
        ),
      );
      publish(
        `更新证书“${certificate.name}”`,
        "证书",
        certificate.name,
        "新证书已校验并提交替换",
        () =>
          setCertificates((items) =>
            items.map((item) =>
              item.id === certificate.id
                ? { ...item, configState: "active" }
                : item,
            ),
          ),
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
      publish(
        `删除证书“${certificate.name}”`,
        "证书",
        certificate.name,
        "证书已从当前环境移除",
        () => undefined,
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
      publish(
        `更新“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”权限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "路由访问权限已提交",
        () => undefined,
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
      publish(
        `更新“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”用量上限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "累计用量上限已提交",
        () => undefined,
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
      publish(
        `移除“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”用量上限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "累计用量上限已移除",
        () => undefined,
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
          configState: policy.targets.length ? "publishing" : "not-applied",
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
      publish(
        `创建流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        `已选择 ${policy.targets.length} 个生效目标`,
        () =>
          setPolicies((items) =>
            items.map((item) =>
              item.id === policy.id ? { ...item, configState: "active" } : item,
            ),
          ),
      );
    },
    updatePolicy: (policy) => {
      setPolicies((items) =>
        items.map((item) =>
          item.id === policy.id
            ? {
                ...policy,
                configState: policy.targets.length
                  ? "publishing"
                  : "not-applied",
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
      publish(
        `更新流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        `已选择 ${policy.targets.length} 个生效目标`,
        () =>
          setPolicies((items) =>
            items.map((item) =>
              item.id === policy.id ? { ...item, configState: "active" } : item,
            ),
          ),
      );
    },
    deletePolicy: (policyID) => {
      const policy = policies.find((item) => item.id === policyID);
      if (!policy) return;
      setPolicies((items) => items.filter((item) => item.id !== policyID));
      publish(
        `删除流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        "策略已从当前环境移除",
        () => undefined,
      );
    },
    retryRelease: (version) => {
      const failed = releaseHistory.find(
        (release) =>
          release.version === version && release.state === "发布失败",
      );
      if (!failed) return;
      const retryVersion = nextVersion.current++;
      setCandidateVersion(retryVersion);
      setReleaseHistory((records) => [
        {
          ...failed,
          version: retryVersion,
          time: `今天 ${currentTime()}`,
          summary: `重试：${failed.summary}`,
          state: "发布中",
          syncedInstances: 0,
          error: undefined,
        },
        ...records,
      ]);
      window.setTimeout(() => {
        setCurrentVersion(retryVersion);
        setCandidateVersion(undefined);
        setReleaseHistory((records) =>
          records.map((record) =>
            record.version === retryVersion
              ? {
                  ...record,
                  state: "已生效",
                  syncedInstances: record.totalInstances,
                }
              : record,
          ),
        );
        setProxyInstances((instances) =>
          instances.map((instance) => ({
            ...instance,
            activeConfigVersion: retryVersion,
          })),
        );
        setGateways((items) => items.map(markFailedAsActive));
        setRoutes((items) => items.map(markFailedAsActive));
        setServices((items) => items.map(markFailedAsActive));
        setCertificates((items) => items.map(markFailedAsActive));
        setPolicies((items) => items.map(markFailedAsActive));
        setAuditRecords((records) => [
          {
            id: crypto.randomUUID(),
            time: currentTime(),
            actor: "林工程师",
            action: "重试发布",
            resourceType: "配置发布",
            resource: `版本 ${retryVersion}`,
            detail: `基于失败版本 ${version} 重新生成配置，全部代理实例已确认`,
            result: "成功",
          },
          ...records,
        ]);
      }, 900);
    },
    simulateReleaseFailure,
    setSimulateReleaseFailure,
    reviewServiceChanges: (serviceID) =>
      setServices((items) =>
        items.map((service) =>
          service.id === serviceID && service.capabilityChanges
            ? {
                ...service,
                capabilityChanges: {
                  ...service.capabilityChanges,
                  reviewed: true,
                },
              }
            : service,
        ),
      ),
    sendDemoRequest: (caller, route) => {
      const id = `req_demo_${Date.now().toString().slice(-6)}`;
      const target = route.targets[0];
      const capability =
        caller.permissions.find((permission) => permission.routeID === route.id)
          ?.scopes[0] ?? route.published[0];
      const request =
        route.type === "API"
          ? capability
          : route.type === "AI"
            ? `${capability} · chat/completions`
            : `tools/call · ${capability}`;
      setRequests((items) => [
        {
          id,
          time: currentTime(),
          type: route.type,
          caller: caller.name,
          route: route.name,
          request,
          target: target
            ? `${target.serviceName}${route.type === "AI" ? ` / ${target.detail}` : ""}`
            : "—",
          result: "成功",
          code: "200",
          latency:
            route.type === "AI"
              ? "TTFT 540 ms · 总计 1.4 s"
              : route.type === "MCP"
                ? "214 ms"
                : "74 ms",
          usage:
            route.type === "AI"
              ? "126 Token"
              : route.type === "MCP"
                ? "1 次工具调用"
                : "2.1 KB",
          cost: route.type === "AI" ? "¥0.01" : "—",
          attempts: target
            ? [
                {
                  service: target.serviceName,
                  actualModel: route.type === "AI" ? target.detail : undefined,
                  result: "成功" as const,
                  code: "200",
                  latency: route.type === "AI" ? "1.39 s" : "71 ms",
                  ttft: route.type === "AI" ? "540 ms" : undefined,
                  inputTokens: route.type === "AI" ? 84 : undefined,
                  outputTokens: route.type === "AI" ? 42 : undefined,
                  cachedTokens: route.type === "AI" ? 16 : undefined,
                  cost: route.type === "AI" ? "¥0.01" : undefined,
                  state: "healthy" as const,
                },
              ]
            : [],
          steps: [
            {
              name: "调用方认证",
              detail: `${caller.name} · 演示密钥`,
              duration: "2 ms",
              state: "healthy",
            },
            {
              name: route.name,
              detail: `${request} → ${target?.serviceName ?? "目标服务"}`,
              duration: "1 ms",
              state: "healthy",
            },
            {
              name: target?.serviceName ?? "目标服务",
              detail: "HTTP 200 · 演示请求成功",
              duration: route.type === "AI" ? "1.39 s" : "71 ms",
              state: "healthy",
            },
          ],
        },
        ...items,
      ]);
      return id;
    },
    resetDemo: () => {
      setGateways(initialGateways);
      setRoutes(initialRoutes);
      setServices(initialServices);
      setCertificates(initialCertificates);
      setCallers(initialCallers);
      setPolicies(initialPolicies);
      setRequests(initialRequests);
      setCurrentVersion(142);
      setCandidateVersion(undefined);
      setSimulateReleaseFailure(false);
      setReleaseHistory(initialReleaseHistory);
      setProxyInstances(initialProxyInstances);
      setAuditRecords(initialAuditRecords);
      nextVersion.current = 143;
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

function markPublishingAsFailed<T extends { configState?: ConfigState }>(
  resource: T,
): T {
  return resource.configState === "publishing"
    ? { ...resource, configState: "failed" }
    : resource;
}

function markFailedAsActive<T extends { configState?: ConfigState }>(
  resource: T,
): T {
  return resource.configState === "failed"
    ? { ...resource, configState: "active" }
    : resource;
}
