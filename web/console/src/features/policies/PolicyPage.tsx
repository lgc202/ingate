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
import { useAuth } from '@/auth/AuthContext';
import { Drawer, Modal, PageFrame, ResourceStatePanel, Toast } from '@/components/ui';
import type { GovernancePolicy, GovernancePolicyKind, PolicyMutationResult } from '@/domain/policy';
import {
  AccessControlPolicyEditor,
  accessControlPolicyPayload,
  createAccessControlPolicyDraft,
  validateAccessControlPolicyDraft,
  type AccessControlPolicyDraft,
} from './AccessControlPolicyEditor';
import { PolicyLibraryTable } from './PolicyLibraryTable';
import {
  createRateLimitPolicyDraft,
  RateLimitPolicyEditor,
  rateLimitPolicyPayload,
  validateRateLimitPolicyDraft,
  type RateLimitPolicyDraft,
} from './RateLimitPolicyEditor';
import {
  createTokenQuotaPolicyDraft,
  TokenQuotaPolicyEditor,
  tokenQuotaPolicyPayload,
  validateTokenQuotaPolicyDraft,
  type TokenQuotaPolicyDraft,
} from './TokenQuotaPolicyEditor';

type PolicyEditorState =
  | { type: 'rateLimit'; draft: RateLimitPolicyDraft }
  | { type: 'accessControl'; draft: AccessControlPolicyDraft }
  | { type: 'tokenQuota'; draft: TokenQuotaPolicyDraft };

export function PolicyPage() {
  const { canWriteConfiguration } = useAuth();
  const workspace = useResource(getPolicyWorkspace);
  const [editor, setEditor] = useState<PolicyEditorState | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<GovernancePolicy | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  if (workspace.loading && !workspace.data) {
    return (
      <PageFrame title="治理策略">
        <ResourceStatePanel title="正在加载治理策略..." message="从管理 API 获取策略列表与关联目标" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="治理策略">
        <ResourceStatePanel title="策略加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const data = workspace.data;
  const allPolicies: GovernancePolicy[] = [
    ...data.rateLimitPolicies.map((p) => ({
      kind: 'RateLimitPolicy' as const,
      id: p.id,
      version: p.version,
      name: p.name,
      description: p.description,
      enabled: p.enabled,
      summary: `${p.rules?.length ?? 0} 条限流规则`,
      ruleCount: p.rules?.length ?? 0,
      targets: p.targets,
      status: p.status,
      createdAt: p.createdAt,
      raw: p,
    })),
    ...data.accessControlPolicies.map((p) => ({
      kind: 'AccessControlPolicy' as const,
      id: p.id,
      version: p.version,
      name: p.name,
      description: p.description,
      enabled: p.enabled,
      summary: `${p.rules?.length ?? 0} 条 ACL 规则`,
      ruleCount: p.rules?.length ?? 0,
      targets: p.targets,
      status: p.status,
      createdAt: p.createdAt,
      raw: p,
    })),
    ...data.tokenQuotaPolicies.map((p) => ({
      kind: 'TokenQuotaPolicy' as const,
      id: p.id,
      version: p.version,
      name: p.name,
      description: p.description,
      enabled: p.enabled,
      summary: `Token Quota Policy`,
      ruleCount: 1,
      targets: p.targets,
      status: p.status,
      createdAt: p.createdAt,
      raw: p,
    })),
  ];

  const reloadAfterMutation = async (resultMessage: string) => {
    await workspace.reload();
    setNotice({ message: resultMessage, tone: 'success' });
    setEditor(null);
  };

  const saveEditor = async () => {
    if (!editor || submitting) return;
    if (!editorIsValid(editor)) return;
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
    if (!deleteCandidate || deleting) return;
    setDeleting(true);
    try {
      const result = await deletePolicyByKind(deleteCandidate.kind, deleteCandidate.id, deleteCandidate.version);
      await reloadAfterMutation(result.message);
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除策略失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const togglePolicyStatus = async (policy: GovernancePolicy) => {
    try {
      const result = await setPolicyEnabledByKind(policy.kind, policy.id, policy.version, !policy.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略状态失败', tone: 'error' });
    }
  };

  return (
    <PageFrame
      title="治理策略"
      subtitle={`全量声明式治理策略（已挂载 ${allPolicies.length} 个配置规则）`}
      actions={canWriteConfiguration ? (
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft() })}
            className="px-3 py-1.5 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors cursor-pointer"
          >
            + 限流策略 (Rate Limit)
          </button>
          <button
            type="button"
            onClick={() => setEditor({ type: 'accessControl', draft: createAccessControlPolicyDraft() })}
            className="px-3 py-1.5 text-xs font-semibold text-white bg-slate-800 hover:bg-slate-900 rounded-lg shadow-xs transition-colors cursor-pointer"
          >
            + 访问控制 (IP / ACL)
          </button>
          <button
            type="button"
            onClick={() => setEditor({ type: 'tokenQuota', draft: createTokenQuotaPolicyDraft() })}
            className="px-3 py-1.5 text-xs font-semibold text-white bg-purple-600 hover:bg-purple-700 rounded-lg shadow-xs transition-colors cursor-pointer"
          >
            + AI Token 配额
          </button>
        </div>
      ) : undefined}
    >
      <div className="space-y-6 mt-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />

        <PolicyLibraryTable
          policies={allPolicies}
          targets={data.targets}
          readOnly={!canWriteConfiguration}
          onEdit={(policy) => {
            if (policy.kind === 'RateLimitPolicy') {
              setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft(policy.raw) });
            } else if (policy.kind === 'AccessControlPolicy') {
              setEditor({ type: 'accessControl', draft: createAccessControlPolicyDraft(policy.raw) });
            } else {
              setEditor({ type: 'tokenQuota', draft: createTokenQuotaPolicyDraft(policy.raw) });
            }
          }}
          onToggle={togglePolicyStatus}
          onDelete={(policy) => setDeleteCandidate(policy)}
        />
      </div>

      {/* Drawer Editor */}
      <Drawer
        title={editor ? `编辑${editorTypeTitle(editor.type)}` : ''}
        subtitle="策略更改后将自动同步至 Envoy 数据面"
        isOpen={Boolean(editor)}
        onClose={() => setEditor(null)}
      >
        {editor && (
          <div className="space-y-5">
            {editor.type === 'rateLimit' ? (
              <RateLimitPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateRateLimitPolicyDraft(editor.draft)}
                onChange={(draft) => setEditor({ type: 'rateLimit', draft })}
              />
            ) : editor.type === 'accessControl' ? (
              <AccessControlPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateAccessControlPolicyDraft(editor.draft)}
                onChange={(draft) => setEditor({ type: 'accessControl', draft })}
              />
            ) : (
              <TokenQuotaPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateTokenQuotaPolicyDraft(editor.draft)}
                onChange={(draft) => setEditor({ type: 'tokenQuota', draft })}
              />
            )}

            <div className="pt-4 border-t border-slate-200 flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => setEditor(null)}
                className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors cursor-pointer"
              >
                取消
              </button>
              <button
                type="button"
                disabled={submitting || !editorIsValid(editor)}
                onClick={saveEditor}
                className="px-4 py-2 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors disabled:opacity-50 cursor-pointer"
              >
                {submitting ? '提交中...' : '保存策略'}
              </button>
            </div>
          </div>
        )}
      </Drawer>

      {/* Delete Confirmation Modal */}
      <Modal
        title="确认删除治理策略"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-600">
            确定要删除策略 <strong className="text-slate-900 font-mono">{deleteCandidate?.name}</strong> ({deleteCandidate?.id}) 吗？
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
              onClick={confirmDeletePolicy}
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

