import type { RefObject } from 'react';
import { useEffect, useState } from 'react';
import {
  Bot,
  Check,
  CircleAlert,
  Copy,
  LoaderCircle,
  Network,
  PencilLine,
  Server,
  Settings2,
  ShieldCheck,
  Sparkles,
  UserRound,
  X,
} from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Button } from '@/components/ui';
import type {
  AgentExecution,
  AgentExecutionStep,
  AssistantMessage,
  ProposedChange,
} from '@/domain/assistant';
import {
  executionStateLabel,
  executionStepLabel,
  proposedChangeErrorMessage,
  proposedChangeStateLabel,
} from '@/domain/assistant';

const suggestions = [
  '创建一个名为 public 的网关，监听 HTTP 端口 80',
  '创建一个名为 orders 的服务，地址是 orders.default.svc，端口 8080',
  '解释 Gateway、Route 和 Service 的关系',
];

export interface LiveAnswer {
  conversationID: string;
  content: string;
}

type TimelineItem =
  | {
    id: string;
    kind: 'message';
    createdAt: string;
    message: AssistantMessage;
  }
  | {
    id: string;
    kind: 'change';
    createdAt: string;
    change: ProposedChange;
  };

interface AssistantMessageListProps {
  messages: AssistantMessage[];
  loading: boolean;
  hasConversation: boolean;
  configured: boolean;
  liveAnswer: LiveAnswer | null;
  execution: AgentExecution | null;
  executionSteps: AgentExecutionStep[];
  proposedChanges: ProposedChange[];
  changeDecision: { id: string; action: 'approve' | 'reject' } | null;
  error: string;
  endRef: RefObject<HTMLDivElement | null>;
  onConfigure: () => void;
  onEdit: (message: AssistantMessage) => void;
  onSuggestion: (value: string) => void;
  onApproveChange: (change: ProposedChange) => void;
  onRejectChange: (change: ProposedChange) => void;
}

export function AssistantMessageList({
  messages,
  loading,
  hasConversation,
  configured,
  liveAnswer,
  execution,
  executionSteps,
  proposedChanges,
  changeDecision,
  error,
  endRef,
  onConfigure,
  onEdit,
  onSuggestion,
  onApproveChange,
  onRejectChange,
}: AssistantMessageListProps) {
  const timeline: TimelineItem[] = [
    ...messages.map((message): TimelineItem => ({
      id: `message:${message.id}`,
      kind: 'message',
      createdAt: message.createdAt,
      message,
    })),
    ...proposedChanges.map((change): TimelineItem => ({
      id: `change:${change.id}`,
      kind: 'change',
      createdAt: change.createdAt,
      change,
    })),
  ].sort(compareTimelineItems);

  return (
    <div className="assistant-message-list">
      {loading && hasConversation ? <p className="assistant-message-loading">正在加载消息</p> : null}
      {!loading && messages.length === 0 && !liveAnswer ? (
        <AssistantWelcome
          configured={configured}
          onConfigure={onConfigure}
          onSuggestion={onSuggestion}
        />
      ) : null}
      {timeline.map((item) => (
        item.kind === 'message' ? (
          <MessageBubble key={item.id} message={item.message} onEdit={onEdit} />
        ) : (
          <ProposedChangeCard
            key={item.id}
            change={item.change}
            decision={changeDecision?.id === item.change.id ? changeDecision.action : null}
            onApprove={onApproveChange}
            onReject={onRejectChange}
          />
        )
      ))}
      {liveAnswer || execution ? (
        <LiveAnswerBubble answer={liveAnswer} execution={execution} steps={executionSteps} />
      ) : null}
      {error ? (
        <div className="assistant-execution-error" role="alert">
          <CircleAlert aria-hidden="true" />
          <span>{error}</span>
        </div>
      ) : null}
      <div ref={endRef} />
    </div>
  );
}

function compareTimelineItems(left: TimelineItem, right: TimelineItem): number {
  const timeOrder = timestampSortKey(left.createdAt).localeCompare(timestampSortKey(right.createdAt));
  if (timeOrder !== 0) return timeOrder;
  return left.id.localeCompare(right.id);
}

function timestampSortKey(value: string): string {
  const match = /^(.*?)(?:\.(\d+))?Z$/.exec(value);
  if (!match) return value;
  const fraction = (match[2] ?? '').padEnd(9, '0').slice(0, 9);
  return `${match[1]}.${fraction}Z`;
}

