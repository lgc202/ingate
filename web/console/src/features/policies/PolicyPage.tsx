import { useState } from 'react';
import {
  deleteAccessControlPolicy,
  deleteRateLimitPolicy,
  getPolicyWorkspace,
  saveAccessControlPolicy,
  saveRateLimitPolicy,
  setAccessControlPolicyEnabled,
  setRateLimitPolicyEnabled,
} from '@/api/policies';
import { useResource } from '@/api/useResource';
import { Button, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import type { GovernancePolicy, GovernancePolicyKind } from '@/domain/policy';
import { policyTargetLabel } from '@/domain/policy';
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

const loadPolicyWorkspace = () => getPolicyWorkspace();

type EditorState =
  | { type: 'rateLimit'; draft: RateLimitPolicyDraft }
  | { type: 'accessControl'; draft: AccessControlPolicyDraft }
  | null;

interface Notice {
  message: string;
  tone: 'success' | 'error';
}

const policyTypeOptions = [
  { value: 'all', label: '全部类型' },
  { value: 'RateLimitPolicy', label: '限流' },
  { value: 'AccessControlPolicy', label: '访问控制' },
];

export function PolicyPage() {
  const workspace = useResource(loadPolicyWorkspace);
  const [query, setQuery] = useState('');
  const [policyKindFilter, setPolicyKindFilter] = useState<GovernancePolicyKind | 'all'>('all');
  const [editor, setEditor] = useState<EditorState>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [submitting, setSubmitting] = useState(false);

  if (workspace.loading) {
    return (
      <PageFrame title="策略" subtitle="定义请求治理规则及其应用范围">
        <ResourceStatePanel title="加载策略数据" message="正在读取限流和访问控制策略。" />
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
    const validation = editor.type === 'rateLimit'
      ? validateRateLimitPolicyDraft(editor.draft)
      : validateAccessControlPolicyDraft(editor.draft);
    if (!validation.valid) {
      return;
    }
    setSubmitting(true);
    try {
      const result = editor.type === 'rateLimit'
        ? await saveRateLimitPolicy(rateLimitPolicyPayload(editor.draft))
        : await saveAccessControlPolicy(accessControlPolicyPayload(editor.draft));
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存策略失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const deletePolicy = async (policy: GovernancePolicy) => {
    try {
      const result = policy.kind === 'RateLimitPolicy'
        ? await deleteRateLimitPolicy(policy.id, policy.name)
        : await deleteAccessControlPolicy(policy.id, policy.name);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除策略失败', tone: 'error' });
    }
  };

  const togglePolicy = async (policy: GovernancePolicy) => {
    try {
      const result = policy.kind === 'RateLimitPolicy'
        ? await setRateLimitPolicyEnabled(policy.id, policy.name, !policy.enabled)
        : await setAccessControlPolicyEnabled(policy.id, policy.name, !policy.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略状态失败', tone: 'error' });
    }
  };

  if (editor) {
    const editorValid = editor.type === 'rateLimit'
      ? validateRateLimitPolicyDraft(editor.draft).valid
      : validateAccessControlPolicyDraft(editor.draft).valid;
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
            ) : (
              <AccessControlPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={validateAccessControlPolicyDraft(editor.draft)}
                onChange={(draft) => {
                  setEditor({ type: 'accessControl', draft });
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
            setEditor(policy.kind === 'RateLimitPolicy'
              ? { type: 'rateLimit', draft: createRateLimitPolicyDraft(policy.raw) }
              : { type: 'accessControl', draft: createAccessControlPolicyDraft(policy.raw) });
          }}
          onToggle={togglePolicy}
          onDelete={deletePolicy}
        />
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function editorTitle(editor: Exclude<EditorState, null>) {
  if (editor.type === 'rateLimit') {
    return editor.draft.id ? '编辑限流策略' : '新建限流策略';
  }
  return editor.draft.id ? '编辑访问控制策略' : '新建访问控制策略';
}
