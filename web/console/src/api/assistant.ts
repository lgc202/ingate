import type {
  AssistantConversation,
  AssistantMessage,
  AgentExecution,
  AgentExecutionStep,
  AssistantStreamEvent,
  AssistantStreamEventType,
  ModelConnection,
  UpdateModelConnectionInput,
} from '@/domain/assistant';

interface ConversationPage {
  conversations: AssistantConversation[];
  nextCursor?: string;
}

interface MessagePage {
  messages: AssistantMessage[];
  nextCursor?: string;
}

const assistantBaseURL = (import.meta.env.VITE_INGATE_ASSISTANT_BASE_URL as string | undefined) ?? '';

export async function listAssistantConversations(limit = 100, cursor = ''): Promise<ConversationPage> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set('cursor', cursor);
  return assistantRequest<ConversationPage>(`/assistant/v1/conversations?${query}`);
}

export async function createAssistantConversation(title: string): Promise<AssistantConversation> {
  return assistantRequest<AssistantConversation>('/assistant/v1/conversations', {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
}

export async function updateAssistantConversation(id: string, title: string): Promise<AssistantConversation> {
  return assistantRequest<AssistantConversation>(`/assistant/v1/conversations/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ title }),
  });
}

export async function deleteAssistantConversation(id: string): Promise<void> {
  await assistantRequest(`/assistant/v1/conversations/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function listAssistantMessages(conversationID: string): Promise<AssistantMessage[]> {
  const messages: AssistantMessage[] = [];
  let cursor = '';
  do {
    const query = new URLSearchParams({ limit: '100' });
    if (cursor) query.set('cursor', cursor);
    const page = await assistantRequest<MessagePage>(
      `/assistant/v1/conversations/${encodeURIComponent(conversationID)}/messages?${query}`,
    );
    messages.push(...(page.messages ?? []));
    cursor = page.nextCursor ?? '';
  } while (cursor);
  return messages;
}

export async function createAgentExecution(conversationID: string, content: string): Promise<AgentExecution> {
  return assistantRequest<AgentExecution>(
    `/assistant/v1/conversations/${encodeURIComponent(conversationID)}/executions`,
    { method: 'POST', body: JSON.stringify({ content }) },
  );
}

export async function getAgentExecution(id: string): Promise<AgentExecution> {
  return assistantRequest<AgentExecution>(`/assistant/v1/executions/${encodeURIComponent(id)}`);
}

export async function listAgentExecutionSteps(executionID: string): Promise<AgentExecutionStep[]> {
  const result = await assistantRequest<{ steps?: AgentExecutionStep[] }>(
    `/assistant/v1/executions/${encodeURIComponent(executionID)}/steps`,
  );
  return result.steps ?? [];
}

export async function cancelAgentExecution(id: string): Promise<AgentExecution> {
  return assistantRequest<AgentExecution>(`/assistant/v1/executions/${encodeURIComponent(id)}:cancel`, {
    method: 'POST',
    body: '{}',
  });
}

export async function getModelConnection(): Promise<ModelConnection> {
  return assistantRequest<ModelConnection>('/assistant/v1/model-connection');
}

export async function updateModelConnection(input: UpdateModelConnectionInput): Promise<ModelConnection> {
  return assistantRequest<ModelConnection>('/assistant/v1/model-connection', {
    method: 'PUT',
    body: JSON.stringify(input),
  });
}

export async function streamAgentExecution(
  executionID: string,
  lastEventID: string,
  signal: AbortSignal,
  onEvent: (event: AssistantStreamEvent) => void,
): Promise<void> {
  const headers = new Headers({ Accept: 'text/event-stream' });
  if (lastEventID) headers.set('Last-Event-ID', lastEventID);
  const response = await fetch(`${assistantBaseURL}/assistant/v1/executions/${encodeURIComponent(executionID)}/events`, {
    credentials: 'same-origin',
    headers,
    signal,
  });
  if (response.status === 401) window.dispatchEvent(new Event('ingate:unauthorized'));
  if (!response.ok || !response.body) throw await responseError(response);

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    const frames = buffer.replaceAll('\r\n', '\n').split('\n\n');
    buffer = frames.pop() ?? '';
    for (const frame of frames) {
      const event = parseSSEFrame(frame);
      if (event) onEvent(event);
    }
    if (done) break;
  }
}

async function assistantRequest<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body) headers.set('Content-Type', 'application/json');
  const response = await fetch(`${assistantBaseURL}${path}`, {
    ...init,
    credentials: 'same-origin',
    headers,
  });
  if (response.status === 401) window.dispatchEvent(new Event('ingate:unauthorized'));
  if (!response.ok) throw await responseError(response);
  if (response.status === 204) return undefined as T;
  const text = await response.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

async function responseError(response: Response): Promise<Error> {
  const fallback = `请求失败：${response.status}`;
  const text = await response.text();
  if (!text) return new Error(fallback);
  try {
    const body = JSON.parse(text) as { message?: unknown; msg?: unknown };
    if (typeof body.message === 'string' && body.message) return new Error(body.message);
    if (typeof body.msg === 'string' && body.msg) return new Error(body.msg);
    return new Error(fallback);
  } catch {
    return new Error(fallback);
  }
}

function parseSSEFrame(frame: string): AssistantStreamEvent | null {
  let id = '';
  let type = '';
  const data: string[] = [];
  for (const line of frame.split('\n')) {
    if (!line || line.startsWith(':')) continue;
    if (line.startsWith('id:')) id = line.slice(3).trim();
    if (line.startsWith('event:')) type = line.slice(6).trim();
    if (line.startsWith('data:')) data.push(line.slice(5).trimStart());
  }
  if (!type) return null;
  let value = '';
  if (data.length) {
    const body = JSON.parse(data.join('\n')) as { value?: unknown };
    if (typeof body.value === 'string') value = body.value;
  }
  return { id, type: type as AssistantStreamEventType, value };
}
