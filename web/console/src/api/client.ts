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

  const text = await response.text();
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new Error(response.ok ? '服务返回了无法识别的响应' : `请求失败：${response.status}`);
  }
  if (!isApiResponse<T>(parsed)) {
    throw new Error(response.ok ? '服务返回了无法识别的响应' : `请求失败：${response.status}`);
  }
  const body = parsed;
  if (!response.ok || body.code < 200 || body.code >= 300) {
    throw new Error(body.msg || `请求失败：${response.status}`);
  }

  return body.data;
}

function isApiResponse<T>(value: unknown): value is ApiResponse<T> {
  return typeof value === 'object'
    && value !== null
    && typeof (value as { code?: unknown }).code === 'number'
    && typeof (value as { msg?: unknown }).msg === 'string'
    && 'data' in value;
}
