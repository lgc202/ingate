import { useEffect, useState } from 'react';
import { ChevronDown, Link2, Plus } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Tabs, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type {
  AccessControlAction,
  AccessControlConditionType,
  AccessControlPolicy,
  AccessControlPolicyPayload,
  GovernancePolicy,
  GovernancePolicyKind,
  PolicyBinding,
  PolicyBindingPayload,
  PolicyTargetKind,
  PolicyWorkspace,
  RateLimitAlgorithm,
  RateLimitFailurePolicy,
  RateLimitKeyType,
  RateLimitMode,
  RateLimitPolicy,
  RateLimitPolicyPayload,
} from '@/domain/policy';
import {
  governancePolicyKey,
  governancePolicyRef,
  policyBindingTargetLabel,
  policyKindLabel,
  policyNamesForBinding,
  policyRefKey,
  policyStatusLabel,
  policyStatusTone,
  policyTargetKindLabel,
} from '@/domain/policy';
import { PolicyMultiSelect } from './PolicyMultiSelect';

const loadPolicyWorkspace = () => consoleRepository.getPolicyWorkspace();

type PolicyTab = 'library' | 'bindings';
type EditorState =
  | { type: 'rateLimit'; draft: RateLimitDraft }
  | { type: 'accessControl'; draft: AccessControlDraft }
  | { type: 'binding'; draft: BindingDraft }
  | null;

interface RateLimitDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  mode: RateLimitMode;
  redisRef: string;
  ruleName: string;
  keyType: RateLimitKeyType;
  keyName: string;
  requests: string;
  windowSeconds: string;
  burst: string;
  algorithm: RateLimitAlgorithm;
  failurePolicy: RateLimitFailurePolicy;
  responseStatusCode: string;
  responseMessage: string;
  quotaHeaderEnabled: boolean;
}

interface AccessControlDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  defaultAction: AccessControlAction;
  ruleName: string;
  ruleAction: AccessControlAction;
  conditionType: AccessControlConditionType;
  conditionName: string;
  conditionValue: string;
  responseStatusCode: string;
  responseMessage: string;
}

