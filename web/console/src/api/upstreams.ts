import { apiListAllByCursor, apiRequest } from './client';
import type { CursorPagedResponse } from './client';
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
