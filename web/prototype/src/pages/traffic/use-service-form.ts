import { useState } from "react";
import type {
  Certificate,
  Service,
  ServiceEndpoint,
  ServiceType,
} from "../../data";
import { normalizedAuthentication } from "./service-model";

export interface ServiceFormProps {
  initial?: Service;
  certificates: Certificate[];
  onClose: () => void;
  onSave: (service: Service) => void;
}

export type ServiceFormState = ReturnType<typeof useServiceForm>;

export function useServiceForm({
  initial,
  certificates,
  onSave,
}: ServiceFormProps) {
  const initialAuthentication = normalizedAuthentication(
    initial?.authentication,
  );
  const [type, setType] = useState<ServiceType>(initial?.type ?? "HTTP");
  const [name, setName] = useState(initial?.name ?? "");
  const [provider, setProvider] = useState(initial?.provider ?? "内部服务");
  const [protocol, setProtocol] = useState(initial?.protocol ?? "HTTP/1.1");
  const [authentication, setAuthentication] = useState(initialAuthentication);
  const [credential, setCredential] = useState("");
  const [credentialName, setCredentialName] = useState("");
  const [credentialRegion, setCredentialRegion] = useState("");
  const [clientCertificateID, setClientCertificateID] = useState(
    initial?.clientCertificateID ?? "",
  );
  const [transportSecurity, setTransportSecurity] = useState<
    Service["transportSecurity"]
  >(initial?.transportSecurity ?? "明文连接");
  const [serverName, setServerName] = useState(initial?.serverName ?? "");
  const [trustMode, setTrustMode] = useState<"系统信任" | "指定信任证书">(
    initial?.trustCertificateID ? "指定信任证书" : "系统信任",
  );
  const [trustCertificateID, setTrustCertificateID] = useState(
    initial?.trustCertificateID ?? "",
  );
  const [endpoints, setEndpoints] = useState<
    Array<{
      address: string;
      weight: number;
    }>
  >(
    initial?.endpoints.map(({ address, weight }) => ({
      address,
      weight,
    })) ?? [
      {
        address: "service.internal:8080",
        weight: 100,
      },
    ],
  );
  const [loadBalancing, setLoadBalancing] = useState<Service["loadBalancing"]>(
    initial?.loadBalancing ?? "轮询",
  );
  const [healthMode, setHealthMode] = useState(
    initial?.healthCheck.startsWith("被动")
      ? "被动检查"
      : initial?.healthCheck.startsWith("TCP")
        ? "TCP"
        : "HTTP",
  );
  const [healthPath, setHealthPath] = useState(
    initial?.healthCheck.match(/HTTP GET (\S+)/)?.[1] ?? "/health",
  );
  const [healthInterval, setHealthInterval] = useState(
    initial?.healthCheck.match(/(\d+) 秒/)?.[1] ?? "10",
  );
  const [models, setModels] = useState(
    initial?.type === "MODEL" ? initial.capabilities.join(", ") : "",
  );
  const [modelPrices, setModelPrices] = useState<
    Record<
      string,
      {
        input: number;
        cachedInput?: number;
        output: number;
      }
    >
  >(
    Object.fromEntries(
      Object.entries(initial?.modelPrices ?? {}).map(([model, price]) => [
        model,
        {
          input: price.input,
          cachedInput: price.cachedInput,
          output: price.output,
        },
      ]),
    ),
  );
  const [modelPriceQuery, setModelPriceQuery] = useState("");
  const [discoveredTools, setDiscoveredTools] = useState<string[]>(
    initial?.type === "MCP" ? initial.capabilities : [],
  );
  const [tested, setTested] = useState(
    Boolean(initial && initial.state !== "unverified"),
  );
  const protocolOptions =
    type === "HTTP"
      ? ["HTTP/1.1", "HTTP/2"]
      : type === "MODEL"
        ? ["OpenAI 兼容 API", "Anthropic Messages", "AWS Bedrock"]
        : ["Streamable HTTP"];
  const authenticationOptions =
    type === "HTTP"
      ? ["无认证", "Bearer Token", "Basic", "mTLS"]
      : type === "MODEL"
        ? ["API Key", "Bearer Token", "自定义请求头", "AWS 签名"]
        : ["无认证", "Bearer Token", "mTLS"];
  const clientCertificates = certificates.filter(
    (item) => item.usage === "客户端证书" && item.state !== "error",
  );
  const trustCertificates = certificates.filter(
    (item) => item.usage === "信任证书" && item.state !== "error",
  );
  const effectiveClientCertificateID = clientCertificates.some(
    (item) => item.id === clientCertificateID,
  )
    ? clientCertificateID
    : "";
  const effectiveTrustCertificateID = trustCertificates.some(
    (item) => item.id === trustCertificateID,
  )
    ? trustCertificateID
    : "";
  const capabilities =
    type === "MODEL"
      ? models
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean)
      : type === "MCP"
        ? discoveredTools
        : [];
  const visiblePriceModels = capabilities.filter((model) =>
    model.toLowerCase().includes(modelPriceQuery.toLowerCase()),
  );
  const balanceWeights = (
    items: Array<{
      address: string;
      weight: number;
    }>,
  ) => {
    const baseWeight = Math.floor(100 / items.length);
    const remainder = 100 - baseWeight * items.length;
    return items.map((item, index) => ({
      ...item,
      weight: baseWeight + (index < remainder ? 1 : 0),
    }));
  };
  const updateEndpoint = (
    index: number,
    changes: Partial<{
      address: string;
      weight: number;
    }>,
  ) => {
    setEndpoints((items) =>
      items.map((item, itemIndex) =>
        itemIndex === index
          ? {
              ...item,
              ...changes,
            }
          : item,
      ),
    );
    setTested(false);
  };
  const changeType = (next: ServiceType) => {
    setType(next);
    setProvider(
      next === "HTTP"
        ? "内部服务"
        : next === "MODEL"
          ? "模型服务商"
          : "MCP 服务",
    );
    setProtocol(
      next === "HTTP"
        ? "HTTP/1.1"
        : next === "MODEL"
          ? "OpenAI 兼容 API"
          : "Streamable HTTP",
    );
    setAuthentication(
      next === "HTTP" ? "无认证" : next === "MODEL" ? "Bearer Token" : "无认证",
    );
    setEndpoints([
      {
        address:
          next === "MCP"
            ? "tools.internal:443/mcp"
            : next === "MODEL"
              ? "api.provider.com:443"
              : "service.internal:8080",
        weight: 100,
      },
    ]);
    setTransportSecurity(next === "HTTP" ? "明文连接" : "TLS");
    setCredential("");
    setCredentialName("");
    setCredentialRegion("");
    setClientCertificateID("");
    setServerName("");
    setTrustMode("系统信任");
    setTrustCertificateID("");
    setModels("");
    setModelPriceQuery("");
    setDiscoveredTools([]);
    setHealthMode(next === "MODEL" ? "被动检查" : "HTTP");
    setTested(false);
  };
  const testConnection = () => {
    if (
      type === "MODEL" &&
      protocol === "OpenAI 兼容 API" &&
      !models.trim()
    ) {
      const discoveredModels = ["qwen-max", "qwen-plus"];
      setModels(discoveredModels.join(", "));
      setModelPrices((prices) =>
        Object.fromEntries(
          discoveredModels.map((model) => [
            model,
            prices[model] ?? {
              input: 0,
              output: 0,
            },
          ]),
        ),
      );
    }
    if (type === "MCP")
      setDiscoveredTools(["web_search", "fetch_page", "extract_text"]);
    setTested(true);
  };
  const needsCredential = authentication !== "无认证";
  const canReuseStoredCredential = Boolean(
    initial &&
      authentication === initialAuthentication &&
      authentication !== "无认证",
  );
  const credentialComplete =
    !needsCredential ||
    (authentication === "mTLS"
      ? Boolean(effectiveClientCertificateID)
      : canReuseStoredCredential ||
        (authentication === "Basic" || authentication === "自定义请求头"
          ? Boolean(credentialName.trim() && credential.trim())
          : authentication === "AWS 签名"
            ? Boolean(
                credentialName.trim() &&
                  credential.trim() &&
                  credentialRegion.trim(),
              )
            : Boolean(credential.trim())));
  const tlsComplete =
    transportSecurity === "明文连接" ||
    trustMode === "系统信任" ||
    Boolean(effectiveTrustCertificateID);
  const canTest =
    endpoints.every((item) => item.address.trim()) &&
    credentialComplete &&
    tlsComplete;
  const canSave = type !== "MODEL" || capabilities.length > 0;
  const endpointRecords: ServiceEndpoint[] = endpoints.map((item) => ({
    ...item,
    weight: endpoints.length === 1 ? 100 : item.weight,
    state: "healthy",
  }));
  const healthCheck =
    healthMode === "被动检查"
      ? "被动健康检查"
      : healthMode === "TCP"
        ? `TCP 连接 · ${healthInterval} 秒`
        : `HTTP GET ${healthPath} · ${healthInterval} 秒`;
  const savedAuthentication =
    authentication === "mTLS"
      ? `mTLS · ${clientCertificates.find((item) => item.id === effectiveClientCertificateID)?.name}`
      : authentication;
  const save = () =>
    onSave({
      id: initial?.id ?? `svc-${Date.now()}`,
      name,
      type,
      endpoints: endpointRecords,
      provider,
      protocol,
      authentication: savedAuthentication,
      clientCertificateID:
        authentication === "mTLS" ? effectiveClientCertificateID : undefined,
      transportSecurity,
      serverName:
        transportSecurity === "TLS" ? serverName || undefined : undefined,
      trustCertificateID:
        transportSecurity === "TLS" && trustMode === "指定信任证书"
          ? effectiveTrustCertificateID
          : undefined,
      loadBalancing,
      healthCheck,
      capabilities,
      modelPrices:
        type === "MODEL"
          ? Object.fromEntries(
              capabilities.map((model) => [
                model,
                {
                  ...(modelPrices[model] ?? {
                    input: 0,
                    output: 0,
                  }),
                  unit: "每百万 Token" as const,
                  updatedAt: new Date().toISOString().slice(0, 10),
                },
              ]),
            )
          : undefined,
      successRate: initial?.successRate ?? "—",
      latency: initial?.latency ?? "—",
      state: tested ? "healthy" : "unverified",
      verificationState: tested ? "verified" : "unverified",
    });
  return {
    certificates,
    type,
    setType,
    name,
    setName,
    provider,
    setProvider,
    protocol,
    setProtocol,
    authentication,
    setAuthentication,
    credential,
    setCredential,
    credentialName,
    setCredentialName,
    credentialRegion,
    setCredentialRegion,
    clientCertificateID,
    setClientCertificateID,
    transportSecurity,
    setTransportSecurity,
    serverName,
    setServerName,
    trustMode,
    setTrustMode,
    trustCertificateID,
    setTrustCertificateID,
    endpoints,
    setEndpoints,
    loadBalancing,
    setLoadBalancing,
    healthMode,
    setHealthMode,
    healthPath,
    setHealthPath,
    healthInterval,
    setHealthInterval,
    models,
    setModels,
    modelPrices,
    setModelPrices,
    modelPriceQuery,
    setModelPriceQuery,
    discoveredTools,
    setDiscoveredTools,
    tested,
    setTested,
    protocolOptions,
    authenticationOptions,
    clientCertificates,
    trustCertificates,
    effectiveClientCertificateID,
    effectiveTrustCertificateID,
    capabilities,
    visiblePriceModels,
    balanceWeights,
    updateEndpoint,
    changeType,
    testConnection,
    needsCredential,
    canReuseStoredCredential,
    credentialComplete,
    tlsComplete,
    canTest,
    canSave,
    endpointRecords,
    healthCheck,
    savedAuthentication,
    save,
  };
}
