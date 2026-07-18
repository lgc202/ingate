import { useRef, useState } from 'react';
import { deleteCertificate, listCertificates, saveCertificate } from '@/api/certificates';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Certificate, CertificateMutationPayload } from '@/domain/certificate';

type CertificateMode = 'list' | 'create' | 'edit';
type CertificateInputMode = 'upload' | 'paste';

interface CertificateDraft extends CertificateMutationPayload {}

interface CertificateNotice {
  message: string;
  tone: 'success' | 'error';
}

const loadCertificates = () => listCertificates();
const maxPEMFileSize = 1024 * 1024;

export function CertificatePage() {
  const certificates = useResource(loadCertificates);
  const [mode, setMode] = useState<CertificateMode>('list');
  const [draft, setDraft] = useState<CertificateDraft>(emptyDraft());
  const [notice, setNotice] = useState<CertificateNotice | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Certificate | null>(null);
  const [inputMode, setInputMode] = useState<CertificateInputMode>('upload');
  const [certificateFileName, setCertificateFileName] = useState('');
  const [privateKeyFileName, setPrivateKeyFileName] = useState('');
  const certificateFileInput = useRef<HTMLInputElement>(null);
  const privateKeyFileInput = useRef<HTMLInputElement>(null);

  if (certificates.loading) {
    return (
      <PageFrame title="证书">
        <ResourceStatePanel title="加载证书数据" message="正在读取证书列表。" />
      </PageFrame>
    );
  }

  if (certificates.error || !certificates.data) {
    return (
      <PageFrame title="证书">
        <ResourceStatePanel title="证书数据加载失败" message={certificates.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const openCreate = () => {
    setDraft(emptyDraft());
    setMode('create');
    setInputMode('upload');
    setSubmitError(null);
    setCertificateFileName('');
    setPrivateKeyFileName('');
  };
  const openEdit = (certificate: Certificate) => {
    setDraft(emptyDraft(certificate));
    setMode('edit');
    setInputMode('upload');
    setSubmitError(null);
    setCertificateFileName('');
    setPrivateKeyFileName('');
  };
  const clearPEMFile = (field: 'certificatePEM' | 'privateKeyPEM') => {
    setDraft((current) => ({ ...current, [field]: '' }));
    if (field === 'certificatePEM') {
      setCertificateFileName('');
      return;
    }
    setPrivateKeyFileName('');
  };
  const changeInputMode = (nextMode: CertificateInputMode) => {
    if (nextMode === inputMode) {
      return;
    }
    setInputMode(nextMode);
    setDraft((current) => ({ ...current, certificatePEM: '', privateKeyPEM: '' }));
    setCertificateFileName('');
    setPrivateKeyFileName('');
    setSubmitError(null);
    if (certificateFileInput.current) {
      certificateFileInput.current.value = '';
    }
    if (privateKeyFileInput.current) {
      privateKeyFileInput.current.value = '';
    }
  };
  const readPEMFile = async (field: 'certificatePEM' | 'privateKeyPEM', input: HTMLInputElement) => {
    const file = input.files?.[0];
    if (!file) {
      return;
    }
    if (file.size > maxPEMFileSize) {
      clearPEMFile(field);
      input.value = '';
      setSubmitError('PEM 文件不能超过 1 MB');
      return;
    }
    try {
      const content = await file.text();
      setDraft((current) => ({ ...current, [field]: content }));
      setSubmitError(null);
      if (field === 'certificatePEM') {
        setCertificateFileName(file.name);
        return;
      }
      setPrivateKeyFileName(file.name);
    } catch {
      clearPEMFile(field);
      input.value = '';
      setSubmitError('读取 PEM 文件失败');
    }
  };
  const updatePEM = (field: 'certificatePEM' | 'privateKeyPEM', value: string) => {
    setDraft((current) => ({ ...current, [field]: value }));
    if (field === 'certificatePEM') {
      setCertificateFileName('');
    } else {
      setPrivateKeyFileName('');
    }
    const input = field === 'certificatePEM' ? certificateFileInput.current : privateKeyFileInput.current;
    if (input) {
      input.value = '';
    }
  };
  const save = async () => {
    setSubmitError(null);
    const validationError = validateDraft(draft, mode);
    if (validationError) {
      setSubmitError(validationError);
      return;
    }
    try {
      await saveCertificate({
        ...draft,
        name: draft.name.trim(),
        description: draft.description.trim(),
        certificatePEM: draft.certificatePEM.trim(),
        privateKeyPEM: draft.privateKeyPEM.trim(),
      });
      await certificates.reload();
      setNotice({ message: `证书已保存：${draft.name.trim()}`, tone: 'success' });
      setMode('list');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '保存证书失败');
    }
  };
  const confirmDelete = async () => {
    if (!deleteCandidate) {
      return;
    }
    try {
      await deleteCertificate(deleteCandidate.id);
      await certificates.reload();
      setNotice({ message: `已删除证书：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除证书失败', tone: 'error' });
      setDeleteCandidate(null);
    }
  };

  if (mode !== 'list') {
    return (
      <PageFrame
        title={mode === 'create' ? '新建证书' : '编辑证书'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <section className="editor-layout">
          <Panel>
            <div className="editor-grid form-only">
              <div className="editor-main-stack">
                <section className="form-section">
                  <div className="form-section-title"><h3>基础信息</h3></div>
                  <div className="field-grid">
                    <label className="field">
                      <span>证书名称</span>
                      <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
                    </label>
                    <label className="field">
                      <span>描述</span>
                      <input value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
                    </label>
                  </div>
                </section>

                <section className="form-section">
                  <div className="form-section-title"><h3>{mode === 'create' ? '证书材料' : '替换证书材料（可选）'}</h3></div>
                  <div className="certificate-input-mode" role="group" aria-label="证书材料录入方式">
                    <button type="button" className={inputMode === 'upload' ? 'active' : ''} aria-pressed={inputMode === 'upload'} onClick={() => changeInputMode('upload')}>上传文件</button>
                    <button type="button" className={inputMode === 'paste' ? 'active' : ''} aria-pressed={inputMode === 'paste'} onClick={() => changeInputMode('paste')}>手动粘贴</button>
                  </div>
                  <p className="form-section-help">
                    {inputMode === 'upload' ? '仅支持 PEM 编码，单个文件不超过 1 MB' : '分别粘贴 PEM 格式的证书链和私钥'}
                  </p>
                  {inputMode === 'upload' ? (
                    <div className="certificate-upload-grid">
                      <label className="certificate-upload-field">
                        <strong>证书链文件</strong>
                        <input
                          ref={certificateFileInput}
                          type="file"
                          accept=".pem,.crt,.cer,text/plain,application/x-pem-file"
                          onChange={(event) => readPEMFile('certificatePEM', event.currentTarget)}
                        />
                        <span>{certificateFileName || (mode === 'create' ? '未选择文件' : '不替换现有证书')}</span>
                      </label>
                      <label className="certificate-upload-field">
                        <strong>私钥文件</strong>
                        <input
                          ref={privateKeyFileInput}
                          type="file"
                          accept=".pem,.key,text/plain,application/x-pem-file"
                          onChange={(event) => readPEMFile('privateKeyPEM', event.currentTarget)}
                        />
                        <span>{privateKeyFileName || (mode === 'create' ? '未选择文件' : '不替换现有私钥')}</span>
                      </label>
                    </div>
                  ) : (
                    <div className="certificate-pem-grid">
                      <label className="field">
                        <span>证书链 PEM</span>
                        <textarea
                          className="certificate-pem-input"
                          value={draft.certificatePEM}
                          placeholder="-----BEGIN CERTIFICATE-----"
                          spellCheck={false}
                          onChange={(event) => updatePEM('certificatePEM', event.target.value)}
                        />
                      </label>
                      <label className="field">
                        <span>私钥 PEM</span>
                        <textarea
                          className="certificate-pem-input"
                          value={draft.privateKeyPEM}
                          placeholder="-----BEGIN PRIVATE KEY-----"
                          spellCheck={false}
                          onChange={(event) => updatePEM('privateKeyPEM', event.target.value)}
                        />
                      </label>
                    </div>
                  )}
                </section>
              </div>
              <div className="form-actions">
                {submitError ? <div className="form-error submit-error" role="alert">{submitError}</div> : null}
                <Button variant="primary" onClick={save}>保存证书</Button>
                <Button variant="ghost" onClick={() => setMode('list')}>取消</Button>
              </div>
            </div>
          </Panel>
        </section>
      </PageFrame>
    );
  }

  return (
    <PageFrame title="证书" actions={<Button variant="primary" onClick={openCreate}>新建证书</Button>}>
      <Panel title="证书列表">
        <div className="table-scroll">
          <table className="table certificate-table">
            <thead>
              <tr>
                <th>证书名称</th>
                <th>域名</th>
                <th>有效期</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {certificates.data.certificates.map((certificate) => {
                const state = certificateState(certificate);
                return (
                  <tr key={certificate.id}>
                    <td>
                      <div className="table-primary">{certificate.name}</div>
                      <div className="table-secondary">{certificate.description}</div>
                    </td>
                    <td>{certificate.dnsNames.length > 0 ? certificate.dnsNames.join('、') : '-'}</td>
                    <td>{certificate.notAfter ? formatDateTime(certificate.notAfter) : '-'}</td>
                    <td><Badge tone={state.tone}>{state.label}</Badge></td>
                    <td>{formatDateTime(certificate.createdAt)}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" type="button" onClick={() => openEdit(certificate)}>编辑</button>
                        <button className="link-button danger" type="button" onClick={() => setDeleteCandidate(certificate)}>删除</button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {certificates.data.certificates.length === 0 ? (
            <div className="table-empty"><EmptyState title="暂无证书" message="启用 HTTPS 网关前，需要先创建一张 TLS 证书。" /></div>
          ) : null}
        </div>
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      {deleteCandidate ? (
        <div className="confirm-overlay" role="presentation" onMouseDown={() => setDeleteCandidate(null)}>
          <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-certificate-title" onMouseDown={(event) => event.stopPropagation()}>
            <h3 id="delete-certificate-title">删除证书</h3>
            <p>确定删除 {deleteCandidate.name}？仍被 HTTPS 网关引用时，系统会拒绝删除。</p>
            <div className="confirm-actions">
              <Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button>
              <Button variant="primary" onClick={confirmDelete}>确认删除</Button>
            </div>
          </div>
        </div>
      ) : null}
    </PageFrame>
  );
}

function emptyDraft(certificate?: Certificate): CertificateDraft {
  return {
    id: certificate?.id,
    version: certificate?.version,
    name: certificate?.name ?? '',
    description: certificate?.description ?? '',
    certificatePEM: '',
    privateKeyPEM: '',
  };
}

function validateDraft(draft: CertificateDraft, mode: CertificateMode) {
  if (!draft.name.trim()) {
    return '证书名称不能为空';
  }
  const hasCertificate = Boolean(draft.certificatePEM.trim());
  const hasPrivateKey = Boolean(draft.privateKeyPEM.trim());
  if (mode === 'create' && (!hasCertificate || !hasPrivateKey)) {
    return '请同时填写证书链和私钥';
  }
  if (hasCertificate !== hasPrivateKey) {
    return '替换证书时必须同时填写证书链和私钥';
  }
  return null;
}

function certificateState(certificate: Certificate): { label: string; tone: 'success' | 'warning' | 'danger' } {
  if (!certificate.notAfter) {
    return { label: '未知', tone: 'warning' };
  }
  const remaining = new Date(certificate.notAfter).getTime() - Date.now();
  if (remaining <= 0) {
    return { label: '已过期', tone: 'danger' };
  }
  if (remaining <= 30 * 24 * 60 * 60 * 1000) {
    return { label: '即将过期', tone: 'warning' };
  }
  return { label: '有效', tone: 'success' };
}