interface BindingDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  targetKind: PolicyTargetKind;
  targetID: string;
  ruleName: string;
  policyKeys: string[];
}

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
const rateLimitKeyTypes: RateLimitKeyType[] = ['IP', 'Header', 'Query', 'Cookie', 'Consumer', 'Route', 'Gateway', 'RouteRule', 'JWTClaim', 'APIKey', 'Tenant'];
const aclConditionTypes: AccessControlConditionType[] = ['IP', 'Header', 'Consumer', 'Tenant'];

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
      <PageFrame title="流量 / 策略" subtitle="管理可复用策略和策略绑定关系">
        <ResourceStatePanel title="加载策略数据" message="正在读取策略和绑定关系。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="流量 / 策略" subtitle="管理可复用策略和策略绑定关系">
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
    const matchedKind = bindingContext ? binding.targetRef.kind === bindingContext.targetKind : targetKindFilter === 'all' || binding.targetRef.kind === targetKindFilter;
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
      draft: createBindingDraft(data, bindingContext ? {
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
        const result = await consoleRepository.saveRateLimitPolicy(rateLimitPayload(editor.draft));
        await reloadAfterMutation(result.message);
        return;
      }
      if (editor.type === 'accessControl') {
        const result = await consoleRepository.saveAccessControlPolicy(accessControlPayload(editor.draft));
        await reloadAfterMutation(result.message);
        return;
      }
      const result = await consoleRepository.savePolicyBinding(bindingPayload(editor.draft));
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存失败', tone: 'error' });
    }
  };

  const deletePolicy = async (policy: GovernancePolicy) => {
    try {
      const result = policy.kind === 'RateLimitPolicy'
        ? await consoleRepository.deleteRateLimitPolicy(policy.id)
        : await consoleRepository.deleteAccessControlPolicy(policy.id);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除策略失败', tone: 'error' });
    }
  };

  const togglePolicy = async (policy: GovernancePolicy) => {
    try {
      const result = policy.kind === 'RateLimitPolicy'
        ? await consoleRepository.setRateLimitPolicyEnabled(policy.id, !policy.enabled)
        : await consoleRepository.setAccessControlPolicyEnabled(policy.id, !policy.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略状态失败', tone: 'error' });
    }
  };

  const deleteBinding = async (binding: PolicyBinding) => {
    try {
      const result = await consoleRepository.deletePolicyBinding(binding.id);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除绑定失败', tone: 'error' });
    }
  };

  const toggleBinding = async (binding: PolicyBinding) => {
    try {
      const result = await consoleRepository.setPolicyBindingEnabled(binding.id, !binding.enabled);
      await reloadAfterMutation(result.message);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新绑定状态失败', tone: 'error' });
    }
  };

  if (editor) {
    return (
      <PageFrame
        title={editorTitle(editor)}
        subtitle="配置策略或策略绑定关系"
        actions={<Button variant="soft" onClick={() => setEditor(null)}>返回列表</Button>}
      >
        <Panel title="基础配置">
          {editor.type === 'rateLimit' ? (
            <RateLimitEditor draft={editor.draft} workspace={data} onChange={(draft) => setEditor({ type: 'rateLimit', draft })} />
          ) : editor.type === 'accessControl' ? (
            <AccessControlEditor draft={editor.draft} onChange={(draft) => setEditor({ type: 'accessControl', draft })} />
          ) : (
            <BindingEditor draft={editor.draft} workspace={data} onChange={(draft) => setEditor({ type: 'binding', draft })} />
          )}
          <div className="form-actions">
            <Button variant="primary" onClick={saveEditor}>保存</Button>
            <Button variant="ghost" onClick={() => setEditor(null)}>取消</Button>
          </div>
        </Panel>
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="流量 / 策略"
      subtitle="管理可复用策略和策略绑定关系"
      actions={
        activeTab === 'library' ? (
          <CreatePolicyMenu
            onCreateRateLimit={() => setEditor({ type: 'rateLimit', draft: createRateLimitDraft(data) })}
            onCreateAccessControl={() => setEditor({ type: 'accessControl', draft: createAccessControlDraft() })}
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
          <PolicyLibraryTable policies={filteredPolicies} bindings={data.bindings} onEdit={(policy) => {
            setEditor(policy.kind === 'RateLimitPolicy'
              ? { type: 'rateLimit', draft: createRateLimitDraft(data, policy.raw as RateLimitPolicy) }
              : { type: 'accessControl', draft: createAccessControlDraft(policy.raw as AccessControlPolicy) });
          }} onToggle={togglePolicy} onDelete={deletePolicy} />
        ) : (
          <PolicyBindingTable bindings={filteredBindings} policies={data.policies} targets={data.targets} onEdit={(binding) => setEditor({ type: 'binding', draft: createBindingDraft(data, binding) })} onToggle={toggleBinding} onDelete={deleteBinding} />
        )}
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function CreatePolicyMenu({
  onCreateRateLimit,
  onCreateAccessControl,
}: {
  onCreateRateLimit: () => void;
  onCreateAccessControl: () => void;
}) {
  return (
    <details className="policy-create-menu">
      <summary className="button primary">
        <Plus size={15} aria-hidden="true" />
        新建策略
        <ChevronDown size={15} aria-hidden="true" />
      </summary>
      <div className="policy-create-menu-popover">
        <button type="button" onClick={onCreateRateLimit}>
          <strong>限流策略</strong>
          <span>控制请求速率，支持 Local 和 Global / Redis</span>
        </button>
        <button type="button" onClick={onCreateAccessControl}>
          <strong>访问控制</strong>
          <span>按 IP、Header、Consumer、Tenant 放行或拒绝</span>
        </button>
      </div>
    </details>
  );
}

function PolicyLibraryTable({
  policies,
  bindings,
  onEdit,
  onToggle,
  onDelete,
}: {
  policies: GovernancePolicy[];
  bindings: PolicyBinding[];
  onEdit: (policy: GovernancePolicy) => void;
  onToggle: (policy: GovernancePolicy) => void;
  onDelete: (policy: GovernancePolicy) => void;
}) {
  if (policies.length === 0) {
    return <div className="table-empty"><EmptyState title="暂无策略" message="当前没有匹配的策略。" /></div>;
  }

  return (
    <div style={{ overflow: 'auto' }}>
      <table className="table">
        <thead>
          <tr>
            <th>策略名称</th>
            <th>类型</th>
            <th>模式</th>
            <th>规则数</th>
            <th>被绑定</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {policies.map((policy) => (
            <tr key={governancePolicyKey(policy)}>
              <td>
                <div className="table-primary">{policy.name}</div>
                <div className="table-secondary">{policy.description || policy.id}</div>
              </td>
              <td>{policyKindLabel(policy.kind)}</td>
              <td>{policy.mode}</td>
              <td>{policy.ruleCount}</td>
              <td>{policyBindingCount(policy, bindings)}</td>
              <td><Badge tone={policyStatusTone(policy.enabled)}>{policyStatusLabel(policy.enabled)}</Badge></td>
              <td>{formatDateTime(policy.createdAt ?? '')}</td>
              <td>
                <div className="row-actions">
                  <button className="link-button" type="button" onClick={() => onEdit(policy)}>编辑</button>
                  <button className="link-button" type="button" onClick={() => onToggle(policy)}>{policy.enabled ? '停用' : '启用'}</button>
                  <button className="link-button danger" type="button" onClick={() => onDelete(policy)}>删除</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PolicyBindingTable({
  bindings,
  policies,
  targets,
  onEdit,
  onToggle,
  onDelete,
}: {
  bindings: PolicyBinding[];
  policies: GovernancePolicy[];
  targets: PolicyWorkspace['targets'];
  onEdit: (binding: PolicyBinding) => void;
  onToggle: (binding: PolicyBinding) => void;
  onDelete: (binding: PolicyBinding) => void;
}) {
  if (bindings.length === 0) {
    return <div className="table-empty"><EmptyState title="暂无策略绑定" message="当前没有匹配的绑定关系。" /></div>;
  }

  return (
    <div style={{ overflow: 'auto' }}>
      <table className="table">
        <thead>
          <tr>
            <th>绑定名称</th>
            <th>作用目标</th>
            <th>绑定策略</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {bindings.map((binding) => (
            <tr key={binding.id}>
              <td>
                <div className="table-primary">{binding.name}</div>
                <div className="table-secondary">{binding.description || binding.id}</div>
              </td>
              <td>{policyBindingTargetLabel(binding, targets)}</td>
              <td>{policyNamesForBinding(binding, policies).join('、')}</td>
              <td><Badge tone={policyStatusTone(binding.enabled)}>{policyStatusLabel(binding.enabled)}</Badge></td>
              <td>{formatDateTime(binding.createdAt ?? '')}</td>
              <td>
                <div className="row-actions">
                  <button className="link-button" type="button" onClick={() => onEdit(binding)}>编辑</button>
                  <button className="link-button" type="button" onClick={() => onToggle(binding)}>{binding.enabled ? '停用' : '启用'}</button>
                  <button className="link-button danger" type="button" onClick={() => onDelete(binding)}>删除</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RateLimitEditor({ draft, workspace, onChange }: { draft: RateLimitDraft; workspace: PolicyWorkspace; onChange: (draft: RateLimitDraft) => void }) {
  const needsKeyName = ['Header', 'Query', 'Cookie', 'JWTClaim'].includes(draft.keyType);

  return (
    <div className="policy-editor-grid">
      <InputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
      <SelectField label="限流模式" value={draft.mode} options={[['Local', 'Local'], ['Global', 'Global / Redis']]} onChange={(mode) => onChange({ ...draft, mode: mode as RateLimitMode })} />
      <InputField label="描述" value={draft.description} onChange={(description) => onChange({ ...draft, description })} />
      {draft.mode === 'Global' ? (
        <SelectField label="Redis 配置" value={draft.redisRef} options={[['', '选择 Redis'], ...workspace.redisStores.map((store) => [store.id, store.name] as [string, string])]} onChange={(redisRef) => onChange({ ...draft, redisRef })} />
      ) : null}
      <InputField label="规则名称" value={draft.ruleName} onChange={(ruleName) => onChange({ ...draft, ruleName })} />
      <SelectField label="计数维度" value={draft.keyType} options={rateLimitKeyTypes.map((type) => [type, rateLimitKeyLabel(type)])} onChange={(keyType) => onChange({ ...draft, keyType: keyType as RateLimitKeyType })} />
      {needsKeyName ? <InputField label="维度名称" value={draft.keyName} onChange={(keyName) => onChange({ ...draft, keyName })} /> : null}
      <SelectField label="算法" value={draft.algorithm} options={[['FixedWindow', '固定窗口'], ['SlidingWindow', '滑动窗口'], ['TokenBucket', '令牌桶']]} onChange={(algorithm) => onChange({ ...draft, algorithm: algorithm as RateLimitAlgorithm })} />
      <InputField label="请求数" value={draft.requests} type="number" onChange={(requests) => onChange({ ...draft, requests })} />
      <InputField label="窗口秒数" value={draft.windowSeconds} type="number" onChange={(windowSeconds) => onChange({ ...draft, windowSeconds })} />
      <InputField label="Burst" value={draft.burst} type="number" onChange={(burst) => onChange({ ...draft, burst })} />
      <SelectField label="失败策略" value={draft.failurePolicy} options={[['FailOpen', '失败放行'], ['FailClose', '失败拒绝']]} onChange={(failurePolicy) => onChange({ ...draft, failurePolicy: failurePolicy as RateLimitFailurePolicy })} />
      <InputField label="超限状态码" value={draft.responseStatusCode} type="number" onChange={(responseStatusCode) => onChange({ ...draft, responseStatusCode })} />
      <InputField label="超限消息" value={draft.responseMessage} onChange={(responseMessage) => onChange({ ...draft, responseMessage })} />
      <label className="policy-check-row">
        <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
        <span>启用策略</span>
      </label>
      <label className="policy-check-row">
        <input type="checkbox" checked={draft.quotaHeaderEnabled} onChange={(event) => onChange({ ...draft, quotaHeaderEnabled: event.target.checked })} />
        <span>返回限流配额 Header</span>
      </label>
    </div>
  );
}

function AccessControlEditor({ draft, onChange }: { draft: AccessControlDraft; onChange: (draft: AccessControlDraft) => void }) {
  const needsConditionName = draft.conditionType === 'Header';

  return (
    <div className="policy-editor-grid">
      <InputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
      <SelectField label="默认动作" value={draft.defaultAction} options={[['Allow', '默认放行'], ['Deny', '默认拒绝']]} onChange={(defaultAction) => onChange({ ...draft, defaultAction: defaultAction as AccessControlAction })} />
      <InputField label="描述" value={draft.description} onChange={(description) => onChange({ ...draft, description })} />
      <InputField label="规则名称" value={draft.ruleName} onChange={(ruleName) => onChange({ ...draft, ruleName })} />
      <SelectField label="规则动作" value={draft.ruleAction} options={[['Allow', '允许'], ['Deny', '拒绝']]} onChange={(ruleAction) => onChange({ ...draft, ruleAction: ruleAction as AccessControlAction })} />
      <SelectField label="匹配类型" value={draft.conditionType} options={aclConditionTypes.map((type) => [type, aclConditionTypeLabel(type)])} onChange={(conditionType) => onChange({ ...draft, conditionType: conditionType as AccessControlConditionType })} />
      {needsConditionName ? <InputField label="匹配名称" value={draft.conditionName} onChange={(conditionName) => onChange({ ...draft, conditionName })} /> : null}
      <InputField label="匹配值" value={draft.conditionValue} onChange={(conditionValue) => onChange({ ...draft, conditionValue })} />
      <InputField label="拒绝状态码" value={draft.responseStatusCode} type="number" onChange={(responseStatusCode) => onChange({ ...draft, responseStatusCode })} />
      <InputField label="拒绝消息" value={draft.responseMessage} onChange={(responseMessage) => onChange({ ...draft, responseMessage })} />
      <label className="policy-check-row">
        <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
        <span>启用策略</span>
      </label>
    </div>
  );
}

function BindingEditor({ draft, workspace, onChange }: { draft: BindingDraft; workspace: PolicyWorkspace; onChange: (draft: BindingDraft) => void }) {
  const targets = workspace.targets.filter((target) => target.kind === draft.targetKind);

  return (
    <div className="policy-editor-grid">
      <InputField label="绑定名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
      <SelectField label="目标类型" value={draft.targetKind} options={targetKindOptions.map((kind) => [kind, policyTargetKindLabel(kind)])} onChange={(targetKind) => onChange({ ...draft, targetKind: targetKind as PolicyTargetKind, targetID: '' })} />
      <InputField label="描述" value={draft.description} onChange={(description) => onChange({ ...draft, description })} />
      <SelectField label="作用目标" value={draft.targetID} options={[['', '选择目标'], ...targets.map((target) => [target.id, target.name] as [string, string])]} onChange={(targetID) => onChange({ ...draft, targetID })} />
      {draft.targetKind === 'Route' ? <InputField label="规则名称" value={draft.ruleName} placeholder="不填表示整条路由" onChange={(ruleName) => onChange({ ...draft, ruleName })} /> : null}
      <label className="policy-check-row">
        <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
        <span>启用绑定</span>
      </label>
      <PolicyMultiSelect
        policies={workspace.policies}
        value={draft.policyKeys}
        onChange={(policyKeys) => onChange({ ...draft, policyKeys })}
      />
    </div>
  );
}

function InputField({ label, value, placeholder, type = 'text', onChange }: { label: string; value: string; placeholder?: string; type?: string; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <input value={value} type={type} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function SelectField({ label, value, options, onChange }: { label: string; value: string; options: [string, string][]; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map(([optionValue, labelText]) => (
          <option key={optionValue || labelText} value={optionValue}>{labelText}</option>
        ))}
      </select>
    </div>
  );
}

function createRateLimitDraft(workspace: PolicyWorkspace, policy?: RateLimitPolicy): RateLimitDraft {
  const rule = policy?.rules[0];
  const keyPart = rule?.key.parts[0];
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    enabled: policy?.enabled ?? true,
    mode: policy?.mode ?? 'Local',
    redisRef: policy?.global?.redisRef ?? workspace.redisStores[0]?.id ?? '',
    ruleName: rule?.name ?? 'default',
    keyType: keyPart?.type ?? 'IP',
    keyName: keyPart?.name ?? '',
    requests: String(rule?.limit.requests ?? 100),
    windowSeconds: String(rule?.limit.windowSeconds ?? 60),
    burst: String(rule?.limit.burst ?? 0),
    algorithm: rule?.algorithm ?? 'FixedWindow',
    failurePolicy: policy?.failurePolicy ?? 'FailOpen',
    responseStatusCode: String(policy?.response?.statusCode ?? 429),
    responseMessage: policy?.response?.message ?? 'Too many requests',
    quotaHeaderEnabled: policy?.response?.quotaHeaderEnabled ?? true,
  };
}

function createAccessControlDraft(policy?: AccessControlPolicy): AccessControlDraft {
  const rule = policy?.rules?.[0];
  const condition = rule?.conditions?.[0];
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    enabled: policy?.enabled ?? true,
    defaultAction: policy?.defaultAction || 'Deny',
    ruleName: rule?.name ?? 'default',
    ruleAction: rule?.action || 'Allow',
    conditionType: condition?.type ?? 'IP',
    conditionName: condition?.name ?? '',
    conditionValue: condition?.value ?? '',
    responseStatusCode: String(policy?.response?.statusCode ?? 403),
    responseMessage: policy?.response?.message ?? 'Forbidden',
  };
}

function bindingTargetContext(searchParams: URLSearchParams, workspace: PolicyWorkspace): BindingTargetContext | null {
  const targetKind = searchParams.get('targetKind') as PolicyTargetKind | null;
  const targetID = searchParams.get('targetID') ?? '';
  const ruleName = searchParams.get('ruleName') ?? '';
  if (!targetKind || !targetKindOptions.includes(targetKind) || !targetID) {
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

function createBindingDraft(workspace: PolicyWorkspace, source?: PolicyBinding | { targetKind: PolicyTargetKind; targetID: string; ruleName?: string }): BindingDraft {
  if (source && 'targetRef' in source) {
    return {
      id: source.id,
      version: source.version,
      name: source.name,
      description: source.description ?? '',
      enabled: source.enabled,
      targetKind: source.targetRef.kind,
      targetID: source.targetRef.name,
      ruleName: source.targetRef.ruleName ?? '',
      policyKeys: source.policies.map(policyRefKey),
    };
  }

  const targetKind = source?.targetKind ?? 'Gateway';
  const targetID = source?.targetID ?? workspace.targets.find((target) => target.kind === targetKind)?.id ?? '';
  const target = workspace.targets.find((item) => item.kind === targetKind && item.id === targetID);
  return {
    name: target ? `${target.name} 策略绑定` : '',
    description: '',
    enabled: true,
    targetKind,
    targetID,
    ruleName: source?.ruleName ?? '',
    policyKeys: [],
  };
}

function rateLimitPayload(draft: RateLimitDraft): RateLimitPolicyPayload {
  const keyPart = { type: draft.keyType, ...(draft.keyName ? { name: draft.keyName } : {}) };
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    mode: draft.mode,
    rules: [
      {
        name: draft.ruleName,
        key: { parts: [keyPart] },
        limit: {
          requests: Number(draft.requests),
          windowSeconds: Number(draft.windowSeconds),
          burst: Number(draft.burst || 0),
        },
        algorithm: draft.algorithm,
      },
    ],
    global: draft.mode === 'Global' ? { redisRef: draft.redisRef } : undefined,
    response: {
      statusCode: Number(draft.responseStatusCode || 429),
      message: draft.responseMessage,
      quotaHeaderEnabled: draft.quotaHeaderEnabled,
    },
    failurePolicy: draft.failurePolicy,
  };
}

function accessControlPayload(draft: AccessControlDraft): AccessControlPolicyPayload {
  const condition = {
    type: draft.conditionType,
    ...(draft.conditionName ? { name: draft.conditionName } : {}),
    value: draft.conditionValue,
  };
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    defaultAction: draft.defaultAction,
    rules: [{
      name: draft.ruleName,
      action: draft.ruleAction,
      conditions: [condition],
    }],
    response: {
      statusCode: Number(draft.responseStatusCode || 403),
      message: draft.responseMessage,
    },
  };
}

function bindingPayload(draft: BindingDraft): PolicyBindingPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    targetRef: {
      kind: draft.targetKind,
      name: draft.targetID,
      ...(draft.targetKind === 'Route' && draft.ruleName ? { ruleName: draft.ruleName } : {}),
    },
    policies: draft.policyKeys.map((key) => {
      const [kind, name] = key.split(':') as [GovernancePolicyKind, string];
      return { kind, name };
    }),
  };
}

function policyBindingCount(policy: GovernancePolicy, bindings: PolicyBinding[]) {
  const ref = governancePolicyRef(policy);
  return bindings.filter((binding) => binding.policies.some((policyRef) => policyRef.kind === ref.kind && policyRef.name === ref.name)).length;
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

function rateLimitKeyLabel(type: RateLimitKeyType) {
  const labels: Record<RateLimitKeyType, string> = {
    IP: '客户端 IP',
    Header: 'Header',
    Query: 'Query',
    Cookie: 'Cookie',
    Consumer: 'Consumer',
    Route: 'Route',
    Gateway: 'Gateway',
    RouteRule: 'RouteRule',
    JWTClaim: 'JWT Claim',
    APIKey: 'API Key',
    Tenant: '租户',
  };
  return labels[type];
}

function aclConditionTypeLabel(type: AccessControlConditionType) {
  const labels: Record<AccessControlConditionType, string> = {
    IP: '客户端 IP',
    Header: 'Header',
    Consumer: 'Consumer',
    Tenant: '租户',
  };
  return labels[type];
}
