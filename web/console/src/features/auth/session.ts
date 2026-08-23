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
  const body = await response.json() as SessionEnvelope;
  if (!response.ok) {
    throw new Error(body.msg || '登录请求失败');
  }
  return body.data;
}
