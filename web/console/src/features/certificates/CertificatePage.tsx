import { useRef, useState } from 'react';
import { deleteCertificate, listCertificates, saveCertificate } from '@/api/certificates';
import { useResource } from '@/api/useResource';
import {
  Badge,
  Button,
  Drawer,
  EmptyState,
  Modal,
  PageFrame,
  Panel,
  ResourceFilterField,
  ResourceListFilters,
  ResourceStatePanel,
  RowActions,
  SearchField,
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone, type ResourceState } from '@/domain/common';
import type { Certificate } from '@/domain/certificate';
import { FileText, KeyRound, Plus } from 'lucide-react';

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

type CertificateStateFilter = 'all' | Exclude<ResourceState, 'Disabled'>;

interface CertificateFilters {
  query: string;
  state: CertificateStateFilter;
}

const maxPEMFileSize = 1024 * 1024;
const emptyCertificateFilters = (): CertificateFilters => ({ query: '', state: 'all' });

export function CertificatePage() {
  const certificates = useResource(listCertificates);
  const [filterDraft, setFilterDraft] = useState<CertificateFilters>(emptyCertificateFilters);
  const [filters, setFilters] = useState<CertificateFilters>(emptyCertificateFilters);
  const [detail, setDetail] = useState<Certificate | null>(null);
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
  const normalizedQuery = filters.query.trim().toLowerCase();
  const visibleCertificates = certificateList.filter((certificate) => (
    `${certificate.name} ${certificate.dnsNames.join(' ')}`.toLowerCase().includes(normalizedQuery)
    && (filters.state === 'all' || certificate.state === filters.state)
  ));

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
      actions={<Button onClick={handleCreateNew}><Plus className="w-4 h-4" />录入证书</Button>}
    >
      <div className="space-y-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />

        <Panel>
          <ResourceListFilters
            summary={certificateFilterSummary(filters)}
            resultLabel={`${visibleCertificates.length} 张证书`}
            onSearch={() => setFilters({ ...filterDraft })}
            onReset={() => {
              const next = emptyCertificateFilters();
              setFilterDraft(next);
              setFilters(next);
            }}
          >
            <ResourceFilterField label="关键词">
              <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索证书名称或 DNS 域名" />
            </ResourceFilterField>
            <ResourceFilterField label="生效状态">
              <select className="select" value={filterDraft.state} onChange={(event) => setFilterDraft((current) => ({ ...current, state: event.target.value as CertificateStateFilter }))}>
                <option value="all">全部生效状态</option>
                <option value="Ready">已生效</option>
                <option value="Pending">待生效</option>
                <option value="Error">异常</option>
              </select>
            </ResourceFilterField>
          </ResourceListFilters>
          {visibleCertificates.length === 0 ? (
            <div className="p-5"><EmptyState title={certificateList.length === 0 ? '暂无 TLS 证书' : '没有匹配的证书'} message={certificateList.length === 0 ? '录入证书后即可配置 HTTPS 网关入口' : '请调整搜索条件'} /></div>
          ) : (
            <div className="table-scroll resource-table-scroll">
              <table className="table resource-table resource-certificate-table">
                <thead>
                  <tr>
                    <th>证书名称</th>
                    <th>DNS 域名</th>
                    <th>有效期截止</th>
                    <th>生效状态</th>
                    <th>更新时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleCertificates.map((item) => (
                    <tr key={item.id}>
                      <td>
                        <div className="resource-table-name">
                          <KeyRound className="text-blue-600" />
                          <strong>{item.name}</strong>
                        </div>
                      </td>

                      <td className="font-mono text-[11px]">
                        <div className="flex flex-wrap gap-1">
                          {(item.dnsNames ?? []).map((dns: string) => (
                            <span key={dns} className="px-1.5 py-0.5 bg-blue-50 text-blue-700 rounded border border-blue-200/60">
                              {dns}
                            </span>
                          ))}
                        </div>
                      </td>

                      <td>
                        <span className="font-mono text-[11px] text-slate-700">
                          {formatDateTime(item.notAfter)}
                        </span>
                      </td>

                      <td><Badge tone={resourceStateTone(item.state)}>{resourceStateLabel(item.state)}</Badge></td>
                      <td className="resource-table-time">{formatDateTime(item.updatedAt || item.createdAt)}</td>
                      <td><RowActions onDetail={() => setDetail(item)} onEdit={() => handleEdit(item)} onDelete={() => setDeleteCandidate(item)} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>

      <Drawer title="证书详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail ? <CertificateDetail certificate={detail} /> : null}
      </Drawer>

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

      <Modal
        title="确认删除证书"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-600">
            删除证书 <strong className="text-slate-900">{deleteCandidate?.name}</strong> 可能导致依赖此证书的 HTTPS 网关握手失败。确认操作？
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

function CertificateDetail({ certificate }: { certificate: Certificate }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{certificate.name}</h3></div>
        <Badge tone={resourceStateTone(certificate.state)}>{resourceStateLabel(certificate.state)}</Badge>
      </section>
      <section className="resource-detail-section">
        <h3>证书范围</h3>
        <div className="resource-detail-list">
          {certificate.dnsNames.map((dnsName) => <article key={dnsName}><div><strong>{dnsName}</strong><small>HTTPS DNS 域名</small></div><Badge tone="accent">TLS</Badge></article>)}
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>有效期</h3>
        <div className="resource-detail-grid">
          <div><span>开始时间</span><strong>{formatDateTime(certificate.notBefore)}</strong></div>
          <div><span>截止时间</span><strong>{formatDateTime(certificate.notAfter)}</strong></div>
          <div><span>录入时间</span><strong>{formatDateTime(certificate.createdAt)}</strong></div>
          <div><span>更新时间</span><strong>{formatDateTime(certificate.updatedAt || certificate.createdAt)}</strong></div>
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>资源信息</h3>
        <div className="resource-detail-grid">
          <div><span>配置状态</span><strong>{certificate.message || resourceStateLabel(certificate.state)}</strong></div>
          <div><span>配置版本</span><strong>{certificate.version}</strong></div>
        </div>
      </section>
    </div>
  );
}

function emptyDraft(): CertificateDraft {
  return {
    name: '',
    certificatePEM: '',
    privateKeyPEM: '',
  };
}

function certificateFilterSummary(filters: CertificateFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.state !== 'all') conditions.push(`生效状态：${resourceStateLabel(filters.state)}`);
  return conditions.join(' · ') || '全部证书';
}

function validateDraft(draft: CertificateDraft, mode: 'create' | 'edit'): string | null {
  if (!draft.name.trim()) return '请输入证书展示名称';
  if (mode === 'create') {
    if (!draft.certificatePEM.trim()) return '请提供证书公钥内容 (PEM)';
    if (!draft.privateKeyPEM.trim()) return '请提供证书私钥内容 (PEM)';
  }
  return null;
}
