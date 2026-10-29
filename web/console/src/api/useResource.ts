import { useEffect, useState } from 'react';
import { listResources, type ListQuery, type Page } from './resources';
import type { Kind, Resources } from '../domain/resources';

export function useResourcePage<K extends Kind>(
  namespace: string,
  kind: K,
  query: ListQuery,
  revision: number,
) {
  const [state, setState] = useState<{
    key?: string;
    page?: Page<Resources[K]>;
    loading: boolean;
    error?: unknown;
  }>({ loading: true });
  const { pageNum, pageSize, displayName, gatewayName } = query;
  const key = JSON.stringify([namespace, kind, pageNum, pageSize, displayName, gatewayName]);

  useEffect(() => {
    const controller = new AbortController();
    setState((previous) => ({
      key,
      page: previous.key === key ? previous.page : undefined,
      loading: true,
    }));
    listResources(
      namespace,
      kind,
      { pageNum, pageSize, displayName, gatewayName },
      controller.signal,
    ).then(
      (page) => {
        if (!controller.signal.aborted) setState({ key, page, loading: false });
      },
      (error) => {
        if (!controller.signal.aborted)
          setState((previous) => ({
            key,
            page: previous.key === key ? previous.page : undefined,
            error,
            loading: false,
          }));
      },
    );

    return () => controller.abort();
  }, [namespace, kind, pageNum, pageSize, displayName, gatewayName, revision, key]);

  return state.key === key ? state : { loading: true };
}
