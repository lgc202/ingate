import { useRef, useState } from 'react';
import { deleteCertificate, listCertificates, saveCertificate } from '@/api/certificates';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Drawer, EmptyState, Modal, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Certificate } from '@/domain/certificate';
import { KeyRound, Plus, Trash2, Edit3, FileText } from 'lucide-react';

type CertificateInputMode = 'upload' | 'paste';

interface CertificateDraft {
  id?: string;
  version?: number;
  name: string;
  certificatePEM: string;
  privateKeyPEM: string;
}

interface CertificateNotice {
  message: string;
  tone: 'success' | 'error';
}

const maxPEMFileSize = 1024 * 1024;

export function CertificatePage() {
  const { canWriteConfiguration } = useAuth();
  const certificates = useResource(listCertificates);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Certificate | null>(null);

  const [inputMode, setInputMode] = useState<CertificateInputMode>('upload');
  const [draft, setDraft] = useState<CertificateDraft>(emptyDraft);
  const [notice, setNotice] = useState<CertificateNotice | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const certFileInputRef = useRef<HTMLInputElement>(null);
  const keyFileInputRef = useRef<HTMLInputElement>(null);

  if (certificates.loading && !certificates.data) {
    return (
      <PageFrame title="TLS 证书">
        <ResourceStatePanel title="正在加载证书数据..." message="从管理 API 获取数据中" />
      </PageFrame>
    );
  }

  if (certificates.error || !certificates.data) {
    return (
      <PageFrame title="TLS 证书">
        <ResourceStatePanel title="证书数据加载失败" message={certificates.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const certificateList = certificates.data.certificates;

  const handleCreateNew = () => {
    setIsEditing(false);
    setDraft(emptyDraft());
    setInputMode('upload');
    setSubmitError(null);
    setDrawerOpen(true);
  };

  const handleEdit = (cert: Certificate) => {
    setIsEditing(true);
    setDraft({
      id: cert.id,
      version: cert.version,
      name: cert.name,
      certificatePEM: '',
      privateKeyPEM: '',
    });
    setInputMode('paste');
    setSubmitError(null);
    setDrawerOpen(true);
  };

  const handleFileUpload = (type: 'cert' | 'key', file?: File) => {
    if (!file) return;
    if (file.size > maxPEMFileSize) {
      setSubmitError('PEM 文件大小不能超过 1 MB');
      return;
    }
    setSubmitError(null);
    const reader = new FileReader();
    reader.onload = () => {
      const text = String(reader.result ?? '');
      if (type === 'cert') {
        setDraft((prev) => ({ ...prev, certificatePEM: text }));
      } else {
        setDraft((prev) => ({ ...prev, privateKeyPEM: text }));
      }
    };
    reader.onerror = () => setSubmitError('读取 PEM 文件失败');
    reader.readAsText(file);
  };

  const handleSave = async () => {
    setSubmitError(null);
    const validationError = validateDraft(draft, isEditing ? 'edit' : 'create');
    if (validationError) {
      setSubmitError(validationError);
      return;
    }

    setSubmitting(true);
    try {
      await saveCertificate(isEditing ? {
        id: draft.id,
        version: draft.version,
        name: draft.name.trim(),
      } : {
        name: draft.name.trim(),
        certificatePEM: draft.certificatePEM.trim(),
        privateKeyPEM: draft.privateKeyPEM.trim(),
      });
      await certificates.reload();
      setNotice({ message: `证书已保存：${draft.name.trim()}`, tone: 'success' });
      setDrawerOpen(false);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '保存证书失败');
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteCandidate) return;
    setDeleting(true);
    try {
      await deleteCertificate(deleteCandidate.id, deleteCandidate.version);
      await certificates.reload();
      setNotice({ message: `证书已删除：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除证书失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  return (
    <PageFrame
      title="TLS 证书"
      subtitle={`已录入 ${certificateList.length} 张 HTTPS 域名与 wildcard TLS 证书`}
      actions={canWriteConfiguration ? (
        <button
          type="button"
          onClick={handleCreateNew}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          录入证书
        </button>
      ) : undefined}
    >
      <div className="space-y-6 mt-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />

        {/* Certificate List High-Density Table */}
        <Panel>
          {certificateList.length === 0 ? (
            <EmptyState
              title="暂无 TLS 证书"
              message={canWriteConfiguration ? '点击右上角按钮录入 HTTPS 证书' : '当前环境还没有证书'}
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50 font-medium">
                    <th className="py-2.5 px-3">证书名称 / ID</th>
                    <th className="py-2.5 px-3">DNS 域名列表</th>
                    <th className="py-2.5 px-3">有效期截止</th>
                    <th className="py-2.5 px-3">录入时间</th>
                    <th className="py-2.5 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 font-normal">
                  {certificateList.map((item) => (
                    <tr key={item.id} className="hover:bg-slate-50/80 transition-colors">
                      <td className="py-3 px-3">
                        <div className="flex items-center gap-2">
                          <KeyRound className="w-4 h-4 text-blue-600 shrink-0" />
                          <div>
                            <div className="font-semibold text-slate-900">{item.name}</div>
                            <div className="text-[11px] font-mono text-slate-400">{item.id}</div>
                          </div>
                        </div>
                      </td>

                      <td className="py-3 px-3 font-mono text-[11px]">
                        <div className="flex flex-wrap gap-1">
                          {(item.dnsNames ?? []).map((dns: string) => (
                            <span key={dns} className="px-1.5 py-0.5 bg-blue-50 text-blue-700 rounded border border-blue-200/60">
                              {dns}
                            </span>
                          ))}
                        </div>
                      </td>

                      <td className="py-3 px-3">
                        <span className="font-mono text-[11px] text-slate-700">
                          {formatDateTime(item.notAfter)}
                        </span>
                      </td>

                      <td className="py-3 px-3 text-slate-400 text-[11px]">
                        {formatDateTime(item.createdAt)}
                      </td>

                      <td className="py-3 px-3 text-right space-x-1">
                        {canWriteConfiguration ? (
                          <>
                        <button
                          type="button"
                          onClick={() => handleEdit(item)}
                          className="p-1.5 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded transition-colors cursor-pointer"
                          title="修改信息"
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
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>

      {/* Slide-over Drawer Form */}
      <Drawer
        title={isEditing ? `修改证书信息: ${draft.name}` : '录入新 TLS 证书'}
        subtitle="上传 PEM 格式公私钥文本，用于 HTTPS Listener SNI 握手"
        isOpen={drawerOpen}
        onClose={() => { setSubmitError(null); setDrawerOpen(false); }}
      >
        <div className="space-y-5">
          {submitError && (
            <div className="p-3 bg-rose-50 border border-rose-200 text-rose-700 text-xs rounded-lg">
              {submitError}
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">
                证书展示名称 <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                placeholder="例如: *.example.com 通配符证书"
                className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-blue-500/20"
              />
            </div>

            {!isEditing && (
              <div className="space-y-4">
                <div className="flex items-center gap-4 text-xs">
                  <label className="flex items-center gap-2 cursor-pointer font-medium text-slate-700">
                    <input
                      type="radio"
                      name="inputMode"
                      checked={inputMode === 'upload'}
                      onChange={() => setInputMode('upload')}
                      className="text-blue-600"
                    />
                    上传 PEM 文件
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer font-medium text-slate-700">
                    <input
                      type="radio"
                      name="inputMode"
                      checked={inputMode === 'paste'}
                      onChange={() => setInputMode('paste')}
                      className="text-blue-600"
                    />
                    直接粘贴 PEM 文本
                  </label>
                </div>

                {inputMode === 'upload' ? (
                  <div className="grid grid-cols-2 gap-4">
                    <div className="p-4 bg-slate-50 border border-slate-200 rounded-xl space-y-2 text-center">
                      <FileText className="w-5 h-5 text-slate-400 mx-auto" />
                      <div className="text-xs font-medium text-slate-700">证书公钥 (.crt / .pem)</div>
                      <input
                        ref={certFileInputRef}
                        type="file"
                        accept=".pem,.crt,.cer"
                        className="hidden"
                        onChange={(e) => handleFileUpload('cert', e.target.files?.[0])}
                      />
                      <button
                        type="button"
                        onClick={() => certFileInputRef.current?.click()}
                        className="px-3 py-1.5 bg-white border border-slate-300 text-xs font-medium rounded-lg hover:bg-slate-50 cursor-pointer"
                      >
                        选择公钥文件
                      </button>
                      {draft.certificatePEM && (
                        <p className="text-[10px] text-emerald-600 font-mono">已加载公钥文件</p>
                      )}
                    </div>

                    <div className="p-4 bg-slate-50 border border-slate-200 rounded-xl space-y-2 text-center">
                      <FileText className="w-5 h-5 text-slate-400 mx-auto" />
                      <div className="text-xs font-medium text-slate-700">私钥 Key (.key / .pem)</div>
                      <input
                        ref={keyFileInputRef}
                        type="file"
                        accept=".pem,.key"
                        className="hidden"
                        onChange={(e) => handleFileUpload('key', e.target.files?.[0])}
                      />
                      <button
                        type="button"
                        onClick={() => keyFileInputRef.current?.click()}
                        className="px-3 py-1.5 bg-white border border-slate-300 text-xs font-medium rounded-lg hover:bg-slate-50 cursor-pointer"
                      >
                        选择私钥文件
                      </button>
                      {draft.privateKeyPEM && (
                        <p className="text-[10px] text-emerald-600 font-mono">已加载私钥文件</p>
                      )}
                    </div>
                  </div>
                ) : (
                  <div className="space-y-3">
                    <div>
                      <label className="block text-xs font-semibold text-slate-700 mb-1">
                        证书公钥内容 (BEGIN CERTIFICATE)
                      </label>
                      <textarea
                        rows={5}
                        value={draft.certificatePEM}
                        onChange={(e) => setDraft({ ...draft, certificatePEM: e.target.value })}
                        placeholder="-----BEGIN CERTIFICATE-----"
                        className="w-full p-2.5 font-mono text-[11px] border border-slate-300 rounded-lg focus:outline-hidden"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-slate-700 mb-1">
                        私钥内容 (BEGIN PRIVATE KEY)
                      </label>
                      <textarea
                        rows={5}
                        value={draft.privateKeyPEM}
                        onChange={(e) => setDraft({ ...draft, privateKeyPEM: e.target.value })}
                        placeholder="-----BEGIN PRIVATE KEY-----"
                        className="w-full p-2.5 font-mono text-[11px] border border-slate-300 rounded-lg focus:outline-hidden"
                      />
                    </div>
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
              {submitting ? '保存中...' : '保存证书'}
            </button>
          </div>
        </div>
      </Drawer>

      {/* Delete Confirmation Modal */}
      <Modal
        title="确认删除证书"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-600">
            删除证书 <strong className="text-slate-900 font-mono">{deleteCandidate?.name}</strong> ({deleteCandidate?.id}) 可能导致依赖此证书的 HTTPS 网关握手失败。确认操作？
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
              onClick={confirmDelete}
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

function emptyDraft(): CertificateDraft {
  return {
    name: '',
    certificatePEM: '',
    privateKeyPEM: '',
  };
}

function validateDraft(draft: CertificateDraft, mode: 'create' | 'edit'): string | null {
  if (!draft.name.trim()) return '请输入证书展示名称';
  if (mode === 'create') {
    if (!draft.certificatePEM.trim()) return '请提供证书公钥内容 (PEM)';
    if (!draft.privateKeyPEM.trim()) return '请提供证书私钥内容 (PEM)';
  }
  return null;
}
