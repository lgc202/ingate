import { useCallback, useState } from 'react';
import { deleteCertificate, listCertificatePage, saveCertificate } from '@/api/certificates';
import { listGateways } from '@/api/gateways';
import { useCursorResource, useResource } from '@/api/useResource';
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
  ResourcePagination,
  ResourceStatePanel,
  RowActions,
  SearchField,
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone, type ResourceState } from '@/domain/common';
import type { Certificate } from '@/domain/certificate';
import { KeyRound, Plus } from 'lucide-react';
import { CertificateDetail } from './CertificateDetail';
import {
  CertificateEditor,
  emptyCertificateDraft,
  validateCertificateDraft,
  type CertificateDraft,
  type CertificateInputMode,
} from './CertificateEditor';

interface CertificateNotice {
  message: string;
  tone: 'success' | 'error';
}

type CertificateStateFilter = 'all' | Exclude<ResourceState, 'Disabled'>;

interface CertificateFilters {
  query: string;
  state: CertificateStateFilter;
}

const emptyCertificateFilters = (): CertificateFilters => ({ query: '', state: 'all' });

export function CertificatePage() {
  const [filterDraft, setFilterDraft] = useState<CertificateFilters>(emptyCertificateFilters);
  const [filters, setFilters] = useState<CertificateFilters>(emptyCertificateFilters);
  const [pageSize, setPageSize] = useState(10);
  const [detail, setDetail] = useState<Certificate | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Certificate | null>(null);

  const [inputMode, setInputMode] = useState<CertificateInputMode>('upload');
  const [draft, setDraft] = useState<CertificateDraft>(emptyCertificateDraft);
  const [notice, setNotice] = useState<CertificateNotice | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const loadPage = useCallback((cursor: string) => listCertificatePage({
    limit: pageSize,
    cursor,
    query: filters.query.trim() || undefined,
    state: filters.state === 'all' ? undefined : filters.state.toUpperCase(),
  }), [filters, pageSize]);
  const certificates = useCursorResource(loadPage, {
    autoRefreshWhen: (data) => data.items.some((certificate) => certificate.state === 'Pending'),
  });
  const currentCertificates = certificates.data?.items ?? [];
  const gateways = useResource(listGateways, { enabled: Boolean(detail) });

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

  const certificateReferences = (certificateID: string) => gateways.data?.gateways.flatMap((gateway) => (
    gateway.listeners
      .filter((listener) => listener.certificateID === certificateID)
      .map((listener) => ({ gatewayID: gateway.id, gatewayName: gateway.name, listenerName: listener.name }))
  )) ?? [];

  const handleCreateNew = () => {
    setIsEditing(false);
    setDraft(emptyCertificateDraft());
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

  const handleSave = async () => {
    setSubmitError(null);
    const validationError = validateCertificateDraft(draft, isEditing ? 'edit' : 'create');
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
            resultLabel={`本页 ${currentCertificates.length} 张证书`}
            onSearch={() => { certificates.reset(); setFilters({ ...filterDraft }); }}
            onReset={() => {
              const next = emptyCertificateFilters();
              setFilterDraft(next);
              setFilters(next);
              certificates.reset();
            }}
          >
            <ResourceFilterField label="关键词">
              <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索证书名称" />
            </ResourceFilterField>
            <ResourceFilterField label="生效状态">
              <select className="select" value={filterDraft.state} onChange={(event) => setFilterDraft((current) => ({ ...current, state: event.target.value as CertificateStateFilter }))}>
                <option value="all">全部生效状态</option>
                <option value="Ready">已生效</option>
                <option value="Pending">待生效</option>
                <option value="Error">生效失败</option>
              </select>
            </ResourceFilterField>
          </ResourceListFilters>
          {currentCertificates.length === 0 ? (
            <div className="p-5"><EmptyState title={filters.query || filters.state !== 'all' ? '没有匹配的证书' : '暂无 TLS 证书'} message={filters.query || filters.state !== 'all' ? '请调整搜索条件' : '录入证书后即可配置 HTTPS 网关入口'} /></div>
          ) : (
            <div className="table-scroll resource-table-scroll">
              <table className="table resource-table resource-certificate-table">
                <thead>
                  <tr>
                    <th>证书名称</th>
                    <th>DNS 域名</th>
                    <th>有效期截止</th>
                    <th>状态</th>
                    <th>更新时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {currentCertificates.map((item) => (
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
                        <Badge tone={certificateExpiryTone(item.notAfter)}>{certificateExpiryLabel(item.notAfter)}</Badge>
                        <div className="table-secondary mt-1">{formatDateTime(item.notAfter)}</div>
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
          {currentCertificates.length > 0 ? <ResourcePagination page={certificates.page} pageSize={pageSize} itemCount={currentCertificates.length} hasNext={certificates.hasNext} onPageChange={(nextPage) => nextPage > certificates.page ? certificates.next() : certificates.previous()} onPageSizeChange={(size) => { certificates.reset(); setPageSize(size); }} /> : null}
        </Panel>
      </div>

      <Drawer title="证书详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail ? <CertificateDetail certificate={detail} references={certificateReferences(detail.id)} /> : null}
      </Drawer>

      <Drawer
        title={isEditing ? `修改证书信息: ${draft.name}` : '录入新 TLS 证书'}
        subtitle="上传 PEM 格式的证书文件和私钥，用于 HTTPS 入口"
        isOpen={drawerOpen}
        onClose={() => { setSubmitError(null); setDrawerOpen(false); }}
      >
        <CertificateEditor
          draft={draft}
          inputMode={inputMode}
          isEditing={isEditing}
          submitting={submitting}
          submitError={submitError}
          onChange={setDraft}
          onInputModeChange={setInputMode}
          onError={setSubmitError}
          onCancel={() => setDrawerOpen(false)}
          onSave={handleSave}
        />
      </Drawer>

      <Modal
        title="确认删除证书"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-600">
            确定删除证书 <strong className="text-slate-900">{deleteCandidate?.name}</strong> 吗？
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

function certificateFilterSummary(filters: CertificateFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.state !== 'all') conditions.push(`生效状态：${resourceStateLabel(filters.state)}`);
  return conditions.join(' · ') || '全部证书';
}

function certificateExpiryLabel(notAfter: string): string {
  const remainingDays = Math.ceil((new Date(notAfter).getTime() - Date.now()) / 86_400_000);
  if (remainingDays < 0) return '已过期';
  if (remainingDays === 0) return '今天到期';
  if (remainingDays <= 30) return `${remainingDays} 天后到期`;
  return '有效';
}

function certificateExpiryTone(notAfter: string): 'success' | 'warning' | 'error' {
  const remainingDays = Math.ceil((new Date(notAfter).getTime() - Date.now()) / 86_400_000);
  if (remainingDays < 0) return 'error';
  if (remainingDays <= 30) return 'warning';
  return 'success';
}
