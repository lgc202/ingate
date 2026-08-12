import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import {
  initialCallers,
  initialCertificates,
  initialGateways,
  initialPolicies,
  initialRequests,
  initialRoutes,
  initialServices,
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
} from './data';

interface PrototypeState {
  gateways: Gateway[];
  routes: GatewayRoute[];
  services: Service[];
  certificates: Certificate[];
  callers: Caller[];
  policies: Policy[];
  requests: RequestRecord[];
  addGateway: (gateway: Gateway) => void;
  addRoute: (route: GatewayRoute) => void;
  addService: (service: Service) => void;
  updateServiceCredential: (serviceID: string, clientCertificateID?: string) => void;
  addCertificate: (certificate: Certificate) => void;
  addCaller: (caller: Caller) => void;
  updateCallerPermissions: (callerID: string, permissions: CallerPermission[]) => void;
  setCallerQuota: (callerID: string, quota: CallerQuota) => void;
  removeCallerQuota: (callerID: string, routeID: string) => void;
  issueCallerKey: (callerID: string, key: CallerAccessKey) => void;
  revokeCallerKey: (callerID: string, keyID: string) => void;
  addPolicy: (policy: Policy) => void;
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

  const value = useMemo<PrototypeState>(() => ({
    gateways,
    routes,
    services,
    certificates,
    callers,
    policies,
    requests,
    addGateway: (gateway) => setGateways((items) => [...items, gateway]),
    addRoute: (route) => setRoutes((items) => [...items, route]),
    addService: (service) => setServices((items) => [...items, service]),
    updateServiceCredential: (serviceID, clientCertificateID) => setServices((items) => items.map((service) => service.id === serviceID ? { ...service, clientCertificateID: clientCertificateID ?? service.clientCertificateID, credentialUpdatedAt: '刚刚' } : service)),
    addCertificate: (certificate) => setCertificates((items) => [...items, certificate]),
    addCaller: (caller) => setCallers((items) => [...items, caller]),
    updateCallerPermissions: (callerID, permissions) => setCallers((items) => items.map((caller) => {
      if (caller.id !== callerID) return caller;
      const routeIDs = new Set(permissions.map((permission) => permission.routeID));
      const quotas = caller.quotas.filter((quota) => routeIDs.has(quota.routeID));
      return { ...caller, permissions, quotas, state: callerState(quotas) };
    })),
    setCallerQuota: (callerID, quota) => setCallers((items) => items.map((caller) => {
      if (caller.id !== callerID) return caller;
      const current = caller.quotas.find((item) => item.routeID === quota.routeID);
      const nextQuota = { ...quota, used: current?.used ?? quota.used };
      const quotas = current
        ? caller.quotas.map((item) => item.routeID === quota.routeID ? nextQuota : item)
        : [...caller.quotas, nextQuota];
      return { ...caller, quotas, state: callerState(quotas) };
    })),
    removeCallerQuota: (callerID, routeID) => setCallers((items) => items.map((caller) => {
      if (caller.id !== callerID) return caller;
      const quotas = caller.quotas.filter((quota) => quota.routeID !== routeID);
      return { ...caller, quotas, state: callerState(quotas) };
    })),
    issueCallerKey: (callerID, key) => setCallers((items) => items.map((caller) => caller.id === callerID ? { ...caller, keys: [...caller.keys, key] } : caller)),
    revokeCallerKey: (callerID, keyID) => setCallers((items) => items.map((caller) => caller.id === callerID ? { ...caller, keys: caller.keys.map((key) => key.id === keyID ? { ...key, state: 'disabled' } : key) } : caller)),
    addPolicy: (policy) => setPolicies((items) => [...items, policy]),
    resetDemo: () => {
      setGateways(initialGateways);
      setRoutes(initialRoutes);
      setServices(initialServices);
      setCertificates(initialCertificates);
      setCallers(initialCallers);
      setPolicies(initialPolicies);
    },
  }), [callers, certificates, gateways, policies, requests, routes, services]);

  return <PrototypeContext.Provider value={value}>{children}</PrototypeContext.Provider>;
}

export function usePrototype() {
  const context = useContext(PrototypeContext);
  if (!context) throw new Error('PrototypeProvider is required');
  return context;
}

function callerState(quotas: CallerQuota[]) {
  return quotas.some((quota) => quota.used >= quota.limit) ? 'warning' as const : 'healthy' as const;
}
