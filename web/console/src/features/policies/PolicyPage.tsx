import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Tabs, Toast } from '@/components/ui';
import { formatDateTime, statusTone } from '@/domain/common';
import type {
  AccessControlAction,
  AccessControlCondition,
  AccessControlConditionType,
  AccessControlPolicy,
  AccessControlRule,
  PolicyBinding,
  PolicyKind,
  PolicyOption,
  PolicyRef,
  PolicyTargetKind,
  PolicyWorkspace,
  PolicyWorkspaceTab,
  RateLimitAlgorithm,
  RateLimitFailurePolicy,
  RateLimitKeyPart,
  RateLimitKeyType,
  RateLimitMode,
  RateLimitPolicy,
  RateLimitRule,
} from '@/domain/policy';
import {
  accessControlActionLabel,
  conditionTypeLabel,
  policyKindLabel,
  policyTargetKindLabel,
  rateLimitAlgorithmLabel,
  rateLimitFailurePolicyLabel,
  rateLimitKeyTypeLabel,
  rateLimitModeLabel,
} from '@/domain/policy';

const loadPolicyWorkspace = () => consoleRepository.getPolicyWorkspace();

type PanelMode = 'list' | 'create' | 'edit';
type DeleteCandidate =
  | { kind: 'rate-limit'; id: string; name: string }
  | { kind: 'access-control'; id: string; name: string }
  | { kind: 'binding'; id: string; name: string };

const tabs: { key: PolicyWorkspaceTab; label: string }[] = [
  { key: 'access-control', label: '访问控制策略' },
  { key: 'rate-limit', label: '限流策略' },
  { key: 'bindings', label: '策略绑定' },
];

const rateLimitModes: RateLimitMode[] = ['Local', 'Global'];
const rateLimitAlgorithms: RateLimitAlgorithm[] = ['FixedWindow', 'SlidingWindow', 'TokenBucket'];
const rateLimitKeyTypes: RateLimitKeyType[] = ['IP', 'Header', 'Query', 'Cookie', 'Consumer', 'Route', 'Gateway', 'RouteRule', 'JWTClaim', 'APIKey', 'Tenant'];
const failurePolicies: RateLimitFailurePolicy[] = ['FailOpen', 'FailClose'];
const accessActions: AccessControlAction[] = ['Allow', 'Deny'];
const conditionTypes: AccessControlConditionType[] = ['IP', 'Header', 'Consumer', 'Tenant'];
const policyKinds: PolicyKind[] = ['AccessControlPolicy', 'RateLimitPolicy'];
const targetKinds: PolicyTargetKind[] = ['Gateway', 'Route'];

