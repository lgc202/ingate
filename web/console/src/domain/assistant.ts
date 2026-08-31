export type AssistantMessageRole =
  | 'MESSAGE_ROLE_USER'
  | 'MESSAGE_ROLE_ASSISTANT';

export type AgentExecutionState =
  | 'AGENT_EXECUTION_STATE_QUEUED'
  | 'AGENT_EXECUTION_STATE_RUNNING'
  | 'AGENT_EXECUTION_STATE_SUCCEEDED'
  | 'AGENT_EXECUTION_STATE_FAILED'
  | 'AGENT_EXECUTION_STATE_CANCELLED'
  | 'AGENT_EXECUTION_STATE_WAITING_APPROVAL';

export type AgentExecutionStepKind =
  | 'AGENT_EXECUTION_STEP_KIND_MODEL_CALL'
  | 'AGENT_EXECUTION_STEP_KIND_TOOL_CALL';

export type AgentExecutionStepState =
  | 'AGENT_EXECUTION_STEP_STATE_RUNNING'
  | 'AGENT_EXECUTION_STEP_STATE_COMPLETED'
  | 'AGENT_EXECUTION_STEP_STATE_FAILED'
  | 'AGENT_EXECUTION_STEP_STATE_CANCELLED'
  | 'AGENT_EXECUTION_STEP_STATE_WAITING_APPROVAL';

export type ProposedChangeKind =
  | 'PROPOSED_CHANGE_KIND_CREATE_GATEWAY'
  | 'PROPOSED_CHANGE_KIND_CREATE_SERVICE';

export type ProposedChangeState =
  | 'PROPOSED_CHANGE_STATE_PENDING_REVIEW'
  | 'PROPOSED_CHANGE_STATE_EXECUTING'
  | 'PROPOSED_CHANGE_STATE_SUCCEEDED'
  | 'PROPOSED_CHANGE_STATE_REJECTED'
  | 'PROPOSED_CHANGE_STATE_FAILED'
  | 'PROPOSED_CHANGE_STATE_OUTCOME_UNKNOWN';

export type ProposedGatewayProtocol =
  | 'PROPOSED_GATEWAY_PROTOCOL_HTTP'
  | 'PROPOSED_GATEWAY_PROTOCOL_HTTPS';

export type ProposedServiceLoadBalancing =
  | 'PROPOSED_SERVICE_LOAD_BALANCING_ROUND_ROBIN'
  | 'PROPOSED_SERVICE_LOAD_BALANCING_LEAST_REQUEST';

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
  startedAt: string;
  finishedAt?: string;
}

export interface ProposedChange {
  id: string;
  conversationId: string;
  executionId: string;
  kind: ProposedChangeKind;
  state: ProposedChangeState;
  summary: string;
  createGateway?: ProposedGateway;
  createService?: ProposedService;
  resourceId: string;
  errorCode: string;
  createdAt: string;
  decidedAt?: string;
  finishedAt?: string;
}

export interface ProposedGateway {
  name: string;
  enabled: boolean;
  listeners: ProposedGatewayListener[];
}

export interface ProposedGatewayListener {
  name: string;
  protocol: ProposedGatewayProtocol;
  port: number;
  hostname: string;
  certificateID: string;
}

export interface ProposedService {
  name: string;
  endpoints: ProposedServiceEndpoint[];
  tlsServerName: string;
  loadBalancing: ProposedServiceLoadBalancing;
  healthCheck?: ProposedServiceHealthCheck;
}

export interface ProposedServiceEndpoint {
  address: string;
  port: number;
  weight: number;
}

