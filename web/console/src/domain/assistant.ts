export type AssistantMessageRole =
  | 'MESSAGE_ROLE_USER'
  | 'MESSAGE_ROLE_ASSISTANT';

export type AgentExecutionState =
  | 'AGENT_EXECUTION_STATE_QUEUED'
  | 'AGENT_EXECUTION_STATE_RUNNING'
  | 'AGENT_EXECUTION_STATE_SUCCEEDED'
  | 'AGENT_EXECUTION_STATE_FAILED'
  | 'AGENT_EXECUTION_STATE_CANCELLED';

export type AgentExecutionStepKind =
  | 'AGENT_EXECUTION_STEP_KIND_MODEL_CALL'
  | 'AGENT_EXECUTION_STEP_KIND_TOOL_CALL';

export type AgentExecutionStepState =
  | 'AGENT_EXECUTION_STEP_STATE_RUNNING'
  | 'AGENT_EXECUTION_STEP_STATE_COMPLETED'
  | 'AGENT_EXECUTION_STEP_STATE_FAILED'
  | 'AGENT_EXECUTION_STEP_STATE_CANCELLED';

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
  executionId: string;
  role: AssistantMessageRole;
  content: string;
  reasoningContent: string;
  createdAt: string;
}

export interface AgentExecution {
  id: string;
  conversationId: string;
  state: AgentExecutionState;
  model: string;
  errorCode: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  cancellationRequested: boolean;
}

export interface AgentExecutionStep {
  id: string;
  executionId: string;
  sequence: number;
  kind: AgentExecutionStepKind;
  state: AgentExecutionStepState;
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
  | 'execution.started'
  | 'message.reasoning.delta'
  | 'message.content.delta'
  | 'execution.completed'
  | 'execution.failed'
  | 'execution.cancelled'
  | 'stream.failed';

export interface AssistantStreamEvent {
  id: string;
  type: AssistantStreamEventType;
  value: string;
}

export function isTerminalExecution(state: AgentExecutionState): boolean {
  return state === 'AGENT_EXECUTION_STATE_SUCCEEDED'
    || state === 'AGENT_EXECUTION_STATE_FAILED'
    || state === 'AGENT_EXECUTION_STATE_CANCELLED';
}

export function executionStateLabel(state: AgentExecutionState): string {
  if (state === 'AGENT_EXECUTION_STATE_QUEUED') return '排队中';
  if (state === 'AGENT_EXECUTION_STATE_RUNNING') return '正在回答';
  if (state === 'AGENT_EXECUTION_STATE_SUCCEEDED') return '已完成';
  if (state === 'AGENT_EXECUTION_STATE_FAILED') return '执行失败';
  return '已取消';
}

export function executionErrorMessage(code: string, steps: AgentExecutionStep[] = []): string {
  const failed = steps.find((step) => step.state === 'AGENT_EXECUTION_STEP_STATE_FAILED');
  if (failed?.kind === 'AGENT_EXECUTION_STEP_KIND_MODEL_CALL') return '模型调用失败，请检查模型连接后重试';
  if (failed && ['list_gateways', 'list_routes', 'list_services'].includes(failed.name)) {
    return '读取网关配置失败，请检查管理服务状态后重试';
  }
  if (failed && ['get_recent_traffic', 'list_recent_failures'].includes(failed.name)) {
    return '观测查询暂时不可用，请稍后重试';
  }
  if (code === 'MODEL_UNAVAILABLE') return '模型暂时不可用，请稍后重试';
  if (code === 'TOOL_UNAVAILABLE') return '助手工具暂时不可用，请稍后重试';
  if (code === 'EVENT_STORE_UNAVAILABLE') return '回答事件暂时无法传输，请稍后重试';
  if (code === 'WORKER_LOST' || code === 'WORKER_STOPPED') return '任务执行已中断，请重新发送';
  return '助手未能完成本次回答，请稍后重试';
}

export function executionStepLabel(step: AgentExecutionStep): string {
  if (step.kind === 'AGENT_EXECUTION_STEP_KIND_MODEL_CALL') return '分析问题';
  if (step.name === 'list_gateways') return '检查网关';
  if (step.name === 'list_routes') return '检查路由';
  if (step.name === 'list_services') return '检查服务';
  if (step.name === 'get_recent_traffic') return '检查近期流量';
  if (step.name === 'list_recent_failures') return '检查近期失败请求';
  return '检查系统信息';
}
