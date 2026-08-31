import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import { MessageSquareText, Settings2 } from 'lucide-react';
import {
  approveProposedChange,
  createAssistantConversation,
  createAgentExecution,
  deleteAssistantConversation,
  getAgentExecution,
  getModelConnection,
  listAssistantConversations,
  listAssistantMessages,
  listProposedChanges,
  rejectProposedChange,
  reviseProposedChange,
  updateAssistantConversation,
} from '@/api/assistant';
import { errorMessage } from '@/api/errors';
import { Badge, Button, PageFrame, Toast } from '@/components/ui';
import type {
  AgentExecution,
  AssistantConversation,
  AssistantMessage,
  ModelConnection,
  ProposedChange,
} from '@/domain/assistant';
import { executionStateLabel, isSettledExecution } from '@/domain/assistant';
import { AssistantConversationList } from './AssistantConversationList';
import { AssistantComposer } from './AssistantComposer';
import { AssistantMessageList } from './AssistantMessageList';
import { ConversationDialogs } from './ConversationDialogs';
import { ModelConnectionDrawer } from './ModelConnectionDrawer';
import {
  clearActiveConversationID,
  clearStoredExecution,
  readActiveConversationID,
  readStoredExecution,
  storeActiveConversationID,
  type StoredExecution,
} from './assistant-session';
import { useAssistantExecution } from './useAssistantExecution';

const emptyModelConnection: ModelConnection = {
  configured: false,
  connectionMode: 'MODEL_CONNECTION_MODE_INGATE',
  protocol: 'MODEL_PROTOCOL_OPENAI_COMPATIBLE',
  endpoint: '',
  model: '',
  apiKeyConfigured: false,
  timeoutSeconds: 120,
  maxOutputTokens: 4096,
  reasoningBudgetTokens: 0,
};

