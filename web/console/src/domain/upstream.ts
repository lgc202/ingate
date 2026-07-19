import type { ResourceStatus } from './common';

export type UpstreamType = 'application' | 'model' | 'agent' | 'mcp';
export type UpstreamProtocol = 'HTTP' | 'OpenAI' | 'Anthropic' | 'Gemini';
export type UpstreamLoadBalancePolicy = 'round_robin' | 'least_request' | 'random';
export type ModelProvider = 'openai' | 'deepseek' | 'qwen' | 'anthropic' | 'gemini' | 'custom';

export interface ModelProviderDefinition {
  value: ModelProvider;
  label: string;
  description: string;
  monogram: string;
  protocol: Exclude<UpstreamProtocol, 'HTTP'>;
  defaultBaseURL: string;
}

export const upstreamTypeOptions: { value: UpstreamType; label: string }[] = [
  { value: 'application', label: '应用服务' },
  { value: 'model', label: '大模型' },
  { value: 'agent', label: 'Agent' },
  { value: 'mcp', label: 'MCP' },
];

export const upstreamLoadBalancePolicyOptions: { value: UpstreamLoadBalancePolicy; label: string }[] = [
  { value: 'round_robin', label: '轮询' },
  { value: 'least_request', label: '最少请求' },
  { value: 'random', label: '随机' },
];

export const upstreamProtocolOptions: { value: UpstreamProtocol; label: string }[] = [
  { value: 'HTTP', label: 'HTTP' },
  { value: 'OpenAI', label: 'OpenAI 兼容' },
  { value: 'Anthropic', label: 'Anthropic 原生' },
  { value: 'Gemini', label: 'Gemini 原生' },
];

export const modelProviderDefinitions: ModelProviderDefinition[] = [
  {
    value: 'openai',
    label: 'OpenAI',
    description: '官方接口',
    monogram: 'O',
    protocol: 'OpenAI',
    defaultBaseURL: 'https://api.openai.com/v1',
  },
  {
    value: 'deepseek',
    label: 'DeepSeek',
    description: 'OpenAI 兼容',
    monogram: 'D',
    protocol: 'OpenAI',
    defaultBaseURL: 'https://api.deepseek.com/v1',
  },
  {
    value: 'qwen',
    label: '通义千问',
    description: '兼容模式',
    monogram: 'Q',
    protocol: 'OpenAI',
    defaultBaseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  },
  {
    value: 'anthropic',
    label: 'Anthropic',
    description: '原生协议',
    monogram: 'A',
    protocol: 'Anthropic',
    defaultBaseURL: 'https://api.anthropic.com/v1',
  },
  {
    value: 'gemini',
    label: 'Gemini',
    description: '原生协议',
    monogram: 'G',
    protocol: 'Gemini',
    defaultBaseURL: 'https://generativelanguage.googleapis.com/v1beta',
  },
  {
    value: 'custom',
    label: '自定义兼容',
    description: 'Ollama / vLLM 等',
    monogram: '+',
    protocol: 'OpenAI',
    defaultBaseURL: '',
  },
];

export function upstreamTypeLabel(type: UpstreamType | string): string {
  return upstreamTypeOptions.find((option) => option.value === type)?.label ?? type;
}

export function upstreamLoadBalancePolicyLabel(policy: UpstreamLoadBalancePolicy | string): string {
  return upstreamLoadBalancePolicyOptions.find((option) => option.value === policy)?.label ?? policy;
}

export function upstreamProtocolLabel(protocol: UpstreamProtocol | string): string {
  return upstreamProtocolOptions.find((option) => option.value === protocol)?.label ?? protocol;
}

export function modelProviderDefinition(provider: ModelProvider | string): ModelProviderDefinition {
  return modelProviderDefinitions.find((item) => item.value === provider)
    ?? modelProviderDefinitions.find((item) => item.value === 'custom')!;
}

export function modelProviderLabel(provider: ModelProvider | string): string {
  return modelProviderDefinition(provider).label;
}

export interface Upstream {
  id: string;
  version?: string;
  name: string;
  type: UpstreamType;
  protocol: UpstreamProtocol;
  model?: ModelServiceConfig;
  tls?: UpstreamTLS;
  apiKeyConfigured: boolean;
  endpoints: UpstreamEndpoint[];
  loadBalancePolicy: UpstreamLoadBalancePolicy;
  healthCheck?: UpstreamHealthCheck;
  status: ResourceStatus;
  createdAt: string;
}

export interface UpstreamList {
  upstreams: Upstream[];
}

export interface UpstreamMutationPayload {
  id?: string;
  version?: string;
  name: string;
  type: UpstreamType;
  protocol: UpstreamProtocol;
  model?: ModelServiceConfig;
  tls?: UpstreamTLS;
  apiKey?: UpstreamAPIKey;
  removeAPIKey?: boolean;
  endpoints: UpstreamEndpoint[];
  loadBalancePolicy: UpstreamLoadBalancePolicy;
  healthCheck?: UpstreamHealthCheck;
}

export interface UpstreamAPIKey {
  value: string;
}

export interface ModelServiceConfig {
  provider: ModelProvider;
  apiBasePath: string;
  models: ModelCatalogItem[];
}

export interface ModelCatalogItem {
  name: string;
  displayName: string;
  enabled: boolean;
}

export interface UpstreamEndpoint {
  id: string;
  address: string;
  port: number;
  weight: number;
  enabled: boolean;
}

export interface UpstreamTLS {
  serverName: string;
}

export interface UpstreamHealthCheck {
  enabled: boolean;
  path?: string;
  intervalSeconds?: number;
  timeoutSeconds?: number;
}

export interface UpstreamMutationResult {
  message: string;
  changeId?: string;
}