function editorTypeTitle(type: PolicyEditorState['type']) {
  switch (type) {
    case 'rateLimit': return '限流策略';
    case 'accessControl': return '访问控制策略';
    case 'tokenQuota': return 'AI Token 配额策略';
  }
}

function editorIsValid(editor: PolicyEditorState): boolean {
  if (editor.type === 'rateLimit') return validateRateLimitPolicyDraft(editor.draft).valid;
  if (editor.type === 'accessControl') return validateAccessControlPolicyDraft(editor.draft).valid;
  return validateTokenQuotaPolicyDraft(editor.draft).valid;
}

function savePolicyEditor(editor: PolicyEditorState): Promise<PolicyMutationResult> {
  if (editor.type === 'rateLimit') return saveRateLimitPolicy(rateLimitPolicyPayload(editor.draft));
  if (editor.type === 'accessControl') return saveAccessControlPolicy(accessControlPolicyPayload(editor.draft));
  return saveTokenQuotaPolicy(tokenQuotaPolicyPayload(editor.draft));
}

function deletePolicyByKind(kind: GovernancePolicyKind, id: string, version?: string): Promise<PolicyMutationResult> {
  if (kind === 'RateLimitPolicy') return deleteRateLimitPolicy(id, version ?? '');
  if (kind === 'AccessControlPolicy') return deleteAccessControlPolicy(id, version ?? '');
  return deleteTokenQuotaPolicy(id, version ?? '');
}

function setPolicyEnabledByKind(kind: GovernancePolicyKind, id: string, version: string | undefined, enabled: boolean): Promise<PolicyMutationResult> {
  if (kind === 'RateLimitPolicy') return setRateLimitPolicyEnabled(id, version ?? '', enabled);
  if (kind === 'AccessControlPolicy') return setAccessControlPolicyEnabled(id, version ?? '', enabled);
  return setTokenQuotaPolicyEnabled(id, version ?? '', enabled);
}