export function PolicyPage() {
  const workspace = useResource(loadPolicyWorkspace);
  const [activeTab, setActiveTab] = useState<PolicyWorkspaceTab>('access-control');
  const [panelMode, setPanelMode] = useState<PanelMode>('list');
  const [query, setQuery] = useState('');
  const [notice, setNotice] = useState<{ message: string; tone?: 'success' | 'error' } | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<DeleteCandidate | null>(null);
  const [accessDraft, setAccessDraft] = useState<AccessControlPolicy | null>(null);
  const [rateLimitDraft, setRateLimitDraft] = useState<RateLimitPolicy | null>(null);
  const [bindingDraft, setBindingDraft] = useState<PolicyBinding | null>(null);

  if (workspace.loading) {
    return (
      <PageFrame title="流量 / 策略" subtitle="管理内置治理策略与资源绑定">
        <ResourceStatePanel title="加载策略数据" message="正在读取访问控制策略、限流策略和策略绑定。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="流量 / 策略" subtitle="管理内置治理策略与资源绑定">
        <ResourceStatePanel title="策略数据加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const data = workspace.data;

  const closeEditor = () => {
    setPanelMode('list');
    setAccessDraft(null);
    setRateLimitDraft(null);
    setBindingDraft(null);
    setSubmitting(false);
  };

  const openCreate = () => {
    setPanelMode('create');
    if (activeTab === 'access-control') {
      setAccessDraft(newAccessControlPolicy());
    }
    if (activeTab === 'rate-limit') {
      setRateLimitDraft(newRateLimitPolicy(data));
    }
    if (activeTab === 'bindings') {
      setBindingDraft(newPolicyBinding(data));
    }
  };

  const submitAccessControlPolicy = async () => {
    if (!accessDraft) {
      return;
    }
    const error = validateAccessControlPolicy(accessDraft);
    if (error) {
      setNotice({ message: error, tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const result = await consoleRepository.saveAccessControlPolicy(accessDraft);
      await workspace.reload();
      setNotice({ message: result.message });
      closeEditor();
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存访问控制策略失败', tone: 'error' });
      setSubmitting(false);
    }
  };

  const submitRateLimitPolicy = async () => {
    if (!rateLimitDraft) {
      return;
    }
    const error = validateRateLimitPolicy(rateLimitDraft);
    if (error) {
      setNotice({ message: error, tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const result = await consoleRepository.saveRateLimitPolicy(rateLimitDraft);
      await workspace.reload();
      setNotice({ message: result.message });
      closeEditor();
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存限流策略失败', tone: 'error' });
      setSubmitting(false);
    }
  };

  const submitPolicyBinding = async () => {
    if (!bindingDraft) {
      return;
    }
    const error = validatePolicyBinding(bindingDraft);
    if (error) {
      setNotice({ message: error, tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const result = await consoleRepository.savePolicyBinding(bindingDraft);
      await workspace.reload();
      setNotice({ message: result.message });
      closeEditor();
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存策略绑定失败', tone: 'error' });
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteCandidate) {
      return;
    }

    setSubmitting(true);
    try {
      if (deleteCandidate.kind === 'access-control') {
        await consoleRepository.deleteAccessControlPolicy(deleteCandidate.id);
      }
      if (deleteCandidate.kind === 'rate-limit') {
        await consoleRepository.deleteRateLimitPolicy(deleteCandidate.id);
      }
      if (deleteCandidate.kind === 'binding') {
        await consoleRepository.deletePolicyBinding(deleteCandidate.id);
      }
      await workspace.reload();
      setNotice({ message: `已删除：${deleteCandidate.name}` });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const setEnabled = async (candidate: DeleteCandidate, enabled: boolean) => {
    try {
      if (candidate.kind === 'access-control') {
        await consoleRepository.setAccessControlPolicyEnabled(candidate.id, enabled);
      }
      if (candidate.kind === 'rate-limit') {
        await consoleRepository.setRateLimitPolicyEnabled(candidate.id, enabled);
      }
      if (candidate.kind === 'binding') {
        await consoleRepository.setPolicyBindingEnabled(candidate.id, enabled);
      }
      await workspace.reload();
      setNotice({ message: `${candidate.name} 已${enabled ? '启用' : '停用'}` });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新启用状态失败', tone: 'error' });
    }
  };

  if (panelMode !== 'list') {
    return (
      <PageFrame
        title={panelMode === 'create' ? '新建策略配置' : '编辑策略配置'}
        subtitle="策略配置和绑定关系分开维护，运行时由控制面自动编译并注入内置插件"
        actions={<Button variant="soft" onClick={closeEditor} disabled={submitting}>返回列表</Button>}
      >
        {activeTab === 'access-control' && accessDraft ? (
          <AccessControlEditor
            draft={accessDraft}
            submitting={submitting}
            onChange={setAccessDraft}
            onCancel={closeEditor}
            onSubmit={submitAccessControlPolicy}
          />
        ) : null}
        {activeTab === 'rate-limit' && rateLimitDraft ? (
          <RateLimitEditor
            draft={rateLimitDraft}
            workspace={data}
            submitting={submitting}
            onChange={setRateLimitDraft}
            onCancel={closeEditor}
            onSubmit={submitRateLimitPolicy}
          />
        ) : null}
        {activeTab === 'bindings' && bindingDraft ? (
          <PolicyBindingEditor
            draft={bindingDraft}
            workspace={data}
            submitting={submitting}
            onChange={setBindingDraft}
            onCancel={closeEditor}
            onSubmit={submitPolicyBinding}
          />
        ) : null}
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="流量 / 策略"
      subtitle="管理限流、访问控制等内置治理策略，并绑定到网关、路由或路由规则"
      actions={
        <>
          <Button variant="soft" onClick={() => setQuery('')}>重置筛选</Button>
          <Button variant="primary" onClick={openCreate}><Plus size={16} />新建</Button>
        </>
      }
    >
      <div className="stat-strip">
        <div className="stat-tile"><span>访问控制策略</span><strong>{data.accessControlPolicies.length}</strong><small>{enabledCount(data.accessControlPolicies)} 个启用</small></div>
        <div className="stat-tile"><span>限流策略</span><strong>{data.rateLimitPolicies.length}</strong><small>{enabledCount(data.rateLimitPolicies)} 个启用</small></div>
        <div className="stat-tile"><span>策略绑定</span><strong>{data.bindings.length}</strong><small>{enabledCount(data.bindings)} 个启用</small></div>
        <div className="stat-tile"><span>Redis 配置</span><strong>{data.redisStores.length}</strong><small>用于全局限流</small></div>
      </div>
      <Panel
        title="策略工作台"
        actions={<input className="toolbar-input" value={query} placeholder="搜索名称 / 描述 / 目标" onChange={(event) => setQuery(event.target.value)} />}
      >
        <Tabs tabs={tabs} active={activeTab} onChange={(key) => {
          setActiveTab(key as PolicyWorkspaceTab);
          setQuery('');
        }} />
        <div className="policy-table-wrap">
          {activeTab === 'access-control' ? (
            <AccessControlTable
              policies={filterItems(data.accessControlPolicies, query, (policy) => [policy.name, policy.description ?? ''])}
              bindings={data.bindings}
              onEdit={(policy) => {
                setAccessDraft(structuredClone(policy));
                setPanelMode('edit');
              }}
              onDelete={(policy) => setDeleteCandidate({ kind: 'access-control', id: policy.id, name: policy.name })}
              onSetEnabled={(policy, enabled) => setEnabled({ kind: 'access-control', id: policy.id, name: policy.name }, enabled)}
            />
          ) : null}
          {activeTab === 'rate-limit' ? (
            <RateLimitTable
              policies={filterItems(data.rateLimitPolicies, query, (policy) => [policy.name, policy.description ?? '', policy.mode])}
              bindings={data.bindings}
              onEdit={(policy) => {
                setRateLimitDraft(structuredClone(policy));
                setPanelMode('edit');
              }}
              onDelete={(policy) => setDeleteCandidate({ kind: 'rate-limit', id: policy.id, name: policy.name })}
              onSetEnabled={(policy, enabled) => setEnabled({ kind: 'rate-limit', id: policy.id, name: policy.name }, enabled)}
            />
          ) : null}
          {activeTab === 'bindings' ? (
            <PolicyBindingTable
              bindings={filterItems(data.bindings, query, (binding) => [binding.name, binding.description ?? '', binding.targetRef.name, binding.targetRef.ruleName ?? ''])}
              workspace={data}
              onEdit={(binding) => {
                setBindingDraft(structuredClone(binding));
                setPanelMode('edit');
              }}
              onDelete={(binding) => setDeleteCandidate({ kind: 'binding', id: binding.id, name: binding.name })}
              onSetEnabled={(binding, enabled) => setEnabled({ kind: 'binding', id: binding.id, name: binding.name }, enabled)}
            />
          ) : null}
        </div>
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      {deleteCandidate ? (
        <div className="confirm-overlay" role="presentation" onMouseDown={() => setDeleteCandidate(null)}>
          <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-policy-title" onMouseDown={(event) => event.stopPropagation()}>
            <h3 id="delete-policy-title">删除确认</h3>
            <p>删除后无法恢复。若策略仍被绑定引用，后端会拒绝本次删除。</p>
            <div className="confirm-meta">
              <span>名称</span><strong>{deleteCandidate.name}</strong>
              <span>ID</span><strong>{deleteCandidate.id}</strong>
            </div>
            <div className="confirm-actions">
              <Button variant="soft" onClick={() => setDeleteCandidate(null)} disabled={submitting}>取消</Button>
              <Button variant="primary" onClick={confirmDelete} disabled={submitting}>{submitting ? '删除中...' : '确认删除'}</Button>
            </div>
          </div>
        </div>
      ) : null}
    </PageFrame>
  );
}

function AccessControlTable({
  policies,
  bindings,
  onEdit,
  onDelete,
  onSetEnabled,
}: {
  policies: AccessControlPolicy[];
  bindings: PolicyBinding[];
  onEdit: (policy: AccessControlPolicy) => void;
  onDelete: (policy: AccessControlPolicy) => void;
  onSetEnabled: (policy: AccessControlPolicy, enabled: boolean) => void;
}) {
  if (policies.length === 0) {
    return <EmptyState title="暂无访问控制策略" message="创建策略后再通过策略绑定投放到网关或路由。" />;
  }
  return (
    <table className="table">
      <thead>
        <tr>
          <th>策略名称</th>
          <th>默认动作</th>
          <th>规则</th>
          <th>绑定数</th>
          <th>状态</th>
          <th>创建时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        {policies.map((policy) => (
          <tr key={policy.id}>
            <td><div className="table-primary">{policy.name}</div><div className="table-secondary">{policy.description || policy.id}</div></td>
            <td>{accessControlActionLabel(policy.defaultAction)}</td>
            <td>{policy.rules?.length ?? 0} 条</td>
            <td>{policyBindingCount(bindings, 'AccessControlPolicy', policy.id)}</td>
            <td><Badge tone={statusTone(policy.enabled ? 'published' : 'disabled')}>{policy.enabled ? '启用' : '停用'}</Badge></td>
            <td>{formatDateTime(policy.createdAt ?? '')}</td>
            <td><RowActions enabled={policy.enabled} onEdit={() => onEdit(policy)} onDelete={() => onDelete(policy)} onSetEnabled={(enabled) => onSetEnabled(policy, enabled)} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RateLimitTable({
  policies,
  bindings,
  onEdit,
  onDelete,
  onSetEnabled,
}: {
  policies: RateLimitPolicy[];
  bindings: PolicyBinding[];
  onEdit: (policy: RateLimitPolicy) => void;
  onDelete: (policy: RateLimitPolicy) => void;
  onSetEnabled: (policy: RateLimitPolicy, enabled: boolean) => void;
}) {
  if (policies.length === 0) {
    return <EmptyState title="暂无限流策略" message="创建本地或全局限流策略后再绑定到网关或路由。" />;
  }
  return (
    <table className="table">
      <thead>
        <tr>
          <th>策略名称</th>
          <th>模式</th>
          <th>规则</th>
          <th>失败策略</th>
          <th>绑定数</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        {policies.map((policy) => (
          <tr key={policy.id}>
            <td><div className="table-primary">{policy.name}</div><div className="table-secondary">{policy.description || policy.id}</div></td>
            <td>{rateLimitModeLabel(policy.mode)}</td>
            <td>{rateLimitRuleSummary(policy)}</td>
            <td>{rateLimitFailurePolicyLabel(policy.failurePolicy)}</td>
            <td>{policyBindingCount(bindings, 'RateLimitPolicy', policy.id)}</td>
            <td><Badge tone={statusTone(policy.enabled ? 'published' : 'disabled')}>{policy.enabled ? '启用' : '停用'}</Badge></td>
            <td><RowActions enabled={policy.enabled} onEdit={() => onEdit(policy)} onDelete={() => onDelete(policy)} onSetEnabled={(enabled) => onSetEnabled(policy, enabled)} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function PolicyBindingTable({
  bindings,
  workspace,
  onEdit,
  onDelete,
  onSetEnabled,
}: {
  bindings: PolicyBinding[];
  workspace: PolicyWorkspace;
  onEdit: (binding: PolicyBinding) => void;
  onDelete: (binding: PolicyBinding) => void;
  onSetEnabled: (binding: PolicyBinding, enabled: boolean) => void;
}) {
  if (bindings.length === 0) {
    return <EmptyState title="暂无策略绑定" message="策略本身不会直接生效，需要通过绑定投放到网关、路由或路由规则。" />;
  }
  return (
    <table className="table">
      <thead>
        <tr>
          <th>绑定名称</th>
          <th>目标</th>
          <th>策略</th>
          <th>状态</th>
          <th>创建时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        {bindings.map((binding) => (
          <tr key={binding.id}>
            <td><div className="table-primary">{binding.name}</div><div className="table-secondary">{binding.description || binding.id}</div></td>
            <td>{targetRefLabel(binding, workspace)}</td>
            <td>{binding.policies.map((policy) => policyRefLabel(policy, workspace)).join('、')}</td>
            <td><Badge tone={statusTone(binding.enabled ? 'published' : 'disabled')}>{binding.enabled ? '启用' : '停用'}</Badge></td>
            <td>{formatDateTime(binding.createdAt ?? '')}</td>
            <td><RowActions enabled={binding.enabled} onEdit={() => onEdit(binding)} onDelete={() => onDelete(binding)} onSetEnabled={(enabled) => onSetEnabled(binding, enabled)} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RowActions({ enabled, onEdit, onDelete, onSetEnabled }: { enabled: boolean; onEdit: () => void; onDelete: () => void; onSetEnabled: (enabled: boolean) => void }) {
  return (
    <div className="row-actions">
      <button className="link-button" type="button" onClick={onEdit}>编辑</button>
      <button className="link-button" type="button" onClick={() => onSetEnabled(!enabled)}>{enabled ? '停用' : '启用'}</button>
      <button className="link-button danger" type="button" onClick={onDelete}>删除</button>
    </div>
  );
}

function AccessControlEditor({
  draft,
  submitting,
  onChange,
  onCancel,
  onSubmit,
}: {
  draft: AccessControlPolicy;
  submitting: boolean;
  onChange: (draft: AccessControlPolicy) => void;
  onCancel: () => void;
  onSubmit: () => void;
}) {
  const rules = draft.rules ?? [];
  return (
    <Panel title="访问控制策略" subtitle="按 IP、Header、Consumer 或 Tenant 判定请求是否允许继续">
      <div className="editor-grid form-only">
        <div className="form-section">
          <div className="field-grid">
            <InputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
            <SelectField label="默认动作" value={draft.defaultAction ?? 'Allow'} options={accessActions.map((action) => ({ value: action, label: accessControlActionLabel(action) }))} onChange={(defaultAction) => onChange({ ...draft, defaultAction: defaultAction as AccessControlAction })} />
            <InputField label="描述" value={draft.description ?? ''} className="field-wide" onChange={(description) => onChange({ ...draft, description })} />
            <SwitchField label="启用策略" checked={draft.enabled} onChange={(enabled) => onChange({ ...draft, enabled })} />
            <InputField label="拒绝状态码" type="number" value={String(draft.response?.statusCode ?? 403)} onChange={(statusCode) => onChange({ ...draft, response: { ...draft.response, statusCode: Number(statusCode) } })} />
            <InputField label="拒绝响应" value={draft.response?.message ?? 'Access denied'} onChange={(message) => onChange({ ...draft, response: { ...draft.response, message } })} />
          </div>
        </div>
        <div className="form-section">
          <div className="form-section-title">
            <h3>规则</h3>
            <p>规则按顺序执行，条件全部命中后执行该规则动作</p>
          </div>
          <div className="policy-rule-list">
            {rules.map((rule, ruleIndex) => (
              <AccessControlRuleEditor
                key={`${rule.name}-${ruleIndex}`}
                rule={rule}
                onChange={(next) => onChange({ ...draft, rules: replaceAt(rules, ruleIndex, next) })}
                onDelete={() => onChange({ ...draft, rules: removeAt(rules, ruleIndex) })}
              />
            ))}
            <Button variant="soft" onClick={() => onChange({ ...draft, rules: [...rules, newAccessControlRule(rules.length + 1)] })}><Plus size={16} />添加规则</Button>
          </div>
        </div>
        <div className="form-actions">
          <Button variant="primary" onClick={onSubmit} disabled={submitting}>{submitting ? '保存中...' : '保存策略'}</Button>
          <Button variant="ghost" onClick={onCancel} disabled={submitting}>取消</Button>
        </div>
      </div>
    </Panel>
  );
}

function AccessControlRuleEditor({ rule, onChange, onDelete }: { rule: AccessControlRule; onChange: (rule: AccessControlRule) => void; onDelete: () => void }) {
  const conditions = rule.conditions ?? [];
  return (
    <div className="detail-card policy-rule-card">
      <div className="policy-rule-head">
        <strong>{rule.name || '未命名规则'}</strong>
        <button className="link-button danger" type="button" onClick={onDelete}><Trash2 size={14} />删除</button>
      </div>
      <div className="field-grid">
        <InputField label="规则名称" value={rule.name} onChange={(name) => onChange({ ...rule, name })} />
        <SelectField label="动作" value={rule.action} options={accessActions.map((action) => ({ value: action, label: accessControlActionLabel(action) }))} onChange={(action) => onChange({ ...rule, action: action as AccessControlAction })} />
      </div>
      <div className="policy-sub-list">
        {conditions.map((condition, index) => (
          <div key={`${condition.type}-${index}`} className="policy-inline-row">
            <select value={condition.type} onChange={(event) => onChange({ ...rule, conditions: replaceAt(conditions, index, { ...condition, type: event.target.value as AccessControlConditionType, name: event.target.value === 'Header' ? condition.name : '' }) })}>
              {conditionTypes.map((type) => <option key={type} value={type}>{conditionTypeLabel(type)}</option>)}
            </select>
            <input value={condition.name ?? ''} placeholder="名称" disabled={condition.type !== 'Header'} onChange={(event) => onChange({ ...rule, conditions: replaceAt(conditions, index, { ...condition, name: event.target.value }) })} />
            <input value={condition.value} placeholder="匹配值" onChange={(event) => onChange({ ...rule, conditions: replaceAt(conditions, index, { ...condition, value: event.target.value }) })} />
            <button className="link-button danger" type="button" onClick={() => onChange({ ...rule, conditions: removeAt(conditions, index) })}>删除</button>
          </div>
        ))}
        <Button variant="soft" onClick={() => onChange({ ...rule, conditions: [...conditions, newAccessControlCondition()] })}><Plus size={16} />添加条件</Button>
      </div>
    </div>
  );
}

function RateLimitEditor({
  draft,
  workspace,
  submitting,
  onChange,
  onCancel,
  onSubmit,
}: {
  draft: RateLimitPolicy;
  workspace: PolicyWorkspace;
  submitting: boolean;
  onChange: (draft: RateLimitPolicy) => void;
  onCancel: () => void;
  onSubmit: () => void;
}) {
  return (
    <Panel title="限流策略" subtitle="Local 模式在插件内计数，Global 模式通过 ingate-dataplane 使用 Redis">
      <div className="editor-grid form-only">
        <div className="form-section">
          <div className="field-grid">
            <InputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
            <SelectField label="限流模式" value={draft.mode} options={rateLimitModes.map((mode) => ({ value: mode, label: rateLimitModeLabel(mode) }))} onChange={(mode) => onChange(changeRateLimitMode(draft, mode as RateLimitMode, workspace))} />
            <InputField label="描述" value={draft.description ?? ''} className="field-wide" onChange={(description) => onChange({ ...draft, description })} />
            <SwitchField label="启用策略" checked={draft.enabled} onChange={(enabled) => onChange({ ...draft, enabled })} />
            <SelectField label="失败策略" value={draft.failurePolicy ?? 'FailOpen'} options={failurePolicies.map((policy) => ({ value: policy, label: rateLimitFailurePolicyLabel(policy) }))} onChange={(failurePolicy) => onChange({ ...draft, failurePolicy: failurePolicy as RateLimitFailurePolicy })} />
            <InputField label="超限状态码" type="number" value={String(draft.response?.statusCode ?? 429)} onChange={(statusCode) => onChange({ ...draft, response: { ...draft.response, statusCode: Number(statusCode) } })} />
            <InputField label="超限响应" value={draft.response?.message ?? 'Too many requests'} onChange={(message) => onChange({ ...draft, response: { ...draft.response, message } })} />
            <SwitchField label="返回配额 Header" checked={Boolean(draft.response?.quotaHeaderEnabled)} onChange={(quotaHeaderEnabled) => onChange({ ...draft, response: { ...draft.response, quotaHeaderEnabled } })} />
          </div>
        </div>
        {draft.mode === 'Global' ? (
          <div className="form-section">
            <div className="form-section-title">
              <h3>全局限流</h3>
              <p>Redis 配置由 RedisStore 统一管理，策略只引用资源 ID</p>
            </div>
            <div className="field-grid">
              <SelectField label="Redis 配置" value={draft.global?.redisRef ?? ''} options={workspace.redisStores.map((store) => ({ value: store.id, label: store.name }))} onChange={(redisRef) => onChange({ ...draft, global: { ...draft.global, redisRef } })} />
              <InputField label="Key 前缀" value={draft.global?.prefix ?? ''} onChange={(prefix) => onChange({ ...draft, global: { ...draft.global, redisRef: draft.global?.redisRef ?? '', prefix } })} />
              <InputField label="调用超时" type="number" value={String(draft.global?.timeoutMillis ?? 50)} onChange={(timeoutMillis) => onChange({ ...draft, global: { ...draft.global, redisRef: draft.global?.redisRef ?? '', timeoutMillis: Number(timeoutMillis) } })} />
            </div>
          </div>
        ) : null}
        <RateLimitRuleList rules={draft.rules} onChange={(rules) => onChange({ ...draft, rules })} />
        <div className="form-actions">
          <Button variant="primary" onClick={onSubmit} disabled={submitting}>{submitting ? '保存中...' : '保存策略'}</Button>
          <Button variant="ghost" onClick={onCancel} disabled={submitting}>取消</Button>
        </div>
      </div>
    </Panel>
  );
}

function RateLimitRuleList({ rules, onChange }: { rules: RateLimitRule[]; onChange: (rules: RateLimitRule[]) => void }) {
  return (
    <div className="form-section">
      <div className="form-section-title">
        <h3>规则</h3>
        <p>每条规则包含计数维度、额度窗口和算法</p>
      </div>
      <div className="policy-rule-list">
        {rules.map((rule, ruleIndex) => (
          <RateLimitRuleEditor
            key={`${rule.name}-${ruleIndex}`}
            rule={rule}
            onChange={(next) => onChange(replaceAt(rules, ruleIndex, next))}
            onDelete={() => onChange(removeAt(rules, ruleIndex))}
          />
        ))}
        <Button variant="soft" onClick={() => onChange([...rules, newRateLimitRule(rules.length + 1)])}><Plus size={16} />添加规则</Button>
      </div>
    </div>
  );
}

function RateLimitRuleEditor({ rule, onChange, onDelete }: { rule: RateLimitRule; onChange: (rule: RateLimitRule) => void; onDelete: () => void }) {
  const parts = rule.key.parts;
  return (
    <div className="detail-card policy-rule-card">
      <div className="policy-rule-head">
        <strong>{rule.name || '未命名规则'}</strong>
        <button className="link-button danger" type="button" onClick={onDelete}><Trash2 size={14} />删除</button>
      </div>
      <div className="field-grid">
        <InputField label="规则名称" value={rule.name} onChange={(name) => onChange({ ...rule, name })} />
        <SelectField label="算法" value={rule.algorithm ?? 'FixedWindow'} options={rateLimitAlgorithms.map((algorithm) => ({ value: algorithm, label: rateLimitAlgorithmLabel(algorithm) }))} onChange={(algorithm) => onChange({ ...rule, algorithm: algorithm as RateLimitAlgorithm })} />
        <InputField label="请求数" type="number" value={String(rule.limit.requests)} onChange={(requests) => onChange({ ...rule, limit: { ...rule.limit, requests: Number(requests) } })} />
        <InputField label="窗口秒数" type="number" value={String(rule.limit.windowSeconds)} onChange={(windowSeconds) => onChange({ ...rule, limit: { ...rule.limit, windowSeconds: Number(windowSeconds) } })} />
        <InputField label="Burst" type="number" value={String(rule.limit.burst ?? 0)} onChange={(burst) => onChange({ ...rule, limit: { ...rule.limit, burst: Number(burst) } })} />
      </div>
      <div className="policy-sub-list">
        {parts.map((part, index) => (
          <div key={`${part.type}-${index}`} className="policy-inline-row">
            <select value={part.type} onChange={(event) => onChange({ ...rule, key: { parts: replaceAt(parts, index, changeRateLimitKeyType(part, event.target.value as RateLimitKeyType)) } })}>
              {rateLimitKeyTypes.map((type) => <option key={type} value={type}>{rateLimitKeyTypeLabel(type)}</option>)}
            </select>
            <input value={part.name ?? ''} placeholder="Header / Query / Cookie 名称" disabled={!rateLimitKeyNeedsName(part.type)} onChange={(event) => onChange({ ...rule, key: { parts: replaceAt(parts, index, { ...part, name: event.target.value }) } })} />
            <button className="link-button danger" type="button" onClick={() => onChange({ ...rule, key: { parts: removeAt(parts, index) } })}>删除</button>
          </div>
        ))}
        <Button variant="soft" onClick={() => onChange({ ...rule, key: { parts: [...parts, { type: 'IP' }] } })}><Plus size={16} />添加计数维度</Button>
      </div>
    </div>
  );
}

function PolicyBindingEditor({
  draft,
  workspace,
  submitting,
  onChange,
  onCancel,
  onSubmit,
}: {
  draft: PolicyBinding;
  workspace: PolicyWorkspace;
  submitting: boolean;
  onChange: (draft: PolicyBinding) => void;
  onCancel: () => void;
  onSubmit: () => void;
}) {
  const targetOptions = draft.targetRef.kind === 'Gateway' ? workspace.gateways : workspace.routes;
  const selectedRoute = workspace.routes.find((route) => route.id === draft.targetRef.name);
  return (
    <Panel title="策略绑定" subtitle="绑定只表达策略投放目标，策略参数仍在强类型策略资源中维护">
      <div className="editor-grid form-only">
        <div className="form-section">
          <div className="field-grid">
            <InputField label="绑定名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
            <SwitchField label="启用绑定" checked={draft.enabled} onChange={(enabled) => onChange({ ...draft, enabled })} />
            <InputField label="描述" value={draft.description ?? ''} className="field-wide" onChange={(description) => onChange({ ...draft, description })} />
            <SelectField label="目标类型" value={draft.targetRef.kind} options={targetKinds.map((kind) => ({ value: kind, label: policyTargetKindLabel(kind) }))} onChange={(kind) => onChange(changeTargetKind(draft, kind as PolicyTargetKind, workspace))} />
            <SelectField label="目标资源" value={draft.targetRef.name} options={targetOptions.map((option) => ({ value: option.id, label: option.name }))} onChange={(name) => onChange({ ...draft, targetRef: { ...draft.targetRef, name, ruleName: '' } })} />
            {draft.targetRef.kind === 'Route' ? (
              <SelectField
                label="路由规则"
                value={draft.targetRef.ruleName ?? ''}
                options={[{ value: '', label: '全部规则' }, ...(selectedRoute?.rules ?? []).map((rule) => ({ value: rule, label: rule }))]}
                onChange={(ruleName) => onChange({ ...draft, targetRef: { ...draft.targetRef, ruleName } })}
              />
            ) : null}
          </div>
        </div>
        <div className="form-section">
          <div className="form-section-title">
            <h3>绑定策略</h3>
            <p>同一个绑定可以包含多种内置治理策略</p>
          </div>
          <div className="policy-sub-list">
            {draft.policies.map((policy, index) => (
              <div key={`${policy.kind}-${index}`} className="policy-inline-row">
                <select value={policy.kind} onChange={(event) => onChange({ ...draft, policies: replaceAt(draft.policies, index, defaultPolicyRef(event.target.value as PolicyKind, workspace)) })}>
                  {policyKinds.map((kind) => <option key={kind} value={kind}>{policyKindLabel(kind)}</option>)}
                </select>
                <select value={policy.name} onChange={(event) => onChange({ ...draft, policies: replaceAt(draft.policies, index, { ...policy, name: event.target.value }) })}>
                  {policyOptions(policy.kind, workspace).map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
                </select>
                <button className="link-button danger" type="button" onClick={() => onChange({ ...draft, policies: removeAt(draft.policies, index) })}>删除</button>
              </div>
            ))}
            <Button variant="soft" onClick={() => onChange({ ...draft, policies: [...draft.policies, defaultPolicyRef('AccessControlPolicy', workspace)] })}><Plus size={16} />添加策略</Button>
          </div>
        </div>
        <div className="form-actions">
          <Button variant="primary" onClick={onSubmit} disabled={submitting}>{submitting ? '保存中...' : '保存绑定'}</Button>
          <Button variant="ghost" onClick={onCancel} disabled={submitting}>取消</Button>
        </div>
      </div>
    </Panel>
  );
}

function InputField({ label, value, onChange, type = 'text', className = '' }: { label: string; value: string; type?: string; className?: string; onChange: (value: string) => void }) {
  return (
    <div className={`field ${className}`.trim()}>
      <label>{label}</label>
      <input type={type} value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function SelectField({ label, value, options, onChange }: { label: string; value: string; options: { value: string; label: string }[]; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
    </div>
  );
}

function SwitchField({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className={`gateway-status route-status-control ${checked ? 'on' : ''}`.trim()}>
      <button className="gateway-switch" type="button" onClick={() => onChange(!checked)}><span /></button>
      {label}
    </label>
  );
}

function newAccessControlPolicy(): AccessControlPolicy {
  return {
    id: '',
    name: '',
    description: '',
    enabled: true,
    defaultAction: 'Deny',
    rules: [newAccessControlRule(1)],
    response: { statusCode: 403, message: 'Access denied' },
  };
}

function newAccessControlRule(index: number): AccessControlRule {
  return {
    name: `rule-${index}`,
    action: 'Allow',
    conditions: [newAccessControlCondition()],
  };
}

function newAccessControlCondition(): AccessControlCondition {
  return { type: 'IP', value: '' };
}

function newRateLimitPolicy(workspace: PolicyWorkspace): RateLimitPolicy {
  return {
    id: '',
    name: '',
    description: '',
    enabled: true,
    mode: 'Local',
    rules: [newRateLimitRule(1)],
    response: { statusCode: 429, message: 'Too many requests', quotaHeaderEnabled: true },
    failurePolicy: 'FailOpen',
    global: workspace.redisStores[0] ? { redisRef: workspace.redisStores[0].id, timeoutMillis: 50 } : undefined,
  };
}

function newRateLimitRule(index: number): RateLimitRule {
  return {
    name: `rule-${index}`,
    key: { parts: [{ type: 'IP' }] },
    limit: { requests: 60, windowSeconds: 60, burst: 0 },
    algorithm: 'FixedWindow',
  };
}

function newPolicyBinding(workspace: PolicyWorkspace): PolicyBinding {
  return {
    id: '',
    name: '',
    description: '',
    enabled: true,
    targetRef: {
      kind: workspace.gateways.length > 0 ? 'Gateway' : 'Route',
      name: workspace.gateways[0]?.id ?? workspace.routes[0]?.id ?? '',
    },
    policies: [defaultPolicyRef('AccessControlPolicy', workspace)],
  };
}

function defaultPolicyRef(kind: PolicyKind, workspace: PolicyWorkspace): PolicyRef {
  const option = policyOptions(kind, workspace)[0];
  return { kind, name: option?.id ?? '' };
}

function policyOptions(kind: PolicyKind, workspace: PolicyWorkspace): PolicyOption[] {
  if (kind === 'AccessControlPolicy') {
    return workspace.accessControlPolicies.map((policy) => ({ id: policy.id, name: policy.name, kind }));
  }
  return workspace.rateLimitPolicies.map((policy) => ({ id: policy.id, name: policy.name, kind }));
}

function changeTargetKind(draft: PolicyBinding, kind: PolicyTargetKind, workspace: PolicyWorkspace): PolicyBinding {
  const options = kind === 'Gateway' ? workspace.gateways : workspace.routes;
  return {
    ...draft,
    targetRef: {
      kind,
      name: options[0]?.id ?? '',
      ruleName: '',
    },
  };
}

function changeRateLimitMode(draft: RateLimitPolicy, mode: RateLimitMode, workspace: PolicyWorkspace): RateLimitPolicy {
  if (mode === 'Global') {
    return {
      ...draft,
      mode,
      global: draft.global ?? { redisRef: workspace.redisStores[0]?.id ?? '', timeoutMillis: 50 },
    };
  }
  return { ...draft, mode, global: undefined };
}

function changeRateLimitKeyType(part: RateLimitKeyPart, type: RateLimitKeyType): RateLimitKeyPart {
  if (rateLimitKeyNeedsName(type)) {
    return { ...part, type };
  }
  return { type };
}

function rateLimitKeyNeedsName(type: RateLimitKeyType) {
  return ['Header', 'Query', 'Cookie', 'JWTClaim'].includes(type);
}

function validateAccessControlPolicy(policy: AccessControlPolicy) {
  if (!policy.name.trim()) {
    return '策略名称不能为空';
  }
  if ((policy.rules ?? []).length === 0 && policy.defaultAction !== 'Deny') {
    return '至少需要一条访问控制规则，或将默认动作设置为拒绝';
  }
  for (const rule of policy.rules ?? []) {
    if (!rule.name.trim()) {
      return '访问控制规则名称不能为空';
    }
    for (const condition of rule.conditions ?? []) {
      if (!condition.value.trim()) {
        return '访问控制条件值不能为空';
      }
      if (condition.type === 'Header' && !condition.name?.trim()) {
        return 'Header 条件必须填写名称';
      }
    }
  }
  if ((policy.response?.statusCode ?? 403) < 0) {
    return '拒绝响应状态码不能小于 0';
  }
  return '';
}

function validateRateLimitPolicy(policy: RateLimitPolicy) {
  if (!policy.name.trim()) {
    return '策略名称不能为空';
  }
  if (policy.mode === 'Global' && !policy.global?.redisRef) {
    return '全局限流必须选择 Redis 配置';
  }
  if (policy.rules.length === 0) {
    return '至少需要一条限流规则';
  }
  for (const rule of policy.rules) {
    if (!rule.name.trim()) {
      return '限流规则名称不能为空';
    }
    if (rule.limit.requests <= 0 || rule.limit.windowSeconds <= 0) {
      return '限流请求数和窗口必须大于 0';
    }
    if (rule.key.parts.length === 0) {
      return '限流规则必须配置计数维度';
    }
    for (const part of rule.key.parts) {
      if (rateLimitKeyNeedsName(part.type) && !part.name?.trim()) {
        return 'Header、Query、Cookie、JWT Claim 限流维度必须填写名称';
      }
    }
  }
  return '';
}

function validatePolicyBinding(binding: PolicyBinding) {
  if (!binding.name.trim()) {
    return '绑定名称不能为空';
  }
  if (!binding.targetRef.name) {
    return '绑定目标不能为空';
  }
  if (binding.policies.length === 0) {
    return '至少需要绑定一个策略';
  }
  if (binding.policies.some((policy) => !policy.name)) {
    return '绑定策略不能为空';
  }
  return '';
}

function enabledCount(items: { enabled: boolean }[]) {
  return items.filter((item) => item.enabled).length;
}

function policyBindingCount(bindings: PolicyBinding[], kind: PolicyKind, id: string) {
  return bindings.filter((binding) => binding.policies.some((policy) => policy.kind === kind && policy.name === id)).length;
}

function rateLimitRuleSummary(policy: RateLimitPolicy) {
  const first = policy.rules[0];
  if (!first) {
    return '0 条';
  }
  return `${policy.rules.length} 条 / ${first.limit.requests} 次/${first.limit.windowSeconds}s`;
}

function targetRefLabel(binding: PolicyBinding, workspace: PolicyWorkspace) {
  const options = binding.targetRef.kind === 'Gateway' ? workspace.gateways : workspace.routes;
  const targetName = options.find((option) => option.id === binding.targetRef.name)?.name ?? binding.targetRef.name;
  const suffix = binding.targetRef.ruleName ? ` / ${binding.targetRef.ruleName}` : '';
  return `${policyTargetKindLabel(binding.targetRef.kind)}：${targetName}${suffix}`;
}

function policyRefLabel(ref: PolicyRef, workspace: PolicyWorkspace) {
  const option = policyOptions(ref.kind, workspace).find((policy) => policy.id === ref.name);
  return `${policyKindLabel(ref.kind)}：${option?.name ?? ref.name}`;
}

function filterItems<T>(items: T[], query: string, values: (item: T) => string[]) {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return items;
  }
  return items.filter((item) => values(item).some((value) => value.toLowerCase().includes(normalizedQuery)));
}

function replaceAt<T>(items: T[], index: number, value: T) {
  return items.map((item, currentIndex) => currentIndex === index ? value : item);
}

function removeAt<T>(items: T[], index: number) {
  return items.filter((_, currentIndex) => currentIndex !== index);
}
