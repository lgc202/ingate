import { useState, type Dispatch, type SetStateAction } from "react";
import {
  initialCertificates,
  initialGateways,
  initialRoutes,
  initialServices,
  type Policy,
} from "../data";
import type { PrototypeContextValue } from "../prototype-context-value";
import type { RecordAudit } from "./operations-state";

type TrafficActions = Pick<
  PrototypeContextValue,
  | "addGateway"
  | "updateGateway"
  | "deleteGateway"
  | "addRoute"
  | "updateRoute"
  | "deleteRoute"
  | "addService"
  | "updateService"
  | "deleteService"
  | "verifyService"
  | "updateServiceCredential"
  | "addCertificate"
  | "updateCertificate"
  | "deleteCertificate"
>;

export function useTrafficState(
  recordAudit: RecordAudit,
  setPolicies: Dispatch<SetStateAction<Policy[]>>,
) {
  const [gateways, setGateways] = useState(initialGateways);
  const [routes, setRoutes] = useState(initialRoutes);
  const [services, setServices] = useState(initialServices);
  const [certificates, setCertificates] = useState(initialCertificates);

  const actions: TrafficActions = {
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
  };

  return {
    state: { gateways, routes, services, certificates },
    actions,
    reset: () => {
      setGateways(initialGateways);
      setRoutes(initialRoutes);
      setServices(initialServices);
      setCertificates(initialCertificates);
    },
  };
}
