export type ApiErrorCode = 'network_error' | 'validation_error' | 'permission_denied' | 'not_found' | 'unknown';

export interface ApiError {
  code: ApiErrorCode;
  message: string;
  fieldErrors?: Record<string, string>;
}

export function normalizeApiError(error: unknown): ApiError {
  if (error instanceof Error) {
    return {
      code: 'unknown',
      message: error.message,
    };
  }

  return {
    code: 'unknown',
    message: '请求处理失败',
  };
}
