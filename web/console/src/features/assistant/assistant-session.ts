const activeConversationKey = 'ingate.assistant.conversation';
const activeExecutionKey = 'ingate.assistant.active-execution';

export interface StoredExecution {
  executionID: string;
  conversationID: string;
  lastEventID: string;
  content: string;
  reasoning: string;
}

export function readActiveConversationID(): string {
  return readLocalValue(activeConversationKey);
}

export function storeActiveConversationID(conversationID: string): void {
  writeLocalValue(activeConversationKey, conversationID);
}

export function clearActiveConversationID(): void {
  removeLocalValue(activeConversationKey);
}

export function readStoredExecution(): StoredExecution | null {
  const value = readSessionValue(activeExecutionKey);
  // 早期版本把流式回答写入长期存储；读取时主动清理旧键。
  removeLocalValue(activeExecutionKey);
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
  removeSessionValue(activeExecutionKey);
  return null;
}

export function storeActiveExecution(execution: StoredExecution): void {
  if (writeSessionValue(activeExecutionKey, JSON.stringify(execution))) return;

  // 流式内容可能超过浏览器存储额度；保留可重新订阅的执行身份即可。
  writeSessionValue(activeExecutionKey, JSON.stringify({
    executionID: execution.executionID,
    conversationID: execution.conversationID,
    lastEventID: '',
    content: '',
    reasoning: '',
  } satisfies StoredExecution));
}

export function clearStoredExecution(): void {
  removeSessionValue(activeExecutionKey);
  removeLocalValue(activeExecutionKey);
}

function readSessionValue(key: string): string {
  try {
    return sessionStorage.getItem(key) ?? '';
  } catch {
    return '';
  }
}

function writeSessionValue(key: string, value: string): boolean {
  try {
    sessionStorage.setItem(key, value);
    return true;
  } catch {
    return false;
  }
}

function removeSessionValue(key: string): void {
  try {
    sessionStorage.removeItem(key);
  } catch {
    // 浏览器禁用会话存储时无需影响当前页面状态。
  }
}

function readLocalValue(key: string): string {
  try {
    return localStorage.getItem(key) ?? '';
  } catch {
    return '';
  }
}

function writeLocalValue(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // 浏览器禁用本地存储时无需影响当前页面状态。
  }
}

function removeLocalValue(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // 浏览器禁用本地存储时无需影响当前页面状态。
  }
}
