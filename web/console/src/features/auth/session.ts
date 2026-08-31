interface SessionEnvelope {
  code: number;
  msg: string;
  data: Session;
}

export interface Session {
  authenticated: boolean;
  username: string;
}

export async function getSession(): Promise<Session> {
  return requestSession('/auth/session', { method: 'GET' });
}

export async function login(username: string, password: string): Promise<Session> {
  return requestSession('/auth/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
}

export async function logout(): Promise<void> {
  await requestSession('/auth/session', { method: 'DELETE' });
}

async function requestSession(path: string, init: RequestInit): Promise<Session> {
  const response = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers: { Accept: 'application/json', ...init.headers },
  });
  const text = await response.text();
  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    throw new Error(response.ok ? '服务返回了无法识别的响应' : '登录请求失败');
  }
  if (!isSessionEnvelope(body)) {
    throw new Error(response.ok ? '服务返回了无法识别的响应' : '登录请求失败');
  }
  if (!response.ok) {
    throw new Error(body.msg || '登录请求失败');
  }
  return body.data;
}

function isSessionEnvelope(value: unknown): value is SessionEnvelope {
  if (typeof value !== 'object' || value === null) return false;
  const envelope = value as Partial<SessionEnvelope>;
  return typeof envelope.code === 'number'
    && typeof envelope.msg === 'string'
    && typeof envelope.data === 'object'
    && envelope.data !== null
    && typeof envelope.data.authenticated === 'boolean'
    && typeof envelope.data.username === 'string';
}