function AssistantWelcome({
  configured,
  onConfigure,
  onSuggestion,
}: {
  configured: boolean;
  onConfigure: () => void;
  onSuggestion: (value: string) => void;
}) {
  return (
    <div className="assistant-welcome">
      <span><Bot aria-hidden="true" /></span>
      <h2>从一个问题开始</h2>
      <p>查询当前配置和流量，或准备创建 Gateway、普通 HTTP Service 的审批项。未经你批准，助手不会修改系统。</p>
      {!configured ? (
        <Button onClick={onConfigure}>
          <Settings2 className="h-4 w-4" aria-hidden="true" />配置模型连接
        </Button>
      ) : (
        <div className="assistant-suggestions">
          {suggestions.map((suggestion) => (
            <button key={suggestion} type="button" onClick={() => onSuggestion(suggestion)}>
              <Sparkles aria-hidden="true" />
              <span>{suggestion}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function ProposedChangeCard({
  change,
  decision,
  onApprove,
  onReject,
}: {
  change: ProposedChange;
  decision: 'approve' | 'reject' | null;
  onApprove: (change: ProposedChange) => void;
  onReject: (change: ProposedChange) => void;
}) {
  const pending = change.state === 'PROPOSED_CHANGE_STATE_PENDING_REVIEW';
  const executing = change.state === 'PROPOSED_CHANGE_STATE_EXECUTING';
  const error = proposedChangeErrorMessage(change);
  return (
    <article className={`assistant-change is-${change.state.replace('PROPOSED_CHANGE_STATE_', '').toLowerCase()}`}>
      <header>
        <span>{change.kind === 'PROPOSED_CHANGE_KIND_CREATE_GATEWAY'
          ? <Network aria-hidden="true" />
          : <Server aria-hidden="true" />}</span>
        <div>
          <small>配置变更</small>
          <strong>{change.summary}</strong>
        </div>
        <i>{proposedChangeStateLabel(change.state)}</i>
      </header>
      {change.createGateway ? <GatewayChangeDetails change={change.createGateway} /> : null}
      {change.createService ? <ServiceChangeDetails change={change.createService} /> : null}
      {change.resourceId ? (
        <p className="assistant-change-result">
          <Check aria-hidden="true" />资源已创建，ID：<code>{change.resourceId}</code>
        </p>
      ) : null}
      {error ? (
        <p className="assistant-change-error">
          <CircleAlert aria-hidden="true" />{error}
        </p>
      ) : null}
      {pending || executing ? (
        <footer>
          <span><ShieldCheck aria-hidden="true" />批准后将立即写入当前环境</span>
          <div>
            <Button
              variant="outline"
              disabled={!pending || decision !== null}
              onClick={() => onReject(change)}
            >
              {decision === 'reject'
                ? <LoaderCircle className="assistant-spin" aria-hidden="true" />
                : <X aria-hidden="true" />}
              {decision === 'reject' ? '提交中' : '拒绝'}
            </Button>
            <Button disabled={!pending || decision !== null} onClick={() => onApprove(change)}>
              {executing || decision === 'approve'
                ? <LoaderCircle className="assistant-spin" aria-hidden="true" />
                : <Check aria-hidden="true" />}
              {executing ? '执行中' : decision === 'approve' ? '提交中' : '批准并创建'}
            </Button>
          </div>
        </footer>
      ) : null}
    </article>
  );
}

function GatewayChangeDetails({ change }: { change: NonNullable<ProposedChange['createGateway']> }) {
  return (
    <dl className="assistant-change-details">
      <div><dt>名称</dt><dd>{change.name}</dd></div>
      <div><dt>创建后</dt><dd>{change.enabled ? '立即启用' : '保持停用'}</dd></div>
      <div className="is-wide">
        <dt>监听入口</dt>
        <dd>{change.listeners.map((listener) => (
          <span key={listener.name}>
            <code>{listener.name}</code>
            {listener.protocol === 'PROPOSED_GATEWAY_PROTOCOL_HTTPS' ? 'HTTPS' : 'HTTP'}
            {' · '}{listener.hostname || '*'}:{listener.port}
            {listener.certificateID ? ` · 证书 ${listener.certificateID}` : ''}
          </span>
        ))}</dd>
      </div>
    </dl>
  );
}

function ServiceChangeDetails({ change }: { change: NonNullable<ProposedChange['createService']> }) {
  return (
    <dl className="assistant-change-details">
      <div><dt>名称</dt><dd>{change.name}</dd></div>
      <div>
        <dt>负载均衡</dt>
        <dd>{change.loadBalancing === 'PROPOSED_SERVICE_LOAD_BALANCING_LEAST_REQUEST'
          ? '最少请求'
          : '轮询'}</dd>
      </div>
      <div><dt>连接上游</dt><dd>{change.tlsServerName ? 'HTTPS' : 'HTTP'}</dd></div>
      <div className="is-wide">
        <dt>服务端点</dt>
        <dd>{change.endpoints.map((endpoint) => (
          <span key={`${endpoint.address}:${endpoint.port}`}>
            <code>{formatEndpoint(endpoint.address, endpoint.port)}</code>
            权重 {endpoint.weight}
          </span>
        ))}</dd>
      </div>
      {change.tlsServerName ? (
        <div><dt>HTTPS 服务名称</dt><dd>{change.tlsServerName}</dd></div>
      ) : null}
      {change.healthCheck ? (
        <div>
          <dt>健康检查</dt>
          <dd>{change.healthCheck.path} · {change.healthCheck.intervalSeconds}s / {change.healthCheck.timeoutSeconds}s</dd>
        </div>
      ) : null}
    </dl>
  );
}

function formatEndpoint(address: string, port: number): string {
  return address.includes(':') ? `[${address}]:${port}` : `${address}:${port}`;
}

function MessageBubble({
  message,
  onEdit,
}: {
  message: AssistantMessage;
  onEdit: (message: AssistantMessage) => void;
}) {
  const user = message.role === 'MESSAGE_ROLE_USER';
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>('idle');

  useEffect(() => {
    if (copyState === 'idle') return undefined;

    const timeout = window.setTimeout(() => setCopyState('idle'), 1500);
    return () => window.clearTimeout(timeout);
  }, [copyState]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(message.content);
      setCopyState('copied');
    } catch {
      setCopyState('failed');
    }
  };

  return (
    <article className={`assistant-message${user ? ' is-user' : ' is-assistant'}`}>
      <span>{user ? <UserRound aria-hidden="true" /> : <Bot aria-hidden="true" />}</span>
      <div>
        <strong>{user ? '你' : 'Ingate 助手'}</strong>
        {user ? (
          <p className="assistant-message-content">{message.content}</p>
        ) : (
          <MarkdownMessage content={message.content} />
        )}
        <div className="assistant-message-actions">
          <button type="button" onClick={() => void copy()}>
            {copyState === 'copied' ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
            {copyState === 'copied' ? '已复制' : copyState === 'failed' ? '复制失败' : '复制'}
          </button>
          {user ? (
            <button type="button" onClick={() => onEdit(message)}>
              <PencilLine aria-hidden="true" />
              编辑
            </button>
          ) : null}
        </div>
      </div>
    </article>
  );
}

function LiveAnswerBubble({
  answer,
  execution,
  steps,
}: {
  answer: LiveAnswer | null;
  execution: AgentExecution | null;
  steps: AgentExecutionStep[];
}) {
  return (
    <article className="assistant-message is-assistant is-live">
      <span><Bot aria-hidden="true" /></span>
      <div>
        <strong>Ingate 助手</strong>
        {steps.length ? <ExecutionProgress steps={steps} /> : null}
        {answer?.content ? (
          <MarkdownMessage content={answer.content} live />
        ) : (
          <div
            className="assistant-typing"
            aria-label={execution ? executionStateLabel(execution.state) : '正在回答'}
          >
            <i /><i /><i />
          </div>
        )}
      </div>
    </article>
  );
}

function MarkdownMessage({ content, live = false }: { content: string; live?: boolean }) {
  return (
    <div className="assistant-message-content assistant-markdown">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
      {live ? <i className="assistant-caret" /> : null}
    </div>
  );
}

function ExecutionProgress({ steps }: { steps: AgentExecutionStep[] }) {
  return (
    <div className="assistant-execution-progress" aria-label="执行进度">
      {steps.map((step) => (
        <div
          key={step.id}
          className={`is-${step.state.replace('AGENT_EXECUTION_STEP_STATE_', '').toLowerCase()}`}
        >
          {step.state === 'AGENT_EXECUTION_STEP_STATE_RUNNING'
            ? <LoaderCircle aria-hidden="true" />
            : step.state === 'AGENT_EXECUTION_STEP_STATE_WAITING_APPROVAL'
              ? <ShieldCheck aria-hidden="true" />
            : step.state === 'AGENT_EXECUTION_STEP_STATE_COMPLETED'
              ? <Check aria-hidden="true" />
              : <CircleAlert aria-hidden="true" />}
          <span>
            <strong>{executionStepLabel(step)}</strong>
            {step.summary ? <small>{step.summary}</small> : null}
          </span>
        </div>
      ))}
    </div>
  );
}
