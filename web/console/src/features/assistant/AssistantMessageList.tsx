import type { RefObject } from 'react';
import { Bot, CircleAlert, Settings2, Sparkles, UserRound } from 'lucide-react';
import { Button } from '@/components/ui';
import type { AssistantMessage, AssistantRun } from '@/domain/assistant';
import { runStateLabel } from '@/domain/assistant';

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
  run: AssistantRun | null;
  error: string;
  endRef: RefObject<HTMLDivElement | null>;
  onConfigure: () => void;
  onSuggestion: (value: string) => void;
}

export function AssistantMessageList({
  messages,
  loading,
  hasConversation,
  configured,
  liveAnswer,
  run,
  error,
  endRef,
  onConfigure,
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
      {messages.map((message) => <MessageBubble key={message.id} message={message} />)}
      {liveAnswer || run ? <LiveAnswerBubble answer={liveAnswer} run={run} /> : null}
      {error ? (
        <div className="assistant-run-error" role="alert">
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

function MessageBubble({ message }: { message: AssistantMessage }) {
  const user = message.role === 'MESSAGE_ROLE_USER';
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
        <p className="assistant-message-content">{message.content}</p>
      </div>
    </article>
  );
}

function LiveAnswerBubble({ answer, run }: { answer: LiveAnswer | null; run: AssistantRun | null }) {
  return (
    <article className="assistant-message is-assistant is-live">
      <span><Bot aria-hidden="true" /></span>
      <div>
        <strong>Ingate 助手</strong>
        {answer?.reasoning ? (
          <details className="assistant-reasoning" open={!answer.content}>
            <summary>思考过程</summary>
            <p>{answer.reasoning}</p>
          </details>
        ) : null}
        {answer?.content ? (
          <p className="assistant-message-content">{answer.content}<i className="assistant-caret" /></p>
        ) : (
          <div className="assistant-typing" aria-label={run ? runStateLabel(run.state) : '正在回答'}>
            <i /><i /><i />
          </div>
        )}
      </div>
    </article>
  );
}
