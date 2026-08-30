import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import {
  MessageSquareText,
  PencilLine,
  Send,
  Settings2,
  Square,
  X,
} from 'lucide-react';
import {
  cancelAgentExecution,
  createAssistantConversation,
  createAgentExecution,
  deleteAssistantConversation,
  getAgentExecution,
  getModelConnection,
  listAssistantConversations,
  listAssistantMessages,
  listAgentExecutionSteps,
  streamAgentExecution,
  updateAssistantConversation,
} from '@/api/assistant';
import { errorMessage } from '@/api/errors';
import { Badge, Button, Modal, PageFrame, Toast } from '@/components/ui';
import type {
  AssistantConversation,
  AssistantMessage,
  AgentExecution,
  AgentExecutionStep,
  AssistantStreamEvent,
  ModelConnection,
} from '@/domain/assistant';
import { executionErrorMessage, executionStateLabel, isTerminalExecution } from '@/domain/assistant';
import { AssistantConversationList } from './AssistantConversationList';
import { AssistantMessageList, type LiveAnswer } from './AssistantMessageList';
import { ModelConnectionDrawer } from './ModelConnectionDrawer';

const activeConversationKey = 'ingate.assistant.conversation';
const activeExecutionKey = 'ingate.assistant.active-execution';

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

interface StoredExecution {
  executionID: string;
  conversationID: string;
  lastEventID: string;
  content: string;
  reasoning: string;
}

