import { useEffect, useState } from 'react';
import { normalizeApiError } from './errors';

export interface ResourceState<T> {
  data: T | null;
  loading: boolean;
  error: Error | null;
}

export function useResource<T>(load: () => Promise<T>): ResourceState<T> {
  const [state, setState] = useState<ResourceState<T>>({
    data: null,
    loading: true,
    error: null,
  });

  useEffect(() => {
    let active = true;

    setState((current) => ({ ...current, loading: true, error: null }));

    load()
      .then((data) => {
        if (active) {
          setState({ data, loading: false, error: null });
        }
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }

        const normalizedError = normalizeApiError(error);

        setState({
          data: null,
          loading: false,
          error: new Error(normalizedError.message),
        });
      });

    return () => {
      active = false;
    };
  }, [load]);

  return state;
}
