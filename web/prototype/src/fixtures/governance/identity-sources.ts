import type { IdentitySource } from "../../models/governance";

export const initialIdentitySources: IdentitySource[] = [
  {
    id: "idp-enterprise",
    name: "企业统一身份",
    provider: "Microsoft Entra ID",
    discoveryURL:
      "https://login.microsoftonline.com/example/v2.0/.well-known/openid-configuration",
    audiences: ["api://ingate-production"],
    state: "healthy",
    lastVerified: "5 分钟前",
  },
  {
    id: "idp-partner",
    name: "合作方身份中心",
    provider: "Keycloak",
    discoveryURL:
      "https://identity.partner.example.com/realms/api/.well-known/openid-configuration",
    audiences: ["partner-api"],
    state: "healthy",
    lastVerified: "昨天",
  },
];
