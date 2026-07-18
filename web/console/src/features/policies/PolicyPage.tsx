import { useEffect, useState } from 'react';
import { Link2 } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import {
  deleteAccessControlPolicy,
  deletePolicyBinding,
  deleteRateLimitPolicy,
  getPolicyWorkspace,
  saveAccessControlPolicy,
  savePolicyBinding,
  saveRateLimitPolicy,
  setAccessControlPolicyEnabled,
  setPolicyBindingEnabled,
  setRateLimitPolicyEnabled,
} from '@/api/policies';
import { useResource } from '@/api/useResource';
import { Button, PageFrame, Panel, ResourceStatePanel, Tabs, Toast } from '@/components/ui';
import type {
  AccessControlPolicy,
  GovernancePolicy,
  GovernancePolicyKind,
  PolicyBinding,
  PolicyTargetKind,
  PolicyWorkspace,
  RateLimitPolicy,
} from '@/domain/policy';
import {
  policyBindingTargetLabel,
  policyNamesForBinding,
  policyTargetKindLabel,
} from '@/domain/policy';
import {
  AccessControlPolicyEditor,
  accessControlPolicyPayload,
  createAccessControlPolicyDraft,
  type AccessControlPolicyDraft,
} from './AccessControlPolicyEditor';
import {
  createPolicyBindingDraft,
  PolicyBindingEditor,
  policyBindingPayload,
  type PolicyBindingDraft,
} from './PolicyBindingEditor';
import { PolicyBindingTable } from './PolicyBindingTable';
import { CreatePolicyMenu, PolicyLibraryTable } from './PolicyLibraryTable';
import {
  createRateLimitPolicyDraft,
  RateLimitPolicyEditor,
  rateLimitPolicyPayload,
  type RateLimitPolicyDraft,
} from './RateLimitPolicyEditor';

const loadPolicyWorkspace = () => getPolicyWorkspace();

type PolicyTab = 'library' | 'bindings';
type EditorState =
  | { type: 'rateLimit'; draft: RateLimitPolicyDraft }
  | { type: 'accessControl'; draft: AccessControlPolicyDraft }
  | { type: 'binding'; draft: PolicyBindingDraft }
  | null;

interface Notice {
  message: string;
  tone: 'success' | 'error';
}

interface BindingTargetContext {
  targetKind: PolicyTargetKind;
  targetID: string;
  ruleName: string;
  label: string;
}

const policyTabs = [
  { key: 'library', label: '策略库' },
  { key: 'bindings', label: '策略绑定' },
];

const policyTypeOptions = [
  { value: 'all', label: '全部类型' },
  { value: 'RateLimitPolicy', label: '限流' },
  { value: 'AccessControlPolicy', label: '访问控制' },
];

const targetKindOptions: PolicyTargetKind[] = ['Gateway', 'Route'];

