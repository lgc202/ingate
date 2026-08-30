import { useCallback, useEffect, useRef, useState } from 'react';
import { errorMessage } from './errors';
import type { CursorPage } from './client';

export interface ResourceState<T> {
  data: T | null;
  loading: boolean;
  error: Error | null;
  reload: (options?: ReloadOptions) => Promise<void>;
}

export interface ReloadOptions {
  silent?: boolean;
}

export interface ResourceOptions<T> {
  enabled?: boolean;
  autoRefreshWhen?: (data: T) => boolean;
  autoRefreshInterval?: number;
  maxAutoRefreshes?: number;
}

export interface CursorResource<T> extends ResourceState<CursorPage<T>> {
  page: number;
  hasPrevious: boolean;
  hasNext: boolean;
  next: () => void;
  previous: () => void;
  reset: () => void;
}

export function useResource<T>(load: () => Promise<T>, options?: ResourceOptions<T>): ResourceState<T> {
  const requestVersion = useRef(0);
  const autoRefreshCount = useRef(0);
  const optionsRef = useRef(options);
  optionsRef.current = options;
  const [state, setState] = useState<Omit<ResourceState<T>, 'reload'>>({
    data: null,
    loading: options?.enabled !== false,
    error: null,
  });

  const reload = useCallback(async (reloadOptions?: ReloadOptions) => {
    const version = ++requestVersion.current;
    const silent = reloadOptions?.silent === true;
    if (!silent) {
      setState((current) => ({ ...current, loading: true, error: null }));
    }

    try {
      const data = await load();
      if (version !== requestVersion.current) {
        return;
      }
      if (silent) {
        setState((current) => ({ ...current, data, loading: false, error: null }));
      } else {
        setState({ data, loading: false, error: null });
      }
    } catch (error: unknown) {
      if (version !== requestVersion.current) {
        return;
      }
      if (silent) {
        setState((current) => ({ ...current, loading: false }));
        return;
      }
      setState({
        data: null,
        loading: false,
        error: new Error(errorMessage(error, '请求处理失败')),
      });
    }
  }, [load]);

  useEffect(() => {
    if (optionsRef.current?.enabled === false) {
      return;
    }
    void reload();

    return () => {
      requestVersion.current++;
    };
  }, [reload, options?.enabled]);

  useEffect(() => {
    const currentOptions = optionsRef.current;
    if (!state.data || !currentOptions?.autoRefreshWhen?.(state.data)) {
      autoRefreshCount.current = 0;
      return;
    }

    const maxAutoRefreshes = currentOptions.maxAutoRefreshes ?? 30;
    if (autoRefreshCount.current >= maxAutoRefreshes) {
      return;
    }
    const timer = window.setTimeout(() => {
      autoRefreshCount.current++;
      void reload({ silent: true });
    }, currentOptions.autoRefreshInterval ?? 2_000);
    return () => window.clearTimeout(timer);
  }, [reload, state.data]);

  return { ...state, reload };
}

// useCursorResource 保存服务端游标历史，使列表只读取当前页且仍能前后翻页
export function useCursorResource<T>(
  load: (cursor: string) => Promise<CursorPage<T>>,
  options?: ResourceOptions<CursorPage<T>>,
): CursorResource<T> {
  const cursors = useRef(['']);
  const [pageIndex, setPageIndex] = useState(0);
  const loadPage = useCallback(() => load(cursors.current[pageIndex] ?? ''), [load, pageIndex]);
  const resource = useResource(loadPage, options);

  const next = useCallback(() => {
    const nextCursor = resource.data?.nextCursor;
    if (!nextCursor) return;
    cursors.current[pageIndex + 1] = nextCursor;
    cursors.current.length = pageIndex + 2;
    setPageIndex((current) => current + 1);
  }, [pageIndex, resource.data?.nextCursor]);
  const previous = useCallback(() => setPageIndex((current) => Math.max(0, current - 1)), []);
  const reset = useCallback(() => {
    cursors.current = [''];
    setPageIndex(0);
  }, []);

  return {
    ...resource,
    page: pageIndex + 1,
    hasPrevious: pageIndex > 0,
    hasNext: Boolean(resource.data?.nextCursor),
    next,
    previous,
    reset,
  };
}
