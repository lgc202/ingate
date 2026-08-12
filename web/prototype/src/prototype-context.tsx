import { createContext, useContext, useRef, useState, type ReactNode } from 'react';
import {
  initialAuditRecords,
  initialCallers,
  initialCertificates,
  initialGateways,
  initialPolicies,
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
  type Gateway,
  type GatewayRoute,
  type Policy,
  type RequestRecord,
  type ReleaseRecord,
  type Service,
} from './data';

interface PrototypeState {
  gateways: Gateway[];
  routes: GatewayRoute[];
  services: Service[];
  certificates: Certificate[];
  callers: Caller[];
  policies: Policy[];
  requests: RequestRecord[];
  currentVersion: number;
  releaseHistory: ReleaseRecord[];
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
  updateServiceCredential: (serviceID: string, clientCertificateID?: string) => void;
  addCertificate: (certificate: Certificate) => void;
  updateCertificate: (certificate: Certificate) => void;
  deleteCertificate: (certificateID: string) => void;
  addCaller: (caller: Caller) => void;
  updateCallerPermissions: (callerID: string, permissions: CallerPermission[]) => void;
  setCallerQuota: (callerID: string, quota: CallerQuota) => void;
  removeCallerQuota: (callerID: string, routeID: string) => void;
  issueCallerKey: (callerID: string, key: CallerAccessKey) => void;
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
  const [requests] = useState(initialRequests);
  const [currentVersion, setCurrentVersion] = useState(142);
  const [releaseHistory, setReleaseHistory] = useState(initialReleaseHistory);
  const [auditRecords, setAuditRecords] = useState(initialAuditRecords);
  const nextVersion = useRef(143);

  const recordAudit = (action: string, resourceType: AuditRecord['resourceType'], resource: string, detail: string) => {
    setAuditRecords((records) => [{ id: crypto.randomUUID(), time: currentTime(), actor: '林工程师', action, resourceType, resource, detail, result: '成功' }, ...records]);
  };

