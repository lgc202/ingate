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
  ResourceStatePanel,
  SearchField,
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, type ResourceState } from '@/domain/common';
import type { GovernancePolicy, GovernancePolicyKind, PolicyMutationResult, PolicyTargetOption } from '@/domain/policy';
import { governancePolicyStatusLabel, policyKindLabel, policyStatusTone, policyTargetKindLabel, policyTargetLabel } from '@/domain/policy';
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

type PolicyKindFilter = 'all' | GovernancePolicyKind;
type PolicyEnabledFilter = 'all' | 'enabled' | 'disabled';
type PolicyStateFilter = 'all' | Exclude<ResourceState, 'Disabled'> | 'Unapplied';

interface PolicyFilters {
  query: string;
  kind: PolicyKindFilter;
  enabled: PolicyEnabledFilter;
  state: PolicyStateFilter;
}

const emptyPolicyFilters = (): PolicyFilters => ({ query: '', kind: 'all', enabled: 'all', state: 'all' });

export function PolicyPage() {
  const workspace = useResource(getPolicyWorkspace);
  const [filterDraft, setFilterDraft] = useState<PolicyFilters>(emptyPolicyFilters);
  const [filters, setFilters] = useState<PolicyFilters>(emptyPolicyFilters);
  const [detail, setDetail] = useState<GovernancePolicy | null>(null);
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
  const normalizedQuery = filters.query.trim().toLowerCase();
  const visiblePolicies = allPolicies.filter((policy) => (
    (filters.kind === 'all' || policy.kind === filters.kind)
    && (filters.enabled === 'all' || (filters.enabled === 'enabled' && policy.enabled) || (filters.enabled === 'disabled' && !policy.enabled))
    && policyMatchesState(policy, filters.state)
    && `${policy.name} ${policy.summary} ${policy.targets.map((target) => policyTargetLabel(target, data.targets)).join(' ')}`.toLowerCase().includes(normalizedQuery)
  ));

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
      actions={(
        <div className="flex items-center gap-2">
          <Button onClick={() => setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft() })}>+ 限流策略</Button>
          <Button variant="secondary" onClick={() => setEditor({ type: 'ipRestriction', draft: createIPRestrictionPolicyDraft() })}>+ IP 访问限制</Button>
        </div>
      )}
    >
      <div className="space-y-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
        <Panel>
          <ResourceListFilters
            summary={policyFilterSummary(filters)}
            resultLabel={`${visiblePolicies.length} 条策略`}
            onSearch={() => setFilters({ ...filterDraft })}
            onReset={() => {
              const next = emptyPolicyFilters();
              setFilterDraft(next);
              setFilters(next);
            }}
          >
            <ResourceFilterField label="关键词">
              <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索策略或应用目标" />
            </ResourceFilterField>
            <ResourceFilterField label="策略类型">
              <select className="select" value={filterDraft.kind} onChange={(event) => setFilterDraft((current) => ({ ...current, kind: event.target.value as PolicyKindFilter }))}>
                <option value="all">全部类型</option>
                <option value="RateLimitPolicy">限流</option>
                <option value="IPRestrictionPolicy">IP 访问限制</option>
              </select>
            </ResourceFilterField>
            <ResourceFilterField label="启用状态">
              <select className="select" value={filterDraft.enabled} onChange={(event) => setFilterDraft((current) => ({ ...current, enabled: event.target.value as PolicyEnabledFilter }))}>
                <option value="all">全部启用状态</option>
                <option value="enabled">已启用</option>
                <option value="disabled">已停用</option>
              </select>
            </ResourceFilterField>
            <ResourceFilterField label="生效状态">
              <select className="select" value={filterDraft.state} onChange={(event) => setFilterDraft((current) => ({ ...current, state: event.target.value as PolicyStateFilter }))}>
                <option value="all">全部生效状态</option>
                <option value="Ready">已生效</option>
                <option value="Pending">待生效</option>
                <option value="Error">异常</option>
                <option value="Unapplied">未应用</option>
              </select>
            </ResourceFilterField>
          </ResourceListFilters>
          <PolicyLibraryTable
            policies={visiblePolicies}
            targets={data.targets}
            onDetail={setDetail}
            onEdit={(policy) => {
              if (policy.kind === 'RateLimitPolicy') {
                setEditor({ type: 'rateLimit', draft: createRateLimitPolicyDraft(policy.raw) });
              } else {
                setEditor({ type: 'ipRestriction', draft: createIPRestrictionPolicyDraft(policy.raw) });
              }
            }}
            onToggle={togglePolicyStatus}
            onDelete={setDeleteCandidate}
          />
        </Panel>
      </div>

      <Drawer title="策略详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail ? <PolicyDetail policy={detail} targets={data.targets} /> : null}
      </Drawer>

      <Drawer
        title={editor ? `${editor.draft.id ? '编辑' : '创建'}${editorTypeTitle(editor.type)}` : ''}
        subtitle="策略可以先保存，选择应用目标后才会影响流量"
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
            确定要删除策略 <strong className="text-slate-900">{deleteCandidate?.name}</strong> 吗？
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

