interface ApiResponse<T> {
  code: number;
  reason?: string;
  msg: string;
  data: T;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: number;
  readonly reason?: string;

  constructor(status: number, code: number, reason: string | undefined, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.reason = reason;
  }
}

export interface PagedResponse {
  page?: {
    nextPageToken?: string;
  };
}

export interface CursorPagedResponse {
  nextCursor?: string;
}

export interface CursorPage<T> {
  items: T[];
  nextCursor: string;
}

export type CursorQuery = Record<string, string | number | boolean | undefined>;

const apiBaseURL = (import.meta.env.VITE_INGATE_API_BASE_URL as string | undefined) ?? '/api/v1';

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  const response = await fetch(`${apiBaseURL}${path}`, {
    ...init,
    credentials: 'same-origin',
    headers,
  });

  if (response.status === 401) {
    window.dispatchEvent(new Event('ingate:unauthorized'));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new ApiError(
      response.status,
      response.status,
      undefined,
      response.ok ? '服务返回了无法识别的响应' : `请求失败：${response.status}`,
    );
  }
  if (!isApiResponse<T>(parsed)) {
    throw new ApiError(
      response.status,
      response.status,
      undefined,
      response.ok ? '服务返回了无法识别的响应' : `请求失败：${response.status}`,
    );
  }
  const body = parsed;
  if (!response.ok || body.code < 200 || body.code >= 300) {
    throw new ApiError(
      response.status,
      body.code,
      body.reason,
      body.msg || `请求失败：${response.status}`,
    );
  }

  return body.data;
}

export async function apiListAll<TPage extends PagedResponse, TItem>(
  path: string,
  items: (page: TPage) => TItem[],
): Promise<TItem[]> {
  const result: TItem[] = [];
  let pageToken = '';
  const visitedTokens = new Set<string>();
  do {
    const query = new URLSearchParams({ pageSize: '200' });
    if (pageToken) query.set('pageToken', pageToken);
    const page = await apiRequest<TPage>(`${path}?${query}`);
    result.push(...items(page));
    pageToken = page.page?.nextPageToken ?? '';
    if (pageToken && visitedTokens.has(pageToken)) {
      throw new Error('服务返回了重复的分页标记');
    }
    visitedTokens.add(pageToken);
  } while (pageToken);
  return result;
}

export async function apiListAllByCursor<TPage extends CursorPagedResponse, TItem>(
  path: string,
  items: (page: TPage) => TItem[],
): Promise<TItem[]> {
  const result: TItem[] = [];
  let cursor = '';
  const visitedCursors = new Set<string>();
  do {
    const query = new URLSearchParams({ limit: '200' });
    if (cursor) query.set('cursor', cursor);
    const page = await apiRequest<TPage>(`${path}?${query}`);
    result.push(...items(page));
    cursor = page.nextCursor ?? '';
    if (cursor && visitedCursors.has(cursor)) {
      throw new Error('服务返回了重复的分页游标');
    }
    visitedCursors.add(cursor);
  } while (cursor);
  return result;
}

export async function apiListPageByCursor<TPage extends CursorPagedResponse, TItem>(
  path: string,
  query: CursorQuery,
  items: (page: TPage) => TItem[],
): Promise<CursorPage<TItem>> {
  const params = new URLSearchParams();
  Object.entries(query).forEach(([name, value]) => {
    if (value !== undefined && value !== '') params.set(name, String(value));
  });
  const page = await apiRequest<TPage>(`${path}?${params}`);
  return { items: items(page), nextCursor: page.nextCursor ?? '' };
}

export function setQueryParameter(
  query: URLSearchParams,
  name: string,
  value?: string,
): void {
  const normalized = value?.trim();
  if (normalized) query.set(name, normalized);
}

function isApiResponse<T>(value: unknown): value is ApiResponse<T> {
  return typeof value === 'object'
    && value !== null
    && typeof (value as { code?: unknown }).code === 'number'
    && (
      (value as { reason?: unknown }).reason === undefined
      || typeof (value as { reason?: unknown }).reason === 'string'
    )
    && typeof (value as { msg?: unknown }).msg === 'string'
    && 'data' in value;
}