export interface ProposedServiceHealthCheck {
  path: string;
  intervalSeconds: number;
  timeoutSeconds: number;
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
  | 'execution.interrupted'
  | 'stream.failed';

export interface AssistantStreamEvent {
  id: string;
  type: AssistantStreamEventType;
  value: string;
}

const configurationTools = new Set([
  'list_gateways',
  'list_routes',
  'list_services',
  'get_route_configuration',
  'get_caller_token_quota',
]);

const observabilityTools = new Set([
  'analyze_traffic',
  'list_recent_failures',
  'get_request_record',
]);

const toolStepLabels: Record<string, string> = {
  list_gateways: '检查网关',
  list_routes: '检查路由',
  list_services: '检查服务',
  get_route_configuration: '核对路由配置',
  analyze_traffic: '分析近期流量',
  list_recent_failures: '检查近期失败请求',
  get_request_record: '检查请求明细',
  get_caller_token_quota: '检查调用方额度',
  create_gateway: '创建网关',
  create_service: '创建服务',
};

export function isTerminalExecution(state: AgentExecutionState): boolean {
  return state === 'AGENT_EXECUTION_STATE_SUCCEEDED'
    || state === 'AGENT_EXECUTION_STATE_FAILED'
    || state === 'AGENT_EXECUTION_STATE_CANCELLED';
}

export function isSettledExecution(state: AgentExecutionState): boolean {
  return isTerminalExecution(state) || state === 'AGENT_EXECUTION_STATE_WAITING_APPROVAL';
}

export function executionStateLabel(state: AgentExecutionState): string {
  switch (state) {
    case 'AGENT_EXECUTION_STATE_QUEUED':
      return '排队中';
    case 'AGENT_EXECUTION_STATE_RUNNING':
      return '正在回答';
    case 'AGENT_EXECUTION_STATE_SUCCEEDED':
      return '已完成';
    case 'AGENT_EXECUTION_STATE_FAILED':
      return '执行失败';
    case 'AGENT_EXECUTION_STATE_CANCELLED':
      return '已取消';
    case 'AGENT_EXECUTION_STATE_WAITING_APPROVAL':
      return '等待审批';
  }
}

export function executionErrorMessage(code: string, steps: AgentExecutionStep[] = []): string {
  const failed = steps.find((step) => step.state === 'AGENT_EXECUTION_STEP_STATE_FAILED');
  if (failed?.kind === 'AGENT_EXECUTION_STEP_KIND_MODEL_CALL') return '模型调用失败，请检查模型连接后重试';
  if (failed && configurationTools.has(failed.name)) {
    return '读取网关配置失败，请检查管理服务状态后重试';
  }
  if (failed && observabilityTools.has(failed.name)) {
    return '观测查询暂时不可用，请稍后重试';
  }
  if (code === 'MODEL_UNAVAILABLE') return '模型暂时不可用，请稍后重试';
  if (code === 'TOOL_UNAVAILABLE') return '助手工具暂时不可用，请稍后重试';
  if (code === 'AGENT_ITERATION_LIMIT') return '问题范围过大，助手未能在限定步骤内完成，请缩小范围后重试';
  if (code === 'WORKER_LOST' || code === 'WORKER_STOPPED') return '任务执行已中断，请重新发送';
  return '助手未能完成本次回答，请稍后重试';
}

export function executionStepLabel(step: AgentExecutionStep): string {
  if (step.kind === 'AGENT_EXECUTION_STEP_KIND_MODEL_CALL') return '分析问题';
  return toolStepLabels[step.name] ?? '检查系统信息';
}

export function proposedChangeStateLabel(state: ProposedChangeState): string {
  switch (state) {
    case 'PROPOSED_CHANGE_STATE_PENDING_REVIEW':
      return '待审批';
    case 'PROPOSED_CHANGE_STATE_EXECUTING':
      return '执行中';
    case 'PROPOSED_CHANGE_STATE_SUCCEEDED':
      return '已创建';
    case 'PROPOSED_CHANGE_STATE_REJECTED':
      return '已拒绝';
    case 'PROPOSED_CHANGE_STATE_FAILED':
      return '创建失败';
    case 'PROPOSED_CHANGE_STATE_OUTCOME_UNKNOWN':
      return '结果待确认';
  }
}

export function proposedChangeErrorMessage(change: ProposedChange): string {
  if (change.state === 'PROPOSED_CHANGE_STATE_FAILED') {
    return '管理服务明确拒绝了这项配置，请根据当前资源状态重新生成提案。';
  }
  if (change.state === 'PROPOSED_CHANGE_STATE_OUTCOME_UNKNOWN') {
    return '创建请求的结果无法确认。系统不会自动重试，请先在资源列表中核对后再决定。';
  }
  return '';
}
