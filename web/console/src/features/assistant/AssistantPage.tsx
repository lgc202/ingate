import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import {
  MessageSquareText,
  Send,
  Settings2,
  Square,
} from 'lucide-react';
import {
  cancelAssistantRun,
  createAssistantConversation,
  createAssistantRun,
  deleteAssistantConversation,
  getAssistantRun,
  getModelConnection,
  listAssistantConversations,
  listAssistantMessages,
  listAssistantRunItems,
  streamAssistantRun,
} from '@/api/assistant';
import { Badge, Button, Modal, PageFrame, Toast } from '@/components/ui';
import type {
  AssistantConversation,
  AssistantMessage,
  AssistantRun,
  AssistantRunItem,
  AssistantStreamEvent,
  ModelConnection,
} from '@/domain/assistant';
import { isTerminalRun, runErrorMessage, runStateLabel } from '@/domain/assistant';
import { AssistantConversationList } from './AssistantConversationList';
import { AssistantMessageList, type LiveAnswer } from './AssistantMessageList';
import { ModelConnectionDrawer } from './ModelConnectionDrawer';

const activeConversationKey = 'ingate.assistant.conversation';
const activeRunKey = 'ingate.assistant.active-run';

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

interface StoredRun {
  runID: string;
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
  const [deleteCandidate, setDeleteCandidate] = useState<AssistantConversation | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [input, setInput] = useState('');
  const [activeRun, setActiveRun] = useState<AssistantRun | null>(null);
  const [runItems, setRunItems] = useState<AssistantRunItem[]>([]);
  const [liveAnswer, setLiveAnswer] = useState<LiveAnswer | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [runError, setRunError] = useState('');
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const streamAbortRef = useRef<AbortController | null>(null);
  const streamGenerationRef = useRef(0);
  const messageLoadGenerationRef = useRef(0);
  const selectedIDRef = useRef<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const editorRef = useRef<HTMLTextAreaElement>(null);

