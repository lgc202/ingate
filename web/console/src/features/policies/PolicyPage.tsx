import { useState } from 'react';
import {
  deleteAccessControlPolicy,
  deleteRateLimitPolicy,
  deleteTokenQuotaPolicy,
  getPolicyWorkspace,
  saveAccessControlPolicy,
  saveRateLimitPolicy,
  saveTokenQuotaPolicy,
  setAccessControlPolicyEnabled,
  setRateLimitPolicyEnabled,
  setTokenQuotaPolicyEnabled,
} from '@/api/policies';
import { useResource } from '@/api/useResource';
import { Button, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import type { GovernancePolicy, GovernancePolicyKind, PolicyMutationResult } from '@/domain/policy';
import { policyKindLabel, policyTargetLabel } from '@/domain/policy';
import {
  AccessControlPolicyEditor,
  accessControlPolicyPayload,
  createAccessControlPolicyDraft,
  type AccessControlPolicyDraft,
  validateAccessControlPolicyDraft,
} from './AccessControlPolicyEditor';
import { CreatePolicyMenu, PolicyLibraryTable } from './PolicyLibraryTable';
import {
  createRateLimitPolicyDraft,
  RateLimitPolicyEditor,
  rateLimitPolicyPayload,
  type RateLimitPolicyDraft,
  validateRateLimitPolicyDraft,
} from './RateLimitPolicyEditor';
import {
  createTokenQuotaPolicyDraft,
  TokenQuotaPolicyEditor,
  tokenQuotaPolicyPayload,
  type TokenQuotaPolicyDraft,
  validateTokenQuotaPolicyDraft,
} from './TokenQuotaPolicyEditor';

const loadPolicyWorkspace = () => getPolicyWorkspace();

type EditorState =
  | { type: 'rateLimit'; draft: RateLimitPolicyDraft }
  | { type: 'accessControl'; draft: AccessControlPolicyDraft }
  | { type: 'tokenQuota'; draft: TokenQuotaPolicyDraft }
  | null;

interface Notice {
  message: string;
  tone: 'success' | 'error';
}

const policyTypeOptions = [
  { value: 'all', label: '全部类型' },
  { value: 'RateLimitPolicy', label: '限流' },
  { value: 'AccessControlPolicy', label: '访问控制' },
  { value: 'TokenQuotaPolicy', label: 'Token 配额' },
];

export function PolicyPage() {
  const workspace = useResource(loadPolicyWorkspace);
  const [query, setQuery] = useState('');
  const [policyKindFilter, setPolicyKindFilter] = useState<GovernancePolicyKind | 'all'>('all');
  const [editor, setEditor] = useState<EditorState>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<GovernancePolicy | null>(null);
  const [deleting, setDeleting] = useState(false);

  if (workspace.loading) {
    return (
      <PageFrame title="策略" subtitle="定义请求治理规则及其应用范围">
        <ResourceStatePanel title="加载策略数据" message="正在读取限流、访问控制和 Token 配额策略。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="策略" subtitle="定义请求治理规则及其应用范围">
        <ResourceStatePanel title="策略数据加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const data = workspace.data;
  const keyword = query.trim().toLowerCase();
  const filteredPolicies = data.policies.filter((policy) => {
    const targetNames = policy.targets.map((target) => policyTargetLabel(target, data.targets));
    const matchedKeyword = !keyword || [
      policy.name,
      policy.description ?? '',
      policy.summary,
      ...targetNames,
    ].some((value) => value.toLowerCase().includes(keyword));
    return matchedKeyword && (policyKindFilter === 'all' || policy.kind === policyKindFilter);
  });

  const reloadAfterMutation = async (resultMessage: string) => {
    await workspace.reload();
    setNotice({ message: resultMessage, tone: 'success' });
    setEditor(null);
  };

  const saveEditor = async () => {
    if (!editor || submitting) {
      return;
    }
    if (!editorIsValid(editor)) {
      return;
    }
    setSubmitting(true);
    try {
      const result = await savePolicyEditor(editor);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存策略失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDeletePolicy = async () => {
    if (!deleteCandidate) {
      return;
    }

    setDeleting(true);
    try {
      let result: PolicyMutationResult;
      if (deleteCandidate.kind === 'RateLimitPolicy') {
        result = await deleteRateLimitPolicy(deleteCandidate.id, deleteCandidate.name);
      } else if (deleteCandidate.kind === 'AccessControlPolicy') {
        result = await deleteAccessControlPolicy(deleteCandidate.id, deleteCandidate.name);
      } else {
        result = await deleteTokenQuotaPolicy(deleteCandidate.id, deleteCandidate.name);
      }
      await reloadAfterMutation(result.message);
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除策略失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const togglePolicy = async (policy: GovernancePolicy) => {
    try {
      let result: PolicyMutationResult;
      if (policy.kind === 'RateLimitPolicy') {
        result = await setRateLimitPolicyEnabled(policy.id, policy.name, !policy.enabled);
      } else if (policy.kind === 'AccessControlPolicy') {
        result = await setAccessControlPolicyEnabled(policy.id, policy.name, !policy.enabled);
      } else {
        result = await setTokenQuotaPolicyEnabled(policy.id, policy.name, !policy.enabled);
      }
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略状态失败', tone: 'error' });
    }
  };

  if (editor) {
    const editorValid = editorIsValid(editor);
    return (
      <PageFrame
        title="策略"
        subtitle={editorTitle(editor)}
        actions={<Button variant="soft" onClick={() => setEditor(null)}>返回列表</Button>}
      >
        <section className="editor-layout">
          <Panel title="配置详情">
            {editor.type === 'rateLimit' ? (
              <RateLimitPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateRateLimitPolicyDraft(editor.draft)}
                onChange={(draft) => {
                  setEditor({ type: 'rateLimit', draft });
                  setNotice(null);
                }}
              />
            ) : editor.type === 'accessControl' ? (
              <AccessControlPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateAccessControlPolicyDraft(editor.draft)}
                onChange={(draft) => {
                  setEditor({ type: 'accessControl', draft });
                  setNotice(null);
                }}
              />
            ) : (
              <TokenQuotaPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateTokenQuotaPolicyDraft(editor.draft)}
                onChange={(draft) => {
                  setEditor({ type: 'tokenQuota', draft });
                  setNotice(null);
                }}
              />
            )}
            <div className="form-actions">
              <Button variant="primary" disabled={submitting || !editorValid} onClick={saveEditor}>{submitting ? '保存中…' : '保存策略'}</Button>
              <Button variant="ghost" disabled={submitting} onClick={() => setEditor(null)}>取消</Button>
            </div>
          </Panel>
        </section>
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="策略"
      subtitle="定义请求治理规则及其应用范围"
      actions={
        <CreatePolicyMenu
          onCreateRateLimit={() => setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft() })}
          onCreateAccessControl={() => setEditor({ type: 'accessControl', draft: createAccessControlPolicyDraft() })}
          onCreateTokenQuota={() => setEditor({ type: 'tokenQuota', draft: createTokenQuotaPolicyDraft() })}
        />
      }
    >
      <Panel title="策略列表" subtitle={`${data.policies.length} 条策略 · 可直接应用到多个网关或路由`}>
        <div className="policy-toolbar">
          <input aria-label="搜索策略" value={query} placeholder="搜索策略名称、内容或应用目标" onChange={(event) => setQuery(event.target.value)} />
          <select aria-label="策略类型" value={policyKindFilter} onChange={(event) => setPolicyKindFilter(event.target.value as GovernancePolicyKind | 'all')}>
            {policyTypeOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </div>
        <PolicyLibraryTable
          policies={filteredPolicies}
          targets={data.targets}
          onEdit={(policy) => {
            if (policy.kind === 'RateLimitPolicy') {
              setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft(policy.raw) });
            } else if (policy.kind === 'AccessControlPolicy') {
              setEditor({ type: 'accessControl', draft: createAccessControlPolicyDraft(policy.raw) });
            } else {
              setEditor({ type: 'tokenQuota', draft: createTokenQuotaPolicyDraft(policy.raw) });
            }
          }}
          onToggle={togglePolicy}
          onDelete={setDeleteCandidate}
        />
      </Panel>
      {deleteCandidate ? (
        <div className="confirm-overlay" role="presentation" onMouseDown={() => {
          if (!deleting) {
            setDeleteCandidate(null);
          }
        }}>
          <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-policy-title" onMouseDown={(event) => event.stopPropagation()}>
            <h3 id="delete-policy-title">删除策略</h3>
            <p>确定删除 {deleteCandidate.name}？策略会从所有应用目标移除，此操作无法撤销。</p>
            <div className="confirm-meta">
              <span>策略类型</span><strong>{policyKindLabel(deleteCandidate.kind)}</strong>
              <span>应用目标</span><strong>{deleteCandidate.targets.length > 0 ? deleteCandidate.targets.length + ' 个' : '未应用'}</strong>
            </div>
            <div className="confirm-actions">
              <Button variant="ghost" disabled={deleting} onClick={() => setDeleteCandidate(null)}>取消</Button>
              <Button variant="primary" disabled={deleting} onClick={confirmDeletePolicy}>{deleting ? '删除中…' : '确认删除'}</Button>
            </div>
          </div>
        </div>
      ) : null}
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function editorTitle(editor: Exclude<EditorState, null>) {
  if (editor.type === 'rateLimit') {
    return editor.draft.id ? '编辑限流策略' : '新建限流策略';
  }
  if (editor.type === 'accessControl') {
    return editor.draft.id ? '编辑访问控制策略' : '新建访问控制策略';
  }
  return editor.draft.id ? '编辑 Token 配额策略' : '新建 Token 配额策略';
}

function editorIsValid(editor: Exclude<EditorState, null>) {
  if (editor.type === 'rateLimit') {
    return validateRateLimitPolicyDraft(editor.draft).valid;
  }
  if (editor.type === 'accessControl') {
    return validateAccessControlPolicyDraft(editor.draft).valid;
  }
  return validateTokenQuotaPolicyDraft(editor.draft).valid;
}

function savePolicyEditor(editor: Exclude<EditorState, null>) {
  if (editor.type === 'rateLimit') {
    return saveRateLimitPolicy(rateLimitPolicyPayload(editor.draft));
  }
  if (editor.type === 'accessControl') {
    return saveAccessControlPolicy(accessControlPolicyPayload(editor.draft));
  }
  return saveTokenQuotaPolicy(tokenQuotaPolicyPayload(editor.draft));
}
