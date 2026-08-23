import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, Gauge, MessageSquare, Plus, ShieldCheck, WandSparkles } from 'lucide-react';
import {
  deleteHeaderTransformationPolicy,
  deleteIPRestrictionPolicy,
  deleteMockResponsePolicy,
  deleteTokenQuotaPolicy,
  getPolicyWorkspace,
  saveHeaderTransformationPolicy,
  saveIPRestrictionPolicy,
  saveMockResponsePolicy,
  saveTokenQuotaPolicy,
  setGovernancePolicyEnabled,
} from '@/api/policies';
import { useResource } from '@/api/useResource';
import {
  createHeaderTransformationPolicyDraft,
  HeaderTransformationPolicyEditor,
  headerTransformationPolicyPayload,
  validateHeaderTransformationPolicyDraft,
  type HeaderTransformationPolicyDraft,
} from './HeaderTransformationPolicyEditor';
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
  ResourcePagination,
  ResourceStatePanel,
  SearchField,
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, type ResourceState } from '@/domain/common';
import { standardPluginPackages } from '@/domain/plugin';
import type { GovernancePolicy, GovernancePolicyKind, PolicyTargetOption } from '@/domain/policy';
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
  createMockResponsePolicyDraft,
  MockResponsePolicyEditor,
  mockResponsePolicyPayload,
  validateMockResponsePolicyDraft,
  type MockResponsePolicyDraft,
} from './MockResponsePolicyEditor';
import {
  createTokenQuotaPolicyDraft,
  TokenQuotaPolicyEditor,
  tokenQuotaPolicyPayload,
  validateTokenQuotaPolicyDraft,
  type TokenQuotaPolicyDraft,
} from './TokenQuotaPolicyEditor';

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

type PolicyEditor =
  | { kind: 'IPRestrictionPolicy'; draft: IPRestrictionPolicyDraft }
  | { kind: 'TokenQuotaPolicy'; draft: TokenQuotaPolicyDraft }
  | { kind: 'HeaderTransformationPolicy'; draft: HeaderTransformationPolicyDraft }
  | { kind: 'MockResponsePolicy'; draft: MockResponsePolicyDraft };

