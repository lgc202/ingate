import { useState } from 'react';
import {
  deleteUpstreamCredential,
  listUpstreamCredentials,
  saveUpstreamCredential,
} from '@/api/credentials';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { UpstreamCredential, UpstreamCredentialMutationPayload } from '@/domain/credential';
import { upstreamCredentialTypeLabel } from '@/domain/credential';

type CredentialMode = 'list' | 'create' | 'edit';

interface CredentialDraft {
  id?: string;
  version?: string;
  name: string;
  apiKey: string;
  configured: boolean;
}

interface CredentialNotice {
  message: string;
  tone: 'success' | 'error';
}

const loadCredentials = () => listUpstreamCredentials();

export function CredentialPage() {
  const credentials = useResource(loadCredentials);
  const [mode, setMode] = useState<CredentialMode>('list');
  const [draft, setDraft] = useState<CredentialDraft>(emptyDraft());
  const [query, setQuery] = useState('');
  const [notice, setNotice] = useState<CredentialNotice | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<UpstreamCredential | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (credentials.loading) {
    return (
      <PageFrame title="访问凭据" subtitle="集中保存调用外部服务所需的访问密钥">
        <ResourceStatePanel title="加载访问凭据" message="正在读取访问凭据列表。" />
      </PageFrame>
    );
  }

  if (credentials.error || !credentials.data) {
    return (
      <PageFrame title="访问凭据" subtitle="集中保存调用外部服务所需的访问密钥">
        <ResourceStatePanel title="访问凭据加载失败" message={credentials.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const normalizedQuery = query.trim().toLowerCase();
  const visibleCredentials = credentials.data.credentials.filter((credential) => (
    !normalizedQuery
    || credential.name.toLowerCase().includes(normalizedQuery)
    || upstreamCredentialTypeLabel(credential.type).toLowerCase().includes(normalizedQuery)
  ));

  const openCreate = () => {
    setDraft(emptyDraft());
    setSubmitError(null);
    setMode('create');
  };

  const openEdit = (credential: UpstreamCredential) => {
    setDraft(emptyDraft(credential));
    setSubmitError(null);
    setMode('edit');
  };

  const closeEditor = () => {
    setDraft(emptyDraft());
    setMode('list');
    setSubmitError(null);
    setSubmitting(false);
  };

  const save = async () => {
    const name = draft.name.trim();
    const apiKey = draft.apiKey.trim();
    if (!name) {
      setSubmitError('访问凭据名称不能为空');
      return;
    }
    if (mode === 'create' && !apiKey) {
      setSubmitError('访问密钥不能为空');
      return;
    }
    if (draft.id && !draft.version) {
      setSubmitError('访问凭据版本缺失，请刷新后重试');
      return;
    }

    const payload: UpstreamCredentialMutationPayload = {
      id: draft.id,
      version: draft.version,
      name,
      type: 'APIKey',
      ...(apiKey ? { apiKey: { value: apiKey } } : {}),
    };

    setSubmitting(true);
    setSubmitError(null);
    try {
      const result = await saveUpstreamCredential(payload);
      await credentials.reload();
      setNotice({ message: result.message, tone: 'success' });
      setDraft(emptyDraft());
      setMode('list');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '保存访问凭据失败');
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteCandidate) {
      return;
    }
    setDeleting(true);
    try {
      await deleteUpstreamCredential(deleteCandidate.id);
      await credentials.reload();
      setNotice({ message: `已删除访问凭据：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除访问凭据失败', tone: 'error' });
      setDeleteCandidate(null);
    } finally {
      setDeleting(false);
    }
  };

  if (mode !== 'list') {
    return (
      <PageFrame
        title={mode === 'create' ? '新建访问凭据' : '编辑访问凭据'}
        subtitle={mode === 'create' ? '保存服务调用所需的 API Key' : '可以修改名称或替换访问密钥'}
        actions={<Button variant="soft" disabled={submitting} onClick={closeEditor}>返回列表</Button>}
      >
        <section className="editor-layout">
          <Panel>
            <div className="editor-grid form-only">
              <div className="editor-main-stack">
                <section className="form-section">
                  <div className="form-section-title">
                    <h3>基础信息</h3>
                    <p>凭据名称用于服务配置中选择，不会发送给目标服务。</p>
                  </div>
                  <div className="field-grid">
                    <label className="field">
                      <span>凭据名称</span>
                      <input
                        value={draft.name}
                        maxLength={64}
                        placeholder="例如：OpenAI 生产密钥"
                        onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                      />
                    </label>
                    <label className="field">
                      <span>凭据类型</span>
                      <select value="APIKey" disabled>
                        <option value="APIKey">API Key</option>
                      </select>
                    </label>
                  </div>
                </section>

                <section className="form-section">
                  <div className="form-section-title">
                    <h3>{mode === 'create' ? '访问密钥' : '替换访问密钥（可选）'}</h3>
                    <p>{mode === 'create' ? '密钥保存后不再回显。' : '留空表示继续使用当前密钥；填写新值会替换当前密钥。'}</p>
                  </div>
                  <div className="credential-secret-card">
                    <label className="field field-wide">
                      <span>API Key</span>
                      <input
                        type="password"
                        value={draft.apiKey}
                        autoComplete="new-password"
                        placeholder={mode === 'create' ? '输入访问密钥' : '留空保留当前密钥'}
                        onChange={(event) => setDraft({ ...draft, apiKey: event.target.value })}
                      />
                    </label>
                    {mode === 'edit' ? (
                      <Badge tone={draft.configured ? 'success' : 'warning'}>
                        {draft.configured ? '当前密钥已配置' : '当前未配置密钥'}
                      </Badge>
                    ) : null}
                  </div>
                </section>
              </div>
              <div className="form-actions">
                {submitError ? <div className="form-error submit-error" role="alert">{submitError}</div> : null}
                <Button variant="primary" disabled={submitting} onClick={save}>{submitting ? '保存中...' : '保存访问凭据'}</Button>
                <Button variant="ghost" disabled={submitting} onClick={closeEditor}>取消</Button>
              </div>
            </div>
          </Panel>
        </section>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="访问凭据"
      subtitle="集中保存调用外部服务所需的访问密钥"
      actions={<Button variant="primary" onClick={openCreate}>新建访问凭据</Button>}
    >
      <Panel
        title="访问凭据列表"
        subtitle="密钥只可写入，控制台不会读取或展示原始内容"
        actions={(
          <div className="table-toolbar">
            <input
              className="toolbar-input"
              value={query}
              placeholder="搜索凭据名称"
              onChange={(event) => setQuery(event.target.value)}
            />
            {query ? <Button variant="soft" onClick={() => setQuery('')}>重置</Button> : null}
          </div>
        )}
      >
        <div className="table-scroll">
          <table className="table credential-table">
            <thead>
              <tr>
                <th>凭据名称</th>
                <th>类型</th>
                <th>密钥状态</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {visibleCredentials.map((credential) => (
                <tr key={credential.id}>
                  <td>
                    <div className="table-primary">{credential.name}</div>
                    <div className="table-secondary">{credential.status.message}</div>
                  </td>
                  <td>{upstreamCredentialTypeLabel(credential.type)}</td>
                  <td>
                    <Badge tone={credential.configured ? 'success' : 'warning'}>
                      {credential.configured ? '已配置' : '未配置'}
                    </Badge>
                  </td>
                  <td>{formatDateTime(credential.createdAt)}</td>
                  <td>
                    <div className="row-actions">
                      <button className="link-button" type="button" onClick={() => openEdit(credential)}>编辑</button>
                      <button className="link-button danger" type="button" onClick={() => setDeleteCandidate(credential)}>删除</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {visibleCredentials.length === 0 ? (
            <div className="table-empty">
              <EmptyState
                title={query ? '没有匹配的访问凭据' : '暂无访问凭据'}
                message={query ? '调整搜索条件后再试。' : '创建访问凭据后，可以在大模型服务中直接选择使用。'}
              />
            </div>
          ) : null}
        </div>
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      {deleteCandidate ? (
        <div className="confirm-overlay" role="presentation" onMouseDown={() => {
          if (!deleting) {
            setDeleteCandidate(null);
          }
        }}>
          <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-credential-title" onMouseDown={(event) => event.stopPropagation()}>
            <h3 id="delete-credential-title">删除访问凭据</h3>
            <p>确定删除 {deleteCandidate.name}？仍被服务使用时，系统会拒绝删除。</p>
            <div className="confirm-actions">
              <Button variant="ghost" disabled={deleting} onClick={() => setDeleteCandidate(null)}>取消</Button>
              <Button variant="primary" disabled={deleting} onClick={confirmDelete}>{deleting ? '删除中...' : '确认删除'}</Button>
            </div>
          </div>
        </div>
      ) : null}
    </PageFrame>
  );
}

function emptyDraft(credential?: UpstreamCredential): CredentialDraft {
  return {
    id: credential?.id,
    version: credential?.version,
    name: credential?.name ?? '',
    apiKey: '',
    configured: credential?.configured ?? false,
  };
}
