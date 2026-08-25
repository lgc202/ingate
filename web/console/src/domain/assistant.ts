export type AssistantMessageRole =
  | 'MESSAGE_ROLE_USER'
  | 'MESSAGE_ROLE_ASSISTANT';

export type AssistantRunState =
  | 'RUN_STATE_QUEUED'
  | 'RUN_STATE_RUNNING'
  | 'RUN_STATE_SUCCEEDED'
  | 'RUN_STATE_FAILED'
  | 'RUN_STATE_CANCELLED';

export type AssistantRunItemKind =
  | 'RUN_ITEM_KIND_MODEL_CALL'
  | 'RUN_ITEM_KIND_TOOL_CALL';

export type AssistantRunItemState =
  | 'RUN_ITEM_STATE_RUNNING'
  | 'RUN_ITEM_STATE_COMPLETED'
  | 'RUN_ITEM_STATE_FAILED'
  | 'RUN_ITEM_STATE_CANCELLED';

export type ModelConnectionMode =
  | 'MODEL_CONNECTION_MODE_DIRECT'
  | 'MODEL_CONNECTION_MODE_INGATE';

export type ModelProtocol =
  | 'MODEL_PROTOCOL_OPENAI_COMPATIBLE'
  | 'MODEL_PROTOCOL_ANTHROPIC';

export interface AssistantConversation {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export interface AssistantMessage {
  id: string;
  conversationId: string;
  runId: string;
  role: AssistantMessageRole;
  content: string;
  reasoningContent: string;
  createdAt: string;
}

export interface AssistantRun {
  id: string;
  conversationId: string;
  state: AssistantRunState;
  model: string;
  errorCode: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  cancellationRequested: boolean;
}

export interface AssistantRunItem {
  id: string;
  runId: string;
  sequence: number;
  kind: AssistantRunItemKind;
  state: AssistantRunItemState;
  name: string;
  summary: string;
  errorCode: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface ModelConnection {
  configured: boolean;
  connectionMode: ModelConnectionMode;
  protocol: ModelProtocol;
  endpoint: string;
  model: string;
  apiKeyConfigured: boolean;
  timeoutSeconds: number;
  maxOutputTokens: number;
  reasoningBudgetTokens: number;
  updatedAt?: string;
}

export interface UpdateModelConnectionInput {
  connectionMode: ModelConnectionMode;
  protocol: ModelProtocol;
  endpoint: string;
  model: string;
  apiKey?: string;
  clearApiKey: boolean;
  timeoutSeconds: number;
  maxOutputTokens: number;
  reasoningBudgetTokens: number;
}

export type AssistantStreamEventType =
  | 'run.started'
  | 'message.reasoning.delta'
  | 'message.content.delta'
  | 'run.completed'
  | 'run.failed'
  | 'run.cancelled'
  | 'stream.failed';

export interface AssistantStreamEvent {
  id: string;
  type: AssistantStreamEventType;
  value: string;
}

export function isTerminalRun(state: AssistantRunState): boolean {
  return state === 'RUN_STATE_SUCCEEDED'
    || state === 'RUN_STATE_FAILED'
    || state === 'RUN_STATE_CANCELLED';
}

export function runStateLabel(state: AssistantRunState): string {
  if (state === 'RUN_STATE_QUEUED') return '排队中';
  if (state === 'RUN_STATE_RUNNING') return '正在回答';
  if (state === 'RUN_STATE_SUCCEEDED') return '已完成';
  if (state === 'RUN_STATE_FAILED') return '执行失败';
  return '已取消';
}

export function runErrorMessage(code: string, items: AssistantRunItem[] = []): string {
  const failed = items.find((item) => item.state === 'RUN_ITEM_STATE_FAILED');
  if (failed?.kind === 'RUN_ITEM_KIND_MODEL_CALL') return '模型调用失败，请检查模型连接后重试';
  if (failed?.name === 'load_skill') return '诊断流程加载失败，请稍后重试';
  if (failed && ['list_gateways', 'list_routes', 'list_services'].includes(failed.name)) {
    return '读取网关配置失败，请检查管理服务状态后重试';
  }
  if (failed && ['get_recent_traffic', 'list_recent_failures'].includes(failed.name)) {
    return '读取观测数据失败，请检查分析服务状态后重试';
  }
  if (code === 'MODEL_UNAVAILABLE') return '模型暂时不可用，请稍后重试';
  if (code === 'TOOL_UNAVAILABLE') return '助手工具暂时不可用，请稍后重试';
  if (code === 'EVENT_STORE_UNAVAILABLE') return '回答事件暂时无法传输，请稍后重试';
  if (code === 'WORKER_LOST' || code === 'WORKER_STOPPED') return '任务执行已中断，请重新发送';
  return '助手未能完成本次回答，请稍后重试';
}

export function runItemLabel(item: AssistantRunItem): string {
  if (item.kind === 'RUN_ITEM_KIND_MODEL_CALL') return '分析问题';
  if (item.name === 'load_skill') return '选择诊断流程';
  if (item.name === 'list_gateways') return '检查网关';
  if (item.name === 'list_routes') return '检查路由';
  if (item.name === 'list_services') return '检查服务';
  if (item.name === 'get_recent_traffic') return '检查近期流量';
  if (item.name === 'list_recent_failures') return '检查近期失败请求';
  return '检查系统信息';
}