export function AssistantPage() {
  const [conversations, setConversations] = useState<AssistantConversation[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [messages, setMessages] = useState<AssistantMessage[]>([]);
  const [proposedChanges, setProposedChanges] = useState<ProposedChange[]>([]);
  const [connection, setConnection] = useState<ModelConnection>(emptyModelConnection);
  const [loading, setLoading] = useState(true);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [connectionOpen, setConnectionOpen] = useState(false);
  const [renameCandidate, setRenameCandidate] = useState<AssistantConversation | null>(null);
  const [renameTitle, setRenameTitle] = useState('');
  const [renaming, setRenaming] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<AssistantConversation | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [editingMessage, setEditingMessage] = useState<AssistantMessage | null>(null);
  const [changeDecision, setChangeDecision] = useState<{
    id: string;
    action: 'approve' | 'reject';
  } | null>(null);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const messageLoadGenerationRef = useRef(0);
  const selectedIDRef = useRef<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const promptHistoryRef = useRef<string[]>([]);
  const promptHistoryIndexRef = useRef(-1);
  const promptDraftRef = useRef('');
  const submitInFlightRef = useRef(false);

  const reloadConversation = useCallback(async (conversationID: string) => {
    const generation = ++messageLoadGenerationRef.current;
    setLoadingMessages(true);
    try {
      const [loadedMessages, loadedChanges] = await Promise.all([
        listAssistantMessages(conversationID),
        listProposedChanges(conversationID),
      ]);
      if (generation === messageLoadGenerationRef.current) {
        setMessages(loadedMessages);
        setProposedChanges(loadedChanges);
      }
    } catch (cause) {
      if (generation === messageLoadGenerationRef.current) {
        setNotice({
          message: errorMessage(cause, '加载会话消息失败'),
          tone: 'error',
        });
      }
    } finally {
      if (generation === messageLoadGenerationRef.current) setLoadingMessages(false);
    }
  }, []);

  const reloadConversations = useCallback(async () => {
    const page = await listAssistantConversations();
    setConversations(page.conversations ?? []);
    setNextCursor(page.nextCursor ?? '');
    return page.conversations ?? [];
  }, []);

  const {
    activeExecution,
    executionSteps,
    liveAnswer,
    cancelling,
    executionError,
    followExecution,
    resetExecutionView,
    showExecutionError,
    cancelExecution,
  } = useAssistantExecution({
    selectedConversationIDRef: selectedIDRef,
    reloadConversation,
    reloadConversations,
    onNotice: setNotice,
  });

  useEffect(() => {
    let cancelled = false;
    void Promise.allSettled([listAssistantConversations(), getModelConnection()])
      .then(async ([conversationResult, connectionResult]) => {
        if (cancelled) return;
        const items = conversationResult.status === 'fulfilled'
          ? conversationResult.value.conversations ?? []
          : [];
        setConversations(items);
        setNextCursor(conversationResult.status === 'fulfilled'
          ? conversationResult.value.nextCursor ?? ''
          : '');
        if (connectionResult.status === 'fulfilled') setConnection(connectionResult.value);
        let failure: unknown;
        if (conversationResult.status === 'rejected') {
          failure = conversationResult.reason;
        } else if (connectionResult.status === 'rejected') {
          failure = connectionResult.reason;
        }
        if (failure) {
          setNotice({
            message: failure instanceof Error ? failure.message : '加载运维助手失败',
            tone: 'error',
          });
        }

        const storedExecution = readStoredExecution();
        const resumableExecution = storedExecution
          && (conversationResult.status === 'rejected'
            || items.some((item) => item.id === storedExecution.conversationID))
          ? storedExecution
          : null;
        if (storedExecution && !resumableExecution) clearStoredExecution();
        const storedConversationID = readActiveConversationID();
        const firstID = resumableExecution?.conversationID
          ?? (storedConversationID && items.some((item) => item.id === storedConversationID)
            ? storedConversationID
            : items[0]?.id);
        if (firstID) {
          selectedIDRef.current = firstID;
          setSelectedID(firstID);
        }
        if (resumableExecution) {
          try {
            const execution = await getAgentExecution(resumableExecution.executionID);
            if (execution.conversationId !== resumableExecution.conversationID) {
              throw new Error('执行与会话的本地恢复信息不一致');
            }
            if (!cancelled && !isSettledExecution(execution.state)) {
              void followExecution(execution, resumableExecution);
            } else if (
              !cancelled
              && execution.state !== 'AGENT_EXECUTION_STATE_WAITING_APPROVAL'
            ) {
              clearStoredExecution();
            }
          } catch {
            clearStoredExecution();
            if (!cancelled && !items.some((item) => item.id === resumableExecution.conversationID)) {
              const fallbackID = items[0]?.id ?? null;
              selectedIDRef.current = fallbackID;
              setSelectedID(fallbackID);
            }
          }
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [followExecution]);

  useEffect(() => {
    if (!selectedID) {
      selectedIDRef.current = null;
      setMessages([]);
      setProposedChanges([]);
      return;
    }
    selectedIDRef.current = selectedID;
    storeActiveConversationID(selectedID);
    void reloadConversation(selectedID);
  }, [reloadConversation, selectedID]);

  const hasExecutingChange = proposedChanges.some(
    (change) => change.state === 'PROPOSED_CHANGE_STATE_EXECUTING',
  );

  useEffect(() => {
    if (!selectedID || !hasExecutingChange) return undefined;
    let cancelled = false;
    const refresh = async () => {
      try {
        const changes = await listProposedChanges(selectedID);
        if (!cancelled && selectedIDRef.current === selectedID) setProposedChanges(changes);
      } catch {
        // 审批请求本身会返回最终状态；轮询只负责恢复并发请求或进程中断后的状态。
      }
    };
    const timer = window.setInterval(() => void refresh(), 1_000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [hasExecutingChange, selectedID]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [liveAnswer?.content, messages, proposedChanges]);

  useEffect(() => {
    promptHistoryRef.current = messages
      .filter((message) => message.role === 'MESSAGE_ROLE_USER')
      .map((message) => message.content.trim())
      .filter(Boolean);
    promptHistoryIndexRef.current = -1;
    promptDraftRef.current = '';
  }, [messages]);

  const newConversation = () => {
    messageLoadGenerationRef.current += 1;
    selectedIDRef.current = null;
    setSelectedID(null);
    setMessages([]);
    setProposedChanges([]);
    setInput('');
    resetExecutionView();
    setEditingMessage(null);
    promptHistoryRef.current = [];
    promptHistoryIndexRef.current = -1;
    promptDraftRef.current = '';
    clearActiveConversationID();
    window.requestAnimationFrame(() => editorRef.current?.focus());
  };

  const selectConversation = (id: string) => {
    messageLoadGenerationRef.current += 1;
    selectedIDRef.current = id;
    setSelectedID(id);
    setMessages([]);
    setProposedChanges([]);
    resetExecutionView();
    setEditingMessage(null);
    promptHistoryRef.current = [];
    promptHistoryIndexRef.current = -1;
    promptDraftRef.current = '';
  };

  const loadMore = async () => {
    if (!nextCursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const page = await listAssistantConversations(100, nextCursor);
      setConversations((current) => [...current, ...(page.conversations ?? [])]);
      setNextCursor(page.nextCursor ?? '');
    } catch (cause) {
      setNotice({ message: errorMessage(cause, '加载更多会话失败'), tone: 'error' });
    } finally {
      setLoadingMore(false);
    }
  };

  const submit = async () => {
    const content = input.trim();
    if (!content || activeExecution || submitInFlightRef.current) return;
    if (!connection.configured) {
      setConnectionOpen(true);
      return;
    }
    submitInFlightRef.current = true;
    setSending(true);
    setInput('');
    resetExecutionView();
    try {
      let conversationID = selectedID;
      if (!conversationID) {
        const conversation = await createAssistantConversation(conversationTitle(content));
        conversationID = conversation.id;
        setConversations((current) => [conversation, ...current]);
        selectedIDRef.current = conversation.id;
        setSelectedID(conversation.id);
      }
      const pendingChanges = proposedChanges.filter(
        (change) => change.conversationId === conversationID
          && change.state === 'PROPOSED_CHANGE_STATE_PENDING_REVIEW',
      );
      if (pendingChanges.length > 1) {
        throw new Error('当前会话存在多个待审批变更，请先刷新页面');
      }
      let execution: AgentExecution;
      let storedExecution: StoredExecution | undefined;
      if (pendingChanges.length === 1) {
        const change = await reviseProposedChange(pendingChanges[0].id, content);
        execution = await getAgentExecution(change.executionId);
        const current = readStoredExecution();
        if (current?.executionID === execution.id) storedExecution = current;
      } else {
        execution = await createAgentExecution(conversationID, content);
      }
      setEditingMessage(null);
      promptHistoryRef.current = [...promptHistoryRef.current, content];
      promptHistoryIndexRef.current = -1;
      promptDraftRef.current = '';
      await reloadConversation(conversationID);
      void followExecution(execution, storedExecution);
    } catch (cause) {
      setInput(content);
      showExecutionError(errorMessage(cause, '发送消息失败'));
    } finally {
      submitInFlightRef.current = false;
      setSending(false);
    }
  };

  const removeConversation = async () => {
    if (!deleteCandidate || deleting) return;
    setDeleting(true);
    try {
      await deleteAssistantConversation(deleteCandidate.id);
      const next = conversations.filter((item) => item.id !== deleteCandidate.id);
      setConversations(next);
      if (selectedID === deleteCandidate.id) {
        const nextID = next[0]?.id ?? null;
        messageLoadGenerationRef.current += 1;
        selectedIDRef.current = nextID;
        setSelectedID(nextID);
        setMessages([]);
        setProposedChanges([]);
      }
      setDeleteCandidate(null);
    } catch (cause) {
      setNotice({ message: errorMessage(cause, '删除会话失败'), tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const renameConversation = async () => {
    const title = renameTitle.trim();
    if (!renameCandidate || !title || renaming) return;
    setRenaming(true);
    try {
      const saved = await updateAssistantConversation(renameCandidate.id, title);
      setConversations((current) => current.map((item) => (item.id === saved.id ? saved : item)));
      setRenameCandidate(null);
      setNotice({ message: '会话名称已更新', tone: 'success' });
    } catch (cause) {
      setNotice({ message: errorMessage(cause, '更新会话名称失败'), tone: 'error' });
    } finally {
      setRenaming(false);
    }
  };

  const decideChange = async (change: ProposedChange, action: 'approve' | 'reject') => {
    if (changeDecision) return;
    setChangeDecision({ id: change.id, action });
    try {
      const saved = action === 'approve'
        ? await approveProposedChange(change.id)
        : await rejectProposedChange(change.id);
      setProposedChanges((current) => current.map((item) => (
        item.id === saved.id ? saved : item
      )));
      const execution = await getAgentExecution(saved.executionId);
      await reloadConversation(saved.conversationId);
      const storedExecution = readStoredExecution();
      void followExecution(
        execution,
        storedExecution?.executionID === execution.id ? storedExecution : undefined,
      );
    } catch (cause) {
      if (selectedIDRef.current === change.conversationId) {
        try {
          setProposedChanges(await listProposedChanges(change.conversationId));
        } catch {
          // 保留当前审批卡，原始错误仍会提示用户。
        }
      }
      setNotice({
        message: errorMessage(
          cause,
          action === 'approve' ? '批准配置变更失败' : '拒绝配置变更失败',
        ),
        tone: 'error',
      });
    } finally {
      setChangeDecision(null);
    }
  };

  const handleEditorKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.nativeEvent.isComposing) return;
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void submit();
      return;
    }
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;

    const history = promptHistoryRef.current;
    if (event.key === 'ArrowUp' && history.length > 0) {
      const index = promptHistoryIndexRef.current;
      const beforeCaret = event.currentTarget.value.slice(0, event.currentTarget.selectionStart);
      // 多行文本中保留方向键的正常移动；开始浏览历史后，方向键只用于切换历史提示词。
      if (index < 0 && beforeCaret.includes('\n')) return;
      event.preventDefault();
      if (index < 0) promptDraftRef.current = input;
      const nextIndex = index < 0 ? history.length - 1 : Math.max(0, index - 1);
      promptHistoryIndexRef.current = nextIndex;
      setEditorValue(history[nextIndex]);
      return;
    }
    if (event.key === 'ArrowDown' && promptHistoryIndexRef.current >= 0) {
      event.preventDefault();
      const nextIndex = promptHistoryIndexRef.current + 1;
      if (nextIndex >= history.length) {
        promptHistoryIndexRef.current = -1;
        setEditorValue(promptDraftRef.current);
        promptDraftRef.current = '';
        return;
      }
      promptHistoryIndexRef.current = nextIndex;
      setEditorValue(history[nextIndex]);
    }
  };

  const setEditorValue = (value: string) => {
    setInput(value);
    window.requestAnimationFrame(() => {
      editorRef.current?.setSelectionRange(value.length, value.length);
    });
  };

  const selectedConversation = conversations.find((item) => item.id === selectedID);
  const currentExecution = activeExecution?.conversationId === selectedID ? activeExecution : null;
  const currentLiveAnswer = liveAnswer?.conversationID === selectedID ? liveAnswer : null;
  const currentExecutionSteps = currentExecution ? executionSteps : [];

  return (
    <PageFrame
      title="运维助手"
      actions={(
        <div className="assistant-header-actions">
          {connection.configured ? (
            <div className="assistant-model-status">
              <i aria-hidden="true" />
              <span>{connection.model}</span>
            </div>
          ) : <Badge tone="warning">尚未配置模型</Badge>}
          <Button variant="outline" onClick={() => setConnectionOpen(true)}>
            <Settings2 className="h-4 w-4" aria-hidden="true" />
            模型设置
          </Button>
        </div>
      )}
    >
      <Toast
        message={notice?.message ?? null}
        tone={notice?.tone}
        onClose={() => setNotice(null)}
      />
      <section className="assistant-workbench">
        <AssistantConversationList
          conversations={conversations}
          selectedID={selectedID}
          activeExecutionConversationID={activeExecution?.conversationId}
          loading={loading}
          loadingMore={loadingMore}
          hasMore={Boolean(nextCursor)}
          onNew={newConversation}
          onSelect={selectConversation}
          onRename={(conversation) => {
            setRenameCandidate(conversation);
            setRenameTitle(conversation.title);
          }}
          onDelete={(conversation) => {
            if (activeExecution?.conversationId === conversation.id) {
              setNotice({ message: '请先停止当前回答，再删除该会话', tone: 'error' });
              return;
            }
            setDeleteCandidate(conversation);
          }}
          onLoadMore={() => void loadMore()}
        />

        <div className="assistant-chat">
          <header className="assistant-chat-header">
            <div>
              <MessageSquareText aria-hidden="true" />
              <div>
                <strong>{selectedConversation?.title ?? '新会话'}</strong>
                <span>{currentExecution
                  ? executionStateLabel(currentExecution.state)
                  : '面向 Ingate 配置与排障场景'}</span>
              </div>
            </div>
            {currentExecution ? (
              <Badge
                tone={currentExecution.state === 'AGENT_EXECUTION_STATE_RUNNING' ? 'accent' : 'neutral'}
              >
                {executionStateLabel(currentExecution.state)}
              </Badge>
            ) : null}
          </header>

          <AssistantMessageList
            messages={messages}
            loading={loadingMessages}
            hasConversation={Boolean(selectedID)}
            configured={connection.configured}
            liveAnswer={currentLiveAnswer}
            execution={currentExecution}
            executionSteps={currentExecutionSteps}
            proposedChanges={proposedChanges}
            changeDecision={changeDecision}
            error={executionError}
            endRef={messagesEndRef}
            onConfigure={() => setConnectionOpen(true)}
            onEdit={(message) => {
              setEditingMessage(message);
              setEditorValue(message.content);
              window.requestAnimationFrame(() => editorRef.current?.focus());
            }}
            onSuggestion={(value) => {
              setEditingMessage(null);
              setInput(value);
              window.requestAnimationFrame(() => editorRef.current?.focus());
            }}
            onApproveChange={(change) => void decideChange(change, 'approve')}
            onRejectChange={(change) => void decideChange(change, 'reject')}
          />

          <AssistantComposer
            activeExecution={activeExecution}
            currentExecution={currentExecution}
            editingMessage={editingMessage}
            input={input}
            modelConfigured={connection.configured}
            sending={sending}
            cancelling={cancelling}
            editorRef={editorRef}
            onReturnToActiveConversation={selectConversation}
            onCancelEdit={() => {
              setEditingMessage(null);
              setInput('');
              window.requestAnimationFrame(() => editorRef.current?.focus());
            }}
            onInputChange={(value) => {
              promptHistoryIndexRef.current = -1;
              promptDraftRef.current = '';
              setInput(value);
            }}
            onKeyDown={handleEditorKeyDown}
            onCancelExecution={() => void cancelExecution()}
            onSubmit={() => void submit()}
          />
        </div>
      </section>

      <ModelConnectionDrawer
        connection={connection}
        open={connectionOpen}
        onClose={() => setConnectionOpen(false)}
        onSaved={(saved) => {
          setConnection(saved);
          setNotice({ message: '模型连接已保存', tone: 'success' });
        }}
      />

      <ConversationDialogs
        renameCandidate={renameCandidate}
        renameTitle={renameTitle}
        renaming={renaming}
        deleteCandidate={deleteCandidate}
        deleting={deleting}
        onRenameTitleChange={setRenameTitle}
        onCloseRename={() => setRenameCandidate(null)}
        onRename={() => void renameConversation()}
        onCloseDelete={() => setDeleteCandidate(null)}
        onDelete={() => void removeConversation()}
      />
    </PageFrame>
  );
}

function conversationTitle(content: string): string {
  const firstLine = content.split('\n', 1)[0].trim();
  const characters = Array.from(firstLine);
  return characters.length > 48 ? `${characters.slice(0, 48).join('')}…` : firstLine;
}