function policyMatchesState(policy: GovernancePolicy, state: PolicyStateFilter): boolean {
  if (state === 'all') return true;
  if (state === 'Unapplied') return policy.enabled && policy.targets.length === 0;
  return policy.enabled && policy.targets.length > 0 && policy.status.state === state;
}

function policyFilterSummary(filters: PolicyFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.kind !== 'all') conditions.push(`类型：${policyKindLabel(filters.kind)}`);
  if (filters.enabled !== 'all') conditions.push(`启用状态：${filters.enabled === 'enabled' ? '已启用' : '已停用'}`);
  if (filters.state !== 'all') {
    conditions.push(`生效状态：${filters.state === 'Unapplied' ? '未应用' : resourceStateLabel(filters.state)}`);
  }
  return conditions.join(' · ') || '全部策略';
}

function PolicyDetail({ policy, targets }: { policy: GovernancePolicy; targets: PolicyTargetOption[] }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{policy.name}</h3></div>
        <Badge tone={policyStatusTone(policy.status)}>{governancePolicyStatusLabel(policy)}</Badge>
      </section>
      <section className="resource-detail-section">
        <h3>策略规则</h3>
        <div className="resource-detail-grid">
          <div><span>策略类型</span><strong>{policyKindLabel(policy.kind)}</strong></div>
          <div><span>规则摘要</span><strong>{policy.summary}</strong></div>
          <div><span>启用状态</span><strong>{policy.enabled ? '已启用' : '已停用'}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(policy.createdAt ?? '')}</strong></div>
          {policy.kind === 'RateLimitPolicy' ? <>
            <div><span>计数对象</span><strong>{rateLimitSubjectLabel(policy.raw.subject.type, policy.raw.subject.headerName)}</strong></div>
            <div><span>请求上限</span><strong>{policy.raw.limit.windowSeconds} 秒内 {policy.raw.limit.requests} 次</strong></div>
          </> : <>
            <div><span>允许地址</span><strong>{policy.raw.allow.join('、') || '未配置'}</strong></div>
            <div><span>拒绝地址</span><strong>{policy.raw.deny.join('、') || '未配置'}</strong></div>
          </>}
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>应用目标</h3>
        {policy.targets.length > 0 ? <div className="resource-detail-list">
          {policy.targets.map((target) => <article key={`${target.kind}:${target.id}`}><div><strong>{policyTargetLabel(target, targets)}</strong><small>{policyTargetKindLabel(target.kind)} · {target.status?.message || '等待系统反馈执行状态'}</small></div><Badge tone={target.status ? policyStatusTone(target.status) : 'neutral'}>{target.status ? target.status.state === 'Ready' ? '已生效' : target.status.state === 'Error' ? '异常' : '待生效' : '未知'}</Badge></article>)}
        </div> : <EmptyState title="尚未应用" message="策略已保存，但当前不影响任何流量" />}
      </section>
    </div>
  );
}

function rateLimitSubjectLabel(type: 'Shared' | 'IP' | 'Header', headerName?: string): string {
  if (type === 'IP') return '客户端 IP';
  if (type === 'Header') return `请求头 ${headerName || '—'}`;
  return '所有请求共享';
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
