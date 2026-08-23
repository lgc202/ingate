import { apiListAllByCursor, apiListPageByCursor, apiRequest } from './client';
import type { CursorPage, CursorPagedResponse } from './client';
import { normalizeResourceState } from '@/domain/common';
import type { Upstream, UpstreamList, UpstreamMutationPayload } from '@/domain/upstream';

interface UpstreamListResponse extends UpstreamList, CursorPagedResponse {}

export async function listUpstreams(): Promise<UpstreamList> {
  const upstreams = await apiListAllByCursor<UpstreamListResponse, Upstream>('/upstreams', (page) => page.upstreams ?? []);
  return {
    upstreams: upstreams.map((upstream) => ({
      ...upstream,
      version: Number(upstream.version),
      state: normalizeResourceState(upstream.state),
    })),
  };
}

export async function listUpstreamPage(input: {
  limit: number; cursor: string; query?: string; state?: string; type?: string;
}): Promise<CursorPage<Upstream>> {
  const page = await apiListPageByCursor<UpstreamListResponse, Upstream>('/upstreams', input, (value) => value.upstreams ?? []);
  return { ...page, items: page.items.map(upstreamFromAPI) };
}

function upstreamFromAPI(upstream: Upstream): Upstream {
  return { ...upstream, version: Number(upstream.version), state: normalizeResourceState(upstream.state) };
}

export async function saveUpstream(payload: UpstreamMutationPayload): Promise<Upstream> {
  const path = payload.id ? `/upstreams/${encodeURIComponent(payload.id)}` : '/upstreams';
  const upstream = await apiRequest<Upstream>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
  return { ...upstream, version: Number(upstream.version), state: normalizeResourceState(upstream.state) };
}

export async function deleteUpstream(id: string, version: number) {
  await apiRequest<Record<string, never>>(`/upstreams/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}