export function AssistantPage() {
  const [conversations, setConversations] = useState<AssistantConversation[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [messages, setMessages] = useState<AssistantMessage[]>([]);
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
  const [editingMessage, setEditingMessage] = useState<AssistantMessage | null>(null);
  const [activeExecution, setActiveExecution] = useState<AgentExecution | null>(null);
  const [executionSteps, setExecutionSteps] = useState<AgentExecutionStep[]>([]);
  const [liveAnswer, setLiveAnswer] = useState<LiveAnswer | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [executionError, setExecutionError] = useState('');
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const streamAbortRef = useRef<AbortController | null>(null);
  const streamGenerationRef = useRef(0);
  const messageLoadGenerationRef = useRef(0);
  const selectedIDRef = useRef<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const promptHistoryRef = useRef<string[]>([]);
  const promptHistoryIndexRef = useRef(-1);
  const promptDraftRef = useRef('');

  const loadMessages = useCallback(async (conversationID: string) => {
    const generation = ++messageLoadGenerationRef.current;
    setLoadingMessages(true);
    try {
      const loaded = await listAssistantMessages(conversationID);
      if (generation === messageLoadGenerationRef.current) setMessages(loaded);
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

  const finishExecution = useCallback(async (execution: AgentExecution) => {
    let steps: AgentExecutionStep[] = [];
    try {
      steps = await listAgentExecutionSteps(execution.id);
      setExecutionSteps(steps);
    } catch {
      // 执行终态是最终事实；步骤读取失败不应覆盖回答结果。
    }
    if (selectedIDRef.current === execution.conversationId) {
      if (execution.state === 'AGENT_EXECUTION_STATE_FAILED') {
        setExecutionError(executionErrorMessage(execution.errorCode, steps));
      }
      if (execution.state === 'AGENT_EXECUTION_STATE_CANCELLED') setExecutionError('本次回答已取消');
    }
    const tasks: Promise<unknown>[] = [reloadConversations().catch(() => undefined)];
    if (selectedIDRef.current === execution.conversationId) tasks.push(loadMessages(execution.conversationId));
    await Promise.all(tasks);
  }, [loadMessages, reloadConversations]);

  const followExecution = useCallback(async (
    initialExecution: AgentExecution,
    storedExecution?: StoredExecution,
  ) => {
    const generation = ++streamGenerationRef.current;
    streamAbortRef.current?.abort();
    const abortController = new AbortController();
    streamAbortRef.current = abortController;
    let execution = initialExecution;
    let lastEventID = storedExecution?.lastEventID ?? '';
    let content = storedExecution?.content ?? '';
    let reasoning = storedExecution?.reasoning ?? '';
    let terminalFromEvent = false;
    setActiveExecution(execution);
    setExecutionSteps([]);
    setLiveAnswer({ conversationID: execution.conversationId, content, reasoning });
    setExecutionError('');
    storeActiveExecution({
      executionID: execution.id,
      conversationID: execution.conversationId,
      lastEventID,
      content,
      reasoning,
    });

    try {
      while (!abortController.signal.aborted && !isTerminalExecution(execution.state)) {
        terminalFromEvent = false;
        try {
          await streamAgentExecution(
            execution.id,
            lastEventID,
            abortController.signal,
            (event) => {
              if (event.id) {
                lastEventID = event.id;
              }
              if (event.type === 'execution.started') {
                setActiveExecution((current) => current
                  ? { ...current, state: 'AGENT_EXECUTION_STATE_RUNNING' }
                  : current);
              }
              if (event.type === 'message.reasoning.delta') reasoning += event.value;
              if (event.type === 'message.content.delta') content += event.value;
              if (event.type === 'stream.failed') {
                setNotice({ message: '实时回答连接已中断，正在恢复', tone: 'error' });
              }
              if (event.type === 'message.reasoning.delta' || event.type === 'message.content.delta') {
                setLiveAnswer({ conversationID: execution.conversationId, content, reasoning });
              }
              storeActiveExecution({
                executionID: execution.id,
                conversationID: execution.conversationId,
                lastEventID,
                content,
                reasoning,
              });
              terminalFromEvent = isTerminalStreamEvent(event) || terminalFromEvent;
            },
          );
        } catch (cause) {
          if (abortController.signal.aborted) return;
          // 网络中断不决定执行结果，先读取持久状态，再从最后一个事件继续订阅。
          if (!(cause instanceof TypeError)) {
            setNotice({ message: '实时回答连接已中断，正在恢复', tone: 'error' });
          }
        }

        execution = await getAgentExecution(execution.id);
        setActiveExecution(execution);
        if (isTerminalExecution(execution.state) || terminalFromEvent) break;
        await delay(800, abortController.signal);
      }
      if (abortController.signal.aborted || generation !== streamGenerationRef.current) return;
      if (!isTerminalExecution(execution.state)) execution = await getAgentExecution(execution.id);
      await finishExecution(execution);
    } catch (cause) {
      if (!abortController.signal.aborted) {
        setExecutionError(errorMessage(cause, '无法读取助手执行状态'));
      }
    } finally {
      if (generation === streamGenerationRef.current) {
        setActiveExecution(null);
        setLiveAnswer(null);
        setCancelling(false);
        clearStoredExecution();
        streamAbortRef.current = null;
      }
    }
  }, [finishExecution]);

  useEffect(() => {
    const executionID = activeExecution?.id;
    if (!executionID || isTerminalExecution(activeExecution.state)) return;
    let cancelled = false;
    const refresh = async () => {
      try {
        const steps = await listAgentExecutionSteps(executionID);
        if (!cancelled) setExecutionSteps(steps);
      } catch {
        // 轮询失败不打断模型回答，终态时还会再读取一次。
      }
    };
    void refresh();
    const timer = window.setInterval(() => void refresh(), 800);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeExecution?.id, activeExecution?.state]);

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
        const storedConversationID = readLocalValue(activeConversationKey);
        const firstID = storedExecution?.conversationID
          ?? (storedConversationID && items.some((item) => item.id === storedConversationID)
            ? storedConversationID
            : items[0]?.id);
        if (firstID) {
          selectedIDRef.current = firstID;
          setSelectedID(firstID);
        }
        if (storedExecution) {
          try {
            const execution = await getAgentExecution(storedExecution.executionID);
            if (!cancelled && !isTerminalExecution(execution.state)) {
              void followExecution(execution, storedExecution);
            } else if (!cancelled) {
              clearStoredExecution();
            }
          } catch {
            clearStoredExecution();
          }
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
      streamGenerationRef.current += 1;
      streamAbortRef.current?.abort();
    };
  }, [followExecution]);

  useEffect(() => {
    if (!selectedID) {
      selectedIDRef.current = null;
      setMessages([]);
      return;
    }
    selectedIDRef.current = selectedID;
    writeLocalValue(activeConversationKey, selectedID);
    void loadMessages(selectedID);
  }, [loadMessages, selectedID]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [liveAnswer?.content, liveAnswer?.reasoning, messages]);

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
    setInput('');
    setExecutionError('');
    setExecutionSteps([]);
    setEditingMessage(null);
    promptHistoryRef.current = [];
    promptHistoryIndexRef.current = -1;
    promptDraftRef.current = '';
    removeLocalValue(activeConversationKey);
    window.requestAnimationFrame(() => editorRef.current?.focus());
  };

  const selectConversation = (id: string) => {
    messageLoadGenerationRef.current += 1;
    selectedIDRef.current = id;
    setSelectedID(id);
    setMessages([]);
    setExecutionError('');
    setExecutionSteps([]);
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
    if (!content || activeExecution) return;
    if (!connection.configured) {
      setConnectionOpen(true);
      return;
    }
    setInput('');
    setExecutionError('');
    setExecutionSteps([]);
    try {
      let conversationID = selectedID;
      if (!conversationID) {
        const conversation = await createAssistantConversation(conversationTitle(content));
        conversationID = conversation.id;
        setConversations((current) => [conversation, ...current]);
        selectedIDRef.current = conversation.id;
        setSelectedID(conversation.id);
      }
      const execution = await createAgentExecution(conversationID, content);
      setEditingMessage(null);
      promptHistoryRef.current = [...promptHistoryRef.current, content];
      promptHistoryIndexRef.current = -1;
      promptDraftRef.current = '';
      await loadMessages(conversationID);
      void followExecution(execution);
    } catch (cause) {
      setInput(content);
      setExecutionError(errorMessage(cause, '发送消息失败'));
    }
  };

  const cancel = async () => {
    if (!activeExecution || cancelling) return;
    setCancelling(true);
    try {
      setActiveExecution(await cancelAgentExecution(activeExecution.id));
    } catch (cause) {
      setCancelling(false);
      setNotice({ message: errorMessage(cause, '取消回答失败'), tone: 'error' });
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
          />

          <div className="assistant-composer-wrap">
            {activeExecution && activeExecution.conversationId !== selectedID ? (
              <button
                className="assistant-active-execution-link"
                type="button"
                onClick={() => selectConversation(activeExecution.conversationId)}
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
                  <button
                    type="button"
                    aria-label="取消编辑"
                    onClick={() => {
                      setEditingMessage(null);
                      setInput('');
                      window.requestAnimationFrame(() => editorRef.current?.focus());
                    }}
                  >
                    <X aria-hidden="true" />
                  </button>
                </div>
              ) : null}
              <textarea
                ref={editorRef}
                value={input}
                rows={3}
                maxLength={65536}
                disabled={Boolean(activeExecution)}
                placeholder={connection.configured ? '描述问题，或询问 Ingate 的配置与排障方法' : '请先配置助手使用的模型'}
                onChange={(event) => {
                  promptHistoryIndexRef.current = -1;
                  promptDraftRef.current = '';
                  setInput(event.target.value);
                }}
                onKeyDown={handleEditorKeyDown}
              />
              <footer>
                <span>Enter 发送 · Shift + Enter 换行 · ↑/↓ 历史提示</span>
                {currentExecution ? (
                  <Button variant="outline" disabled={cancelling} onClick={() => void cancel()}>
                    <Square className="h-3.5 w-3.5 fill-current" aria-hidden="true" />
                    {cancelling ? '正在停止' : '停止回答'}
                  </Button>
                ) : (
                  <Button disabled={!input.trim() || Boolean(activeExecution)} onClick={() => void submit()}>
                    <Send className="h-4 w-4" aria-hidden="true" />
                    {editingMessage ? '重新发送' : '发送'}
                  </Button>
                )}
              </footer>
            </div>
          </div>
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

      <Modal
        title="重命名会话"
        isOpen={Boolean(renameCandidate)}
        onClose={() => setRenameCandidate(null)}
      >
        <form
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            void renameConversation();
          }}
        >
          <label className="field">
            <span>会话名称</span>
            <input
              autoFocus
              maxLength={160}
              value={renameTitle}
              onChange={(event) => setRenameTitle(event.target.value)}
            />
          </label>
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
            <Button type="button" variant="ghost" onClick={() => setRenameCandidate(null)}>取消</Button>
            <Button
              type="submit"
              disabled={renaming || !renameTitle.trim() || renameTitle.trim() === renameCandidate?.title}
            >
              {renaming ? '保存中...' : '保存'}
            </Button>
          </div>
        </form>
      </Modal>

      <Modal title="删除会话" isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)}>
        <div className="space-y-5">
          <p className="text-sm leading-6 text-slate-600">
            删除后，该会话中的消息将无法恢复。确定删除“<strong className="text-slate-900">{deleteCandidate?.title}</strong>”吗？
          </p>
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
            <Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button>
            <Button variant="danger" disabled={deleting} onClick={() => void removeConversation()}>
              {deleting ? '删除中...' : '确认删除'}
            </Button>
          </div>
        </div>
      </Modal>
    </PageFrame>
  );
}