  const publish = (summary: string, resourceType: AuditRecord['resourceType'], resource: string, detail: string, complete: () => void) => {
    const version = nextVersion.current++;
    setCurrentVersion(version);
    setReleaseHistory((records) => [{ version, time: `今天 ${currentTime()}`, summary, resources: '1 项资源', state: '发布中' }, ...records]);
    recordAudit(summary, resourceType, resource, detail);
    window.setTimeout(() => {
      complete();
      setReleaseHistory((records) => records.map((record) => record.version === version ? { ...record, state: '已生效' } : record));
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
    currentVersion,
    releaseHistory,
    auditRecords,
    addGateway: (gateway) => {
      setGateways((items) => [...items, { ...gateway, state: 'pending' }]);
      publish(`创建网关“${gateway.name}”`, '网关', gateway.name, '网关配置已提交，正在同步到代理实例', () => setGateways((items) => items.map((item) => item.id === gateway.id ? { ...item, state: 'healthy' } : item)));
    },
    updateGateway: (gateway) => {
      setGateways((items) => items.map((item) => item.id === gateway.id ? { ...gateway, state: 'pending' } : item));
      setRoutes((items) => items.map((route) => route.gatewayID === gateway.id ? { ...route, gatewayName: gateway.name } : route));
      setPolicies((items) => items.map((policy) => ({ ...policy, targets: policy.targets.map((target) => target.kind === '网关' && target.id === gateway.id ? { ...target, name: gateway.name } : target) })));
      publish(`更新网关“${gateway.name}”`, '网关', gateway.name, '网关配置已提交，正在同步到代理实例', () => setGateways((items) => items.map((item) => item.id === gateway.id ? { ...item, state: 'healthy' } : item)));
    },
    deleteGateway: (gatewayID) => {
      const gateway = gateways.find((item) => item.id === gatewayID);
      if (!gateway) return;
      setGateways((items) => items.filter((item) => item.id !== gatewayID));
      publish(`删除网关“${gateway.name}”`, '网关', gateway.name, '网关已从当前环境移除', () => undefined);
    },
    addRoute: (route) => {
      setRoutes((items) => [...items, { ...route, state: 'pending' }]);
      publish(`创建路由“${route.name}”`, '路由', route.name, `${route.host}${route.path} 已提交发布`, () => setRoutes((items) => items.map((item) => item.id === route.id ? { ...item, state: 'healthy' } : item)));
    },
    updateRoute: (route) => {
      setRoutes((items) => items.map((item) => item.id === route.id ? { ...route, state: 'pending' } : item));
      setPolicies((items) => items.map((policy) => ({ ...policy, targets: policy.targets.map((target) => target.kind === '路由' && target.id === route.id ? { ...target, name: route.name } : target) })));
      publish(`更新路由“${route.name}”`, '路由', route.name, `${route.host}${route.path} 已提交发布`, () => setRoutes((items) => items.map((item) => item.id === route.id ? { ...item, state: 'healthy' } : item)));
    },
    deleteRoute: (routeID) => {
      const route = routes.find((item) => item.id === routeID);
      if (!route) return;
      setRoutes((items) => items.filter((item) => item.id !== routeID));
      publish(`删除路由“${route.name}”`, '路由', route.name, '路由已从当前环境移除', () => undefined);
    },
    addService: (service) => {
      setServices((items) => [...items, { ...service, state: service.state === 'unverified' ? 'unverified' : 'pending' }]);
      publish(`创建服务“${service.name}”`, '服务', service.name, `${service.type === 'MODEL' ? '模型' : service.type} 服务连接配置已保存`, () => setServices((items) => items.map((item) => item.id === service.id && item.state === 'pending' ? { ...item, state: 'healthy' } : item)));
    },
    updateService: (service) => {
      const nextState = service.state === 'unverified' ? 'unverified' : 'pending';
      setServices((items) => items.map((item) => item.id === service.id ? { ...service, state: nextState } : item));
      setRoutes((items) => items.map((route) => ({ ...route, targets: route.targets.map((target) => target.serviceID === service.id ? { ...target, serviceName: service.name } : target) })));
      publish(`更新服务“${service.name}”`, '服务', service.name, '服务连接配置已保存', () => setServices((items) => items.map((item) => item.id === service.id && item.state === 'pending' ? { ...item, state: 'healthy' } : item)));
    },
    deleteService: (serviceID) => {
      const service = services.find((item) => item.id === serviceID);
      if (!service) return;
      setServices((items) => items.filter((item) => item.id !== serviceID));
      publish(`删除服务“${service.name}”`, '服务', service.name, '服务连接配置已移除', () => undefined);
    },
    updateServiceCredential: (serviceID, clientCertificateID) => {
      const service = services.find((item) => item.id === serviceID);
      if (!service) return;
      setServices((items) => items.map((item) => item.id === serviceID ? { ...item, state: 'pending', clientCertificateID: clientCertificateID ?? item.clientCertificateID, credentialUpdatedAt: '刚刚' } : item));
      publish(`更新服务“${service.name}”凭据`, '服务', service.name, '新凭据已验证并提交切换', () => setServices((items) => items.map((item) => item.id === serviceID ? { ...item, state: 'healthy' } : item)));
    },
    addCertificate: (certificate) => {
      setCertificates((items) => [...items, { ...certificate, state: 'pending' }]);
      publish(`导入证书“${certificate.name}”`, '证书', certificate.name, '证书已校验并提交配置', () => setCertificates((items) => items.map((item) => item.id === certificate.id ? { ...item, state: 'healthy' } : item)));
    },
    updateCertificate: (certificate) => {
      setCertificates((items) => items.map((item) => item.id === certificate.id ? { ...certificate, state: 'pending' } : item));
      publish(`更新证书“${certificate.name}”`, '证书', certificate.name, '新证书已校验并提交替换', () => setCertificates((items) => items.map((item) => item.id === certificate.id ? { ...item, state: 'healthy' } : item)));
    },
    deleteCertificate: (certificateID) => {
      const certificate = certificates.find((item) => item.id === certificateID);
      if (!certificate) return;
      setCertificates((items) => items.filter((item) => item.id !== certificateID));
      publish(`删除证书“${certificate.name}”`, '证书', certificate.name, '证书已从当前环境移除', () => undefined);
    },
    addCaller: (caller) => {
      setCallers((items) => [...items, caller]);
      recordAudit('创建调用方', '调用方', caller.name, caller.permissions.length ? `已授予 ${caller.permissions.length} 条路由权限` : '尚未授予路由权限');
    },
    updateCallerPermissions: (callerID, permissions) => {
      setCallers((items) => items.map((caller) => {
        if (caller.id !== callerID) return caller;
        const routeIDs = new Set(permissions.map((permission) => permission.routeID));
        const quotas = caller.quotas.filter((quota) => routeIDs.has(quota.routeID));
        return { ...caller, permissions, quotas, state: callerState(quotas) };
      }));
      publish(`更新“${callers.find((item) => item.id === callerID)?.name ?? '调用方'}”权限`, '调用方', callers.find((item) => item.id === callerID)?.name ?? callerID, '路由访问权限已提交', () => undefined);
    },
    setCallerQuota: (callerID, quota) => {
      setCallers((items) => items.map((caller) => {
        if (caller.id !== callerID) return caller;
        const current = caller.quotas.find((item) => item.routeID === quota.routeID);
        const nextQuota = { ...quota, used: current?.used ?? quota.used };
        const quotas = current
          ? caller.quotas.map((item) => item.routeID === quota.routeID ? nextQuota : item)
          : [...caller.quotas, nextQuota];
        return { ...caller, quotas, state: callerState(quotas) };
      }));
      publish(`更新“${callers.find((item) => item.id === callerID)?.name ?? '调用方'}”用量上限`, '调用方', callers.find((item) => item.id === callerID)?.name ?? callerID, '累计用量上限已提交', () => undefined);
    },
    removeCallerQuota: (callerID, routeID) => {
      setCallers((items) => items.map((caller) => {
        if (caller.id !== callerID) return caller;
        const quotas = caller.quotas.filter((quota) => quota.routeID !== routeID);
        return { ...caller, quotas, state: callerState(quotas) };
      }));
      publish(`移除“${callers.find((item) => item.id === callerID)?.name ?? '调用方'}”用量上限`, '调用方', callers.find((item) => item.id === callerID)?.name ?? callerID, '累计用量上限已移除', () => undefined);
    },
    issueCallerKey: (callerID, key) => {
      setCallers((items) => items.map((caller) => caller.id === callerID ? { ...caller, keys: [...caller.keys, key] } : caller));
      const caller = callers.find((item) => item.id === callerID);
      recordAudit('签发密钥', '调用方', caller?.name ?? callerID, `签发“${key.name}”访问密钥，有效期至 ${key.expiresAt}`);
    },
    revokeCallerKey: (callerID, keyID) => {
      const caller = callers.find((item) => item.id === callerID);
      const key = caller?.keys.find((item) => item.id === keyID);
      setCallers((items) => items.map((item) => item.id === callerID ? { ...item, keys: item.keys.map((current) => current.id === keyID ? { ...current, state: 'disabled' } : current) } : item));
      recordAudit('停用密钥', '调用方', caller?.name ?? callerID, `停用“${key?.name ?? keyID}”访问密钥`);
    },
    addPolicy: (policy) => {
      setPolicies((items) => [...items, { ...policy, state: policy.targets.length ? 'pending' : 'disabled' }]);
      if (!policy.targets.length) {
        recordAudit('创建流量策略', '流量策略', policy.name, '策略已保存，尚未选择生效目标');
        return;
      }
      publish(`创建流量策略“${policy.name}”`, '流量策略', policy.name, `已选择 ${policy.targets.length} 个生效目标`, () => setPolicies((items) => items.map((item) => item.id === policy.id ? { ...item, state: 'healthy' } : item)));
    },
    updatePolicy: (policy) => {
      setPolicies((items) => items.map((item) => item.id === policy.id ? { ...policy, state: policy.targets.length ? 'pending' : 'disabled' } : item));
      if (!policy.targets.length) {
        recordAudit('更新流量策略', '流量策略', policy.name, '策略已保存，尚未选择生效目标');
        return;
      }
      publish(`更新流量策略“${policy.name}”`, '流量策略', policy.name, `已选择 ${policy.targets.length} 个生效目标`, () => setPolicies((items) => items.map((item) => item.id === policy.id ? { ...item, state: 'healthy' } : item)));
    },
    deletePolicy: (policyID) => {
      const policy = policies.find((item) => item.id === policyID);
      if (!policy) return;
      setPolicies((items) => items.filter((item) => item.id !== policyID));
      publish(`删除流量策略“${policy.name}”`, '流量策略', policy.name, '策略已从当前环境移除', () => undefined);
    },
    resetDemo: () => {
      setGateways(initialGateways);
      setRoutes(initialRoutes);
      setServices(initialServices);
      setCertificates(initialCertificates);
      setCallers(initialCallers);
      setPolicies(initialPolicies);
      setCurrentVersion(142);
      setReleaseHistory(initialReleaseHistory);
      setAuditRecords(initialAuditRecords);
      nextVersion.current = 143;
    },
  };

  return <PrototypeContext.Provider value={value}>{children}</PrototypeContext.Provider>;
}

function currentTime() {
  return new Date().toLocaleTimeString('zh-CN', { hour12: false });
}

export function usePrototype() {
  const context = useContext(PrototypeContext);
  if (!context) throw new Error('PrototypeProvider is required');
  return context;
}

function callerState(quotas: CallerQuota[]) {
  return quotas.some((quota) => quota.used >= quota.limit) ? 'warning' as const : 'healthy' as const;
}
