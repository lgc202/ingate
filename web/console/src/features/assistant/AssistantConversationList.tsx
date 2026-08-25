import { useState } from 'react';
import { MessageSquareText, MoreHorizontal, Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui';
import type { AssistantConversation } from '@/domain/assistant';

interface AssistantConversationListProps {
  conversations: AssistantConversation[];
  selectedID: string | null;
  activeRunConversationID?: string;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onNew: () => void;
  onSelect: (id: string) => void;
  onDelete: (conversation: AssistantConversation) => void;
  onLoadMore: () => void;
}

export function AssistantConversationList({
  conversations,
  selectedID,
  activeRunConversationID,
  loading,
  loadingMore,
  hasMore,
  onNew,
  onSelect,
  onDelete,
  onLoadMore,
}: AssistantConversationListProps) {
  return (
    <aside className="assistant-conversations">
      <div className="assistant-conversation-toolbar">
        <Button className="w-full" onClick={onNew}>
          <Plus className="h-4 w-4" aria-hidden="true" />
          新建会话
        </Button>
      </div>
      <div className="assistant-conversation-list">
        {loading ? <p className="assistant-list-state">正在加载会话</p> : null}
        {!loading && conversations.length === 0 ? (
          <p className="assistant-list-state">还没有会话</p>
        ) : null}
        {conversations.map((conversation) => (
          <ConversationItem
            key={conversation.id}
            conversation={conversation}
            active={selectedID === conversation.id}
            running={activeRunConversationID === conversation.id}
            onSelect={() => onSelect(conversation.id)}
            onDelete={() => onDelete(conversation)}
          />
        ))}
        {hasMore ? (
          <button className="assistant-load-more" type="button" disabled={loadingMore} onClick={onLoadMore}>
            {loadingMore ? '加载中...' : '加载更多'}
          </button>
        ) : null}
      </div>
    </aside>
  );
}

function ConversationItem({
  conversation,
  active,
  running,
  onSelect,
  onDelete,
}: {
  conversation: AssistantConversation;
  active: boolean;
  running: boolean;
  onSelect: () => void;
  onDelete: () => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className={`assistant-conversation-item${active ? ' is-active' : ''}`}>
      <button type="button" onClick={onSelect}>
        <MessageSquareText aria-hidden="true" />
        <span>
          <strong>{conversation.title}</strong>
          <small>{running ? '正在回答' : formatConversationTime(conversation.updatedAt)}</small>
        </span>
      </button>
      <div className="assistant-conversation-more">
        <button
          type="button"
          aria-label="会话操作"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((current) => !current)}
        >
          <MoreHorizontal aria-hidden="true" />
        </button>
        {menuOpen ? (
          <div>
            <button type="button" onClick={() => { setMenuOpen(false); onDelete(); }}>
              <Trash2 aria-hidden="true" />删除
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function formatConversationTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const now = new Date();
  if (date.toDateString() === now.toDateString()) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  }
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
}