function isTerminalStreamEvent(event: AssistantStreamEvent): boolean {
  return event.type === 'execution.completed'
    || event.type === 'execution.failed'
    || event.type === 'execution.cancelled';
}

function conversationTitle(content: string): string {
  const firstLine = content.split('\n', 1)[0].trim();
  return firstLine.length > 48 ? `${firstLine.slice(0, 48)}…` : firstLine;
}

function readStoredExecution(): StoredExecution | null {
  const value = readLocalValue(activeExecutionKey);
  if (!value) return null;
  try {
    const stored = JSON.parse(value) as Record<string, unknown>;
    if (
      typeof stored.executionID === 'string'
      && typeof stored.conversationID === 'string'
      && typeof stored.lastEventID === 'string'
      && typeof stored.content === 'string'
      && typeof stored.reasoning === 'string'
    ) {
      return {
        executionID: stored.executionID,
        conversationID: stored.conversationID,
        lastEventID: stored.lastEventID,
        content: stored.content,
        reasoning: stored.reasoning,
      };
    }
  } catch {
    // 非法 JSON 与字段不完整的存储值使用同一种清理路径。
  }
  removeLocalValue(activeExecutionKey);
  return null;
}

function storeActiveExecution(execution: StoredExecution) {
  if (writeLocalValue(activeExecutionKey, JSON.stringify(execution))) {
    return;
  }

  // 流式内容可能超过浏览器存储额度；保留可重新订阅的执行身份即可。
  writeLocalValue(activeExecutionKey, JSON.stringify({
    executionID: execution.executionID,
    conversationID: execution.conversationID,
    lastEventID: '',
    content: '',
    reasoning: '',
  } satisfies StoredExecution));
}

function clearStoredExecution() {
  removeLocalValue(activeExecutionKey);
}

function readLocalValue(key: string): string {
  try {
    return localStorage.getItem(key) ?? '';
  } catch {
    return '';
  }
}

function writeLocalValue(key: string, value: string): boolean {
  try {
    localStorage.setItem(key, value);
    return true;
  } catch {
    return false;
  }
}

function removeLocalValue(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // 浏览器禁用本地存储时无需影响当前页面状态。
  }
}

function delay(duration: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const finish = () => {
      window.clearTimeout(timer);
      signal.removeEventListener('abort', finish);
      resolve();
    };
    const timer = window.setTimeout(finish, duration);
    signal.addEventListener('abort', finish, { once: true });
    if (signal.aborted) finish();
  });
}
