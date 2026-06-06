import { useEffect, useRef, useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Tabs, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { KeyValue } from '@/domain/common';
import type { HeaderMatch, RouteComposerPreview, RouteGatewayOption, RoutePageView, RoutePolicyCapability, RouteResource, RouteTargetOption, RouteTargetPayload, RouteValidationReport } from '@/domain/route';
import {
  routePolicyCapabilityRequestHeaderModifier,
  routePolicyCapabilityResponseHeaderModifier,
  routePolicyCapabilityRetry,
  routePolicyCapabilityTimeout,
} from '@/domain/route';
import { serviceTypeLabel } from '@/domain/service';
import type { RouteComposerDraft } from './composer';
import {
  buildRoutePublishPayload,
  createRouteComposerDraft,
  formatTargetServices,
  normalizeHostnames,
  parseHostnames,
  targetWeightSum,
  validateRouteComposerDraft,
} from './composer';

const detailTabs = [
  { key: 'overview', label: '概览' },
  { key: 'match', label: '匹配规则' },
  { key: 'target', label: '目标服务' },
  { key: 'events', label: '事件' },
];

const loadRouteWorkspace = () => consoleRepository.getRouteWorkspace();
type RouteEnabledFilter = 'all' | 'enabled' | 'disabled';

interface RouteFilters {
  keyword: string;
  gatewayName: string;
  serviceName: string;
  enabled: RouteEnabledFilter;
}

const emptyRouteFilters: RouteFilters = {
  keyword: '',
  gatewayName: 'all',
  serviceName: 'all',
  enabled: 'all',
};

const httpMethods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const;
const defaultRouteTimeoutMillis = 30000;

export function RoutePage() {
  const [mode, setMode] = useState<'list' | 'detail' | 'composer'>('list');
  const [tab, setTab] = useState('overview');
  const [selectedRouteId, setSelectedRouteId] = useState('');
  const [filterDraft, setFilterDraft] = useState<RouteFilters>(emptyRouteFilters);
  const [filters, setFilters] = useState<RouteFilters>(emptyRouteFilters);
  const [draftState, setDraftState] = useState<RouteComposerDraft | null>(null);
  const [serverValidation, setServerValidation] = useState<RouteValidationReport | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<RouteResource | null>(null);
  const [disableCandidate, setDisableCandidate] = useState<RouteResource | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [toggling, setToggling] = useState(false);
  const workspace = useResource(loadRouteWorkspace);

  if (workspace.loading) {
    return (
      <PageFrame title="流量 / 路由" subtitle="管理请求匹配、目标服务和策略配置">
        <ResourceStatePanel title="加载路由数据" message="正在读取路由列表和详情。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="流量 / 路由" subtitle="管理请求匹配、目标服务和策略配置">
        <ResourceStatePanel title="路由数据加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const routeWorkspace = workspace.data;
  const availableRoutes = routeWorkspace.routes;
  const selectedRoute = availableRoutes.find((route) => route.id === selectedRouteId) ?? availableRoutes[0];
  const routeEnabled = (route: RouteResource) => route.enabled;
  const selectedRouteView = selectedRoute ? { ...selectedRoute, enabled: routeEnabled(selectedRoute) } : undefined;
  const gatewayOptions = routeWorkspace.composer.gateways;
  const serviceOptions = Array.from(new Set([
    ...routeWorkspace.composer.targets.map((target) => target.id),
    ...availableRoutes.flatMap((route) => routeTargetIDs(route)),
  ])).sort();
  const visibleRoutes = availableRoutes.filter((route) => {
    const keyword = filters.keyword.trim().toLowerCase();
    const rule = primaryRouteRule(route);
    const matchedKeyword = !keyword || [
      route.name,
      rule?.pathPrefix ?? '',
      ...routeTargetIDs(route),
      ...routeTargetLabels(route, routeWorkspace.composer.targets),
      ...route.gatewayIDs,
      ...routeGatewayLabels(route.gatewayIDs, gatewayOptions),
      ...route.hostnames,
    ].some((value) => value.toLowerCase().includes(keyword));
    const matchedGateway = filters.gatewayName === 'all' || route.gatewayIDs.includes(filters.gatewayName);
    const matchedService = filters.serviceName === 'all' || routeTargetIDs(route).includes(filters.serviceName);
    const matchedEnabled = filters.enabled === 'all' || (filters.enabled === 'enabled' ? routeEnabled(route) : !routeEnabled(route));

    return matchedKeyword && matchedGateway && matchedService && matchedEnabled;
  });
  const hasActiveFilters = Boolean(filters.keyword.trim() || filters.gatewayName !== 'all' || filters.serviceName !== 'all' || filters.enabled !== 'all');
  const draft = draftState ?? createRouteComposerDraft(routeWorkspace.composer);
  const validation = validateRouteComposerDraft(draft);
  const publishPayload = buildRoutePublishPayload(draft);
  const activeValidation = serverValidation ?? validation;

  const handleDraftChange = (nextDraft: RouteComposerDraft) => {
    setDraftState(nextDraft);
    setServerValidation(null);
  };

  const updateFilterDraft = (patch: Partial<RouteFilters>) => {
    setFilterDraft((current) => ({ ...current, ...patch }));
  };

  const resetFilters = () => {
    setFilterDraft(emptyRouteFilters);
    setFilters(emptyRouteFilters);
  };

  const openCreate = () => {
    setMode('composer');
    setDraftState(createRouteComposerDraft(routeWorkspace.composer));
    setServerValidation(null);
    setNotice(null);
    setSubmitting(false);
  };

  const openEdit = (route: RouteResource) => {
    const rule = primaryRouteRule(route);
    const targetServices = routeTargetServices(route);

    setSelectedRouteId(route.id);
    setMode('composer');
    setDraftState({
      ...createRouteComposerDraft(routeWorkspace.composer),
      id: route.id,
      version: route.version,
      name: route.name,
      ruleName: rule?.name ?? 'main',
      methods: rule?.methods ?? [],
      path: rule?.pathPrefix ?? '/',
      gatewayIDs: route.gatewayIDs,
      hostnames: route.hostnames,
      headers: rule?.headers ?? [],
      targetServices,
      enabled: route.enabled,
      enabledPolicyCapabilities: enabledCapabilitiesFromRule(rule),
      policySettings: policySettingsFromRule(rule),
    });
    setServerValidation(null);
    setNotice(null);
    setSubmitting(false);
  };

  const deleteRoute = (route: RouteResource) => {
    setDeleteCandidate(route);
  };

  const confirmDeleteRoute = async () => {
    if (!deleteCandidate) {
      return;
    }

    setDeleting(true);
    try {
      await consoleRepository.deleteRoute(deleteCandidate.id);
      await workspace.reload();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '删除路由失败');
      setDeleteCandidate(null);
      setDeleting(false);
      return;
    }

    setSelectedRouteId((current) => {
      if (current !== deleteCandidate.id) {
        return current;
      }

      return availableRoutes.find((route) => route.id !== deleteCandidate.id)?.id ?? '';
    });
    setNotice(`已删除路由：${deleteCandidate.name}`);
    setDeleteCandidate(null);
    setDeleting(false);
  };

  const toggleRouteEnabled = async (route: RouteResource) => {
    if (routeEnabled(route)) {
      setDisableCandidate(route);
      return;
    }

    setToggling(true);
    try {
      await consoleRepository.setRouteEnabled(route.id, true);
      await workspace.reload();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '启用路由失败');
      setToggling(false);
      return;
    }

    setNotice(`已启用路由：${route.name}`);
    setToggling(false);
  };

  const confirmDisableRoute = async () => {
    if (!disableCandidate) {
      return;
    }

    setToggling(true);
    try {
      await consoleRepository.setRouteEnabled(disableCandidate.id, false);
      await workspace.reload();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '停用路由失败');
      setDisableCandidate(null);
      setToggling(false);
      return;
    }

    setNotice(`已停用路由：${disableCandidate.name}`);
    setDisableCandidate(null);
    setToggling(false);
  };

  const saveRoute = async () => {
    const policyValidationMessage = findPolicyValidationMessage(routeWorkspace.composer, draft);
    if (policyValidationMessage) {
      setNotice(policyValidationMessage);
      return;
    }

    const validationResult = await consoleRepository.validateRouteDraft(publishPayload);
    setServerValidation(validationResult);

    if (!validationResult.valid) {
      return;
    }

    setSubmitting(true);
    try {
      const result = await consoleRepository.saveRouteDraft(publishPayload);
      await workspace.reload();
      setSelectedRouteId(result.changeId ?? publishPayload.id ?? '');
      setNotice(result.message);
      setMode('list');
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '保存路由失败');
    } finally {
      setSubmitting(false);
    }
  };

  if (mode === 'detail') {
    return (
      <PageFrame
        title="路由详情"
        subtitle={selectedRouteView ? selectedRouteView.name : '未选择路由'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedRouteView ? (
            <>
              <Tabs tabs={detailTabs} active={tab} onChange={setTab} />
              <div style={{ height: 12 }} />
              {renderRouteDetail(tab, selectedRouteView, routeWorkspace.composer.gateways, routeWorkspace.composer.targets)}
            </>
          ) : null}
        </Panel>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="流量 / 路由"
      subtitle="管理请求匹配、目标服务和策略配置"
      actions={
        mode === 'list'
          ? (
            <Button variant="primary" onClick={openCreate}>新建路由</Button>
          )
          : <Button variant="soft" disabled={submitting} onClick={() => setMode('list')}>返回列表</Button>
      }
    >
      {mode === 'composer' ? (
        <section className="route-workbench">
          <div className="route-workbench-top">
            <div>
              <h2>{draft.id ? '编辑路由' : '创建路由'}</h2>
              <p>配置请求匹配条件、目标服务和路由级策略参数；保存后系统自动生效。</p>
            </div>
            <div className="route-workbench-meta">
              <Badge tone={draft.enabled ? 'green' : 'neutral'}>{draft.enabled ? '启用' : '停用'}</Badge>
              <span>{draft.enabledPolicyCapabilities.length > 0 ? `${draft.enabledPolicyCapabilities.length} 个策略` : '未绑定策略'}</span>
            </div>
          </div>
          <RouteMatchHeader draft={draft} gateways={routeWorkspace.composer.gateways} targets={routeWorkspace.composer.targets} />
          <div className="route-workbench-grid">
            <RouteComposer
              composer={routeWorkspace.composer}
              draft={draft}
              validation={activeValidation}
              gatewayOptions={gatewayOptions}
              onDraftChange={handleDraftChange}
            />
          </div>
          <div className="route-workbench-actions">
            <Button variant="ghost" disabled={submitting} onClick={() => setMode('list')}>取消</Button>
            <div className="toolbar">
              <Button variant="primary" disabled={!activeValidation.valid || submitting} onClick={saveRoute}>{submitting ? '保存中...' : '保存路由'}</Button>
            </div>
          </div>
        </section>
      ) : (
        <>
          <Panel
            title="路由列表"
          >
            <div className="gateway-query">
              <div className="gateway-query-grid">
                <label className="query-control">
                  <span>路径 / Host</span>
                  <input value={filterDraft.keyword} placeholder="请输入路径、Host、服务或网关" onChange={(event) => updateFilterDraft({ keyword: event.target.value })} />
                </label>
                <label className="query-control">
                  <span>所属网关</span>
                  <select value={filterDraft.gatewayName} onChange={(event) => updateFilterDraft({ gatewayName: event.target.value })}>
                    <option value="all">全部网关</option>
                    {gatewayOptions.map((gateway) => (
                      <option key={gateway.id} value={gateway.id}>{gateway.name}</option>
                    ))}
                  </select>
                </label>
                <label className="query-control">
                  <span>目标服务</span>
                  <select value={filterDraft.serviceName} onChange={(event) => updateFilterDraft({ serviceName: event.target.value })}>
                    <option value="all">全部服务</option>
                    {serviceOptions.map((serviceID) => (
                      <option key={serviceID} value={serviceID}>{targetLabel(serviceID, routeWorkspace.composer.targets)}</option>
                    ))}
                  </select>
                </label>
                <label className="query-control">
                  <span>启用状态</span>
                  <select value={filterDraft.enabled} onChange={(event) => updateFilterDraft({ enabled: event.target.value as RouteEnabledFilter })}>
                    <option value="all">全部</option>
                    <option value="enabled">启用</option>
                    <option value="disabled">停用</option>
                  </select>
                </label>
              </div>
              <div className="query-actions">
                <Button variant="soft" onClick={resetFilters}>重置</Button>
                <Button variant="primary" onClick={() => setFilters(filterDraft)}>查询</Button>
              </div>
            </div>
            {renderRouteTable(
              visibleRoutes,
              selectedRouteView?.id,
              routeWorkspace.composer.gateways,
              routeWorkspace.composer.targets,
              routeEnabled,
              toggleRouteEnabled,
              toggling,
              setSelectedRouteId,
              (route) => {
                setSelectedRouteId(route.id);
                setMode('detail');
              },
              openEdit,
              deleteRoute,
            )}
            {visibleRoutes.length === 0 ? (
              <div className="table-empty">
                <EmptyState
                  title={hasActiveFilters ? '没有匹配的路由' : '暂无路由'}
                  message={hasActiveFilters ? '调整查询条件后再试，或重置筛选查看全部路由。' : '当前还没有请求转发规则，可以先新建一条路由。'}
                />
              </div>
            ) : null}
          </Panel>
          <Toast message={notice} onClose={() => setNotice(null)} />
          {deleteCandidate ? (
            <div className="confirm-overlay" role="presentation" onMouseDown={() => {
              if (!deleting) {
                setDeleteCandidate(null);
              }
            }}>
              <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-route-title" onMouseDown={(event) => event.stopPropagation()}>
                <h3 id="delete-route-title">删除路由</h3>
                <p>确定删除 {deleteCandidate.name}？删除后这条路由不会再进入目标服务。</p>
                <div className="confirm-meta">
                  <span>所属网关</span><strong>{formatGatewayIDs(deleteCandidate.gatewayIDs, routeWorkspace.composer.gateways)}</strong>
                  <span>目标服务</span><strong>{routeTargetSummary(deleteCandidate, routeWorkspace.composer.targets)}</strong>
                </div>
                <div className="confirm-actions">
                  <Button variant="ghost" disabled={deleting} onClick={() => setDeleteCandidate(null)}>取消</Button>
                  <Button variant="primary" disabled={deleting} onClick={confirmDeleteRoute}>{deleting ? '删除中...' : '确认删除'}</Button>
                </div>
              </div>
            </div>
          ) : null}
          {disableCandidate ? (
            <div className="confirm-overlay" role="presentation" onMouseDown={() => {
              if (!toggling) {
                setDisableCandidate(null);
              }
            }}>
              <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="disable-route-title" onMouseDown={(event) => event.stopPropagation()}>
                <h3 id="disable-route-title">停用路由</h3>
                <p>停用 {disableCandidate.name} 后，命中该路由的请求将不再转发到目标服务。</p>
                <div className="confirm-meta">
                  <span>匹配 Host</span><strong>{formatHostnames(disableCandidate.hostnames)}</strong>
                  <span>目标服务</span><strong>{routeTargetSummary(disableCandidate, routeWorkspace.composer.targets)}</strong>
                </div>
                <div className="confirm-actions">
                  <Button variant="ghost" disabled={toggling} onClick={() => setDisableCandidate(null)}>取消</Button>
                  <Button variant="primary" disabled={toggling} onClick={confirmDisableRoute}>{toggling ? '停用中...' : '确认停用'}</Button>
                </div>
              </div>
            </div>
          ) : null}
        </>
      )}
    </PageFrame>
  );
}

