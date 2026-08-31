import type { Gateway } from "../../models/traffic";

export const initialGateways: Gateway[] = [
  {
    id: "gw-prod",
    name: "生产网关",
    listeners: [
      {
        id: "listener-prod-https",
        protocol: "HTTPS",
        port: 443,
        bindings: [
          { domain: "api.example.com", certificateID: "cert-wildcard" },
          { domain: "mcp.example.com", certificateID: "cert-wildcard" },
        ],
      },
      {
        id: "listener-prod-http",
        protocol: "HTTP",
        port: 80,
        bindings: [
          { domain: "api.example.com" },
          { domain: "mcp.example.com" },
        ],
      },
    ],
    state: "healthy",
  },
  {
    id: "gw-internal",
    name: "内部网关",
    listeners: [
      {
        id: "listener-internal-https",
        protocol: "HTTPS",
        port: 8443,
        bindings: [
          { domain: "inside.example.com", certificateID: "cert-internal" },
        ],
      },
    ],
    state: "healthy",
  },
];
