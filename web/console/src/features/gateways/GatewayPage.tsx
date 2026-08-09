import { useState } from 'react';
import { Link } from 'react-router-dom';
import { listCertificates } from '@/api/certificates';
import { deleteGateway, listGateways, saveGateway, setGatewayEnabled } from '@/api/gateways';
import { getPolicyWorkspace } from '@/api/policies';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Badge, Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Gateway } from '@/domain/gateway';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';
import type { GatewayFormDraft } from './form';
import {
  buildGatewayPayload,
  createGatewayDraft,
  validateGatewayDraft,
} from './form';
import { Plus, Trash2, Edit3, Layers3, Power, Globe, KeyRound } from 'lucide-react';

export function GatewayPage() {
  const { canWriteConfiguration } = useAuth();
  const gateways = useResource(listGateways);
  const certificates = useResource(listCertificates);
  const policies = useResource(getPolicyWorkspace);

  const [selectedGatewayId, setSelectedGatewayId] = useState<string>('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Gateway | null>(null);

  const [draft, setDraftState] = useState<GatewayFormDraft>(() => createGatewayDraft());
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (gateways.loading && !gateways.data) {
    return (
      <PageFrame title="网关">
        <ResourceStatePanel title="正在加载网关列表..." message="从管理 API 获取数据中" />
      </PageFrame>
    );
  }

  if (gateways.error || !gateways.data) {
    return (
      <PageFrame title="网关">
        <ResourceStatePanel title="网关加载失败" message={gateways.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const gatewayList = gateways.data.gateways;
  const activeGateway = gatewayList.find((item) => item.id === selectedGatewayId) ?? gatewayList[0] ?? null;

  const certificateList = certificates.data?.certificates ?? [];
  const policyWorkspace = policies.data ?? null;

  const handleCreateNew = () => {
    setIsEditing(false);
    setDraftState(createGatewayDraft());
    setDrawerOpen(true);
  };

  const handleEdit = (gateway: Gateway) => {
    setIsEditing(true);
    setDraftState(createGatewayDraft(gateway));
    setDrawerOpen(true);
  };

  const toggleGatewayStatus = async (gateway: Gateway) => {
    try {
      await setGatewayEnabled(gateway.id, !gateway.enabled);
      await gateways.reload();
      setNotice({
        message: `网关 ${gateway.name} 状态已更新`,
        tone: 'success',
      });
    } catch (err) {
      setNotice({ message: err instanceof Error ? err.message : '状态切换失败', tone: 'error' });
    }
  };

  const confirmDeleteGateway = async () => {
    if (!deleteCandidate) return;
    setDeleting(true);
    try {
      await deleteGateway(deleteCandidate.id);
      await gateways.reload();
      setNotice({ message: `已成功删除网关 ${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (err) {
      setNotice({ message: err instanceof Error ? err.message : '删除失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const draftValidation = validateGatewayDraft(draft);

  const handleSave = async () => {
    if (!draftValidation.valid) {
      setNotice({ message: '包含不合法的输入字段', tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const payload = buildGatewayPayload(draft);
      const result = await saveGateway(payload);
      await gateways.reload();
      setSelectedGatewayId(result.changeId ?? payload.id ?? '');
      setNotice({ message: result.message, tone: 'success' });
      setDrawerOpen(false);
    } catch (err) {
      setNotice({ message: err instanceof Error ? err.message : '保存失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageFrame
      title="网关"
      subtitle={`当前已配置 ${gatewayList.length} 个网关入口`}
      actions={canWriteConfiguration ? (
        <button
          type="button"
          onClick={handleCreateNew}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          新建网关
        </button>
      ) : undefined}
    >
      <div className="space-y-6 mt-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />

        {/* Gateways High-Density Table */}
        <Panel>
          {gatewayList.length === 0 ? (
            <EmptyState
              title="暂无网关配置"
              message={canWriteConfiguration ? '点击右上角按钮创建网关' : '当前环境还没有网关配置'}
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50 font-medium">
                    <th className="py-2.5 px-3">名称 / ID</th>
                    <th className="py-2.5 px-3">状态</th>
                    <th className="py-2.5 px-3">监听器 (Listeners)</th>
                    <th className="py-2.5 px-3">关联 Hostnames</th>
                    <th className="py-2.5 px-3">创建时间</th>
                    <th className="py-2.5 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 font-normal">
                  {gatewayList.map((item) => {
                    const isSelected = activeGateway?.id === item.id;
                    const listeners = item.listeners ?? [];
                    return (
                      <tr
                        key={item.id}
                        onClick={() => setSelectedGatewayId(item.id)}
                        className={`hover:bg-slate-50/80 transition-colors cursor-pointer ${
                          isSelected ? 'bg-blue-50/40' : ''
                        }`}
                      >
                        <td className="py-3 px-3">
                          <div className="flex items-center gap-2">
                            <Layers3 className="w-4 h-4 text-blue-600 shrink-0" />
                            <div>
                              <div className="font-semibold text-slate-900">{item.name}</div>
                              <div className="text-[11px] font-mono text-slate-400">{item.id}</div>
                            </div>
                          </div>
                        </td>

                        <td className="py-3 px-3">
                          {canWriteConfiguration ? (
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                toggleGatewayStatus(item);
                              }}
                              className="focus:outline-hidden cursor-pointer"
                            >
                              <Badge tone={item.enabled ? 'success' : 'neutral'}>
                                <Power className="w-3 h-3" />
                                {item.enabled ? '运行中' : '已禁用'}
                              </Badge>
                            </button>
                          ) : (
                            <Badge tone={item.enabled ? 'success' : 'neutral'}>
                              <Power className="w-3 h-3" />
                              {item.enabled ? '运行中' : '已禁用'}
                            </Badge>
                          )}
                        </td>

                        <td className="py-3 px-3">
                          <div className="flex flex-wrap gap-1">
                            {listeners.map((l) => (
                              <span
                                key={l.port}
                                className="px-1.5 py-0.5 bg-slate-100 text-slate-700 font-mono text-[10px] rounded border border-slate-200"
                              >
                                {l.protocol}:{l.port}
                              </span>
                            ))}
                          </div>
                        </td>

                        <td className="py-3 px-3">
                          <div className="flex flex-wrap gap-1 font-mono text-[11px] text-slate-600">
                            {(item.hostnames ?? []).map((h) => (
                              <span key={h} className="inline-flex items-center gap-1 px-1.5 py-0.5 bg-slate-50 border border-slate-200 rounded">
                                <Globe className="w-3 h-3 text-slate-400" />
                                {h}
                              </span>
                            ))}
                          </div>
                        </td>

                        <td className="py-3 px-3 text-slate-400 text-[11px]">
                          {formatDateTime(item.createdAt)}
                        </td>

                        <td className="py-3 px-3 text-right space-x-1" onClick={(e) => e.stopPropagation()}>
                          {canWriteConfiguration ? (
                            <>
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
                            </>
                          ) : <span className="text-slate-400">—</span>}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        {/* Governance Policies Target Panel */}
        {activeGateway && policyWorkspace && (
          <Panel title={`网关【${activeGateway.name}】挂载的治理策略`}>
            <GovernancePolicyPanel
              targetKind="Gateway"
              targetID={activeGateway.id}
              targetName={activeGateway.name}
              workspace={policyWorkspace}
              onChanged={() => policies.reload()}
            />
          </Panel>
        )}
      </div>

      {/* Slide-over Drawer Form */}
      <Drawer
        title={isEditing ? `编辑网关: ${draft.name}` : '新建网关实例'}
        subtitle="定义端口监听器 (HTTP/HTTPS) 与 TLS 关联证书"
        isOpen={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        <div className="space-y-5">
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">
                展示名称 <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                value={draft.name}
                onChange={(e) => setDraftState({ ...draft, name: e.target.value })}
                placeholder="例如: 生产 API 统一入口"
                className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-blue-500/20"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">
                绑定域名 (Hostnames, 逗号分隔)
              </label>
              <input
                type="text"
                value={draft.hostnames.join(', ')}
                onChange={(e) =>
                  setDraftState({
                    ...draft,
                    hostnames: e.target.value
                      .split(',')
                      .map((s) => s.trim())
                      .filter(Boolean),
                  })
                }
                placeholder="api.example.com, *.ai.example.com"
                className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg font-mono"
              />
            </div>

            {/* Listener Options */}
            <div className="p-4 bg-slate-50 border border-slate-200 rounded-xl space-y-3">
              <h4 className="text-xs font-semibold text-slate-700">端口监听器设置</h4>
              <div className="space-y-2">
                <label className="flex items-center gap-2 text-xs font-medium text-slate-700 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={draft.listeners.some((l) => l.protocol === 'HTTP')}
                    onChange={(e) => {
                      const hasHTTP = e.target.checked;
                      const newListeners = draft.listeners.filter((l) => l.protocol !== 'HTTP');
                      if (hasHTTP) newListeners.push({ protocol: 'HTTP', port: 80 });
                      setDraftState({ ...draft, listeners: newListeners });
                    }}
                    className="rounded border-slate-300 text-blue-600"
                  />
                  启用 HTTP 监听 (端口 80)
                </label>

                <label className="flex items-center gap-2 text-xs font-medium text-slate-700 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={draft.listeners.some((l) => l.protocol === 'HTTPS')}
                    onChange={(e) => {
                      const hasHTTPS = e.target.checked;
                      const newListeners = draft.listeners.filter((l) => l.protocol !== 'HTTPS');
                      if (hasHTTPS) newListeners.push({ protocol: 'HTTPS', port: 443 });
                      setDraftState({ ...draft, listeners: newListeners });
                    }}
                    className="rounded border-slate-300 text-blue-600"
                  />
                  启用 HTTPS 监听 (端口 443)
                </label>
              </div>
            </div>

            {/* HTTPS TLS Certificate */}
            {draft.listeners.some((l) => l.protocol === 'HTTPS') && (
              <div className="p-4 bg-blue-50/50 border border-blue-200/60 rounded-xl space-y-3">
                <div className="flex items-center gap-2">
                  <KeyRound className="w-4 h-4 text-blue-600" />
                  <h4 className="text-xs font-semibold text-blue-900">HTTPS TLS 证书关联</h4>
                </div>

                {certificateList.length === 0 ? (
                  <p className="text-xs text-amber-700">
                    当前暂未录入 TLS 证书，无法完成 HTTPS 绑定。请先在 <Link to="/certificates" className="underline font-semibold">证书管理</Link> 中添加。
                  </p>
                ) : (
                  <div>
                    <select
                      value={draft.listeners.find((l) => l.protocol === 'HTTPS')?.certificateID ?? ''}
                      onChange={(e) => {
                        const certID = e.target.value;
                        const newListeners = draft.listeners.map((l) =>
                          l.protocol === 'HTTPS' ? { ...l, certificateID: certID } : l,
                        );
                        setDraftState({ ...draft, listeners: newListeners });
                      }}
                      className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white"
                    >
                      <option value="">请选择绑定证书...</option>
                      {certificateList.map((cert) => (
                        <option key={cert.id} value={cert.id}>
                          {cert.name} ({(cert.dnsNames ?? []).join(', ')})
                        </option>
                      ))}
                    </select>
                  </div>
                )}
              </div>
            )}
          </div>

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
              {submitting ? '提交中...' : '保存网关'}
            </button>
          </div>
        </div>
      </Drawer>

      {/* Delete Modal */}
      <Modal
        title="确认删除网关"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-600">
            删除网关 <strong className="text-slate-900 font-mono">{deleteCandidate?.name}</strong> ({deleteCandidate?.id}) 将停止对端口的流量接收。确认操作？
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
              onClick={confirmDeleteGateway}
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
