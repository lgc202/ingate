interface ApiResponse<T> {
  code: number;
  msg: string;
  data: T;
}

const apiBaseUrl = (import.meta.env.VITE_INGATE_API_BASE_URL as string | undefined) ?? '/api/v1';

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  });

  const body = await response.json() as ApiResponse<T>;
  if (!response.ok || body.code < 200 || body.code >= 300) {
    throw new Error(body.msg || `请求失败：${response.status}`);
  }

  return body.data;
}
