// 对应 api/admin/v1 的 ProtoJSON 契约；版本、int64 和稳定名称不转为数字。
export interface Resource {
  name: string;
  namespace: string;
  displayName: string;
  version: string;
  createTime?: string;
}

export interface Listener {
  name: string;
  protocol: 'LISTENER_PROTOCOL_HTTP';
  port: number;
  hostname?: string;
}

export interface Gateway extends Resource {
  listeners?: Listener[];
}
export interface Endpoint {
  address: string;
  port: number;
  weight: number;
}
export interface BackendTLS {
  sni: string;
  wellKnownCaCertificates: 'System';
}
export interface Backend extends Resource {
  endpoints?: Endpoint[];
  tls?: BackendTLS | null;
}
export interface BackendRef {
  name: string;
}
export interface AIBackend extends Resource {
  backendRef?: BackendRef;
  schema?: { name: 'OpenAI'; prefix?: string };
}

export interface ParentRef {
  name: string;
  sectionName?: string;
}
export interface HTTPMatch {
  path?: {
    type?: 'PATH_MATCH_TYPE_EXACT' | 'PATH_MATCH_TYPE_PREFIX' | 'PATH_MATCH_TYPE_UNSPECIFIED';
    value?: string;
  };
}
export interface HTTPRule {
  name?: string;
  matches?: HTTPMatch[];
  backendRefs?: BackendRef[];
}
export interface HTTPRoute extends Resource {
  parentRefs?: ParentRef[];
  rules?: HTTPRule[];
}
export interface ModelMatch {
  headers?: { name: string; type?: string; value: string }[];
}
export interface AIBackendRef {
  name: string;
  modelNameOverride?: string;
}
export interface ModelRule {
  name?: string;
  matches?: ModelMatch[];
  backendRefs?: AIBackendRef[];
}
export interface ModelRoute extends Resource {
  parentRefs?: ParentRef[];
  rules?: ModelRule[];
}
export interface MCPBackendRef {
  name: string;
  path?: string;
}
export interface MCPRoute extends Resource {
  parentRefs?: ParentRef[];
  path?: string;
  backendRefs?: MCPBackendRef[];
}

export interface Resources {
  gateways: Gateway;
  backends: Backend;
  'ai-backends': AIBackend;
  'http-routes': HTTPRoute;
  'model-routes': ModelRoute;
  'mcp-routes': MCPRoute;
}
export type Kind = keyof Resources;
export type AnyResource = Resources[Kind];
export type RouteKind = 'http-routes' | 'model-routes' | 'mcp-routes';
export type ServiceKind = 'backends' | 'ai-backends';
export type Spec<T extends Resource> = T extends Resource
  ? Omit<T, keyof Resource> & { displayName: string }
  : never;
export type ResourceSpec = Spec<AnyResource>;

export const labels: Record<Kind, string> = {
  gateways: '网关',
  backends: '网络服务',
  'ai-backends': '模型服务',
  'http-routes': 'HTTP 路由',
  'model-routes': '模型路由',
  'mcp-routes': 'MCP 路由',
};

export function title(resource: Resource): string {
  return resource.displayName || resource.name;
}

export function timestamp(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';

  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'short',
    timeStyle: 'short',
    hour12: false,
  }).format(date);
}

// 完整 PUT 只提交协议允许的配置，保留所有规则、匹配及引用，绝不降为列表摘要。
export function resourceSpec<K extends Kind>(kind: K, resource: Resources[K]): Spec<Resources[K]> {
  const displayName = resource.displayName;
  let result: object;
  switch (kind) {
    case 'gateways':
      result = {
        displayName,
        listeners: (resource as Gateway).listeners ?? [],
      };
      break;
    case 'backends': {
      const backend = resource as Backend;
      result = {
        displayName,
        endpoints: backend.endpoints ?? [],
        tls: backend.tls ?? null,
      };
      break;
    }
    case 'ai-backends': {
      const backend = resource as AIBackend;
      result = {
        displayName,
        backendRef: backend.backendRef,
        schema: backend.schema,
      };
      break;
    }
    case 'http-routes':
    case 'model-routes': {
      const route = resource as HTTPRoute | ModelRoute;
      result = {
        displayName,
        parentRefs: route.parentRefs ?? [],
        rules: route.rules ?? [],
      };
      break;
    }
    case 'mcp-routes': {
      const route = resource as MCPRoute;
      result = {
        displayName,
        parentRefs: route.parentRefs ?? [],
        path: route.path,
        backendRefs: route.backendRefs ?? [],
      };
      break;
    }
  }

  return structuredClone(result) as Spec<Resources[K]>;
}

export function newSpec(kind: Kind): ResourceSpec {
  const displayName = '';
  switch (kind) {
    case 'gateways':
      return {
        displayName,
        listeners: [
          {
            name: 'http',
            protocol: 'LISTENER_PROTOCOL_HTTP',
            port: 8080,
            hostname: '',
          },
        ],
      } as Spec<Gateway>;
    case 'backends':
      return {
        displayName,
        endpoints: [{ address: '', port: 8080, weight: 1 }],
        tls: null,
      } as Spec<Backend>;
    case 'ai-backends':
      return {
        displayName,
        backendRef: { name: '' },
        schema: { name: 'OpenAI', prefix: '/v1' },
      } as Spec<AIBackend>;
    case 'http-routes':
      return {
        displayName,
        parentRefs: [{ name: '' }],
        rules: [
          {
            matches: [{ path: { type: 'PATH_MATCH_TYPE_PREFIX', value: '/' } }],
            backendRefs: [{ name: '' }],
          },
        ],
      } as Spec<HTTPRoute>;
    case 'model-routes':
      return {
        displayName,
        parentRefs: [{ name: '' }],
        rules: [
          {
            matches: [
              {
                headers: [{ name: 'x-ai-eg-model', type: 'Exact', value: '' }],
              },
            ],
            backendRefs: [{ name: '' }],
          },
        ],
      } as Spec<ModelRoute>;
    case 'mcp-routes':
      return {
        displayName,
        parentRefs: [{ name: '' }],
        path: '/mcp',
        backendRefs: [{ name: '', path: '/mcp' }],
      } as Spec<MCPRoute>;
  }
}
