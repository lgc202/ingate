import { useCallback, useEffect, useRef, useState } from 'react';
import { normalizeApiError } from './errors';

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
  autoRefreshWhen?: (data: T) => boolean;
  autoRefreshInterval?: number;
  maxAutoRefreshes?: number;
}

export function useResource<T>(load: () => Promise<T>, options?: ResourceOptions<T>): ResourceState<T> {
  const requestVersion = useRef(0);
  const autoRefreshCount = useRef(0);
  const optionsRef = useRef(options);
  optionsRef.current = options;
  const [state, setState] = useState<Omit<ResourceState<T>, 'reload'>>({
    data: null,
    loading: true,
    error: null,
  });

  const reload = useCallback(async (options?: ReloadOptions) => {
    const version = ++requestVersion.current;
    const silent = options?.silent === true;
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
      const normalizedError = normalizeApiError(error);
      setState({
        data: null,
        loading: false,
        error: new Error(normalizedError.message),
      });
    }
  }, [load]);

  useEffect(() => {
    void reload();

    return () => {
      requestVersion.current++;
    };
  }, [reload]);

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
