import { useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Tabs } from '@/components/ui';
import type { KeyValue } from '@/domain/common';
import { healthLabel, runtimeSyncStatusLabel, statusTone } from '@/domain/common';
import type { RouteComposerPreview, RoutePageView, RouteResource, RouteValidationReport } from '@/domain/route';
import { serviceTypeLabel } from '@/domain/service';
import type { RouteComposerDraft } from './composer';
import {
  buildRoutePublishPayload,
  createRouteComposerDraft,
  normalizeHostnames,
  parseHostnames,
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
const routePolicyTimeoutName = '超时控制';
const routePolicyRetryName = '失败重试';
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
  const gatewayOptions = Array.from(new Set([...routeWorkspace.composer.gatewayNames, ...availableRoutes.flatMap((route) => route.gatewayNames)])).sort();
  const serviceOptions = Array.from(new Set([...routeWorkspace.composer.targets.map((target) => target.name), ...availableRoutes.map((route) => route.serviceName)])).sort();
  const visibleRoutes = availableRoutes.filter((route) => {
    const keyword = filters.keyword.trim().toLowerCase();
    const matchedKeyword = !keyword || [route.path, route.serviceName, ...route.gatewayNames, ...route.hostnames].some((value) => value.toLowerCase().includes(keyword));
    const matchedGateway = filters.gatewayName === 'all' || route.gatewayNames.includes(filters.gatewayName);
    const matchedService = filters.serviceName === 'all' || route.serviceName === filters.serviceName;
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
    setSelectedRouteId(route.id);
    setMode('composer');
    setDraftState({
      ...createRouteComposerDraft(routeWorkspace.composer),
      id: route.id,
      version: route.version,
      methods: route.methods,
      path: route.path,
      gatewayNames: route.gatewayNames,
      hostnames: route.hostnames,
      serviceName: route.serviceName,
      enabled: route.enabled,
      selectedTargetName: route.serviceName,
      enabledPolicyNames: route.policyBindings?.map((binding) => binding.policyName) ?? [],
      policySettings: policySettingsFromBindings(route.policyBindings ?? []),
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
    setNotice(`已删除路由：${formatRouteMatch(deleteCandidate)}`);
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

    setNotice(`已启用路由：${formatRouteMatch(route)}`);
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

    setNotice(`已停用路由：${formatRouteMatch(disableCandidate)}`);
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
      setSelectedRouteId(routeIdFromPayload(publishPayload));
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
        subtitle={selectedRouteView ? formatRouteMatch(selectedRouteView) : '未选择路由'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedRouteView ? (
            <>
              <Tabs tabs={detailTabs} active={tab} onChange={setTab} />
              <div style={{ height: 12 }} />
              {renderRouteDetail(tab, selectedRouteView)}
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
            <div className="toolbar">
              <Button variant="primary" disabled={!activeValidation.valid || submitting} onClick={saveRoute}>{submitting ? '保存中...' : '保存路由'}</Button>
            </div>
          </div>
          <RouteMatchHeader draft={draft} />
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
                    {gatewayOptions.map((gatewayName) => (
                      <option key={gatewayName} value={gatewayName}>{gatewayName}</option>
                    ))}
                  </select>
                </label>
                <label className="query-control">
                  <span>目标服务</span>
                  <select value={filterDraft.serviceName} onChange={(event) => updateFilterDraft({ serviceName: event.target.value })}>
                    <option value="all">全部服务</option>
                    {serviceOptions.map((serviceName) => (
                      <option key={serviceName} value={serviceName}>{serviceName}</option>
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
          {notice ? (
            <div className="page-notice" role="status">
              <span />
              {notice}
              <button type="button" onClick={() => setNotice(null)} aria-label="关闭提示">×</button>
            </div>
          ) : null}
          {deleteCandidate ? (
            <div className="confirm-overlay" role="presentation" onMouseDown={() => {
              if (!deleting) {
                setDeleteCandidate(null);
              }
            }}>
              <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-route-title" onMouseDown={(event) => event.stopPropagation()}>
                <h3 id="delete-route-title">删除路由</h3>
                <p>确定删除 {formatRouteMatch(deleteCandidate)}？删除后这条匹配规则不会再进入目标服务。</p>
                <div className="confirm-meta">
                  <span>所属网关</span><strong>{formatGatewayNames(deleteCandidate.gatewayNames)}</strong>
                  <span>目标服务</span><strong>{deleteCandidate.serviceName}</strong>
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
                <p>停用 {formatRouteMatch(disableCandidate)} 后，命中该规则的请求将不再转发到目标服务。</p>
                <div className="confirm-meta">
                  <span>匹配 Host</span><strong>{formatHostnames(disableCandidate.hostnames)}</strong>
                  <span>目标服务</span><strong>{disableCandidate.serviceName}</strong>
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

function RouteMatchHeader({ draft }: { draft: RouteComposerDraft }) {
  return (
    <section className="route-match-header">
      <div className="route-match-title">
        <div>
          <span>匹配条件</span>
          <strong>{formatMethods(draft.methods)} {draft.path || '/'}</strong>
        </div>
      </div>
      <div className="route-match-strip">
        <SummaryPill label="所属网关" value={formatGatewayNames(draft.gatewayNames)} />
        <SummaryPill label="Host" value={formatHostnames(draft.hostnames)} />
        <SummaryPill label="目标服务" value={draft.serviceName || '-'} />
        <SummaryPill label="策略" value={draft.enabledPolicyNames.length > 0 ? `${draft.enabledPolicyNames.length} 个` : '未绑定'} />
      </div>
    </section>
  );
}

function SummaryPill({ label, value }: { label: string; value: string }) {
  return (
    <div className="route-summary-pill">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function findPolicyValidationMessage(composer: RouteComposerPreview, draft: RouteComposerDraft) {
  const errors = validateEnabledPolicySettings(composer.policies, draft.enabledPolicyNames, draft.policySettings);
  const invalidPolicyName = draft.enabledPolicyNames.find((policyName) => Object.keys(errors[policyName] ?? {}).length > 0);
  return invalidPolicyName ? `请补齐策略参数：${invalidPolicyName}` : '';
}

function renderRouteTable(
  routes: RouteResource[],
  selectedRouteId: string | undefined,
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
            <th>匹配条件</th>
            <th>所属网关</th>
            <th>目标服务</th>
            <th>策略</th>
            <th>启用状态</th>
            <th>生效状态</th>
            <th>最近变更</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {routes.map((route) => (
            <tr key={route.id} className={route.id === selectedRouteId ? 'selected' : ''} onClick={() => onSelect(route.id)}>
              <td>
                <div className="table-primary">
                  <Badge tone={route.methods.includes('POST') ? 'green' : 'amber'}>{formatMethods(route.methods)}</Badge> {route.path}
                </div>
                <div className="table-secondary">Host: {formatHostnames(route.hostnames)}</div>
              </td>
              <td>
                <div className="table-primary">{formatGatewayNames(route.gatewayNames)}</div>
                <div className="table-secondary">{route.gatewayNames.length} 个网关</div>
              </td>
              <td>
                <div className="table-primary">{route.serviceName}</div>
                <div className="table-secondary">{route.traffic} req/s · 成功率 {route.successRate}</div>
              </td>
              <td>{route.policyCount > 0 ? `${route.policyCount} 个` : '未绑定'}</td>
              <td>
                <div className={`gateway-status ${routeEnabled(route) ? 'on' : ''}`.trim()}>
                  <button
                    className="gateway-switch"
                    type="button"
                    role="switch"
                    disabled={toggling}
                    aria-checked={routeEnabled(route)}
                    aria-label={`${formatRouteMatch(route)} ${routeEnabled(route) ? '已启用' : '已停用'}`}
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
              <td>
                <Badge tone={statusTone(route.runtimeStatus)}>
                  {runtimeSyncStatusLabel(route.runtimeStatus)}
                </Badge>
              </td>
              <td>{route.lastChangedAt}</td>
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
  gatewayOptions: string[];
  onDraftChange: (draft: RouteComposerDraft) => void;
}) {
  const updateDraft = (patch: Partial<RouteComposerDraft>) => {
    onDraftChange({ ...draft, ...patch });
  };
  const fieldErrors = routeFieldErrors(validation);
  const selectedTarget = composer.targets.find((target) => target.name === draft.selectedTargetName) ?? composer.targets.find((target) => target.name === draft.serviceName);

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
          <SectionTitle title="基础信息" description="定义这条路由在控制台和运行时中的基本身份。" />
          <div className="field-grid">
            <InputField
              label="路由标识"
              value={draft.id ?? ''}
              maxLength={63}
              disabled={Boolean(draft.version)}
              placeholder="留空时按服务、方法和路径自动生成"
              info="路由标识保存后会作为资源名称使用；建议只在创建时填写，需符合 DNS 标签格式"
              onChange={(value) => updateDraft({ id: value })}
            />
            <div className="field">
              <FieldLabel label="启用状态" info="关闭后路由配置保留，但不会下发生效" />
              <div className={`gateway-status ${draft.enabled ? 'on' : ''}`.trim()}>
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
          <SectionTitle title="匹配条件" description="定义请求如何进入这条路由；Host 可留空，表示不限制 Host。" />
          <div className="field-grid">
            <GatewayMultiSelect options={gatewayOptions} value={draft.gatewayNames} error={fieldErrors.gateway} onChange={(gatewayNames) => updateDraft({ gatewayNames })} />
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
          </div>
        </section>

        <section id="route-target" className="detail-card composer-card route-form-section">
          <SectionTitle title="目标服务" description="选择路由命中后要转发到的服务；第一阶段先支持一个目标服务。" />
          <TargetSelector
            targets={composer.targets}
            selectedTargetName={draft.selectedTargetName}
            error={fieldErrors.service}
            onTargetChange={(targetName) => updateDraft({ selectedTargetName: targetName, serviceName: targetName })}
          />
        </section>

        <section id="route-policies" className="detail-card composer-card route-form-section">
          <SectionTitle title="策略配置" description="按能力开关启用路由级治理策略；未启用的能力不会下发到运行时。" />
          <RoutePolicyBindings
            policies={composer.policies}
            selectedTarget={selectedTarget}
            enabledPolicyNames={draft.enabledPolicyNames}
            settings={draft.policySettings}
            onAddPolicy={(policyName) => {
              if (draft.enabledPolicyNames.includes(policyName)) {
                return;
              }

              updateDraft({ enabledPolicyNames: [...draft.enabledPolicyNames, policyName] });
            }}
            onRemovePolicy={(policyName) => updateDraft({ enabledPolicyNames: draft.enabledPolicyNames.filter((name) => name !== policyName) })}
            onSettingChange={(policyName, key, value) => updateDraft({
              policySettings: {
                ...draft.policySettings,
                [policyName]: {
                  ...(draft.policySettings[policyName] ?? {}),
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
    path: validation.items.find((item) => item.label === '匹配规则' && item.status === 'critical')?.message,
    service: validation.items.find((item) => item.label === '目标服务' && item.status === 'critical')?.message,
    gateway: validation.items.find((item) => item.label === '网关' && item.status === 'critical')?.message,
    host: validation.items.find((item) => item.label === '匹配域名' && item.status === 'critical')?.message,
  };
}

function SectionTitle({ title, description }: { title: string; description: string }) {
  return (
    <div className="route-section-title">
      <div>
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

function renderRouteDetail(tab: string, selectedRoute: RoutePageView['routes'][number] | undefined) {
  const details: KeyValue[] = selectedRoute ? routeDetailsForTab(selectedRoute, tab) : [];

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

function routeDetailsForTab(route: RoutePageView['routes'][number], tab: string): KeyValue[] {
  if (tab === 'match') {
    return [
      { label: '方法', value: formatMethods(route.methods) },
      { label: '路径', value: route.path },
      { label: '所属网关', value: formatGatewayNames(route.gatewayNames) },
      { label: '匹配域名', value: formatHostnames(route.hostnames) },
    ];
  }

  if (tab === 'target') {
    return [
      { label: '目标服务', value: route.serviceName },
      { label: '请求量', value: `${route.traffic} req/s` },
      { label: '成功率', value: route.successRate },
    ];
  }

  if (tab === 'events') {
    return [
      { label: '最近变更', value: route.lastChangedAt },
      { label: '启用状态', value: route.enabled ? '启用' : '停用' },
      { label: '生效状态', value: runtimeSyncStatusLabel(route.runtimeStatus) },
      { label: '变更摘要', value: formatRouteMatch(route) },
    ];
  }

  return [
    { label: '方法', value: formatMethods(route.methods) },
    { label: '路径', value: route.path },
    { label: '所属网关', value: formatGatewayNames(route.gatewayNames) },
    { label: '目标服务', value: route.serviceName },
    { label: '流量', value: `${route.traffic} req/s` },
    { label: '成功率', value: route.successRate },
    { label: '启用状态', value: route.enabled ? '启用' : '停用' },
    { label: '生效状态', value: runtimeSyncStatusLabel(route.runtimeStatus) },
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

function GatewayMultiSelect({ options, value, error, onChange }: { options: string[]; value: string[]; error?: string; onChange: (value: string[]) => void }) {
  return (
    <div className={`field field-wide route-relation-field ${error ? 'invalid' : ''}`.trim()}>
      <MultiSelectDropdown
        label="所属网关"
        emptyLabel="请选择网关"
        required
        info="一个路由可以挂到多个网关；保存后会分别在这些网关下生效"
        options={options.map((gatewayName) => ({ value: gatewayName, label: gatewayName }))}
        value={value}
        onChange={onChange}
      />
      {error ? <div className="form-error">{error}</div> : null}
      <div className="mini-card-meta">网关与路由是多对多关系；路由命中后再转发到一个目标服务。</div>
    </div>
  );
}

function TargetSelector({
  targets,
  selectedTargetName,
  error,
  onTargetChange,
}: {
  targets: RouteComposerPreview['targets'];
  selectedTargetName: string;
  error?: string;
  onTargetChange: (targetName: string) => void;
}) {
  const selectedTarget = targets.find((target) => target.name === selectedTargetName) ?? targets[0];

  return (
    <div className="target-config">
      <label className={`field ${error ? 'invalid' : ''}`.trim()}>
        <FieldLabel label="目标服务" required info="当前路由命中后转发到这个服务；后续会扩展多目标和权重" />
        <select value={selectedTargetName} onChange={(event) => onTargetChange(event.target.value)}>
          {targets.map((target) => (
            <option key={target.name} value={target.name}>{target.name} · {serviceTypeLabel(target.type)}</option>
          ))}
        </select>
        {error ? <div className="form-error">{error}</div> : null}
      </label>
      <div className="target-service-card">
        <div className="target-service-head">
          <div>
            <span className="mini-card-meta">服务信息</span>
            <strong>{selectedTarget?.name ?? '-'}</strong>
          </div>
          <Badge tone={selectedTarget?.healthStatus === 'healthy' ? 'green' : selectedTarget?.healthStatus === 'warning' ? 'amber' : selectedTarget?.healthStatus === 'unknown' ? 'neutral' : 'red'}>
            {healthLabel(selectedTarget?.healthStatus ?? 'unknown')}
          </Badge>
        </div>
        <div className="target-service-grid">
          <span>类型</span>
          <strong>{selectedTarget ? serviceTypeLabel(selectedTarget.type) : '-'}</strong>
          <span>地址</span>
          <strong>{selectedTarget?.endpoint ?? '未配置'}</strong>
          <span>近况</span>
          <strong>{selectedTarget?.meta ?? '-'}</strong>
          <span>引用路由</span>
          <strong>{typeof selectedTarget?.referencedRoutes === 'number' ? `${selectedTarget.referencedRoutes} 条` : '-'}</strong>
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
  const toggleValue = (nextValue: string) => {
    onChange(value.includes(nextValue) ? value.filter((item) => item !== nextValue) : [...value, nextValue]);
  };
  const displayValue = value.length > 0
    ? options.filter((option) => value.includes(option.value)).map((option) => option.label).join('、')
    : emptyLabel;

  return (
    <div className="field">
      <FieldLabel label={label} required={required} info={info} />
      <details className="multi-select">
        <summary>
          <span>{displayValue}</span>
          <span aria-hidden="true">⌄</span>
        </summary>
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
      </details>
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

function formatHostnames(hostnames: string[]) {
  return hostnames.length > 0 ? hostnames.join('、') : '不限制 Host';
}

function formatGatewayNames(gatewayNames: string[]) {
  return gatewayNames.length > 0 ? gatewayNames.join('、') : '-';
}

function formatMethods(methods: string[]) {
  return methods.length > 0 ? methods.join('、') : '全部方法';
}

function formatRouteMatch(route: Pick<RouteResource, 'methods' | 'path'>) {
  return `${formatMethods(route.methods)} ${route.path}`;
}

function routeIdFromPayload(payload: ReturnType<typeof buildRoutePublishPayload>) {
  if (payload.id) {
    return payload.id;
  }

  const method = payload.methods[0] ?? 'any';
  const value = `${payload.serviceName}-${method}-${payload.path}`
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 63)
    .replace(/-+$/g, '');

  return value || 'route';
}

function policySettingsFromBindings(bindings: NonNullable<RouteResource['policyBindings']>): RouteComposerDraft['policySettings'] {
  return Object.fromEntries(bindings.map((binding) => [
    binding.policyName,
    Object.fromEntries(Object.entries(binding.parameters).map(([key, value]) => [
      key,
      Array.isArray(value) ? value.map(String).join(',') : String(value),
    ])),
  ]));
}

function RoutePolicyBindings({
  policies,
  selectedTarget,
  enabledPolicyNames,
  settings,
  onAddPolicy,
  onRemovePolicy,
  onSettingChange,
}: {
  policies: RouteComposerPreview['policies'];
  selectedTarget?: RouteComposerPreview['targets'][number];
  enabledPolicyNames: string[];
  settings: Record<string, Record<string, string>>;
  onAddPolicy: (policyName: string) => void;
  onRemovePolicy: (policyName: string) => void;
  onSettingChange: (policyName: string, key: string, value: string) => void;
}) {
  const applicablePolicies = policies.filter((policy) => policyAppliesToTarget(policy.name, selectedTarget));
  const enabledPolicySet = new Set(enabledPolicyNames);
  const enabledCount = applicablePolicies.filter((policy) => enabledPolicySet.has(policy.name)).length;
  const policyErrors = validateEnabledPolicySettings(applicablePolicies, enabledPolicyNames, settings);

  const togglePolicy = (policyName: string) => {
    if (enabledPolicySet.has(policyName)) {
      onRemovePolicy(policyName);
      return;
    }

    onAddPolicy(policyName);
  };

  return (
    <div className="route-policy-bindings">
      <div className="policy-bindings-head">
        <div>
          <h4>路由策略</h4>
          <p>按当前目标服务选择治理能力；未启用的能力不会下发到运行时。</p>
        </div>
        <Badge tone={enabledCount > 0 ? 'green' : 'neutral'}>已启用 {enabledCount} 个</Badge>
      </div>

      {applicablePolicies.length > 0 ? (
        <div className="route-policy-capability-list">
          {applicablePolicies.map((policy) => {
            const enabled = enabledPolicySet.has(policy.name);
            const policySettings = settings[policy.name] ?? {};
            const errors = enabled ? policyErrors[policy.name] ?? {} : {};

            return (
              <article key={policy.name} className={`route-policy-capability ${enabled ? 'enabled' : ''}`.trim()}>
                <div className="route-policy-capability-head">
                  <button className="route-policy-toggle" type="button" onClick={() => togglePolicy(policy.name)}>
                    <span className={`switch ${enabled ? 'on' : ''}`} aria-hidden="true" />
                    <span>
                      <strong>{policy.name}</strong>
                      <small>{policy.meta}</small>
                    </span>
                  </button>
                  <Badge tone={isAiPolicy(policy.name) ? 'amber' : 'neutral'}>{policyCategory(policy.name)}</Badge>
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
                          onChange={(value) => onSettingChange(policy.name, param.key, value)}
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

  if (policy.name.includes('Header')) {
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
  enabledPolicyNames: string[],
  settings: RouteComposerDraft['policySettings'],
): Record<string, Record<string, string>> {
  const errors: Record<string, Record<string, string>> = {};

  enabledPolicyNames.forEach((policyName) => {
    const policy = policies.find((item) => item.name === policyName);
    if (!policy) {
      return;
    }
    errors[policyName] = validatePolicySettings(policy, settings[policyName] ?? {});
  });

  const retryErrors = errors[routePolicyRetryName];
  if (!retryErrors || !enabledPolicyNames.includes(routePolicyRetryName)) {
    return errors;
  }

  const totalTimeoutMillis = numericPolicySetting(settings[routePolicyTimeoutName], 'timeoutMillis', defaultRouteTimeoutMillis);
  const perTryTimeoutMillis = numericPolicySetting(settings[routePolicyRetryName], 'perTryTimeoutMillis', 0);
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

function policyCategory(policyName: string): string {
  if (policyName.includes('认证')) {
    return '访问控制';
  }
  if (policyName.includes('限流') || policyName.includes('配额') || policyName.includes('Token')) {
    return '流量控制';
  }
  if (policyName.includes('校验') || policyName.includes('安全')) {
    return '安全防护';
  }
  if (policyName.includes('Header')) {
    return '请求处理';
  }
  if (policyName.includes('超时') || policyName.includes('重试')) {
    return '可靠性';
  }

  return '通用策略';
}

function policyAppliesToTarget(policyName: string, target?: RouteComposerPreview['targets'][number]): boolean {
  return !isAiPolicy(policyName) || isAiTarget(target);
}

function isAiTarget(target?: RouteComposerPreview['targets'][number]): boolean {
  return Boolean(target?.type.includes('模型') || target?.type.includes('Agent') || target?.type.includes('MCP'));
}

function isAiPolicy(policyName: string): boolean {
  return policyName.includes('Token') || policyName.includes('Prompt') || policyName.includes('模型');
}