export function PolicyPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab') === 'bindings' ? 'bindings' : 'library';
  const workspace = useResource(loadPolicyWorkspace);
  const [activeTab, setActiveTab] = useState<PolicyTab>(requestedTab);
  const [query, setQuery] = useState('');
  const [policyKindFilter, setPolicyKindFilter] = useState<GovernancePolicyKind | 'all'>('all');
  const [targetKindFilter, setTargetKindFilter] = useState<PolicyTargetKind | 'all'>('all');
  const [editor, setEditor] = useState<EditorState>(null);
  const [notice, setNotice] = useState<Notice | null>(null);

  useEffect(() => {
    setActiveTab(requestedTab);
  }, [requestedTab]);

  if (workspace.loading) {
    return (
      <PageFrame title="策略" subtitle="管理可复用策略和策略绑定关系">
        <ResourceStatePanel title="加载策略数据" message="正在读取策略和绑定关系。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="策略" subtitle="管理可复用策略和策略绑定关系">
        <ResourceStatePanel title="策略数据加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const data = workspace.data;
  const bindingContext = activeTab === 'bindings' ? bindingTargetContext(searchParams, data) : null;
  const filteredPolicies = data.policies.filter((policy) => {
    const keyword = query.trim().toLowerCase();
    const matchedKeyword = !keyword || [policy.name, policy.description ?? '', policy.mode].some((value) => value.toLowerCase().includes(keyword));
    const matchedKind = policyKindFilter === 'all' || policy.kind === policyKindFilter;
    return matchedKeyword && matchedKind;
  });
  const filteredBindings = data.bindings.filter((binding) => {
    const keyword = query.trim().toLowerCase();
    const matchedKeyword = !keyword || [
      binding.name,
      binding.description ?? '',
      policyBindingTargetLabel(binding, data.targets),
      ...policyNamesForBinding(binding, data.policies),
    ].some((value) => value.toLowerCase().includes(keyword));
    const matchedKind = bindingContext
      ? binding.targetRef.kind === bindingContext.targetKind
      : targetKindFilter === 'all' || binding.targetRef.kind === targetKindFilter;
    const matchedContext = !bindingContext || (
      binding.targetRef.name === bindingContext.targetID
      && (!bindingContext.ruleName || !binding.targetRef.ruleName || binding.targetRef.ruleName === bindingContext.ruleName)
    );
    return matchedKeyword && matchedKind && matchedContext;
  });

  const switchTab = (tab: string) => {
    const nextTab = tab as PolicyTab;
    setActiveTab(nextTab);
    setEditor(null);
    setSearchParams(nextTab === 'bindings' ? { tab: 'bindings' } : {});
  };

  const reloadAfterMutation = async (resultMessage: string) => {
    await workspace.reload();
    setNotice({ message: resultMessage, tone: 'success' });
    setEditor(null);
  };

  const createBinding = () => {
    setEditor({
      type: 'binding',
      draft: createPolicyBindingDraft(data, bindingContext ? {
        targetKind: bindingContext.targetKind,
        targetID: bindingContext.targetID,
        ruleName: bindingContext.ruleName,
      } : undefined),
    });
  };

  const saveEditor = async () => {
    if (!editor) {
      return;
    }

    try {
      if (editor.type === 'rateLimit') {
        const result = await saveRateLimitPolicy(rateLimitPolicyPayload(editor.draft));
        await reloadAfterMutation(result.message);
        return;
      }
      if (editor.type === 'accessControl') {
        const result = await saveAccessControlPolicy(accessControlPolicyPayload(editor.draft));
        await reloadAfterMutation(result.message);
        return;
      }
      const result = await savePolicyBinding(policyBindingPayload(editor.draft));
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存失败', tone: 'error' });
    }
  };

  const deletePolicy = async (policy: GovernancePolicy) => {
    try {
      const result = policy.kind === 'RateLimitPolicy'
        ? await deleteRateLimitPolicy(policy.id)
        : await deleteAccessControlPolicy(policy.id);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除策略失败', tone: 'error' });
    }
  };

  const togglePolicy = async (policy: GovernancePolicy) => {
    try {
      const result = policy.kind === 'RateLimitPolicy'
        ? await setRateLimitPolicyEnabled(policy.id, !policy.enabled)
        : await setAccessControlPolicyEnabled(policy.id, !policy.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略状态失败', tone: 'error' });
    }
  };

  const deleteBinding = async (binding: PolicyBinding) => {
    try {
      const result = await deletePolicyBinding(binding.id);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除绑定失败', tone: 'error' });
    }
  };

  const toggleBinding = async (binding: PolicyBinding) => {
    try {
      const result = await setPolicyBindingEnabled(binding.id, !binding.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新绑定状态失败', tone: 'error' });
    }
  };

  if (editor) {
    return (
      <PageFrame
        title="策略"
        subtitle={editorTitle(editor)}
        actions={<Button variant="soft" onClick={() => setEditor(null)}>返回列表</Button>}
      >
        <section className="editor-layout">
          <Panel title="配置详情">
            {editor.type === 'rateLimit' ? (
              <RateLimitPolicyEditor draft={editor.draft} onChange={(draft) => setEditor({ type: 'rateLimit', draft })} />
            ) : editor.type === 'accessControl' ? (
              <AccessControlPolicyEditor draft={editor.draft} onChange={(draft) => setEditor({ type: 'accessControl', draft })} />
            ) : (
              <PolicyBindingEditor draft={editor.draft} workspace={data} onChange={(draft) => setEditor({ type: 'binding', draft })} />
            )}
            <div className="form-actions">
              <Button variant="primary" onClick={saveEditor}>保存</Button>
              <Button variant="ghost" onClick={() => setEditor(null)}>取消</Button>
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
      subtitle="管理可复用策略和策略绑定关系"
      actions={
        activeTab === 'library' ? (
          <CreatePolicyMenu
            onCreateRateLimit={() => setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft() })}
            onCreateAccessControl={() => setEditor({ type: 'accessControl', draft: createAccessControlPolicyDraft() })}
          />
        ) : (
          <Button variant="primary" onClick={createBinding}>
            <Link2 size={15} aria-hidden="true" />新建绑定
          </Button>
        )
      }
    >
      <Panel>
        <Tabs tabs={policyTabs} active={activeTab} onChange={switchTab} />
        <div className="policy-toolbar">
          <input value={query} placeholder={activeTab === 'library' ? '搜索策略名称 / 模式' : '搜索绑定名称 / 目标 / 策略'} onChange={(event) => setQuery(event.target.value)} />
          {activeTab === 'library' ? (
            <select value={policyKindFilter} onChange={(event) => setPolicyKindFilter(event.target.value as GovernancePolicyKind | 'all')}>
              {policyTypeOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          ) : bindingContext ? (
            <div className="policy-context-filter">
              <div>
                <span>当前筛选</span>
                <strong>{bindingContext.label}</strong>
              </div>
              <button type="button" onClick={() => setSearchParams({ tab: 'bindings' })}>清除筛选</button>
            </div>
          ) : (
            <select value={targetKindFilter} onChange={(event) => setTargetKindFilter(event.target.value as PolicyTargetKind | 'all')}>
              <option value="all">全部目标</option>
              {targetKindOptions.map((kind) => (
                <option key={kind} value={kind}>{policyTargetKindLabel(kind)}</option>
              ))}
            </select>
          )}
        </div>
        {activeTab === 'library' ? (
          <PolicyLibraryTable
            policies={filteredPolicies}
            bindings={data.bindings}
            onEdit={(policy) => {
              setEditor(policy.kind === 'RateLimitPolicy'
                ? { type: 'rateLimit', draft: createRateLimitPolicyDraft(policy.raw as RateLimitPolicy) }
                : { type: 'accessControl', draft: createAccessControlPolicyDraft(policy.raw as AccessControlPolicy) });
            }}
            onToggle={togglePolicy}
            onDelete={deletePolicy}
          />
        ) : (
          <PolicyBindingTable
            bindings={filteredBindings}
            policies={data.policies}
            targets={data.targets}
            onEdit={(binding) => setEditor({ type: 'binding', draft: createPolicyBindingDraft(data, binding) })}
            onToggle={toggleBinding}
            onDelete={deleteBinding}
          />
        )}
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function bindingTargetContext(searchParams: URLSearchParams, workspace: PolicyWorkspace): BindingTargetContext | null {
  const targetKind = searchParams.get('targetKind');
  const targetID = searchParams.get('targetID') ?? '';
  const ruleName = searchParams.get('ruleName') ?? '';
  if ((targetKind !== 'Gateway' && targetKind !== 'Route') || !targetID) {
    return null;
  }

  const target = workspace.targets.find((item) => item.kind === targetKind && item.id === targetID);
  if (!target) {
    return null;
  }

  return {
    targetKind,
    targetID,
    ruleName,
    label: ruleName
      ? `${policyTargetKindLabel(targetKind)} / ${target.name} / ${ruleName}`
      : `${policyTargetKindLabel(targetKind)} / ${target.name}`,
  };
}

function editorTitle(editor: Exclude<EditorState, null>) {
  if (editor.type === 'rateLimit') {
    return editor.draft.id ? '编辑限流策略' : '新建限流策略';
  }
  if (editor.type === 'accessControl') {
    return editor.draft.id ? '编辑访问控制策略' : '新建访问控制策略';
  }
  return editor.draft.id ? '编辑策略绑定' : '新建策略绑定';
}
