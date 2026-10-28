export class APIError extends Error {
  readonly status: number;
  readonly reason: string;

  constructor(status: number, reason: string, message: string) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.reason = reason;
  }
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set('Accept', 'application/protojson');
  if (options.body) headers.set('Content-Type', 'application/json');
  let response: Response;
  let text: string;
  try {
    response = await fetch('/api/admin/v1' + path, {
      ...options,
      // 只使用同源 Cookie；不接收、存储、拼装 Bearer Token。
      credentials: 'same-origin',
      signal: options.signal
        ? AbortSignal.any([options.signal, AbortSignal.timeout(15000)])
        : AbortSignal.timeout(15000),
      headers,
    });
    text = await response.text();
  } catch (cause) {
    if (options.signal?.aborted) throw cause;
    throw new APIError(
      0,
      'NETWORK_ERROR',
      options.body || options.method === 'DELETE'
        ? '未能确认写入结果，请刷新查询后再决定是否重试，避免重复提交。'
        : '无法连接管理 API 或请求超时，请检查服务和同源代理。',
    );
  }
  let body: unknown;
  try {
    body = text ? JSON.parse(text) : {};
  } catch {
    throw new APIError(
      response.status,
      'INVALID_RESPONSE',
      '管理 API 未返回有效 JSON，请检查同源代理配置。',
    );
  }

  if (!response.ok) {
    const error = body as { reason?: string; message?: string } | null;
    throw new APIError(
      response.status,
      error?.reason ?? 'REQUEST_FAILED',
      error?.message ?? `请求失败（${response.status}）`,
    );
  }

  return body as T;
}

export function errorMessage(error: unknown): string {
  if (error instanceof APIError && error.status === 401) {
    return '浏览器登录尚未接通或会话已过期。当前 Admin API 使用 Bearer 认证；前端不会在浏览器存储 Token。';
  }
  if (error instanceof APIError && error.status === 409)
    return '资源版本已变化，请刷新后重新编辑；不会自动覆盖其他人的修改。';

  return error instanceof Error ? error.message : '操作失败，请重试。';
}
