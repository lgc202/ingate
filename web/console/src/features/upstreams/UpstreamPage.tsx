import { useState } from 'react';
import { deleteUpstream, listUpstreams, saveUpstream } from '@/api/upstreams';
import { useResource } from '@/api/useResource';
import { Badge, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { ModelProvider, Upstream, UpstreamType } from '@/domain/upstream';
import {
  modelProviderDefinitions,
  modelProviderLabel,
  upstreamLoadBalancePolicyLabel,
  upstreamLoadBalancePolicyOptions,
  upstreamProtocolLabel,
  upstreamTypeLabel,
  upstreamTypeOptions,
} from '@/domain/upstream';
import type { UpstreamFormDraft } from './form';
import {
  buildUpstreamPayload,
  changeModelProvider,
  changeUpstreamType,
  cleanEndpointAddress,
  createUpstreamDraft,
  validateUpstreamDraft,
} from './form';
import { Plus, Trash2, Edit3, Server, Sparkles, ChevronDown } from 'lucide-react';

export function UpstreamPage() {
  const upstreams = useResource(listUpstreams);
  const [selectedUpstreamId, setSelectedUpstreamId] = useState<string>('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Upstream | null>(null);

  const [draft, setDraftState] = useState<UpstreamFormDraft>(() => createUpstreamDraft());
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  if (upstreams.loading && !upstreams.data) {
    return (
      <PageFrame title="上游服务">
        <ResourceStatePanel title="正在加载服务配置..." message="从管理 API 获取数据中" />
      </PageFrame>
    );
  }

  if (upstreams.error || !upstreams.data) {
    return (
      <PageFrame title="上游服务">
        <ResourceStatePanel title="服务加载失败" message={upstreams.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const upstreamList = upstreams.data.upstreams;
  const activeUpstream = upstreamList.find((item) => item.id === selectedUpstreamId) ?? upstreamList[0] ?? null;

  const handleCreateNew = () => {
    setIsEditing(false);
    setDraftState(createUpstreamDraft());
    setDrawerOpen(true);
  };

  const handleEdit = (upstream: Upstream) => {
    setIsEditing(true);
    setDraftState(createUpstreamDraft(upstream));
    setDrawerOpen(true);
  };

  const confirmDeleteUpstream = async () => {
    if (!deleteCandidate) return;
    setDeleting(true);
    try {
      await deleteUpstream(deleteCandidate.id);
      await upstreams.reload();
      setNotice({ message: `已成功删除服务 ${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const validation = validateUpstreamDraft(draft);

  const handleSave = async () => {
    // If it's a model service, automatically set httpsEnabled based on modelBaseURL
    const finalDraft =
      draft.type === 'model'
        ? { ...draft, httpsEnabled: draft.modelBaseURL.startsWith('https://') }
        : draft;

    const finalValidation = validateUpstreamDraft(finalDraft);
    if (!finalValidation.valid) {
      setNotice({ message: finalValidation.summary, tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const payload = buildUpstreamPayload(finalDraft);
      const result = await saveUpstream(payload);
      await upstreams.reload();
      setSelectedUpstreamId(result.changeId ?? payload.id ?? '');
      setNotice({ message: result.message, tone: 'success' });
      setDrawerOpen(false);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageFrame
      title="上游服务"
      subtitle={`已配置 ${upstreamList.length} 个 AI 模型与应用微服务端点`}
      actions={
        <button
          type="button"
          onClick={handleCreateNew}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          创建服务
        </button>
      }
    >
      <div className="space-y-6 mt-2">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />

        {/* Clean High Density Upstream Table */}
        <Panel>
          {upstreamList.length === 0 ? (
            <EmptyState title="暂无配置的上游服务" message="点击右上角按钮创建大模型服务或应用微服务端点" />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50 font-medium">
                    <th className="py-2.5 px-3">名称 / ID</th>
                    <th className="py-2.5 px-3">类型与协议</th>
                    <th className="py-2.5 px-3">负载均衡</th>
                    <th className="py-2.5 px-3">目标 Endpoints / Models</th>
                    <th className="py-2.5 px-3">创建时间</th>
                    <th className="py-2.5 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 font-normal">
                  {upstreamList.map((item) => {
                    const isSelected = activeUpstream?.id === item.id;
                    const isModel = item.type === 'model';
                    const models = (item as any).spec?.model?.models ?? [];
                    const provider = (item as any).spec?.model?.provider;

                    return (
                      <tr
                        key={item.id}
                        onClick={() => setSelectedUpstreamId(item.id)}
                        className={`hover:bg-slate-50/80 transition-colors cursor-pointer ${
                          isSelected ? 'bg-blue-50/40' : ''
                        }`}
                      >
                        <td className="py-3 px-3">
                          <div className="flex items-center gap-2">
                            {isModel ? (
                              <Sparkles className="w-4 h-4 text-purple-600 shrink-0" />
                            ) : (
                              <Server className="w-4 h-4 text-blue-600 shrink-0" />
                            )}
                            <div>
                              <div className="font-semibold text-slate-900">{item.name}</div>
                              <div className="text-[11px] font-mono text-slate-400">{item.id}</div>
                            </div>
                          </div>
                        </td>

                        <td className="py-3 px-3 space-y-1">
                          <div className="flex items-center gap-1.5">
                            <Badge tone={isModel ? 'purple' : 'accent'}>
                              {upstreamTypeLabel(item.type)}
                            </Badge>
                            {provider ? <Badge tone="neutral">{modelProviderLabel(provider)}</Badge> : null}
                          </div>
                          <div className="text-[11px] text-slate-500">
                            {upstreamConnectionSummary(item)}
                          </div>
                        </td>

                        <td className="py-3 px-3 text-slate-700 font-mono text-[11px]">
                          {upstreamLoadBalancePolicyLabel(item.loadBalancePolicy)}
                        </td>

                        <td className="py-3 px-3">
                          {isModel ? (
                            <div className="flex flex-wrap gap-1">
                              {models.slice(0, 3).map((m: any) => (
                                <span
                                  key={m.name}
                                  className="px-1.5 py-0.5 bg-purple-50 text-purple-700 text-[10px] font-mono rounded border border-purple-200/50"
                                >
                                  {m.name}
                                </span>
                              ))}
                              {models.length > 3 && (
                                <span className="text-[10px] text-slate-400 font-mono">
                                  +{models.length - 3} 个模型
                                </span>
                              )}
                            </div>
                          ) : (
                            <div className="space-y-0.5 font-mono text-[11px] text-slate-600">
                              {(item.endpoints ?? []).slice(0, 2).map((ep, idx) => (
                                <div key={idx}>
                                  {ep.address}:{ep.port} (权重 {ep.weight})
                                </div>
                              ))}
                              {(item.endpoints ?? []).length > 2 && (
                                <div className="text-[10px] text-slate-400">
                                  +{(item.endpoints ?? []).length - 2} 个端点
                                </div>
                              )}
                            </div>
                          )}
                        </td>

                        <td className="py-3 px-3 text-slate-400 text-[11px]">
                          {formatDateTime(item.createdAt)}
                        </td>

                        <td className="py-3 px-3 text-right space-x-1" onClick={(e) => e.stopPropagation()}>
                          <button
                            type="button"
                            onClick={() => handleEdit(item)}
                            className="p-1.5 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded transition-colors cursor-pointer"
                            title="编辑"
                          >
                            <Edit3 className="w-3.5 h-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteCandidate(item)}
                            className="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded transition-colors cursor-pointer"
                            title="删除"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>

      {/* Slide-over Drawer Form */}
      <Drawer
        title={isEditing ? `编辑服务: ${draft.name}` : '新建上游服务'}
        subtitle="选择服务类型（大模型服务 / 应用微服务）并配置通信参数"
        isOpen={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        <div className="space-y-5">
          {/* Service Basic Details */}
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">
                展示名称 <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                value={draft.name}
                onChange={(e) => setDraftState({ ...draft, name: e.target.value })}
                placeholder="例如: DeepSeek 服务 / 订单后端"
                className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-blue-500/20"
              />
              {validation.errors.name && (
                <p className="text-[11px] text-rose-600 mt-1">{validation.errors.name}</p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">服务类型</label>
                <select
                  value={draft.type}
                  disabled={isEditing}
                  onChange={(e) => setDraftState(changeUpstreamType(draft, e.target.value as UpstreamType))}
                  className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white focus:outline-hidden"
                >
                  {upstreamTypeOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">负载均衡策略</label>
                <select
                  value={draft.loadBalancePolicy}
                  onChange={(e) => setDraftState({ ...draft, loadBalancePolicy: e.target.value as any })}
                  className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white focus:outline-hidden"
                >
                  {upstreamLoadBalancePolicyOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            {/* Model Provider Options (AI 大模型服务: 极简 3 字段) */}
            {draft.type === 'model' && (
              <div className="p-4 bg-purple-50/50 border border-purple-200/60 rounded-xl space-y-4">
                <div className="flex items-center gap-2">
                  <Sparkles className="w-4 h-4 text-purple-600" />
                  <h4 className="text-xs font-semibold text-purple-900">大模型 Provider 配置</h4>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">选择模型厂商 (Provider)</label>
                  <select
                    value={draft.modelProvider}
                    onChange={(e) => {
                      const next = changeModelProvider(draft, e.target.value as ModelProvider);
                      const baseURL = next.modelBaseURL || draft.modelBaseURL;
                      setDraftState({
                        ...draft,
                        ...next,
                        httpsEnabled: baseURL.startsWith('https://'),
                      });
                    }}
                    className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white font-medium"
                  >
                    {modelProviderDefinitions.map((p) => (
                      <option key={p.value} value={p.value}>
                        {p.label} ({p.protocol})
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">模型 API Base URL</label>
                  <input
                    type="text"
                    value={draft.modelBaseURL}
                    onChange={(e) =>
                      setDraftState({
                        ...draft,
                        modelBaseURL: e.target.value,
                        httpsEnabled: e.target.value.startsWith('https://'),
                      })
                    }
                    placeholder="https://api.openai.com/v1"
                    className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg font-mono"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">API Key 鉴权凭据</label>
                  <input
                    type="password"
                    value={draft.apiKey}
                    onChange={(e) => setDraftState({ ...draft, apiKey: e.target.value })}
                    placeholder="sk-xxxxxxxxxxxxxxxx"
                    className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg font-mono"
                  />
                </div>
              </div>
            )}

            {/* Endpoints List & Advanced Network Config for Application microservices */}
            {draft.type === 'application' && (
              <>
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold text-slate-700">上游实例 Endpoints</label>
                    <button
                      type="button"
                      onClick={() =>
                        setDraftState({
                          ...draft,
                          endpoints: [
                            ...draft.endpoints,
                            { id: String(Date.now()), address: '', port: 8080, weight: 100, enabled: true },
                          ],
                        })
                      }
                      className="text-[11px] font-semibold text-blue-600 hover:text-blue-800 cursor-pointer"
                    >
                      + 添加 Endpoint
                    </button>
                  </div>

                  {/* Explicit Header Row */}
                  <div className="grid grid-cols-[1fr_80px_80px_28px] gap-2 px-1 text-[11px] font-medium text-slate-500">
                    <div>地址 / Host</div>
                    <div>端口 (Port)</div>
                    <div>权重 (Weight)</div>
                    <div></div>
                  </div>

                  <div className="space-y-2">
                    {draft.endpoints.map((ep, idx) => (
                      <div key={idx} className="grid grid-cols-[1fr_80px_80px_28px] gap-2 items-center">
                        <input
                          type="text"
                          placeholder="例如: www.baidu.com (不含 http://)"
                          value={ep.address}
                          onChange={(e) => {
                            const cleaned = cleanEndpointAddress(e.target.value);
                            const list = [...draft.endpoints];
                            list[idx] = { ...list[idx], address: cleaned };
                            setDraftState({ ...draft, endpoints: list });
                          }}
                          onBlur={(e) => {
                            const cleaned = cleanEndpointAddress(e.target.value);
                            if (cleaned !== ep.address) {
                              const list = [...draft.endpoints];
                              list[idx] = { ...list[idx], address: cleaned };
                              setDraftState({ ...draft, endpoints: list });
                            }
                          }}
                          className="px-3 py-1.5 text-xs border border-slate-300 rounded-md font-mono"
                        />
                        <input
                          type="number"
                          placeholder="80"
                          value={ep.port}
                          onChange={(e) => {
                            const list = [...draft.endpoints];
                            list[idx] = { ...list[idx], port: Number(e.target.value) };
                            setDraftState({ ...draft, endpoints: list });
                          }}
                          className="px-3 py-1.5 text-xs border border-slate-300 rounded-md font-mono"
                        />
                        <input
                          type="number"
                          placeholder="100"
                          value={ep.weight}
                          onChange={(e) => {
                            const list = [...draft.endpoints];
                            list[idx] = { ...list[idx], weight: Number(e.target.value) };
                            setDraftState({ ...draft, endpoints: list });
                          }}
                          className="px-3 py-1.5 text-xs border border-slate-300 rounded-md font-mono"
                        />
                        {draft.endpoints.length > 1 ? (
                          <button
                            type="button"
                            onClick={() => {
                              const list = draft.endpoints.filter((_, i) => i !== idx);
                              setDraftState({ ...draft, endpoints: list });
                            }}
                            className="p-1 text-slate-400 hover:text-rose-600 cursor-pointer flex justify-center"
                            title="删除端点"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        ) : (
                          <div />
                        )}
                      </div>
                    ))}
                  </div>
                  {validation.errors.endpoints && (
                    <p className="text-[11px] text-rose-600 mt-1">{validation.errors.endpoints}</p>
                  )}
                </div>

                {/* Advanced Network Settings in Collapsible Details for Application services */}
                <details className="group border border-slate-200 rounded-xl bg-slate-50/50 overflow-hidden">
                  <summary className="px-4 py-3 text-xs font-semibold text-slate-700 bg-slate-100/60 hover:bg-slate-100 flex items-center justify-between cursor-pointer select-none">
                    <span>高级网络与安全配置 (可选)</span>
                    <ChevronDown className="w-4 h-4 text-slate-400 group-open:rotate-180 transition-transform" />
                  </summary>
                  <div className="p-4 space-y-3 bg-white border-t border-slate-200">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs font-medium text-slate-600 mb-1">传输协议</label>
                        <select
                          value={draft.protocol}
                          onChange={(e) => setDraftState({ ...draft, protocol: e.target.value as any })}
                          className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white"
                        >
                          <option value="HTTP">HTTP/1.1</option>
                          <option value="HTTP2">HTTP/2</option>
                          <option value="GRPC">gRPC</option>
                        </select>
                      </div>

                      <div className="flex items-center pt-5">
                        <label className="flex items-center gap-2 cursor-pointer text-xs font-medium text-slate-700">
                          <input
                            type="checkbox"
                            checked={draft.httpsEnabled}
                            onChange={(e) => setDraftState({ ...draft, httpsEnabled: e.target.checked })}
                            className="rounded border-slate-300 text-blue-600"
                          />
                          启用 TLS (SNI 校验)
                        </label>
                      </div>
                    </div>
                  </div>
                </details>
              </>
            )}
          </div>

          {/* Form Action Footer */}
          <div className="pt-4 border-t border-slate-200 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={() => setDrawerOpen(false)}
              className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors cursor-pointer"
            >
              取消
            </button>
            <button
              type="button"
              disabled={submitting}
              onClick={handleSave}
              className="px-4 py-2 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors disabled:opacity-50 cursor-pointer"
            >
              {submitting ? '提交中...' : '保存配置'}
            </button>
          </div>
        </div>
      </Drawer>

      {/* Delete Confirmation Modal */}
      <Modal
        title="确认删除服务"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-600">
            确定要删除上游服务 <strong className="text-slate-900 font-mono">{deleteCandidate?.name}</strong> ({deleteCandidate?.id}) 吗？
          </p>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setDeleteCandidate(null)}
              className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg cursor-pointer"
            >
              取消
            </button>
            <button
              type="button"
              disabled={deleting}
              onClick={confirmDeleteUpstream}
              className="px-4 py-2 text-xs font-semibold text-white bg-rose-600 hover:bg-rose-700 rounded-lg shadow-xs cursor-pointer"
            >
              {deleting ? '删除中...' : '确认删除'}
            </button>
          </div>
        </div>
      </Modal>
    </PageFrame>
  );
}

function upstreamConnectionSummary(upstream: Upstream) {
  const transport = upstream.tls ? 'HTTPS' : 'HTTP';
  return upstream.protocol !== 'HTTP'
    ? `${transport} · ${upstreamProtocolLabel(upstream.protocol)}`
    : transport;
}
