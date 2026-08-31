import type { RefObject } from 'react';
import { useEffect, useState } from 'react';
import { Bot, Check, CircleAlert, Copy, LoaderCircle, PencilLine, Settings2, Sparkles, UserRound } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Button } from '@/components/ui';
import type { AgentExecution, AgentExecutionStep, AssistantMessage } from '@/domain/assistant';
import { executionStateLabel, executionStepLabel } from '@/domain/assistant';

const suggestions = [
  '解释 Gateway、Route 和 Service 的关系',
  '路由没有转发到服务时应该检查什么？',
  '如何判断请求失败发生在客户端还是服务端？',
];

export interface LiveAnswer {
  conversationID: string;
  content: string;
  reasoning: string;
}

interface AssistantMessageListProps {
  messages: AssistantMessage[];
  loading: boolean;
  hasConversation: boolean;
  configured: boolean;
  liveAnswer: LiveAnswer | null;
  execution: AgentExecution | null;
  executionSteps: AgentExecutionStep[];
  error: string;
  endRef: RefObject<HTMLDivElement | null>;
  onConfigure: () => void;
  onEdit: (message: AssistantMessage) => void;
  onSuggestion: (value: string) => void;
}

export function AssistantMessageList({
  messages,
  loading,
  hasConversation,
  configured,
  liveAnswer,
  execution,
  executionSteps,
  error,
  endRef,
  onConfigure,
  onEdit,
  onSuggestion,
}: AssistantMessageListProps) {
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
      {messages.map((message) => (
        <MessageBubble key={message.id} message={message} onEdit={onEdit} />
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
      <p>询问 Ingate 的资源关系、配置方法与排障思路。当前助手不会直接修改网关资源。</p>
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
        {!user && message.reasoningContent ? (
          <details className="assistant-reasoning">
            <summary>思考过程</summary>
            <p>{message.reasoningContent}</p>
          </details>
        ) : null}
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
        {answer?.reasoning ? (
          <details className="assistant-reasoning" open={!answer.content}>
            <summary>思考过程</summary>
            <p>{answer.reasoning}</p>
          </details>
        ) : null}
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
