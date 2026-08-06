import { useEffect, useMemo, useState } from 'react';
import { Check, Copy, Edit3, Key, Plus, Power, RefreshCw, Trash2 } from 'lucide-react';
import {
  createAccessKey,
  deleteAccessKey,
  listAccessKeys,
  rotateAccessKey,
  setAccessKeyEnabled,
  updateAccessKey,
} from '@/api/accessKeys';
import { getRouteWorkspace } from '@/api/routes';
import { useResource } from '@/api/useResource';
import {
  Badge,
  Button,
  Drawer,
  EmptyState,
  Modal,
  PageFrame,
  Panel,
  ResourceStatePanel,
  Toast,
} from '@/components/ui';
import type { AccessKey, AccessKeyMutationPayload } from '@/domain/accessKey';
import { formatMaskedKey, getAccessKeyStatus } from '@/domain/accessKey';
import { formatDateTime } from '@/domain/common';

type ExpirationOption = 'never' | '7d' | '30d' | '90d' | 'custom';

interface AccessKeyNotice {
  message: string;
  tone: 'success' | 'error';
}

export function AccessKeyPage() {
  const workspace = useResource(listAccessKeys);
  const routeWorkspace = useResource(getRouteWorkspace);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingKey, setEditingKey] = useState<AccessKey | null>(null);

  // 表单状态
  const [formName, setFormName] = useState('');
  const [formModels, setFormModels] = useState<string[]>([]);
  const [formExpOption, setFormExpOption] = useState<ExpirationOption>('never');
  const [formCustomExp, setFormCustomExp] = useState('');
  const [originalCustomExp, setOriginalCustomExp] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // 展示一次性 Secret 的 Modal
  const [secretModalState, setSecretModalState] = useState<{
    title: string;
    secret: string;
  } | null>(null);
  const [copied, setCopied] = useState(false);

  // 确认对话框
  const [toggleCandidate, setToggleCandidate] = useState<AccessKey | null>(null);
  const [rotateCandidate, setRotateCandidate] = useState<AccessKey | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<AccessKey | null>(null);
  const [actionBusy, setActionBusy] = useState(false);

  const [notice, setNotice] = useState<AccessKeyNotice | null>(null);

  // 提取路由配置中已发布的模型别名并去重排序
  const publishedModels = useMemo(() => {
    if (!routeWorkspace.data) return [];
    const modelSet = new Set<string>();
    routeWorkspace.data.routes.forEach((route) => {
      route.rules.forEach((rule) => {
        rule.modelRouting?.models.forEach((m) => {
          if (m.model?.trim()) {
            modelSet.add(m.model.trim());
          }
        });
      });
    });
    return Array.from(modelSet).sort();
  }, [routeWorkspace.data]);

  // 计算编辑和新建时可选的模型列表（包含已授权但当前未发布的模型）
  const selectableModels = useMemo(() => {
    const modelSet = new Set<string>(publishedModels);
    formModels.forEach((m) => {
      if (m?.trim()) modelSet.add(m.trim());
    });
    if (editingKey?.allowedModels) {
      editingKey.allowedModels.forEach((m) => {
        if (m?.trim()) modelSet.add(m.trim());
      });
    }
    return Array.from(modelSet).sort();
  }, [publishedModels, formModels, editingKey]);

  // 打开新建抽屉
  const handleOpenCreate = () => {
    setEditingKey(null);
    setFormName('');
    setFormModels([]);
    setFormExpOption('never');
    setFormCustomExp('');
    setOriginalCustomExp('');
    setFormError(null);
    setDrawerOpen(true);
  };

  // 打开编辑抽屉
  const handleOpenEdit = (key: AccessKey) => {
    setEditingKey(key);
    setFormName(key.name);
    setFormModels(key.allowedModels || []);

    if (!key.expiresAt) {
      setFormExpOption('never');
      setFormCustomExp('');
      setOriginalCustomExp('');
    } else {
      setFormExpOption('custom');
      // 转换 ISO 时间为 local datetime-local 格式 (YYYY-MM-DDTHH:mm)
      const d = new Date(key.expiresAt);
      if (!Number.isNaN(d.getTime())) {
        const isoLocal = new Date(d.getTime() - d.getTimezoneOffset() * 60000)
          .toISOString()
          .slice(0, 16);
        setFormCustomExp(isoLocal);
        setOriginalCustomExp(isoLocal);
      } else {
        setFormCustomExp('');
        setOriginalCustomExp('');
      }
    }
    setFormError(null);
    setDrawerOpen(true);
  };

  // 计算 expiresAt ISO 字符串
  const calculateExpiresAt = (): string | undefined => {
    if (formExpOption === 'never') return undefined;
    const now = Date.now();
    if (formExpOption === '7d') return new Date(now + 7 * 86400 * 1000).toISOString();
    if (formExpOption === '30d') return new Date(now + 30 * 86400 * 1000).toISOString();
    if (formExpOption === '90d') return new Date(now + 90 * 86400 * 1000).toISOString();
    if (formExpOption === 'custom' && formCustomExp) {
      const d = new Date(formCustomExp);
      return d.toISOString();
    }
    return undefined;
  };

  // 提交表单（新建或编辑）
  const handleSubmitForm = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    const trimmedName = formName.trim();
    if (!trimmedName) {
      setFormError('密钥名称不能为空');
      return;
    }

    if (formExpOption === 'custom') {
      if (!formCustomExp) {
        setFormError('请选择自定义有效截止时间');
        return;
      }
      const expTime = new Date(formCustomExp).getTime();
      const isNewCustomDate = !editingKey || formCustomExp !== originalCustomExp;
      if (isNewCustomDate && (Number.isNaN(expTime) || expTime <= Date.now())) {
        setFormError('自定义有效期必须是未来时间');
        return;
      }
    }

    const payload: AccessKeyMutationPayload = {
      name: trimmedName,
      allowedModels: formModels,
      expiresAt: calculateExpiresAt(),
    };

    setSubmitting(true);
    try {
      if (editingKey) {
        await updateAccessKey(editingKey.id, payload);
        setNotice({ message: `访问密钥“${trimmedName}”更新成功`, tone: 'success' });
        setDrawerOpen(false);
        try {
          await workspace.reload();
        } catch {
          // 列表刷新失败静默捕获，避免遮挡成功通知
        }
      } else {
        const res = await createAccessKey(payload);
        setDrawerOpen(false);
        setSecretModalState({
          title: `访问密钥“${trimmedName}”创建成功`,
          secret: res.secret,
        });
        void workspace.reload({ silent: true });
      }
    } catch (err: unknown) {
      setFormError(err instanceof Error ? err.message : '操作失败');
    } finally {
      setSubmitting(false);
    }
  };

  // 启停处理
  const confirmToggleEnabled = async () => {
    if (!toggleCandidate) return;
    setActionBusy(true);
    const targetState = !toggleCandidate.enabled;
    try {
      await setAccessKeyEnabled(toggleCandidate.id, targetState);
      setNotice({
        message: `访问密钥“${toggleCandidate.name}”已${targetState ? '启用' : '停用'}`,
        tone: 'success',
      });
      setToggleCandidate(null);
      workspace.reload();
    } catch (err: unknown) {
      setNotice({
        message: err instanceof Error ? err.message : '更新状态失败',
        tone: 'error',
      });
    } finally {
      setActionBusy(false);
    }
  };

  // 密钥轮换处理
  const confirmRotate = async () => {
    if (!rotateCandidate) return;
    setActionBusy(true);
    try {
      const res = await rotateAccessKey(rotateCandidate.id);
      const name = rotateCandidate.name;
      setRotateCandidate(null);
      setSecretModalState({
        title: `访问密钥“${name}”轮换成功`,
        secret: res.secret,
      });
      void workspace.reload({ silent: true });
    } catch (err: unknown) {
      setNotice({
        message: err instanceof Error ? err.message : '密钥轮换失败',
        tone: 'error',
      });
    } finally {
      setActionBusy(false);
    }
  };

  // 删除处理
  const confirmDelete = async () => {
    if (!deleteCandidate) return;
    setActionBusy(true);
    try {
      await deleteAccessKey(deleteCandidate.id);
      setNotice({ message: `访问密钥“${deleteCandidate.name}”已删除`, tone: 'success' });
      setDeleteCandidate(null);
      workspace.reload();
    } catch (err: unknown) {
      setNotice({
        message: err instanceof Error ? err.message : '删除密钥失败',
        tone: 'error',
      });
    } finally {
      setActionBusy(false);
    }
  };

  // 复制完整密钥
  const handleCopySecret = async () => {
    if (secretModalState?.secret) {
      try {
        await navigator.clipboard.writeText(secretModalState.secret);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      } catch (err: unknown) {
        setNotice({
          message: err instanceof Error ? err.message : '复制失败，请手动选择并复制密钥',
          tone: 'error',
        });
      }
    }
  };

  // 关闭 Secret 展示弹窗（必须彻底销毁内存状态，防泄露）
  const handleCloseSecretModal = () => {
    setSecretModalState(null);
    setCopied(false);
  };

  if (workspace.loading && !workspace.data) {
    return (
      <PageFrame title="访问密钥">
        <ResourceStatePanel title="正在加载访问密钥列表..." message="正在连接管理 API 获取数据" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="访问密钥">
        <ResourceStatePanel
          title="数据加载失败"
          message={workspace.error?.message ?? '无法获取访问密钥数据'}
        />
      </PageFrame>
    );
  }

  const accessKeys = workspace.data.accessKeys || [];

  return (
    <PageFrame
      title="访问密钥"
      subtitle="应用或开发者调用 Ingate AI 网关模型 API 时使用的身份认证凭证"
      actions={
        <Button variant="primary" onClick={handleOpenCreate} className="flex items-center gap-1.5">
          <Plus className="w-4 h-4" />
          创建访问密钥
        </Button>
      }
    >
      {accessKeys.length === 0 ? (
        <Panel>
          <div className="space-y-4 text-center py-6">
            <EmptyState
              title="尚未创建访问密钥"
              message="创建密钥后，应用可以使用它调用已发布的模型接口"
            />
            <div className="flex justify-center">
              <Button variant="primary" onClick={handleOpenCreate} className="flex items-center gap-1.5">
                <Plus className="w-4 h-4" />
                创建访问密钥
              </Button>
            </div>
          </div>
        </Panel>
      ) : (
        <Panel>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs border-collapse">
              <thead>
                <tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50">
                  <th className="py-2.5 px-3 font-semibold">名称</th>
                  <th className="py-2.5 px-3 font-semibold">密钥标识</th>
                  <th className="py-2.5 px-3 font-semibold">状态</th>
                  <th className="py-2.5 px-3 font-semibold">允许访问的模型</th>
                  <th className="py-2.5 px-3 font-semibold">有效期</th>
                  <th className="py-2.5 px-3 font-semibold">最后使用时间</th>
                  <th className="py-2.5 px-3 font-semibold">创建时间</th>
                  <th className="py-2.5 px-3 font-semibold text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {accessKeys.map((item) => {
                  const status = getAccessKeyStatus(item);
                  return (
                    <tr key={item.id} className="hover:bg-slate-50/80 transition-colors">
                      <td className="py-3 px-3">
                        <div className="font-semibold text-slate-900">{item.name}</div>
                      </td>
                      <td className="py-3 px-3 font-mono text-[11px] text-slate-600">
                        {formatMaskedKey(item.prefix, item.suffix)}
                      </td>
                      <td className="py-3 px-3">
                        {status === 'disabled' && <Badge tone="neutral">已停用</Badge>}
                        {status === 'expired' && <Badge tone="error">已过期</Badge>}
                        {status === 'active' && <Badge tone="success">使用中</Badge>}
                      </td>
                      <td className="py-3 px-3">
                        <AllowedModelsSummary
                          allowedModels={item.allowedModels}
                          publishedModels={publishedModels}
                        />
                      </td>
                      <td className="py-3 px-3 text-slate-600">
                        {item.expiresAt ? formatDateTime(item.expiresAt) : '永不过期'}
                      </td>
                      <td className="py-3 px-3 text-slate-500 text-[11px]">
                        {item.lastUsedAt ? formatDateTime(item.lastUsedAt) : '从未使用'}
                      </td>
                      <td className="py-3 px-3 text-slate-400 text-[11px]">
                        {formatDateTime(item.createdAt)}
                      </td>
                      <td className="py-3 px-3 text-right space-x-1" onClick={(e) => e.stopPropagation()}>
                        <button
                          type="button"
                          onClick={() => handleOpenEdit(item)}
                          className="p-1.5 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded cursor-pointer"
                          title="编辑"
                          aria-label="编辑"
                        >
                          <Edit3 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => setToggleCandidate(item)}
                          className={`p-1.5 rounded cursor-pointer ${
                            item.enabled
                              ? 'text-slate-400 hover:text-amber-600 hover:bg-amber-50'
                              : 'text-slate-400 hover:text-emerald-600 hover:bg-emerald-50'
                          }`}
                          title={item.enabled ? '停用' : '启用'}
                          aria-label={item.enabled ? '停用' : '启用'}
                        >
                          <Power className="w-3.5 h-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => setRotateCandidate(item)}
                          className="p-1.5 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 rounded cursor-pointer"
                          title="轮换密钥"
                          aria-label="轮换密钥"
                        >
                          <RefreshCw className="w-3.5 h-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => setDeleteCandidate(item)}
                          className="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded cursor-pointer"
                          title="删除"
                          aria-label="删除"
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
        </Panel>
      )}

      {/* 创建/编辑 Drawer */}
      <Drawer
        title={editingKey ? `编辑访问密钥` : '创建访问密钥'}
        subtitle={
          editingKey
            ? `修改密钥名称、权限范围与失效时间`
            : '创建密钥后，可用于调用 Ingate 已发布的 AI 大模型服务'
        }
        isOpen={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        <form onSubmit={handleSubmitForm} className="space-y-5 text-xs">
          {formError && (
            <div className="p-3 bg-rose-50 border border-rose-200 text-rose-800 rounded-lg">
              {formError}
            </div>
          )}

          <div>
            <label className="block font-medium text-slate-700 mb-1">
              密钥名称 <span className="text-rose-500">*</span>
            </label>
            <input
              type="text"
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-xs focus:border-indigo-500 focus:outline-none"
              placeholder="例如：生产环境 iOS 应用"
              value={formName}
              onChange={(e) => setFormName(e.target.value)}
            />
          </div>

          <div>
            <label className="block font-medium text-slate-700 mb-1">允许访问的模型</label>
            <p className="text-[11px] text-slate-500 mb-2">
              未勾选任何模型时，默认允许访问全部已发布大模型
            </p>
            {selectableModels.length === 0 ? (
              <div className="p-3 bg-slate-50 border border-slate-200 rounded text-slate-500">
                暂无已发布的大模型路由。可在创建密钥后随时补充模型绑定。
              </div>
            ) : (
              <div className="space-y-1.5 border border-slate-200 rounded-md p-3 max-h-44 overflow-y-auto bg-slate-50/50">
                {selectableModels.map((model) => {
                  const isChecked = formModels.includes(model);
                  const isPublished = publishedModels.includes(model);
                  return (
                    <label
                      key={model}
                      className="flex items-center justify-between gap-2 cursor-pointer text-slate-800 hover:text-indigo-600"
                    >
                      <div className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                          checked={isChecked}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setFormModels((prev) => [...prev, model]);
                            } else {
                              setFormModels((prev) => prev.filter((m) => m !== model));
                            }
                          }}
                        />
                        <span className="font-mono text-xs">{model}</span>
                      </div>
                      {!isPublished && <Badge tone="warning">当前未发布</Badge>}
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          <div>
            <label className="block font-medium text-slate-700 mb-2">有效期设置</label>
            <div className="space-y-2">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="expiration"
                  value="never"
                  checked={formExpOption === 'never'}
                  onChange={() => setFormExpOption('never')}
                  className="text-indigo-600 focus:ring-indigo-500"
                />
                <span>永不过期</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="expiration"
                  value="7d"
                  checked={formExpOption === '7d'}
                  onChange={() => setFormExpOption('7d')}
                  className="text-indigo-600 focus:ring-indigo-500"
                />
                <span>7 天</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="expiration"
                  value="30d"
                  checked={formExpOption === '30d'}
                  onChange={() => setFormExpOption('30d')}
                  className="text-indigo-600 focus:ring-indigo-500"
                />
                <span>30 天</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="expiration"
                  value="90d"
                  checked={formExpOption === '90d'}
                  onChange={() => setFormExpOption('90d')}
                  className="text-indigo-600 focus:ring-indigo-500"
                />
                <span>90 天</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="expiration"
                  value="custom"
                  checked={formExpOption === 'custom'}
                  onChange={() => setFormExpOption('custom')}
                  className="text-indigo-600 focus:ring-indigo-500"
                />
                <span>自定义截止时间</span>
              </label>
            </div>

            {formExpOption === 'custom' && (
              <div className="mt-2.5">
                <input
                  type="datetime-local"
                  className="w-full rounded-md border border-slate-300 px-3 py-1.5 text-xs focus:border-indigo-500"
                  value={formCustomExp}
                  onChange={(e) => setFormCustomExp(e.target.value)}
                />
              </div>
            )}
          </div>

          <div className="flex justify-end gap-2 pt-4 border-t border-slate-200">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setDrawerOpen(false)}
            >
              取消
            </Button>
            <Button type="submit" variant="primary" size="sm" disabled={submitting}>
              {submitting ? '提交中...' : editingKey ? '保存修改' : '确认创建'}
            </Button>
          </div>
        </form>
      </Drawer>

      {/* 展示一次性完整 Secret 的 Modal */}
      {secretModalState && (
        <Modal title={secretModalState.title} isOpen={true} onClose={handleCloseSecretModal}>
          <div className="space-y-4 text-xs">
            <div className="p-3 bg-amber-50 border border-amber-200 text-amber-900 rounded-lg font-medium">
              完整密钥仅在本次显示，关闭后无法再次查看。请妥善保存。
            </div>

            <div>
              <label className="block text-slate-700 font-medium mb-1">完整访问密钥 (Secret)</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  readOnly
                  value={secretModalState.secret}
                  className="flex-1 font-mono bg-slate-50 border border-slate-300 rounded-md px-3 py-2 text-xs select-all focus:outline-none"
                />
                <Button
                  type="button"
                  variant="secondary"
                  onClick={handleCopySecret}
                  className="flex items-center gap-1"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
                  {copied ? '已复制' : '复制'}
                </Button>
              </div>
            </div>

            <div className="flex justify-end pt-3">
              <Button type="button" variant="primary" onClick={handleCloseSecretModal}>
                我已保存，关闭
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* 启停确认框 */}
      {toggleCandidate && (
        <Modal
          title={toggleCandidate.enabled ? '停用访问密钥' : '启用访问密钥'}
          isOpen={true}
          onClose={() => setToggleCandidate(null)}
        >
          <div className="space-y-4 text-xs">
            <p className="text-slate-700">
              确定要{toggleCandidate.enabled ? '停用' : '启用'}访问密钥“{toggleCandidate.name}
              ”吗？
            </p>
            <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
              <Button
                variant="secondary"
                size="sm"
                disabled={actionBusy}
                onClick={() => setToggleCandidate(null)}
              >
                取消
              </Button>
              <Button
                variant={toggleCandidate.enabled ? 'danger' : 'primary'}
                size="sm"
                disabled={actionBusy}
                onClick={confirmToggleEnabled}
              >
                {actionBusy ? '处理中...' : toggleCandidate.enabled ? '确认停用' : '确认启用'}
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* 轮换确认框 */}
      {rotateCandidate && (
        <Modal title="轮换访问密钥" isOpen={true} onClose={() => setRotateCandidate(null)}>
          <div className="space-y-4 text-xs">
            <div className="p-3 bg-rose-50 border border-rose-200 text-rose-800 rounded-lg">
              轮换密钥后原密钥将立即失效，且不可恢复。使用原密钥的应用将无法进行认证。
            </div>
            <p className="text-slate-700">确认要轮换访问密钥“{rotateCandidate.name}”吗？</p>
            <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
              <Button
                variant="secondary"
                size="sm"
                disabled={actionBusy}
                onClick={() => setRotateCandidate(null)}
              >
                取消
              </Button>
              <Button variant="danger" size="sm" disabled={actionBusy} onClick={confirmRotate}>
                {actionBusy ? '轮换中...' : '确认轮换'}
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* 删除确认框 */}
      {deleteCandidate && (
        <Modal title="删除访问密钥" isOpen={true} onClose={() => setDeleteCandidate(null)}>
          <div className="space-y-4 text-xs">
            <p className="text-slate-700">
              确定要删除访问密钥“{deleteCandidate.name}”吗？删除后不可恢复。
            </p>
            <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
              <Button
                variant="secondary"
                size="sm"
                disabled={actionBusy}
                onClick={() => setDeleteCandidate(null)}
              >
                取消
              </Button>
              <Button variant="danger" size="sm" disabled={actionBusy} onClick={confirmDelete}>
                {actionBusy ? '删除中...' : '确认删除'}
              </Button>
            </div>
          </div>
        </Modal>
      )}

      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

// 汇总允许访问模型的组件
function AllowedModelsSummary({
  allowedModels,
  publishedModels,
}: {
  allowedModels: string[];
  publishedModels: string[];
}) {
  if (!allowedModels || allowedModels.length === 0) {
    return <span className="text-slate-600 font-medium">全部已发布模型</span>;
  }

  const firstFew = allowedModels.slice(0, 2);
  const remainingCount = allowedModels.length - firstFew.length;

  return (
    <div className="flex flex-wrap gap-1 items-center">
      {firstFew.map((model) => {
        const isPublished = publishedModels.includes(model);
        return (
          <span key={model} className="inline-flex items-center gap-1">
            <span className="font-mono text-[11px] bg-slate-100 text-slate-800 px-1.5 py-0.5 rounded">
              {model}
            </span>
            {!isPublished && <Badge tone="warning">当前未发布</Badge>}
          </span>
        );
      })}
      {remainingCount > 0 && (
        <span className="text-[11px] text-slate-500 font-medium">等 {remainingCount} 个</span>
      )}
    </div>
  );
}
