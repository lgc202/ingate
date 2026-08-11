import { useState } from 'react';
import {
  deleteIPRestrictionPolicy,
  deleteRateLimitPolicy,
  getPolicyWorkspace,
  saveIPRestrictionPolicy,
  saveRateLimitPolicy,
  setGovernancePolicyEnabled,
} from '@/api/policies';
import { useResource } from '@/api/useResource';
import { useAuth } from '@/auth/AuthContext';
import { Drawer, Modal, PageFrame, ResourceStatePanel, Toast } from '@/components/ui';
import type { GovernancePolicy, GovernancePolicyKind, PolicyMutationResult } from '@/domain/policy';
import {
  createIPRestrictionPolicyDraft,
  IPRestrictionPolicyEditor,
  ipRestrictionPolicyPayload,
  validateIPRestrictionPolicyDraft,
  type IPRestrictionPolicyDraft,
} from './IPRestrictionPolicyEditor';
import { PolicyLibraryTable } from './PolicyLibraryTable';
import {
  createRateLimitPolicyDraft,
  RateLimitPolicyEditor,
  rateLimitPolicyPayload,
  validateRateLimitPolicyDraft,
  type RateLimitPolicyDraft,
} from './RateLimitPolicyEditor';

type PolicyEditorState =
	| { type: 'rateLimit'; draft: RateLimitPolicyDraft }
	| { type: 'ipRestriction'; draft: IPRestrictionPolicyDraft };

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
      <PageFrame title="流量策略">
        <ResourceStatePanel title="正在加载流量策略..." message="从管理 API 获取策略列表与关联目标" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="流量策略">
        <ResourceStatePanel title="策略加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const data = workspace.data;
  const allPolicies = data.policies;

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
      const result = await setGovernancePolicyEnabled(policy, !policy.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略状态失败', tone: 'error' });
    }
  };

  return (
    <PageFrame
      title="流量策略"
		subtitle={`统一管理普通 API 和 AI 请求的限流与 IP 访问限制（共 ${allPolicies.length} 条策略）`}
      actions={canWriteConfiguration ? (
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft() })}
            className="px-3 py-1.5 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors cursor-pointer"
          >
            + 限流策略
          </button>
          <button
            type="button"
            onClick={() => setEditor({ type: 'ipRestriction', draft: createIPRestrictionPolicyDraft() })}
            className="px-3 py-1.5 text-xs font-semibold text-white bg-slate-800 hover:bg-slate-900 rounded-lg shadow-xs transition-colors cursor-pointer"
          >
            + IP 访问限制
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
            } else if (policy.kind === 'IPRestrictionPolicy') {
              setEditor({ type: 'ipRestriction', draft: createIPRestrictionPolicyDraft(policy.raw) });
			}
          }}
          onToggle={togglePolicyStatus}
          onDelete={(policy) => setDeleteCandidate(policy)}
        />
      </div>

      <Drawer
        title={editor ? `编辑${editorTypeTitle(editor.type)}` : ''}
        subtitle="保存后自动发布到网关实例"
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
			) : (
              <IPRestrictionPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateIPRestrictionPolicyDraft(editor.draft)}
                onChange={(draft) => setEditor({ type: 'ipRestriction', draft })}
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
    case 'ipRestriction': return 'IP 访问限制策略';
  }
}

function editorIsValid(editor: PolicyEditorState): boolean {
	if (editor.type === 'rateLimit') return validateRateLimitPolicyDraft(editor.draft).valid;
	return validateIPRestrictionPolicyDraft(editor.draft).valid;
}

function savePolicyEditor(editor: PolicyEditorState): Promise<PolicyMutationResult> {
	if (editor.type === 'rateLimit') return saveRateLimitPolicy(rateLimitPolicyPayload(editor.draft));
	return saveIPRestrictionPolicy(ipRestrictionPolicyPayload(editor.draft));
}

function deletePolicyByKind(kind: GovernancePolicyKind, id: string, version?: string | number): Promise<PolicyMutationResult> {
	if (kind === 'RateLimitPolicy') return deleteRateLimitPolicy(id, Number(version));
	return deleteIPRestrictionPolicy(id, Number(version));
}
