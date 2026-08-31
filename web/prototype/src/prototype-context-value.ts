import type {
  AuditRecord,
  Caller,
  CallerAccessKey,
  CallerPermission,
  CallerQuota,
  Certificate,
  Gateway,
  GatewayRoute,
  IdentitySource,
  Policy,
  RequestRecord,
  Service,
} from "./data";

export interface PrototypeContextValue {
  gateways: Gateway[];
  routes: GatewayRoute[];
  services: Service[];
  certificates: Certificate[];
  callers: Caller[];
  identitySources: IdentitySource[];
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
  updateCaller: (caller: Caller) => void;
  deleteCaller: (callerID: string) => void;
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
  addIdentitySource: (identitySource: IdentitySource) => void;
  updateIdentitySource: (identitySource: IdentitySource) => void;
  deleteIdentitySource: (identitySourceID: string) => void;
  addPolicy: (policy: Policy) => void;
  updatePolicy: (policy: Policy) => void;
  deletePolicy: (policyID: string) => void;
  resetDemo: () => void;
}
