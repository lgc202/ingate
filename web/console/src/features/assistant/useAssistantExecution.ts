import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import {
  cancelAgentExecution,
  getAgentExecution,
  listAgentExecutionSteps,
  streamAgentExecution,
} from '@/api/assistant';
import { errorMessage } from '@/api/errors';
import type {
  AgentExecution,
  AgentExecutionStep,
  AssistantStreamEvent,
} from '@/domain/assistant';
import { executionErrorMessage, isSettledExecution } from '@/domain/assistant';
import {
  clearStoredExecution,
  storeActiveExecution,
  type StoredExecution,
} from './assistant-session';

const executionPollIntervalMillis = 800;
const executionPersistenceIntervalMillis = 1_000;

interface ExecutionNotice {
  message: string;
  tone: 'success' | 'error';
}

interface UseAssistantExecutionOptions {
  selectedConversationIDRef: RefObject<string | null>;
  reloadConversation: (conversationID: string) => Promise<void>;
  reloadConversations: () => Promise<unknown>;
  onNotice: (notice: ExecutionNotice) => void;
}

export function useAssistantExecution({
  selectedConversationIDRef,
  reloadConversation,
  reloadConversations,
  onNotice,
}: UseAssistantExecutionOptions) {
  const [activeExecution, setActiveExecution] = useState<AgentExecution | null>(null);
  const [executionSteps, setExecutionSteps] = useState<AgentExecutionStep[]>([]);
  const [liveAnswer, setLiveAnswer] = useState<{
    conversationID: string;
    content: string;
  } | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [executionError, setExecutionError] = useState('');
  const streamAbortRef = useRef<AbortController | null>(null);
  const streamGenerationRef = useRef(0);

  const finishExecution = useCallback(async (execution: AgentExecution) => {
    let steps: AgentExecutionStep[] = [];
    try {
      steps = await listAgentExecutionSteps(execution.id);
      setExecutionSteps(steps);
    } catch {
      // 执行终态是最终事实；步骤读取失败不应覆盖回答结果。
    }
    if (selectedConversationIDRef.current === execution.conversationId) {
      if (execution.state === 'AGENT_EXECUTION_STATE_FAILED') {
        setExecutionError(executionErrorMessage(execution.errorCode, steps));
      }
      if (execution.state === 'AGENT_EXECUTION_STATE_CANCELLED') {
        setExecutionError('本次回答已取消');
      }
    }

    const reloadTasks: Promise<unknown>[] = [reloadConversations().catch(() => undefined)];
    if (selectedConversationIDRef.current === execution.conversationId) {
      reloadTasks.push(reloadConversation(execution.conversationId));
    }
    await Promise.all(reloadTasks);
  }, [reloadConversation, reloadConversations, selectedConversationIDRef]);

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
    let terminalFromEvent = false;
    let lastStoredAt = Date.now();

    setActiveExecution(execution);
    setExecutionSteps([]);
    setLiveAnswer({ conversationID: execution.conversationId, content });
    setExecutionError('');
    storeActiveExecution({
      executionID: execution.id,
      conversationID: execution.conversationId,
      lastEventID,
      content,
    });

    try {
      while (!abortController.signal.aborted && !isSettledExecution(execution.state)) {
        terminalFromEvent = false;
        try {
          await streamAgentExecution(
            execution.id,
            lastEventID,
            abortController.signal,
            (event) => {
              if (event.id) lastEventID = event.id;
              if (event.type === 'execution.started') {
                setActiveExecution((current) => current
                  ? { ...current, state: 'AGENT_EXECUTION_STATE_RUNNING' }
                  : current);
              }
              if (event.type === 'message.content.delta') content += event.value;
              if (event.type === 'stream.failed') {
                onNotice({ message: '实时回答连接已中断，正在恢复', tone: 'error' });
              }
              if (event.type === 'message.content.delta') {
                setLiveAnswer({ conversationID: execution.conversationId, content });
              }

              terminalFromEvent = isTerminalStreamEvent(event) || terminalFromEvent;
              const now = Date.now();
              if (terminalFromEvent || now - lastStoredAt >= executionPersistenceIntervalMillis) {
                storeActiveExecution({
                  executionID: execution.id,
                  conversationID: execution.conversationId,
                  lastEventID,
                  content,
                });
                lastStoredAt = now;
              }
            },
          );
        } catch (cause) {
          if (abortController.signal.aborted) return;
          // 网络中断不决定执行结果，先读取持久状态，再从最后一个事件继续订阅。
          if (!(cause instanceof TypeError)) {
            onNotice({ message: '实时回答连接已中断，正在恢复', tone: 'error' });
          }
        }

        execution = await getAgentExecution(execution.id);
        setActiveExecution(execution);
        // 同一个 Execution 可以经历多次审批恢复。事件流中可能仍保留上一阶段的
        // interrupted 事件，是否结束跟随必须以 MySQL 执行状态为准。
        if (isSettledExecution(execution.state)) break;
        if (terminalFromEvent) {
          // 这是已恢复执行的历史阶段终点。游标已经前移，下一轮只展示当前阶段
          // 新产生的回答，不能把审批前的未完成文本再次拼接进来。
          content = '';
          setLiveAnswer({ conversationID: execution.conversationId, content });
          storeActiveExecution({
            executionID: execution.id,
            conversationID: execution.conversationId,
            lastEventID,
            content,
          });
        }
        await delay(executionPollIntervalMillis, abortController.signal);
      }
      if (abortController.signal.aborted || generation !== streamGenerationRef.current) return;
      if (!isSettledExecution(execution.state)) execution = await getAgentExecution(execution.id);
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
        if (execution.state === 'AGENT_EXECUTION_STATE_WAITING_APPROVAL') {
          // 审批恢复仍沿用同一个 Execution 和 Redis Stream。保留游标但丢弃本阶段
          // 未形成最终回答的流式文本，避免恢复后重放旧中断和旧思考内容。
          storeActiveExecution({
            executionID: execution.id,
            conversationID: execution.conversationId,
            lastEventID,
            content: '',
          });
        } else {
          clearStoredExecution();
        }
        streamAbortRef.current = null;
      }
    }
  }, [finishExecution, onNotice]);

  useEffect(() => {
    const executionID = activeExecution?.id;
    if (!executionID || isSettledExecution(activeExecution.state)) return;
    let cancelled = false;
    const refreshSteps = async () => {
      try {
        const steps = await listAgentExecutionSteps(executionID);
        if (!cancelled) setExecutionSteps(steps);
      } catch {
        // 轮询失败不打断模型回答，终态时还会再读取一次。
      }
    };
    void refreshSteps();
    const timer = window.setInterval(() => void refreshSteps(), executionPollIntervalMillis);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeExecution?.id, activeExecution?.state]);

  useEffect(() => () => {
    streamGenerationRef.current += 1;
    streamAbortRef.current?.abort();
  }, []);

  const resetExecutionView = useCallback(() => {
    setExecutionError('');
    setExecutionSteps([]);
  }, []);

  const showExecutionError = useCallback((message: string) => {
    setExecutionError(message);
  }, []);

  const cancelExecution = useCallback(async () => {
    if (!activeExecution || cancelling) return;
    setCancelling(true);
    try {
      setActiveExecution(await cancelAgentExecution(activeExecution.id));
    } catch (cause) {
      setCancelling(false);
      onNotice({ message: errorMessage(cause, '取消回答失败'), tone: 'error' });
    }
  }, [activeExecution, cancelling, onNotice]);

  return {
    activeExecution,
    executionSteps,
    liveAnswer,
    cancelling,
    executionError,
    followExecution,
    resetExecutionView,
    showExecutionError,
    cancelExecution,
  };
}

function isTerminalStreamEvent(event: AssistantStreamEvent): boolean {
  return event.type === 'execution.completed'
    || event.type === 'execution.failed'
    || event.type === 'execution.cancelled'
    || event.type === 'execution.interrupted';
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
