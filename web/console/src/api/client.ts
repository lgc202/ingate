interface ApiResponse<T> {
  code: number;
  msg: string;
  data: T;
}

export interface PagedResponse {
  page?: {
    nextPageToken?: string;
  };
}

export interface CursorPagedResponse {
  nextCursor?: string;
}

const apiBaseUrl = (import.meta.env.VITE_INGATE_API_BASE_URL as string | undefined) ?? '/api/v1';

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    credentials: 'same-origin',
    headers,
  });

  if (response.status === 401) {
    window.dispatchEvent(new Event('ingate:unauthorized'));
  }

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

export async function apiListAll<TPage extends PagedResponse, TItem>(
  path: string,
  items: (page: TPage) => TItem[],
): Promise<TItem[]> {
  const result: TItem[] = [];
  let pageToken = '';
  do {
    const query = new URLSearchParams({ pageSize: '200' });
    if (pageToken) query.set('pageToken', pageToken);
    const page = await apiRequest<TPage>(`${path}?${query}`);
    result.push(...items(page));
    pageToken = page.page?.nextPageToken ?? '';
  } while (pageToken);
  return result;
}

export async function apiListAllByCursor<TPage extends CursorPagedResponse, TItem>(
  path: string,
  items: (page: TPage) => TItem[],
): Promise<TItem[]> {
  const result: TItem[] = [];
  let cursor = '';
  do {
    const query = new URLSearchParams({ limit: '200' });
    if (cursor) query.set('cursor', cursor);
    const page = await apiRequest<TPage>(`${path}?${query}`);
    result.push(...items(page));
    cursor = page.nextCursor ?? '';
  } while (cursor);
  return result;
}

function isApiResponse<T>(value: unknown): value is ApiResponse<T> {
  return typeof value === 'object'
    && value !== null
    && typeof (value as { code?: unknown }).code === 'number'
    && typeof (value as { msg?: unknown }).msg === 'string'
    && 'data' in value;
}