  const loadMessages = useCallback(async (conversationID: string) => {
    const generation = ++messageLoadGenerationRef.current;
    setLoadingMessages(true);
    try {
      const loaded = await listAssistantMessages(conversationID);
      if (generation === messageLoadGenerationRef.current) setMessages(loaded);
    } catch (cause) {
      if (generation === messageLoadGenerationRef.current) {
        setNotice({
          message: cause instanceof Error ? cause.message : '加载会话消息失败',
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

  const finishRun = useCallback(async (run: AssistantRun) => {
    let items: AssistantRunItem[] = [];
    try {
      items = await listAssistantRunItems(run.id);
      setRunItems(items);
    } catch {
      // Run 终态仍是最终事实；执行步骤读取失败不应覆盖回答结果。
    }
    if (selectedIDRef.current === run.conversationId) {
      if (run.state === 'RUN_STATE_FAILED') setRunError(runErrorMessage(run.errorCode, items));
      if (run.state === 'RUN_STATE_CANCELLED') setRunError('本次回答已取消');
    }
    const tasks: Promise<unknown>[] = [reloadConversations().catch(() => undefined)];
    if (selectedIDRef.current === run.conversationId) tasks.push(loadMessages(run.conversationId));
    await Promise.all(tasks);
  }, [loadMessages, reloadConversations]);

  const followRun = useCallback(async (initialRun: AssistantRun, storedRun?: StoredRun) => {
    const generation = ++streamGenerationRef.current;
    streamAbortRef.current?.abort();
    const abortController = new AbortController();
    streamAbortRef.current = abortController;
    let run = initialRun;
    let lastEventID = storedRun?.lastEventID ?? '';
    let content = storedRun?.content ?? '';
    let reasoning = storedRun?.reasoning ?? '';
    let terminalFromEvent = false;
    setActiveRun(run);
    setRunItems([]);
    setLiveAnswer({ conversationID: run.conversationId, content, reasoning });
    setRunError('');
    storeActiveRun({ runID: run.id, conversationID: run.conversationId, lastEventID, content, reasoning });

    try {
      while (!abortController.signal.aborted && !isTerminalRun(run.state)) {
        terminalFromEvent = false;
        try {
          await streamAssistantRun(
            run.id,
            lastEventID,
            abortController.signal,
            (event) => {
              if (event.id) {
                lastEventID = event.id;
              }
              if (event.type === 'run.started') {
                setActiveRun((current) => current ? { ...current, state: 'RUN_STATE_RUNNING' } : current);
              }
              if (event.type === 'message.reasoning.delta') reasoning += event.value;
              if (event.type === 'message.content.delta') content += event.value;
              if (event.type === 'stream.failed') {
                setNotice({ message: '实时回答连接已中断，正在恢复', tone: 'error' });
              }
              if (event.type === 'message.reasoning.delta' || event.type === 'message.content.delta') {
                setLiveAnswer({ conversationID: run.conversationId, content, reasoning });
              }
              storeActiveRun({
                runID: run.id,
                conversationID: run.conversationId,
                lastEventID,
                content,
                reasoning,
              });
              terminalFromEvent = isTerminalStreamEvent(event) || terminalFromEvent;
            },
          );
        } catch (cause) {
          if (abortController.signal.aborted) return;
          // 网络中断不决定 Run 结果，先读取持久状态，再从最后一个事件继续订阅。
          if (!(cause instanceof TypeError)) {
            setNotice({ message: '实时回答连接已中断，正在恢复', tone: 'error' });
          }
        }

        run = await getAssistantRun(run.id);
        setActiveRun(run);
        if (isTerminalRun(run.state) || terminalFromEvent) break;
        await delay(800, abortController.signal);
      }
      if (abortController.signal.aborted || generation !== streamGenerationRef.current) return;
      if (!isTerminalRun(run.state)) run = await getAssistantRun(run.id);
      await finishRun(run);
    } catch (cause) {
      if (!abortController.signal.aborted) {
        setRunError(cause instanceof Error ? cause.message : '无法读取助手执行状态');
      }
    } finally {
      if (generation === streamGenerationRef.current) {
        setActiveRun(null);
        setLiveAnswer(null);
        setCancelling(false);
        clearStoredRun();
        streamAbortRef.current = null;
      }
    }
  }, [finishRun]);

  useEffect(() => {
    const runID = activeRun?.id;
    if (!runID || isTerminalRun(activeRun.state)) return;
    let cancelled = false;
    const refresh = async () => {
      try {
        const items = await listAssistantRunItems(runID);
        if (!cancelled) setRunItems(items);
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
  }, [activeRun?.id, activeRun?.state]);

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

        const storedRun = readStoredRun();
        const storedConversationID = localStorage.getItem(activeConversationKey);
        const firstID = storedRun?.conversationID
          ?? (storedConversationID && items.some((item) => item.id === storedConversationID)
            ? storedConversationID
            : items[0]?.id);
        if (firstID) {
          selectedIDRef.current = firstID;
          setSelectedID(firstID);
        }
        if (storedRun) {
          try {
            const run = await getAssistantRun(storedRun.runID);
            if (!cancelled && !isTerminalRun(run.state)) {
              void followRun(run, storedRun);
            } else if (!cancelled) {
              clearStoredRun();
            }
          } catch {
            clearStoredRun();
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
  }, [followRun]);

  useEffect(() => {
    if (!selectedID) {
      selectedIDRef.current = null;
      setMessages([]);
      return;
    }
    selectedIDRef.current = selectedID;
    localStorage.setItem(activeConversationKey, selectedID);
    void loadMessages(selectedID);
  }, [loadMessages, selectedID]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [liveAnswer?.content, liveAnswer?.reasoning, messages]);

  const newConversation = () => {
    messageLoadGenerationRef.current += 1;
    selectedIDRef.current = null;
    setSelectedID(null);
    setMessages([]);
    setInput('');
    setRunError('');
    setRunItems([]);
    localStorage.removeItem(activeConversationKey);
    window.requestAnimationFrame(() => editorRef.current?.focus());
  };

  const selectConversation = (id: string) => {
    messageLoadGenerationRef.current += 1;
    selectedIDRef.current = id;
    setSelectedID(id);
    setRunError('');
    setRunItems([]);
  };

  const loadMore = async () => {
    if (!nextCursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const page = await listAssistantConversations(100, nextCursor);
      setConversations((current) => [...current, ...(page.conversations ?? [])]);
      setNextCursor(page.nextCursor ?? '');
    } catch (cause) {
      setNotice({ message: cause instanceof Error ? cause.message : '加载更多会话失败', tone: 'error' });
    } finally {
      setLoadingMore(false);
    }
  };

  const submit = async () => {
    const content = input.trim();
    if (!content || activeRun) return;
    if (!connection.configured) {
      setConnectionOpen(true);
      return;
    }
    setInput('');
    setRunError('');
    setRunItems([]);
    try {
      let conversationID = selectedID;
      if (!conversationID) {
        const conversation = await createAssistantConversation(conversationTitle(content));
        conversationID = conversation.id;
        setConversations((current) => [conversation, ...current]);
        selectedIDRef.current = conversation.id;
        setSelectedID(conversation.id);
      }
      const run = await createAssistantRun(conversationID, content);
      await loadMessages(conversationID);
      void followRun(run);
    } catch (cause) {
      setInput(content);
      setRunError(cause instanceof Error ? cause.message : '发送消息失败');
    }
  };

  const cancel = async () => {
    if (!activeRun || cancelling) return;
    setCancelling(true);
    try {
      setActiveRun(await cancelAssistantRun(activeRun.id));
    } catch (cause) {
      setCancelling(false);
      setNotice({ message: cause instanceof Error ? cause.message : '取消回答失败', tone: 'error' });
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
      setNotice({ message: cause instanceof Error ? cause.message : '删除会话失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const handleEditorKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    void submit();
  };

  const selectedConversation = conversations.find((item) => item.id === selectedID);
  const currentRun = activeRun?.conversationId === selectedID ? activeRun : null;
  const currentLiveAnswer = liveAnswer?.conversationID === selectedID ? liveAnswer : null;
  const currentRunItems = currentRun ? runItems : [];

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
          activeRunConversationID={activeRun?.conversationId}
          loading={loading}
          loadingMore={loadingMore}
          hasMore={Boolean(nextCursor)}
          onNew={newConversation}
          onSelect={selectConversation}
          onDelete={(conversation) => {
            if (activeRun?.conversationId === conversation.id) {
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
                <span>{currentRun ? runStateLabel(currentRun.state) : '面向 Ingate 配置与排障场景'}</span>
              </div>
            </div>
            {currentRun ? <Badge tone={currentRun.state === 'RUN_STATE_RUNNING' ? 'accent' : 'neutral'}>{runStateLabel(currentRun.state)}</Badge> : null}
          </header>

          <AssistantMessageList
            messages={messages}
            loading={loadingMessages}
            hasConversation={Boolean(selectedID)}
            configured={connection.configured}
            liveAnswer={currentLiveAnswer}
            run={currentRun}
            runItems={currentRunItems}
            error={runError}
            endRef={messagesEndRef}
            onConfigure={() => setConnectionOpen(true)}
            onSuggestion={(value) => {
              setInput(value);
              window.requestAnimationFrame(() => editorRef.current?.focus());
            }}
          />

          <div className="assistant-composer-wrap">
            {activeRun && activeRun.conversationId !== selectedID ? (
              <button className="assistant-active-run-link" type="button" onClick={() => selectConversation(activeRun.conversationId)}>
                助手正在另一个会话中回答，点击返回
              </button>
            ) : null}
            <div className="assistant-composer">
              <textarea
                ref={editorRef}
                value={input}
                rows={3}
                maxLength={65536}
                disabled={Boolean(activeRun)}
                placeholder={connection.configured ? '描述问题，或询问 Ingate 的配置与排障方法' : '请先配置助手使用的模型'}
                onChange={(event) => setInput(event.target.value)}
                onKeyDown={handleEditorKeyDown}
              />
              <footer>
                <span>Enter 发送 · Shift + Enter 换行</span>
                {currentRun ? (
                  <Button variant="outline" disabled={cancelling} onClick={() => void cancel()}>
                    <Square className="h-3.5 w-3.5 fill-current" aria-hidden="true" />
                    {cancelling ? '正在停止' : '停止回答'}
                  </Button>
                ) : (
                  <Button disabled={!input.trim() || Boolean(activeRun)} onClick={() => void submit()}>
                    <Send className="h-4 w-4" aria-hidden="true" />
                    发送
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
  return event.type === 'run.completed'
    || event.type === 'run.failed'
    || event.type === 'run.cancelled';
}

function conversationTitle(content: string): string {
  const firstLine = content.split('\n', 1)[0].trim();
  return firstLine.length > 48 ? `${firstLine.slice(0, 48)}…` : firstLine;
}

function readStoredRun(): StoredRun | null {
  const value = localStorage.getItem(activeRunKey);
  if (!value) return null;
  try {
    const stored = JSON.parse(value) as Partial<StoredRun>;
    if (stored.runID && stored.conversationID) {
      return {
        runID: stored.runID,
        conversationID: stored.conversationID,
        lastEventID: stored.lastEventID ?? '',
        content: stored.content ?? '',
        reasoning: stored.reasoning ?? '',
      };
    }
  } catch {
    localStorage.removeItem(activeRunKey);
  }
  return null;
}

function storeActiveRun(run: StoredRun) {
  localStorage.setItem(activeRunKey, JSON.stringify(run));
}

function clearStoredRun() {
  localStorage.removeItem(activeRunKey);
}

function delay(duration: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = window.setTimeout(resolve, duration);
    signal.addEventListener('abort', () => {
      window.clearTimeout(timer);
      resolve();
    }, { once: true });
  });
}