export function PolicyPage() {
  const workspace = useResource(getPolicyWorkspace, {
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
      const result = await deletePolicy(deleteCandidate);
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
        {editor && (
          <div className="space-y-5">
            {editor.kind === 'IPRestrictionPolicy' ? (
              <IPRestrictionPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={{
                  ...validateIPRestrictionPolicyDraft(editor.draft),
                  errors: showValidation ? validateIPRestrictionPolicyDraft(editor.draft).errors : {},
                }}
                onChange={(draft) => setEditor({ kind: 'IPRestrictionPolicy', draft })}
              />
            ) : editor.kind === 'TokenQuotaPolicy' ? (
              <TokenQuotaPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={{
                  ...validateTokenQuotaPolicyDraft(editor.draft),
                  errors: showValidation ? validateTokenQuotaPolicyDraft(editor.draft).errors : {},
                }}
                onChange={(draft) => setEditor({ kind: 'TokenQuotaPolicy', draft })}
              />
            ) : editor.kind === 'HeaderTransformationPolicy' ? (
              <HeaderTransformationPolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={{
                  ...validateHeaderTransformationPolicyDraft(editor.draft),
                  errors: showValidation ? validateHeaderTransformationPolicyDraft(editor.draft).errors : {},
                }}
                onChange={(draft) => setEditor({ kind: 'HeaderTransformationPolicy', draft })}
              />
            ) : (
              <MockResponsePolicyEditor
                draft={editor.draft}
                targets={data.targets}
                validation={{
                  ...validateMockResponsePolicyDraft(editor.draft),
                  errors: showValidation ? validateMockResponsePolicyDraft(editor.draft).errors : {},
                }}
                onChange={(draft) => setEditor({ kind: 'MockResponsePolicy', draft })}
              />
            )}

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

function CreatePolicyMenu({
  transformerAvailable,
  mockResponseAvailable,
  onSelect,
}: {
  transformerAvailable: boolean;
  mockResponseAvailable: boolean;
  onSelect: (kind: GovernancePolicyKind) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;

    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  const select = (kind: GovernancePolicyKind) => {
    setOpen(false);
    onSelect(kind);
  };

  return (
    <div ref={rootRef} className="policy-create">
      <Button aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen((current) => !current)}>
        <Plus className="h-4 w-4" />创建策略<ChevronDown className={`h-4 w-4 policy-create-chevron${open ? ' is-open' : ''}`} />
      </Button>
      {open ? (
        <div className="policy-create-menu" role="menu" aria-label="选择策略类型">
          <div className="policy-create-menu-title">选择策略类型</div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">访问控制</div>
            <button type="button" role="menuitem" onClick={() => select('IPRestrictionPolicy')}>
              <span className="policy-create-icon"><ShieldCheck aria-hidden="true" /></span>
              <span><strong>IP 访问限制</strong><small>按来源地址限制网关或路由访问</small></span>
            </button>
          </div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">流量处理</div>
            <button type="button" role="menuitem" disabled={!transformerAvailable} onClick={() => select('HeaderTransformationPolicy')}>
              <span className="policy-create-icon"><WandSparkles aria-hidden="true" /></span>
              <span><strong>请求响应转换</strong><small>按路由修改请求与响应 Header</small></span>
            </button>
            {!transformerAvailable ? <Link className="policy-create-prerequisite" to="/plugins" onClick={() => setOpen(false)}>请先安装请求响应转换插件</Link> : null}
            <button type="button" role="menuitem" disabled={!mockResponseAvailable} onClick={() => select('MockResponsePolicy')}>
              <span className="policy-create-icon"><MessageSquare aria-hidden="true" /></span>
              <span><strong>模拟响应</strong><small>不访问服务，直接返回固定 HTTP 响应</small></span>
            </button>
            {!mockResponseAvailable ? <Link className="policy-create-prerequisite" to="/plugins" onClick={() => setOpen(false)}>请先安装模拟响应插件</Link> : null}
          </div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">AI 治理</div>
            <button type="button" role="menuitem" onClick={() => select('TokenQuotaPolicy')}>
              <span className="policy-create-icon"><Gauge aria-hidden="true" /></span>
              <span><strong>Token 额度</strong><small>按调用方限制模型 Token 用量</small></span>
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function createPolicyEditor(kind: GovernancePolicyKind): PolicyEditor {
  if (kind === 'IPRestrictionPolicy') return { kind, draft: createIPRestrictionPolicyDraft() };
  if (kind === 'TokenQuotaPolicy') return { kind, draft: createTokenQuotaPolicyDraft() };
  if (kind === 'HeaderTransformationPolicy') return { kind, draft: createHeaderTransformationPolicyDraft() };
  return { kind, draft: createMockResponsePolicyDraft() };
}

function editPolicyEditor(policy: GovernancePolicy): PolicyEditor {
  if (policy.kind === 'IPRestrictionPolicy') return { kind: policy.kind, draft: createIPRestrictionPolicyDraft(policy.raw) };
  if (policy.kind === 'TokenQuotaPolicy') return { kind: policy.kind, draft: createTokenQuotaPolicyDraft(policy.raw) };
  if (policy.kind === 'HeaderTransformationPolicy') return { kind: policy.kind, draft: createHeaderTransformationPolicyDraft(policy.raw) };
  return { kind: policy.kind, draft: createMockResponsePolicyDraft(policy.raw) };
}

function validatePolicyEditor(editor: PolicyEditor) {
  if (editor.kind === 'IPRestrictionPolicy') return validateIPRestrictionPolicyDraft(editor.draft);
  if (editor.kind === 'TokenQuotaPolicy') return validateTokenQuotaPolicyDraft(editor.draft);
  if (editor.kind === 'HeaderTransformationPolicy') return validateHeaderTransformationPolicyDraft(editor.draft);
  return validateMockResponsePolicyDraft(editor.draft);
}

function savePolicyEditor(editor: PolicyEditor) {
  if (editor.kind === 'IPRestrictionPolicy') return saveIPRestrictionPolicy(ipRestrictionPolicyPayload(editor.draft));
  if (editor.kind === 'TokenQuotaPolicy') return saveTokenQuotaPolicy(tokenQuotaPolicyPayload(editor.draft));
  if (editor.kind === 'HeaderTransformationPolicy') return saveHeaderTransformationPolicy(headerTransformationPolicyPayload(editor.draft));
  return saveMockResponsePolicy(mockResponsePolicyPayload(editor.draft));
}

function deletePolicy(policy: GovernancePolicy) {
  const version = Number(policy.version);
  if (policy.kind === 'IPRestrictionPolicy') return deleteIPRestrictionPolicy(policy.id, version);
  if (policy.kind === 'TokenQuotaPolicy') return deleteTokenQuotaPolicy(policy.id, version);
  if (policy.kind === 'HeaderTransformationPolicy') return deleteHeaderTransformationPolicy(policy.id, version);
  return deleteMockResponsePolicy(policy.id, version);
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
          <div><span>生效状态</span><strong>{governancePolicyStatusLabel(policy)}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(policy.createdAt ?? '')}</strong></div>
          <PolicyRuleDetails policy={policy} />
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>应用目标</h3>
        {policy.targets.length > 0 ? <div className="resource-detail-list">
          {policy.targets.map((target) => <article key={`${target.kind}:${target.id}`}><div><strong>{policyTargetLabel(target, targets)}</strong><small>{policyTargetKindLabel(target.kind)} · {target.status?.message || '等待系统反馈执行状态'}</small></div><Badge tone={target.status ? policyStatusTone(target.status) : 'neutral'}>{target.status ? target.status.state === 'Ready' ? policy.kind === 'TokenQuotaPolicy' ? '已启用' : '已生效' : target.status.state === 'Error' ? '生效失败' : '待生效' : '未知'}</Badge></article>)}
        </div> : <EmptyState title="尚未应用" message="策略已保存，但当前不影响任何流量" />}
      </section>
    </div>
  );
}

function PolicyRuleDetails({ policy }: { policy: GovernancePolicy }) {
  if (policy.kind === 'IPRestrictionPolicy') {
    return <><div><span>允许地址</span><strong>{policy.raw.allow.join('、') || '未配置'}</strong></div><div><span>拒绝地址</span><strong>{policy.raw.deny.join('、') || '未配置'}</strong></div></>;
  }
  if (policy.kind === 'TokenQuotaPolicy') {
    return <><div><span>周期时区</span><strong>{policy.raw.timeZone}</strong></div><div><span>额度上限</span><strong>{policy.summary}</strong></div></>;
  }
  if (policy.kind === 'HeaderTransformationPolicy') {
    return <><div><span>请求规则</span><strong>{policy.raw.requestRules.length} 条</strong></div><div><span>响应规则</span><strong>{policy.raw.responseRules.length} 条</strong></div></>;
  }
  return <><div><span>HTTP 状态码</span><strong>{policy.raw.statusCode}</strong></div><div><span>内容类型</span><strong>{policy.raw.contentType}</strong></div><div><span>响应 Header</span><strong>{policy.raw.headers.length} 个</strong></div><div className="resource-detail-grid-wide"><span>响应正文</span><pre className="mock-response-detail-body">{policy.raw.body || '空正文'}</pre></div></>;
}
