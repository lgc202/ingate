import { useState } from 'react';
import {
  deleteGovernancePolicy,
  getPolicyEditorOptions,
  getPolicyListWorkspace,
  setGovernancePolicyEnabled,
} from '@/api/policies';
import { useResource } from '@/api/useResource';
import {
  Drawer,
  Modal,
  PageFrame,
  Panel,
  ResourceFilterField,
  ResourceListFilters,
  ResourcePagination,
  ResourceStatePanel,
  SearchField,
  Toast,
} from '@/components/ui';
import { resourceStateLabel, type ResourceState } from '@/domain/common';
import { standardPluginPackages } from '@/domain/plugin';
import type { GovernancePolicy, GovernancePolicyKind } from '@/domain/policy';
import { policyKindLabel, policyTargetLabel } from '@/domain/policy';
import { CreatePolicyMenu } from './CreatePolicyMenu';
import { PolicyDetail } from './PolicyDetail';
import {
  createPolicyEditor,
  editPolicyEditor,
  PolicyEditorForm,
  savePolicyEditor,
  validatePolicyEditor,
  type PolicyEditor,
} from './PolicyEditorForm';
import { PolicyLibraryTable } from './PolicyLibraryTable';

type PolicyEnabledFilter = 'all' | 'enabled' | 'disabled';
type PolicyStateFilter = 'all' | Exclude<ResourceState, 'Disabled'> | 'Unapplied';
type PolicyKindFilter = 'all' | GovernancePolicyKind;

interface PolicyFilters {
  query: string;
  kind: PolicyKindFilter;
  enabled: PolicyEnabledFilter;
  state: PolicyStateFilter;
}

const emptyPolicyFilters = (): PolicyFilters => ({ query: '', kind: 'all', enabled: 'all', state: 'all' });

export function PolicyPage() {
  const workspace = useResource(getPolicyListWorkspace, {
    autoRefreshWhen: (data) => data.policies.some((policy) => policy.enabled && policy.status.state === 'Pending'),
  });
  const [filterDraft, setFilterDraft] = useState<PolicyFilters>(emptyPolicyFilters);
  const [filters, setFilters] = useState<PolicyFilters>(emptyPolicyFilters);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [detail, setDetail] = useState<GovernancePolicy | null>(null);
  const [editor, setEditor] = useState<PolicyEditor | null>(null);
  const [showValidation, setShowValidation] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<GovernancePolicy | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);
  const editorOptions = useResource(getPolicyEditorOptions, { enabled: Boolean(editor) });

  if (workspace.loading && !workspace.data) {
    return (
      <PageFrame title="策略">
        <ResourceStatePanel title="正在加载策略..." message="从管理 API 获取策略列表与关联目标" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="策略">
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
  const pageCount = Math.max(1, Math.ceil(visiblePolicies.length / pageSize));
  const currentPage = Math.min(page, pageCount);
  const pagedPolicies = visiblePolicies.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  const reloadAfterMutation = async (resultMessage: string) => {
    await workspace.reload();
    setNotice({ message: resultMessage, tone: 'success' });
    setEditor(null);
    setShowValidation(false);
  };

  const saveEditor = async () => {
    if (!editor || submitting) return;
    const validation = validatePolicyEditor(editor);
    if (!validation.valid) {
      setShowValidation(true);
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
    if (!deleteCandidate || deleting) return;
    setDeleting(true);
    try {
      const result = await deleteGovernancePolicy(deleteCandidate);
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
      title="策略"
      actions={<CreatePolicyMenu
        transformerAvailable={data.installedPluginPackages.includes(standardPluginPackages.transformer)}
        mockResponseAvailable={data.installedPluginPackages.includes(standardPluginPackages.mockResponse)}
        onSelect={(kind) => {
        setShowValidation(false);
        setEditor(createPolicyEditor(kind));
        }}
      />}
    >
      <div className="space-y-4">
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
        <Panel>
          <ResourceListFilters
            summary={policyFilterSummary(filters)}
            resultLabel={`${visiblePolicies.length} 条策略`}
            onSearch={() => { setPage(1); setFilters({ ...filterDraft }); }}
            onReset={() => {
              const next = emptyPolicyFilters();
              setFilterDraft(next);
              setFilters(next);
              setPage(1);
            }}
          >
            <ResourceFilterField label="关键词">
              <SearchField value={filterDraft.query} onChange={(query) => setFilterDraft((current) => ({ ...current, query }))} placeholder="搜索策略或应用目标" />
            </ResourceFilterField>
            <ResourceFilterField label="策略类型">
              <select className="select" value={filterDraft.kind} onChange={(event) => setFilterDraft((current) => ({ ...current, kind: event.target.value as PolicyKindFilter }))}>
                <option value="all">全部策略类型</option>
                <option value="IPRestrictionPolicy">IP 访问限制</option>
                <option value="RateLimitPolicy">请求限流</option>
                <option value="TokenQuotaPolicy">Token 额度</option>
                <option value="HeaderTransformationPolicy">请求响应转换</option>
                <option value="MockResponsePolicy">模拟响应</option>
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
                <option value="Error">生效失败</option>
                <option value="Unapplied">未应用</option>
              </select>
            </ResourceFilterField>
          </ResourceListFilters>
          <PolicyLibraryTable
            policies={pagedPolicies}
            targets={data.targets}
            onDetail={setDetail}
            onEdit={(policy) => {
              setShowValidation(false);
              setEditor(editPolicyEditor(policy));
            }}
            onToggle={togglePolicyStatus}
            onDelete={setDeleteCandidate}
          />
          {visiblePolicies.length > 0 ? <ResourcePagination page={currentPage} pageSize={pageSize} total={visiblePolicies.length} onPageChange={setPage} onPageSizeChange={(size) => { setPage(1); setPageSize(size); }} /> : null}
        </Panel>
      </div>

      <Drawer title="策略详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail ? <PolicyDetail policy={detail} targets={data.targets} /> : null}
      </Drawer>

      <Drawer
        title={editor ? `${editor.draft.id ? '编辑' : '创建'} ${policyKindLabel(editor.kind)}` : ''}
        subtitle="策略可以先保存，选择应用目标后才会影响流量"
        isOpen={Boolean(editor)}
        onClose={() => { setEditor(null); setShowValidation(false); }}
      >
        {editor && editorOptions.loading && !editorOptions.data ? (
          <ResourceStatePanel title="正在加载应用目标" message="正在读取可应用此策略的资源" />
        ) : editor && editorOptions.error ? (
          <ResourceStatePanel title="应用目标加载失败" message={editorOptions.error.message} />
        ) : editor && (
          <div className="space-y-5">
            <PolicyEditorForm
              editor={editor}
              targets={editorOptions.data ?? []}
              showValidation={showValidation}
              onChange={setEditor}
            />

            <div className="pt-4 border-t border-slate-200 flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => { setEditor(null); setShowValidation(false); }}
                className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors cursor-pointer"
              >
                取消
              </button>
              <button
                type="button"
                disabled={submitting}
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
        title="确认删除策略"
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
  if (filters.kind !== 'all') conditions.push(`策略类型：${policyKindLabel(filters.kind)}`);
  if (filters.enabled !== 'all') conditions.push(`启用状态：${filters.enabled === 'enabled' ? '已启用' : '已停用'}`);
  if (filters.state !== 'all') {
    conditions.push(`生效状态：${filters.state === 'Unapplied' ? '未应用' : resourceStateLabel(filters.state)}`);
  }
  return conditions.join(' · ') || '全部策略';
}
