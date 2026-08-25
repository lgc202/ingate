export type AssistantMessageRole =
  | 'MESSAGE_ROLE_USER'
  | 'MESSAGE_ROLE_ASSISTANT';

export type AssistantRunState =
  | 'RUN_STATE_QUEUED'
  | 'RUN_STATE_RUNNING'
  | 'RUN_STATE_SUCCEEDED'
  | 'RUN_STATE_FAILED'
  | 'RUN_STATE_CANCELLED';

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
  | 'run.cancelled';

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

export function runErrorMessage(code: string): string {
  if (code === 'MODEL_UNAVAILABLE') return '模型暂时不可用，请稍后重试';
  if (code === 'EVENT_STORE_UNAVAILABLE') return '回答事件暂时无法传输，请稍后重试';
  if (code === 'WORKER_LOST' || code === 'WORKER_STOPPED') return '任务执行已中断，请重新发送';
  return '助手未能完成本次回答，请稍后重试';
}