function RouteMatchHeader({
  draft,
  gateways,
  targets,
}: {
  draft: RouteComposerDraft;
  gateways: RouteGatewayOption[];
  targets: RouteTargetOption[];
}) {
  const targetSummary = formatTargetServices(draft.targetServices, targets);
  const flowItems = [
    { label: '入口网关', value: formatGatewayIDs(draft.gatewayIDs, gateways), meta: `${draft.gatewayIDs.length || 0} 个网关` },
    { label: '匹配请求', value: `${formatMethods(draft.methods)} ${draft.path || '/'}`, meta: formatHostnames(draft.hostnames) },
    { label: '转发服务', value: targetSummary, meta: `${draft.targetServices.length} 个目标 / 总权重 ${targetWeightSum(draft.targetServices)}` },
    { label: '治理策略', value: draft.enabledPolicyCapabilities.length > 0 ? `${draft.enabledPolicyCapabilities.length} 个策略` : '未绑定策略', meta: draft.enabled ? '保存后自动生效' : '当前停用' },
  ];

  return (
    <section className="route-flow-hero">
      <div className="route-flow-head">
        <div>
          <span>当前路由</span>
          <strong>{draft.name.trim() || '请输入路由名称'}</strong>
          <small>{formatMethods(draft.methods)} {draft.path || '/'}</small>
        </div>
        <Badge tone={draft.enabled ? 'green' : 'neutral'}>{draft.enabled ? '运行中' : '已停用'}</Badge>
      </div>
      <div className="route-flow-lane">
        {flowItems.map((item, index) => (
          <div key={item.label} className="route-flow-segment">
            <div className="route-flow-index">{index + 1}</div>
            <div className="route-flow-copy">
              <span>{item.label}</span>
              <strong>{item.value}</strong>
              <small>{item.meta}</small>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function findPolicyValidationMessage(composer: RouteComposerPreview, draft: RouteComposerDraft) {
  const errors = validateEnabledPolicySettings(composer.policies, draft.enabledPolicyCapabilities, draft.policySettings);
  const invalidCapability = draft.enabledPolicyCapabilities.find((capability) => Object.keys(errors[capability] ?? {}).length > 0);
  const invalidPolicy = composer.policies.find((policy) => policy.capability === invalidCapability);
  return invalidPolicy ? `请补齐策略参数：${invalidPolicy.displayName}` : '';
}

function renderRouteTable(
  routes: RouteResource[],
  selectedRouteId: string | undefined,
  gateways: RouteGatewayOption[],
  targets: RouteTargetOption[],
  routeEnabled: (route: RouteResource) => boolean,
  onToggleEnabled: (route: RouteResource) => void,
  toggling: boolean,
  onSelect: (id: string) => void,
  onDetail: (route: RouteResource) => void,
  onEdit: (route: RouteResource) => void,
  onDelete: (route: RouteResource) => void,
) {
  return (
    <div style={{ overflow: 'auto' }}>
      <table className="table">
        <thead>
          <tr>
            <th>路由名称</th>
            <th>匹配条件</th>
            <th>所属网关</th>
            <th>目标服务</th>
            <th>策略</th>
            <th>启用状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {routes.map((route) => (
            <tr key={route.id} className={route.id === selectedRouteId ? 'selected' : ''} onClick={() => onSelect(route.id)}>
              <td>
                <div className="table-primary">{route.name}</div>
                <div className="table-secondary">{route.rules.length} 条规则</div>
              </td>
              <td>
                <div className="table-primary">
                  <Badge tone={(primaryRouteRule(route)?.methods ?? []).includes('POST') ? 'green' : 'amber'}>{formatMethods(primaryRouteRule(route)?.methods ?? [])}</Badge> {primaryRouteRule(route)?.pathPrefix ?? '-'}
                </div>
                <div className="table-secondary">Host: {formatHostnames(route.hostnames)}</div>
              </td>
              <td>
                <div className="table-primary">{formatGatewayIDs(route.gatewayIDs, gateways)}</div>
                <div className="table-secondary">{route.gatewayIDs.length} 个网关</div>
              </td>
              <td>
                <div className="table-primary">{routeTargetSummary(route, targets)}</div>
                <div className="table-secondary">{routeTargetServices(route).length} 个目标</div>
              </td>
              <td>{routePolicyCount(route) > 0 ? `${routePolicyCount(route)} 个` : '未绑定'}</td>
              <td>
                <div className={`gateway-status ${routeEnabled(route) ? 'on' : ''}`.trim()}>
                  <button
                    className="gateway-switch"
                    type="button"
                    role="switch"
                    disabled={toggling}
                    aria-checked={routeEnabled(route)}
                    aria-label={`${route.name} ${routeEnabled(route) ? '已启用' : '已停用'}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      onToggleEnabled(route);
                    }}
                  >
                    <span aria-hidden="true" />
                  </button>
                  <strong>{routeEnabled(route) ? '启用' : '停用'}</strong>
                </div>
              </td>
              <td>{formatDateTime(route.createdAt)}</td>
              <td>
                <div className="row-actions">
                  <button className="link-button" type="button" onClick={(event) => {
                    event.stopPropagation();
                    onDetail(route);
                  }}>详情</button>
                  <button className="link-button" type="button" onClick={(event) => {
                    event.stopPropagation();
                    onEdit(route);
                  }}>编辑</button>
                  <button className="link-button danger" type="button" onClick={(event) => {
                    event.stopPropagation();
                    onDelete(route);
                  }}>删除</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RouteComposer({
  composer,
  draft,
  validation,
  gatewayOptions,
  onDraftChange,
}: {
  composer: RouteComposerPreview;
  draft: RouteComposerDraft;
  validation: RouteValidationReport;
  gatewayOptions: RouteGatewayOption[];
  onDraftChange: (draft: RouteComposerDraft) => void;
}) {
  const updateDraft = (patch: Partial<RouteComposerDraft>) => {
    onDraftChange({ ...draft, ...patch });
  };
  const fieldErrors = routeFieldErrors(validation);

  return (
    <div className="route-form-layout">
      <nav className="route-form-nav" aria-label="路由配置导航">
        <a href="#route-basic">基础信息</a>
        <a href="#route-match">匹配条件</a>
        <a href="#route-target">目标服务</a>
        <a href="#route-policies">策略配置</a>
      </nav>

      <div className="route-form-sections">
        <section id="route-basic" className="detail-card composer-card route-form-section">
          <SectionTitle number="01" title="基础信息" description="定义这条路由在控制台和运行时中的基本身份。" />
          <div className="field-grid">
            <InputField
              label="路由名称"
              value={draft.name}
              required
              maxLength={64}
              placeholder="例如：用户查询接口"
              info="面向控制台用户的唯一名称；资源 ID 由后端生成，不需要手动维护"
              error={fieldErrors.name}
              onChange={(name) => updateDraft({ name })}
            />
            <div className="field">
              <FieldLabel label="启用状态" info="关闭后路由配置保留，但不会下发生效" />
              <div className={`gateway-status route-status-control ${draft.enabled ? 'on' : ''}`.trim()}>
                <button
                  className="gateway-switch"
                  type="button"
                  role="switch"
                  aria-checked={draft.enabled}
                  onClick={() => updateDraft({ enabled: !draft.enabled })}
                >
                  <span />
                </button>
                <span>{draft.enabled ? '启用' : '停用'}</span>
              </div>
            </div>
          </div>
        </section>

        <section id="route-match" className="detail-card composer-card route-form-section">
          <SectionTitle number="02" title="匹配条件" description="定义请求如何进入这条路由；Host 可留空，表示不限制 Host。" />
          <div className="field-grid">
            <GatewayMultiSelect options={gatewayOptions} value={draft.gatewayIDs} error={fieldErrors.gateway} onChange={(gatewayIDs) => updateDraft({ gatewayIDs })} />
            <InputField
              label="规则名称"
              value={draft.ruleName}
              required
              maxLength={63}
              placeholder="main"
              info="Route 内规则的稳定名称，同一条 Route 内不能重复"
              error={fieldErrors.rule}
              onChange={(ruleName) => updateDraft({ ruleName })}
            />
            <MethodSelector value={draft.methods} onChange={(methods) => updateDraft({ methods })} />
            <InputField
              label="路径"
              value={draft.path}
              required
              maxLength={256}
              placeholder="/"
              info="当前阶段按路径前缀匹配；必须以 / 开头"
              error={fieldErrors.path}
              onChange={(value) => updateDraft({ path: value })}
            />
            <HostnameEditor value={draft.hostnames} error={fieldErrors.host} onChange={(hostnames) => updateDraft({ hostnames })} />
            <HeaderMatchEditor value={draft.headers} error={fieldErrors.header} onChange={(headers) => updateDraft({ headers })} />
          </div>
        </section>

        <section id="route-target" className="detail-card composer-card route-form-section">
          <SectionTitle number="03" title="目标服务" description="选择路由命中后要转发到的服务；多个目标按权重分流。" />
          <TargetSelector
            targets={composer.targets}
            selectedTargets={draft.targetServices}
            error={fieldErrors.service}
            onTargetsChange={(targetServices) => updateDraft({
              targetServices,
            })}
          />
        </section>

        <section id="route-policies" className="detail-card composer-card route-form-section">
          <SectionTitle number="04" title="规则策略" description="配置当前规则的 Header、超时和重试等原生治理能力。" />
          <RoutePolicyBindings
            policies={composer.policies}
            enabledPolicyCapabilities={draft.enabledPolicyCapabilities}
            settings={draft.policySettings}
            onAddPolicy={(capability) => {
              if (draft.enabledPolicyCapabilities.includes(capability)) {
                return;
              }

              updateDraft({ enabledPolicyCapabilities: [...draft.enabledPolicyCapabilities, capability] });
            }}
            onRemovePolicy={(capability) => updateDraft({ enabledPolicyCapabilities: draft.enabledPolicyCapabilities.filter((item) => item !== capability) })}
            onSettingChange={(capability, key, value) => updateDraft({
              policySettings: {
                ...draft.policySettings,
                [capability]: {
                  ...(draft.policySettings[capability] ?? {}),
                  [key]: value,
                },
              },
            })}
          />
        </section>
      </div>
    </div>
  );
}

function routeFieldErrors(validation: RouteValidationReport) {
  return {
    name: validation.items.find((item) => item.label === '路由名称' && item.status === 'critical')?.message,
    path: validation.items.find((item) => (item.label === '匹配规则' || item.label === '匹配路径') && item.status === 'critical')?.message,
    rule: validation.items.find((item) => item.label === '规则名称' && item.status === 'critical')?.message,
    service: validation.items.find((item) => item.label === '目标服务' && item.status === 'critical')?.message,
    gateway: validation.items.find((item) => item.label === '网关' && item.status === 'critical')?.message,
    host: validation.items.find((item) => item.label === '匹配域名' && item.status === 'critical')?.message,
    header: validation.items.find((item) => item.label === 'Header 匹配' && item.status === 'critical')?.message,
  };
}

function SectionTitle({ number, title, description }: { number: string; title: string; description: string }) {
  return (
    <div className="route-section-title">
      <span className="route-section-number">{number}</span>
      <div>
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

function renderRouteDetail(
  tab: string,
  selectedRoute: RoutePageView['routes'][number] | undefined,
  gateways: RouteGatewayOption[],
  targets: RouteTargetOption[],
) {
  const details: KeyValue[] = selectedRoute ? routeDetailsForTab(selectedRoute, tab, gateways, targets) : [];

  return (
    <div className="detail-card">
      <div className="kv">
        {details.flatMap((item) => [
          <div key={`${item.label}-label`}>{item.label}</div>,
          <div key={`${item.label}-value`}>{item.value}</div>,
        ])}
      </div>
    </div>
  );
}

function routeDetailsForTab(route: RoutePageView['routes'][number], tab: string, gateways: RouteGatewayOption[], targets: RouteTargetOption[]): KeyValue[] {
  const rule = primaryRouteRule(route);

  if (tab === 'match') {
    return [
      { label: '方法', value: formatMethods(rule?.methods ?? []) },
      { label: '路径', value: rule?.pathPrefix ?? '-' },
      { label: '所属网关', value: formatGatewayIDs(route.gatewayIDs, gateways) },
      { label: '匹配域名', value: formatHostnames(route.hostnames) },
      { label: 'Header 条件', value: formatHeaderMatches(rule?.headers ?? []) },
    ];
  }

  if (tab === 'target') {
    return [
      { label: '目标服务', value: routeTargetSummary(route, targets) },
      { label: '目标数量', value: `${routeTargetServices(route).length} 个` },
      { label: '总权重', value: String(targetWeightSum(routeTargetServices(route))) },
    ];
  }

  if (tab === 'events') {
    return [
      { label: '创建时间', value: formatDateTime(route.createdAt) },
      { label: '启用状态', value: route.enabled ? '启用' : '停用' },
      { label: '变更摘要', value: formatRouteMatch(route) },
    ];
  }

  return [
    { label: '路由名称', value: route.name },
    { label: '方法', value: formatMethods(rule?.methods ?? []) },
    { label: '路径', value: rule?.pathPrefix ?? '-' },
    { label: '所属网关', value: formatGatewayIDs(route.gatewayIDs, gateways) },
    { label: '目标服务', value: routeTargetSummary(route, targets) },
    { label: '启用状态', value: route.enabled ? '启用' : '停用' },
  ];
}

function FieldLabel({ label, required, info }: { label: string; required?: boolean; info?: string }) {
  return (
    <span className="field-label">
      <span>
        {required ? <span className="required-mark">*</span> : null}
        {label}
      </span>
      {info ? <span className="field-help" role="img" tabIndex={0} data-tooltip={info} aria-label={info}>?</span> : null}
    </span>
  );
}

function InputField({
  label,
  value,
  error,
  required,
  info,
  maxLength,
  placeholder,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  error?: string;
  required?: boolean;
  info?: string;
  maxLength?: number;
  placeholder?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <FieldLabel label={label} required={required} info={info} />
      <input value={value} maxLength={maxLength} placeholder={placeholder} disabled={disabled} onChange={(event) => onChange(event.target.value)} />
      {typeof maxLength === 'number' ? <div className="field-counter">{value.length}/{maxLength}</div> : null}
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function MethodSelector({ value, onChange }: { value: RouteComposerDraft['methods']; onChange: (value: RouteComposerDraft['methods']) => void }) {
  return (
    <MultiSelectDropdown
      label="请求方法"
      emptyLabel="全部方法"
      info="不选择时表示全部 HTTP 方法都可以匹配"
      options={httpMethods.map((method) => ({ value: method, label: method }))}
      value={value}
      onChange={(methods) => onChange(methods as RouteComposerDraft['methods'])}
    />
  );
}

function GatewayMultiSelect({ options, value, error, onChange }: { options: RouteGatewayOption[]; value: string[]; error?: string; onChange: (value: string[]) => void }) {
  return (
    <div className={`field field-wide route-relation-field ${error ? 'invalid' : ''}`.trim()}>
      <MultiSelectDropdown
        label="所属网关"
        emptyLabel="请选择网关"
        required
        info="一个路由可以挂到多个网关；保存后会分别在这些网关下生效"
        options={options.map((gateway) => ({ value: gateway.id, label: gateway.name }))}
        value={value}
        onChange={onChange}
      />
      {error ? <div className="form-error">{error}</div> : null}
      <div className="mini-card-meta">网关与路由是多对多关系；路由命中后可转发到一个或多个目标服务。</div>
    </div>
  );
}

function TargetSelector({
  targets,
  selectedTargets,
  error,
  onTargetsChange,
}: {
  targets: RouteComposerPreview['targets'];
  selectedTargets: RouteTargetPayload[];
  error?: string;
  onTargetsChange: (targets: RouteTargetPayload[]) => void;
}) {
  const [addTargetID, setAddTargetID] = useState(targets[0]?.id ?? '');
  const selectedIDs = new Set(selectedTargets.map((target) => target.upstreamID));
  const availableTargets = targets.filter((target) => !selectedIDs.has(target.id));
  const candidateTargetID = availableTargets.some((target) => target.id === addTargetID)
    ? addTargetID
    : availableTargets[0]?.id ?? '';

  const addTarget = () => {
    if (!candidateTargetID) {
      return;
    }
    onTargetsChange([...selectedTargets, { upstreamID: candidateTargetID, weight: 100 }]);
  };

  const updateTargetWeight = (upstreamID: string, weight: number) => {
    onTargetsChange(selectedTargets.map((target) => (target.upstreamID === upstreamID ? { ...target, weight } : target)));
  };

  const removeTarget = (upstreamID: string) => {
    onTargetsChange(selectedTargets.filter((target) => target.upstreamID !== upstreamID));
  };

  return (
    <div className="target-config">
      <div className={`field target-picker ${error ? 'invalid' : ''}`.trim()}>
        <FieldLabel label="添加目标服务" required info="一个路由可以转发到多个服务；运行时会按照每个目标的权重做加权分流" />
        <div className="inline-input">
          <select value={candidateTargetID} disabled={availableTargets.length === 0} onChange={(event) => setAddTargetID(event.target.value)}>
            {availableTargets.length === 0 ? <option value="">所有服务都已选择</option> : null}
            {availableTargets.map((target) => (
              <option key={target.id} value={target.id}>{target.name} · {serviceTypeLabel(target.type)}</option>
            ))}
          </select>
          <Button variant="soft" type="button" disabled={!candidateTargetID} onClick={addTarget}>添加目标</Button>
        </div>
        {error ? <div className="form-error">{error}</div> : null}
      </div>

      <div className="target-service-card target-service-card-list">
        <div className="target-service-head">
          <div>
            <span className="mini-card-meta">已选目标服务</span>
            <strong>{selectedTargets.length} 个目标 / 总权重 {targetWeightSum(selectedTargets)}</strong>
          </div>
          <Badge tone={selectedTargets.length > 1 ? 'green' : 'neutral'}>{selectedTargets.length > 1 ? '加权分流' : '单目标'}</Badge>
        </div>
        <div className="target-service-list">
          {selectedTargets.length === 0 ? (
            <div className="target-service-empty">请选择至少一个目标服务。</div>
          ) : selectedTargets.map((target) => {
            const service = targets.find((item) => item.id === target.upstreamID);
            const serviceName = service?.name ?? target.upstreamID;

            return (
              <div key={target.upstreamID} className="target-service-row">
                <div className="target-service-main">
                  <strong>{serviceName}</strong>
                  <span>{service ? serviceTypeLabel(service.type) : '未知类型'} · {service?.endpoint ?? '未配置地址'}</span>
                </div>
                <Badge tone="neutral">{service?.meta ?? '未配置端点'}</Badge>
                <label className="target-weight-field">
                  <span>权重</span>
                  <input
                    value={target.weight}
                    type="number"
                    min={1}
                    max={100}
                    onChange={(event) => updateTargetWeight(target.upstreamID, Number(event.target.value))}
                  />
                </label>
                <button className="link-button danger" type="button" onClick={() => removeTarget(target.upstreamID)}>删除</button>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function MultiSelectDropdown({
  label,
  emptyLabel,
  required,
  info,
  options,
  value,
  onChange,
}: {
  label: string;
  emptyLabel: string;
  required?: boolean;
  info?: string;
  options: { value: string; label: string }[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const toggleValue = (nextValue: string) => {
    onChange(value.includes(nextValue) ? value.filter((item) => item !== nextValue) : [...value, nextValue]);
  };
  const displayValue = value.length > 0
    ? options.filter((option) => value.includes(option.value)).map((option) => option.label).join('、')
    : emptyLabel;

  useEffect(() => {
    if (!open) {
      return;
    }

    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', closeOnOutsideClick);
    return () => document.removeEventListener('mousedown', closeOnOutsideClick);
  }, [open]);

  return (
    <div className="field">
      <FieldLabel label={label} required={required} info={info} />
      <div ref={rootRef} className={`multi-select ${open ? 'open' : ''}`.trim()}>
        <button className="multi-select-trigger" type="button" onClick={() => setOpen(!open)}>
          <span>{displayValue}</span>
          <span aria-hidden="true">⌄</span>
        </button>
        {open ? (
          <div className="multi-select-menu">
            <button className={value.length === 0 ? 'active' : ''} type="button" onClick={() => onChange([])}>
              {emptyLabel}
            </button>
            {options.map((option) => (
              <button key={option.value} className={value.includes(option.value) ? 'active' : ''} type="button" onClick={() => toggleValue(option.value)}>
                <span className="multi-check">{value.includes(option.value) ? '✓' : ''}</span>
                {option.label}
              </button>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function HostnameEditor({ value, error, onChange }: { value: string[]; error?: string; onChange: (value: string[]) => void }) {
  const [inputValue, setInputValue] = useState('');

  const addHostnames = () => {
    const nextHostnames = normalizeHostnames([...value, ...parseHostnames(inputValue)]);

    onChange(nextHostnames);
    setInputValue('');
  };

  const removeHostname = (hostname: string) => {
    onChange(value.filter((item) => item !== hostname));
  };

  return (
    <div className={`field field-wide ${error ? 'invalid' : ''}`.trim()}>
      <FieldLabel label="匹配域名" info="留空表示不校验 Host；填写后只匹配这些域名，支持 *.example.com" />
      <div className="inline-input">
        <input
          value={inputValue}
          maxLength={253}
          placeholder="api.example.com，支持逗号或空格分隔"
          onChange={(event) => setInputValue(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              addHostnames();
            }
          }}
        />
        <Button variant="soft" type="button" onClick={addHostnames}>添加</Button>
      </div>
      <div className="field-counter">{inputValue.length}/253</div>
      <div className="tag-list">
        {value.length === 0 ? (
          <span className="mini-card-meta">不限制 Host。需要按域名匹配时再添加，可使用 *.example.com。</span>
        ) : value.map((hostname) => (
          <button key={hostname} className="tag-chip" type="button" onClick={() => removeHostname(hostname)} title="点击移除">
            {hostname}
            <span aria-hidden="true">×</span>
          </button>
        ))}
      </div>
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function HeaderMatchEditor({ value, error, onChange }: { value: HeaderMatch[]; error?: string; onChange: (value: HeaderMatch[]) => void }) {
  const addHeader = () => {
    onChange([...value, { name: '', value: '' }]);
  };
  const updateHeader = (index: number, patch: Partial<HeaderMatch>) => {
    onChange(value.map((header, currentIndex) => (currentIndex === index ? { ...header, ...patch } : header)));
  };
  const removeHeader = (index: number) => {
    onChange(value.filter((_, currentIndex) => currentIndex !== index));
  };

  return (
    <div className={`field field-wide header-match-editor ${error ? 'invalid' : ''}`.trim()}>
      <div className="field-row-label">
        <FieldLabel label="Header 匹配" info="可选；填写后只有满足这些 Header 精确匹配条件的请求才会命中当前规则" />
        <Button variant="soft" type="button" onClick={addHeader}>添加条件</Button>
      </div>
      {value.length === 0 ? (
        <span className="mini-card-meta">不限制 Header。需要按 Header 匹配时再添加条件。</span>
      ) : (
        <div className="header-match-list">
          {value.map((header, index) => (
            <div key={index} className="header-match-row">
              <input value={header.name} placeholder="Header 名称，例如 x-tenant-id" onChange={(event) => updateHeader(index, { name: event.target.value })} />
              <input value={header.value} placeholder="Header 值" onChange={(event) => updateHeader(index, { value: event.target.value })} />
              <button className="link-button danger" type="button" onClick={() => removeHeader(index)}>删除</button>
            </div>
          ))}
        </div>
      )}
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function formatHostnames(hostnames: string[]) {
  return hostnames.length > 0 ? hostnames.join('、') : '不限制 Host';
}

function formatHeaderMatches(headers: HeaderMatch[]) {
  if (headers.length === 0) {
    return '不限制 Header';
  }
  return headers.map((header) => `${header.name}=${header.value}`).join('、');
}

function formatGatewayIDs(gatewayIDs: string[], gateways: RouteGatewayOption[] = []) {
  if (gatewayIDs.length === 0) {
    return '-';
  }
  return routeGatewayLabels(gatewayIDs, gateways).join('、');
}

function formatMethods(methods: string[]) {
  return methods.length > 0 ? methods.join('、') : '全部方法';
}

function primaryRouteRule(route: Pick<RouteResource, 'rules'>) {
  return route.rules?.[0];
}

function routeTargetServices(route: Pick<RouteResource, 'rules'>): RouteTargetPayload[] {
  return primaryRouteRule(route)?.targets ?? [];
}

function routeTargetIDs(route: Pick<RouteResource, 'rules'>) {
  return routeTargetServices(route).map((target) => target.upstreamID);
}

function routeTargetLabels(route: Pick<RouteResource, 'rules'>, targets: RouteTargetOption[] = []) {
  return routeTargetServices(route).map((target) => targetLabel(target.upstreamID, targets));
}

function routeGatewayLabels(gatewayIDs: string[], gateways: RouteGatewayOption[] = []) {
  return gatewayIDs.map((gatewayID) => gatewayLabel(gatewayID, gateways));
}

function routeTargetSummary(route: Pick<RouteResource, 'rules'>, targets: RouteTargetOption[] = []) {
  return formatTargetServices(routeTargetServices(route), targets);
}

function routePolicyCount(route: Pick<RouteResource, 'rules'>) {
  return route.rules.reduce((count, rule) => {
    let next = count;
    if (rule.requestHeaderModifier) {
      next++;
    }
    if (rule.responseHeaderModifier) {
      next++;
    }
    if (rule.timeout) {
      next++;
    }
    if (rule.retry) {
      next++;
    }
    return next;
  }, 0);
}

function formatRouteMatch(route: Pick<RouteResource, 'rules'>) {
  const rule = primaryRouteRule(route);
  return `${formatMethods(rule?.methods ?? [])} ${rule?.pathPrefix ?? '-'}`;
}

function enabledCapabilitiesFromRule(rule: RouteResource['rules'][number] | undefined): RoutePolicyCapability[] {
  if (!rule) {
    return [];
  }

  const capabilities: RoutePolicyCapability[] = [];
  if (rule.requestHeaderModifier) {
    capabilities.push(routePolicyCapabilityRequestHeaderModifier);
  }
  if (rule.responseHeaderModifier) {
    capabilities.push(routePolicyCapabilityResponseHeaderModifier);
  }
  if (rule.timeout) {
    capabilities.push(routePolicyCapabilityTimeout);
  }
  if (rule.retry) {
    capabilities.push(routePolicyCapabilityRetry);
  }
  return capabilities;
}

function policySettingsFromRule(rule: RouteResource['rules'][number] | undefined): RouteComposerDraft['policySettings'] {
  if (!rule) {
    return {};
  }

  const settings: RouteComposerDraft['policySettings'] = {};
  if (rule.requestHeaderModifier) {
    settings.RequestHeaderModifier = {
      setHeadersOn: rule.requestHeaderModifier.set?.map((item) => item.name).join(',') ?? '',
      value: rule.requestHeaderModifier.set?.[0]?.value ?? '',
      removeHeadersOn: rule.requestHeaderModifier.remove?.join(',') ?? '',
    };
  }
  if (rule.responseHeaderModifier) {
    settings.ResponseHeaderModifier = {
      setHeadersOn: rule.responseHeaderModifier.set?.map((item) => item.name).join(',') ?? '',
      value: rule.responseHeaderModifier.set?.[0]?.value ?? '',
      removeHeadersOn: rule.responseHeaderModifier.remove?.join(',') ?? '',
    };
  }
  if (rule.timeout) {
    settings.Timeout = {
      timeoutMillis: String(rule.timeout.requestMillis),
    };
  }
  if (rule.retry) {
    settings.Retry = {
      attempts: String(rule.retry.attempts),
      perTryTimeoutMillis: String(rule.retry.perTryTimeoutMillis),
    };
  }
  return settings;
}

function gatewayLabel(gatewayID: string, gateways: RouteGatewayOption[]) {
  return gateways.find((gateway) => gateway.id === gatewayID)?.name ?? shortResourceID(gatewayID);
}

function targetLabel(upstreamID: string, targets: RouteTargetOption[]) {
  return targets.find((target) => target.id === upstreamID)?.name ?? shortResourceID(upstreamID);
}

function shortResourceID(id: string) {
  if (id.length <= 12) {
    return id;
  }
  return `${id.slice(0, 8)}...${id.slice(-4)}`;
}

function RoutePolicyBindings({
  policies,
  enabledPolicyCapabilities,
  settings,
  onAddPolicy,
  onRemovePolicy,
  onSettingChange,
}: {
  policies: RouteComposerPreview['policies'];
  enabledPolicyCapabilities: RoutePolicyCapability[];
  settings: Record<string, Record<string, string>>;
  onAddPolicy: (capability: RoutePolicyCapability) => void;
  onRemovePolicy: (capability: RoutePolicyCapability) => void;
  onSettingChange: (capability: RoutePolicyCapability, key: string, value: string) => void;
}) {
  const applicablePolicies = policies;
  const enabledPolicySet = new Set(enabledPolicyCapabilities);
  const enabledCount = applicablePolicies.filter((policy) => enabledPolicySet.has(policy.capability)).length;
  const policyErrors = validateEnabledPolicySettings(applicablePolicies, enabledPolicyCapabilities, settings);

  const togglePolicy = (capability: RoutePolicyCapability) => {
    if (enabledPolicySet.has(capability)) {
      onRemovePolicy(capability);
      return;
    }

    onAddPolicy(capability);
  };

  return (
    <div className="route-policy-bindings">
      <div className="policy-bindings-head">
        <div>
          <h4>路由策略</h4>
          <p>配置当前 RouteRule 的原生治理能力；未启用的能力不会写入规则。</p>
        </div>
        <Badge tone={enabledCount > 0 ? 'green' : 'neutral'}>已启用 {enabledCount} 个</Badge>
      </div>

      {applicablePolicies.length > 0 ? (
        <div className="route-policy-capability-list">
          {applicablePolicies.map((policy) => {
            const enabled = enabledPolicySet.has(policy.capability);
            const policySettings = settings[policy.capability] ?? {};
            const errors = enabled ? policyErrors[policy.capability] ?? {} : {};

            return (
              <article key={policy.capability} className={`route-policy-capability ${enabled ? 'enabled' : ''}`.trim()}>
                <div className="route-policy-capability-head">
                  <button className="route-policy-toggle" type="button" onClick={() => togglePolicy(policy.capability)}>
                    <span className={`switch ${enabled ? 'on' : ''}`} aria-hidden="true" />
                    <span>
                      <strong>{policy.displayName}</strong>
                      <small>{policy.meta}</small>
                    </span>
                  </button>
                  <Badge tone="neutral">{policyCategory(policy.capability)}</Badge>
                </div>
                {enabled ? (
                  <>
                    <div className="route-policy-summary">{policyParamSummary(policy, policySettings)}</div>
                    <div className="policy-param-grid">
                      {policy.params.map((param) => (
                        <PolicyParamField
                          key={param.key}
                          param={param}
                          value={policySettings[param.key] ?? param.defaultValue}
                          error={errors[param.key]}
                          onChange={(value) => onSettingChange(policy.capability, param.key, value)}
                        />
                      ))}
                    </div>
                  </>
                ) : null}
              </article>
            );
          })}
        </div>
      ) : (
        <div className="route-policy-empty">
          <strong>当前目标暂无可用策略能力</strong>
          <span>请选择服务后再配置路由级治理能力。</span>
        </div>
      )}
    </div>
  );
}

function PolicyParamField({
  param,
  value,
  error,
  onChange,
}: {
  param: RouteComposerPreview['policies'][number]['params'][number];
  value: string;
  error?: string;
  onChange: (value: string) => void;
}) {
  const controlType = param.inputType ?? (param.options ? 'select' : 'text');
  const maxLength = controlType === 'text' ? 128 : undefined;

  return (
    <div className={`policy-param-field ${error ? 'invalid' : ''}`.trim()}>
      <FieldLabel label={param.label} required={param.required} info={param.placeholder} />
      <div className={`policy-param-control ${controlType === 'multiselect' ? 'popover-control' : ''}`.trim()}>
        {controlType === 'select' && param.options ? (
          <select value={value} onChange={(event) => onChange(event.target.value)}>
            {param.options.map((option) => (
              <option key={option} value={option}>{option}</option>
            ))}
          </select>
        ) : controlType === 'multiselect' && param.options ? (
          <InlineMultiSelect
            emptyLabel="请选择"
            options={param.options}
            value={parsePolicyMultiValue(value)}
            onChange={(nextValue) => onChange(formatPolicyMultiValue(nextValue))}
          />
        ) : controlType === 'number' ? (
          <div className="unit-input">
            <input
              type="number"
              min={param.min}
              max={param.max}
              value={value}
              placeholder={param.placeholder}
              onChange={(event) => onChange(event.target.value)}
            />
            {param.unit ? <span>{param.unit}</span> : null}
          </div>
        ) : (
          <input value={value} maxLength={maxLength} placeholder={param.placeholder} onChange={(event) => onChange(event.target.value)} />
        )}
      </div>
      {typeof maxLength === 'number' ? <div className="field-counter">{value.length}/{maxLength}</div> : null}
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function validatePolicySettings(policy: RouteComposerPreview['policies'][number], settings: Record<string, string>): Record<string, string> {
  const errors: Record<string, string> = {};

  if (policy.capability === routePolicyCapabilityRequestHeaderModifier || policy.capability === routePolicyCapabilityResponseHeaderModifier) {
    const setHeaderNames = (settings.setHeadersOn ?? '').trim();
    const headerValue = (settings.value ?? '').trim();
    const removeHeaderNames = (settings.removeHeadersOn ?? '').trim();

    if (!setHeaderNames && !removeHeaderNames) {
      errors.setHeadersOn = '请填写写入或删除 Header 名称';
      return errors;
    }

    if (setHeaderNames && !headerValue) {
      errors.value = '请填写 Header 值';
    }

    if (headerValue && !setHeaderNames) {
      errors.setHeadersOn = '请填写写入 Header 名称';
    }

    return errors;
  }

  policy.params.forEach((param) => {
    const value = settings[param.key] ?? param.defaultValue;
    const normalizedValue = value.trim();
    const controlType = param.inputType ?? (param.options ? 'select' : 'text');

    if (param.required && !normalizedValue) {
      errors[param.key] = `请填写${param.label}`;
      return;
    }

    if (controlType === 'multiselect' && param.required && parsePolicyMultiValue(value).length === 0) {
      errors[param.key] = `请至少选择一个${param.label}`;
      return;
    }

    if (controlType === 'number') {
      const numericValue = Number(value);

      if (!normalizedValue || Number.isNaN(numericValue)) {
        errors[param.key] = `${param.label}必须是数字`;
        return;
      }

      if (typeof param.min === 'number' && numericValue < param.min) {
        errors[param.key] = `${param.label}不能小于 ${param.min}`;
        return;
      }

      if (typeof param.max === 'number' && numericValue > param.max) {
        errors[param.key] = `${param.label}不能大于 ${param.max}`;
      }
    }
  });

  return errors;
}

function validateEnabledPolicySettings(
  policies: RouteComposerPreview['policies'],
  enabledPolicyCapabilities: RoutePolicyCapability[],
  settings: RouteComposerDraft['policySettings'],
): Record<string, Record<string, string>> {
  const errors: Record<string, Record<string, string>> = {};

  enabledPolicyCapabilities.forEach((capability) => {
    const policy = policies.find((item) => item.capability === capability);
    if (!policy) {
      return;
    }
    errors[capability] = validatePolicySettings(policy, settings[capability] ?? {});
  });

  const retryErrors = errors[routePolicyCapabilityRetry];
  if (!retryErrors || !enabledPolicyCapabilities.includes(routePolicyCapabilityRetry)) {
    return errors;
  }

  const totalTimeoutMillis = numericPolicySetting(settings[routePolicyCapabilityTimeout], 'timeoutMillis', defaultRouteTimeoutMillis);
  const perTryTimeoutMillis = numericPolicySetting(settings[routePolicyCapabilityRetry], 'perTryTimeoutMillis', 0);
  if (perTryTimeoutMillis > 0 && perTryTimeoutMillis > totalTimeoutMillis) {
    retryErrors.perTryTimeoutMillis = `单次尝试超时不能大于请求总超时 ${totalTimeoutMillis}ms`;
  }

  return errors;
}

function numericPolicySetting(settings: Record<string, string> | undefined, key: string, fallback: number) {
  const value = Number(settings?.[key] ?? fallback);
  return Number.isFinite(value) ? value : fallback;
}

function InlineMultiSelect({
  emptyLabel,
  options,
  value,
  onChange,
}: {
  emptyLabel: string;
  options: string[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const displayValue = value.length > 0 ? value.join('、') : emptyLabel;

  return (
    <details className="inline-multi-select">
      <summary>
        <span>{displayValue}</span>
        <span aria-hidden="true">⌄</span>
      </summary>
      <div className="inline-multi-select-menu">
        {options.map((option) => (
          <button
            key={option}
            type="button"
            className={value.includes(option) ? 'active' : ''}
            onClick={() => onChange(value.includes(option) ? value.filter((item) => item !== option) : [...value, option])}
          >
            <span>{value.includes(option) ? '✓' : ''}</span>
            {option}
          </button>
        ))}
      </div>
    </details>
  );
}

function parsePolicyMultiValue(value: string): string[] {
  return value.split(/[,，、]/).map((item) => item.trim()).filter(Boolean);
}

function formatPolicyMultiValue(value: string[]): string {
  return value.join(',');
}

function policyParamSummary(policy: RouteComposerPreview['policies'][number], settings: Record<string, string>) {
  const summary = policy.params
    .map((param) => ({ label: param.label, value: settings[param.key] ?? param.defaultValue }))
    .filter((item) => item.value.trim())
    .slice(0, 2)
    .map((item) => `${item.label}: ${item.value}`)
    .join(' / ');

  return summary || '-';
}

function policyCategory(capability: RoutePolicyCapability): string {
  if (capability === routePolicyCapabilityRequestHeaderModifier || capability === routePolicyCapabilityResponseHeaderModifier) {
    return 'Header 处理';
  }
  if (capability === routePolicyCapabilityTimeout || capability === routePolicyCapabilityRetry) {
    return '可靠性';
  }

  return '通用策略';
}
