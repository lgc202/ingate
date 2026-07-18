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

export function useResource<T>(load: () => Promise<T>): ResourceState<T> {
  const requestVersion = useRef(0);
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

  return { ...state, reload };
}
