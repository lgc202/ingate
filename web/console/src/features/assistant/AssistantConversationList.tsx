import { useEffect, useRef, useState } from 'react';
import { MessageSquareText, MoreHorizontal, Pencil, Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui';
import type { AssistantConversation } from '@/domain/assistant';

interface AssistantConversationListProps {
  conversations: AssistantConversation[];
  selectedID: string | null;
  activeExecutionConversationID?: string;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onNew: () => void;
  onSelect: (id: string) => void;
  onRename: (conversation: AssistantConversation) => void;
  onDelete: (conversation: AssistantConversation) => void;
  onLoadMore: () => void;
}

export function AssistantConversationList({
  conversations,
  selectedID,
  activeExecutionConversationID,
  loading,
  loadingMore,
  hasMore,
  onNew,
  onSelect,
  onRename,
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
            running={activeExecutionConversationID === conversation.id}
            onSelect={() => onSelect(conversation.id)}
            onRename={() => onRename(conversation)}
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
  onRename,
  onDelete,
}: {
  conversation: AssistantConversation;
  active: boolean;
  running: boolean;
  onSelect: () => void;
  onRename: () => void;
  onDelete: () => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return undefined;

    const closeOnPointerDown = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false);
    };
    const closeOnKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };
    const close = () => setMenuOpen(false);
    document.addEventListener('pointerdown', closeOnPointerDown);
    document.addEventListener('keydown', closeOnKeyDown);
    window.addEventListener('resize', close);
    window.addEventListener('scroll', close, true);
    return () => {
      document.removeEventListener('pointerdown', closeOnPointerDown);
      document.removeEventListener('keydown', closeOnKeyDown);
      window.removeEventListener('resize', close);
      window.removeEventListener('scroll', close, true);
    };
  }, [menuOpen]);

  return (
    <div className={`assistant-conversation-item${active ? ' is-active' : ''}`}>
      <button
        type="button"
        title={conversation.title}
        onClick={() => {
          setMenuOpen(false);
          onSelect();
        }}
      >
        <MessageSquareText aria-hidden="true" />
        <span>
          <strong title={conversation.title}>{conversation.title}</strong>
          <small>{running ? '正在回答' : formatConversationTime(conversation.updatedAt)}</small>
        </span>
      </button>
      <div ref={menuRef} className="assistant-conversation-more">
        <button
          type="button"
          aria-label="会话操作"
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((current) => !current)}
        >
          <MoreHorizontal aria-hidden="true" />
        </button>
        {menuOpen ? (
          <div role="menu">
            <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); onRename(); }}>
              <Pencil aria-hidden="true" />重命名
            </button>
            <button className="is-danger" type="button" role="menuitem" onClick={() => { setMenuOpen(false); onDelete(); }}>
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
