import { type KeyboardEvent, type RefObject } from 'react';
import { PencilLine, Send, Square, X } from 'lucide-react';
import { Button } from '@/components/ui';
import type { AssistantMessage, AgentExecution } from '@/domain/assistant';

interface AssistantComposerProps {
  activeExecution: AgentExecution | null;
  currentExecution: AgentExecution | null;
  editingMessage: AssistantMessage | null;
  input: string;
  modelConfigured: boolean;
  sending: boolean;
  cancelling: boolean;
  editorRef: RefObject<HTMLTextAreaElement | null>;
  onReturnToActiveConversation: (conversationID: string) => void;
  onCancelEdit: () => void;
  onInputChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  onCancelExecution: () => void;
  onSubmit: () => void;
}

export function AssistantComposer({
  activeExecution,
  currentExecution,
  editingMessage,
  input,
  modelConfigured,
  sending,
  cancelling,
  editorRef,
  onReturnToActiveConversation,
  onCancelEdit,
  onInputChange,
  onKeyDown,
  onCancelExecution,
  onSubmit,
}: AssistantComposerProps) {
  const inputDisabled = Boolean(activeExecution) || sending;

  return (
    <div className="assistant-composer-wrap">
      {activeExecution && !currentExecution ? (
        <button
          className="assistant-active-execution-link"
          type="button"
          onClick={() => onReturnToActiveConversation(activeExecution.conversationId)}
        >
          助手正在另一个会话中回答，点击返回
        </button>
      ) : null}
      <div className="assistant-composer">
        {editingMessage ? (
          <div className="assistant-editing-prompt">
            <PencilLine aria-hidden="true" />
            <span>
              <strong>编辑并重新发送</strong>
              原消息会保留在会话中
            </span>
            <button type="button" aria-label="取消编辑" onClick={onCancelEdit}>
              <X aria-hidden="true" />
            </button>
          </div>
        ) : null}
        <textarea
          ref={editorRef}
          value={input}
          rows={3}
          maxLength={65536}
          disabled={inputDisabled}
          placeholder={modelConfigured ? '描述问题，或询问 Ingate 的配置与排障方法' : '请先配置助手使用的模型'}
          onChange={(event) => onInputChange(event.target.value)}
          onKeyDown={onKeyDown}
        />
        <footer>
          <span>Enter 发送 · Shift + Enter 换行 · ↑/↓ 历史提示</span>
          {currentExecution ? (
            <Button variant="outline" disabled={cancelling} onClick={onCancelExecution}>
              <Square className="h-3.5 w-3.5 fill-current" aria-hidden="true" />
              {cancelling ? '正在停止' : '停止回答'}
            </Button>
          ) : (
            <Button disabled={!input.trim() || inputDisabled} onClick={onSubmit}>
              <Send className="h-4 w-4" aria-hidden="true" />
              {sending ? '发送中...' : editingMessage ? '重新发送' : '发送'}
            </Button>
          )}
        </footer>
      </div>
    </div>
  );
}
