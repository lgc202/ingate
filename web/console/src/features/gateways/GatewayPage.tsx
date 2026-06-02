import { useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import { healthLabel, runtimeSyncStatusLabel, statusTone } from '@/domain/common';
import type { Gateway, GatewayCertificateOption, GatewayListener, GatewayMutationPayload, GatewayValidationReport } from '@/domain/gateway';
import type { GatewayFormDraft } from './form';
import {
  buildGatewayPayload,
  createGatewayDraft,
  createGatewayListener,
  formatHostnames,
  formatListeners,
  normalizeHostnames,
  parseHostnames,
  validateGatewayDraft,
} from './form';
import type { GatewayHostMode } from './form';

const loadGateways = () => consoleRepository.listGateways();
type GatewayPanelMode = 'list' | 'detail' | 'create' | 'edit';
type GatewayEnabledFilter = 'all' | 'enabled' | 'disabled';

interface GatewayFilters {
  keyword: string;
  host: string;
  enabled: GatewayEnabledFilter;
}

const emptyGatewayFilters: GatewayFilters = {
  keyword: '',
  host: '',
  enabled: 'all',
};

function mergeGateways(baseGateways: Gateway[], savedGateways: Record<string, Gateway>) {
  const merged = baseGateways.map((gateway) => savedGateways[gateway.id] ?? gateway);
  const existingIds = new Set(merged.map((gateway) => gateway.id));
  const created = Object.values(savedGateways).filter((gateway) => !existingIds.has(gateway.id));

  return [...created, ...merged];
}

function buildGatewayFromPayload(payload: GatewayMutationPayload, original: Gateway | null): Gateway {
  const listenerSummary = payload.listeners.map((listener) => `${listener.protocol}:${listener.port || '-'}`).join(' / ');
  const id = payload.id ?? payload.name;

  return {
    id,
    version: undefined,
    name: payload.name,
    description: payload.description || original?.description || '未填写描述',
    runtimeGroupId: payload.runtimeGroupId,
    runtimeGroupName: payload.runtimeGroupName,
    listeners: listenerSummary,
    listenerItems: payload.listeners,
    hostPolicy: payload.hostnames.length > 0 ? payload.hostnames.join('、') : '不限制 Host',
    hostnames: payload.hostnames,
    routeCount: original?.routeCount ?? 0,
    serviceCount: original?.serviceCount ?? 0,
    enabled: original?.enabled ?? true,
    runtimeStatus: 'unknown',
    healthStatus: original?.healthStatus ?? 'unknown',
    latestSnapshotVersion: original?.latestSnapshotVersion,
    lastChangedAt: '刚刚',
  };
}

export function GatewayPage() {
  const gateways = useResource(loadGateways);
  const [selectedGatewayId, setSelectedGatewayId] = useState('gw-public');
  const [panelMode, setPanelMode] = useState<GatewayPanelMode>('list');
  const [filterDraft, setFilterDraft] = useState<GatewayFilters>(emptyGatewayFilters);
  const [filters, setFilters] = useState<GatewayFilters>(emptyGatewayFilters);
  const [savedGateways, setSavedGateways] = useState<Record<string, Gateway>>({});
  const [hiddenGatewayIds, setHiddenGatewayIds] = useState<string[]>([]);
  const [enabledOverrides, setEnabledOverrides] = useState<Record<string, boolean>>({});
  const [draftState, setDraftState] = useState<GatewayFormDraft | null>(null);
  const [serverValidation, setServerValidation] = useState<GatewayValidationReport | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Gateway | null>(null);
  const [disableCandidate, setDisableCandidate] = useState<Gateway | null>(null);

  if (gateways.loading) {
    return (
      <PageFrame title="流量 / 网关" subtitle="管理流量入口、监听器和 Host 策略">
        <ResourceStatePanel title="加载网关数据" message="正在读取网关列表。" />
      </PageFrame>
    );
  }

  if (gateways.error || !gateways.data) {
    return (
      <PageFrame title="流量 / 网关" subtitle="管理流量入口、监听器和 Host 策略">
        <ResourceStatePanel title="网关数据加载失败" message={gateways.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const allGateways = mergeGateways(gateways.data.gateways, savedGateways);
  const availableGateways = allGateways.filter((gateway) => !hiddenGatewayIds.includes(gateway.id));
  const selectedGateway = availableGateways.find((gateway) => gateway.id === selectedGatewayId) ?? availableGateways[0] ?? null;
  const gatewayEnabled = (gateway: Gateway) => enabledOverrides[gateway.id] ?? gateway.enabled;
  const selectedGatewayView = selectedGateway ? { ...selectedGateway, enabled: gatewayEnabled(selectedGateway) } : null;
  const visibleGateways = availableGateways.filter((gateway) => {
    const keyword = filters.keyword.trim().toLowerCase();
    const host = filters.host.trim().toLowerCase();
    const matchedKeyword = !keyword || [gateway.name, gateway.description].some((value) => value.toLowerCase().includes(keyword));
    const matchedHost = !host || [gateway.hostPolicy, ...gateway.hostnames].some((value) => value.toLowerCase().includes(host));
    const matchedEnabled = filters.enabled === 'all' || (filters.enabled === 'enabled' ? gatewayEnabled(gateway) : !gatewayEnabled(gateway));

    return matchedKeyword && matchedHost && matchedEnabled;
  });
  const hasActiveFilters = Boolean(filters.keyword.trim() || filters.host.trim() || filters.enabled !== 'all');
  const draft = draftState ?? createGatewayDraft(panelMode === 'edit' ? selectedGateway : null);
  const clientValidation = validateGatewayDraft(draft);
  const activeValidation = serverValidation ?? clientValidation;
  const payload = buildGatewayPayload(draft);
  const openCreate = () => {
    setPanelMode('create');
    setDraftState(createGatewayDraft(null));
    setServerValidation(null);
    setNotice(null);
  };

  const openEdit = (gateway: Gateway) => {
    setSelectedGatewayId(gateway.id);
    setPanelMode('edit');
    setDraftState(createGatewayDraft(gateway));
    setServerValidation(null);
    setNotice(null);
  };

  const requestDeleteGateway = (gateway: Gateway) => {
    if (gateway.routeCount > 0) {
      return;
    }

    setDeleteCandidate(gateway);
  };

  const confirmDeleteGateway = async () => {
    if (!deleteCandidate) {
      return;
    }

    try {
      await consoleRepository.deleteGateway(deleteCandidate.id);
      setHiddenGatewayIds((ids) => [...ids, deleteCandidate.id]);
      setSelectedGatewayId((current) => {
        if (current !== deleteCandidate.id) {
          return current;
        }

        return availableGateways.find((gateway) => gateway.id !== deleteCandidate.id)?.id ?? '';
      });
      setNotice(`已删除网关：${deleteCandidate.name}`);
      setDeleteCandidate(null);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '删除网关失败');
    }
  };

  const toggleGatewayEnabled = async (gateway: Gateway) => {
    if (gatewayEnabled(gateway)) {
      setDisableCandidate(gateway);
      return;
    }

    try {
      await consoleRepository.setGatewayEnabled(gateway.id, true);
      setEnabledOverrides((current) => ({ ...current, [gateway.id]: true }));
      setNotice(`已启用网关：${gateway.name}`);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '启用网关失败');
    }
  };

  const confirmDisableGateway = async () => {
    if (!disableCandidate) {
      return;
    }

    try {
      await consoleRepository.setGatewayEnabled(disableCandidate.id, false);
      setEnabledOverrides((current) => ({ ...current, [disableCandidate.id]: false }));
      setNotice(`已停用网关：${disableCandidate.name}`);
      setDisableCandidate(null);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '停用网关失败');
    }
  };

  const updateFilterDraft = (patch: Partial<GatewayFilters>) => {
    setFilterDraft((current) => ({ ...current, ...patch }));
  };

  const resetFilters = () => {
    setFilterDraft(emptyGatewayFilters);
    setFilters(emptyGatewayFilters);
  };

  const updateDraft = (patch: Partial<GatewayFormDraft>) => {
    setDraftState({ ...draft, ...patch });
    setServerValidation(null);
  };

  const handleGatewaySubmit = async () => {
    const validation = await consoleRepository.validateGatewayDraft(payload);
    setServerValidation(validation);

    if (!validation.valid) {
      return;
    }

    try {
      await consoleRepository.saveGatewayDraft(payload);
      const originalGateway = panelMode === 'edit' ? selectedGatewayView : null;
      const savedGateway = buildGatewayFromPayload(payload, originalGateway);

      setSavedGateways((current) => ({ ...current, [savedGateway.id]: savedGateway }));
      setHiddenGatewayIds((ids) => ids.filter((id) => id !== savedGateway.id));
      setSelectedGatewayId(savedGateway.id);
      setNotice(`网关已保存：${payload.name}`);
      setPanelMode('list');
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '保存网关失败');
    }
  };

  if (panelMode === 'detail') {
    return (
      <PageFrame
        title="网关详情"
        subtitle={selectedGatewayView?.name ?? '未选择网关'}
        actions={<Button variant="soft" onClick={() => setPanelMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedGatewayView ? <GatewayDetail gateway={selectedGatewayView} /> : null}
        </Panel>
      </PageFrame>
    );
  }

  if (panelMode !== 'list') {
    return (
      <PageFrame
        title={panelMode === 'create' ? '新建网关' : '编辑网关'}
        subtitle="配置流量入口、监听端口和 Host 匹配策略"
        actions={<Button variant="soft" onClick={() => setPanelMode('list')}>返回列表</Button>}
      >
        <section className="editor-layout">
          <GatewayFormPanel
            mode={panelMode}
            draft={draft}
            validation={activeValidation}
            originalGateway={panelMode === 'edit' ? selectedGateway : null}
            certificates={gateways.data.certificates}
            onDraftChange={updateDraft}
            onSubmit={handleGatewaySubmit}
            onCancel={() => setPanelMode('list')}
          />
        </section>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="流量 / 网关"
      subtitle="管理流量入口、监听器和 Host 策略"
      actions={
        <Button variant="primary" onClick={openCreate}>新建网关</Button>
      }
    >
        <Panel title="网关列表">
          <div className="gateway-query">
            <div className="gateway-query-grid">
              <label className="query-control">
                <span>网关名称</span>
                <input value={filterDraft.keyword} placeholder="请输入名称或描述" onChange={(event) => updateFilterDraft({ keyword: event.target.value })} />
              </label>
              <label className="query-control">
                <span>Host 匹配</span>
                <input value={filterDraft.host} placeholder="请输入域名或通配符" onChange={(event) => updateFilterDraft({ host: event.target.value })} />
              </label>
              <label className="query-control">
                  <span>启用状态</span>
                <select value={filterDraft.enabled} onChange={(event) => updateFilterDraft({ enabled: event.target.value as GatewayEnabledFilter })}>
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
          <div style={{ overflow: 'auto' }}>
            <table className="table">
              <thead>
                <tr>
                  <th>网关名称</th>
                  <th>监听器</th>
                  <th>Host 策略</th>
                  <th>路由数</th>
                  <th>服务数</th>
                  <th>启用状态</th>
                  <th>生效状态</th>
                  <th>最近变更</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleGateways.map((gateway) => (
                  <tr
                    key={gateway.id}
                    className={gateway.id === selectedGateway?.id ? 'selected' : ''}
                    onClick={() => setSelectedGatewayId(gateway.id)}
                  >
                    <td>
                      <div className="table-primary">{gateway.name}</div>
                      <div className="table-secondary">{gateway.description}</div>
                    </td>
                    <td>
                      <div className="table-primary">{gateway.listeners}</div>
                      <div className="table-secondary">{gateway.listenerItems.filter((listener) => listener.protocol === 'HTTPS').length} 个 HTTPS</div>
                    </td>
                    <td>
                      <div className="table-primary">{gateway.hostPolicy}</div>
                      <div className="table-secondary">{gateway.hostnames.length > 0 ? `${gateway.hostnames.length} 个 Host` : '进入路由匹配'}</div>
                    </td>
                    <td>{gateway.routeCount}</td>
                    <td>{gateway.serviceCount}</td>
                    <td>
                      <div className={`gateway-status ${gatewayEnabled(gateway) ? 'on' : ''}`.trim()}>
                        <button
                          className="gateway-switch"
                          type="button"
                          role="switch"
                          aria-checked={gatewayEnabled(gateway)}
                          aria-label={`${gateway.name} ${gatewayEnabled(gateway) ? '已启用' : '已停用'}`}
                          onClick={(event) => {
                            event.stopPropagation();
                            toggleGatewayEnabled(gateway);
                          }}
                        >
                          <span aria-hidden="true" />
                        </button>
                        <strong>{gatewayEnabled(gateway) ? '启用' : '停用'}</strong>
                      </div>
                    </td>
                    <td>
                      <Badge tone={statusTone(gateway.runtimeStatus)}>
                        {runtimeSyncStatusLabel(gateway.runtimeStatus)}
                      </Badge>
                    </td>
                    <td>{gateway.lastChangedAt}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setSelectedGatewayId(gateway.id);
                          setPanelMode('detail');
                        }}>详情</button>
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          openEdit(gateway);
                        }}>编辑</button>
                        <button className="link-button danger" type="button" onClick={(event) => {
                          event.stopPropagation();
                          requestDeleteGateway(gateway);
                        }} disabled={gateway.routeCount > 0} title={gateway.routeCount > 0 ? '仍有关联路由，不能删除' : undefined}>删除</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {visibleGateways.length === 0 ? (
              <div className="table-empty">
                <EmptyState
                  title={hasActiveFilters ? '没有匹配的网关' : '暂无网关'}
                  message={hasActiveFilters ? '调整查询条件后再试，或重置筛选查看全部网关。' : '当前还没有流量入口，可以先新建一个网关。'}
                />
              </div>
            ) : null}
          </div>
        </Panel>
        {notice ? (
          <div className="page-notice" role="status">
            <span />
            {notice}
            <button type="button" onClick={() => setNotice(null)} aria-label="关闭提示">×</button>
          </div>
        ) : null}
        {deleteCandidate ? (
          <div className="confirm-overlay" role="presentation" onMouseDown={() => setDeleteCandidate(null)}>
            <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-gateway-title" onMouseDown={(event) => event.stopPropagation()}>
              <h3 id="delete-gateway-title">删除网关</h3>
              <p>确定删除 {deleteCandidate.name}？如果后续接入真实后端，仍有关联路由时会拒绝删除。</p>
              <div className="confirm-meta">
                <span>监听器</span><strong>{deleteCandidate.listeners}</strong>
                <span>关联路由</span><strong>{deleteCandidate.routeCount} 条</strong>
              </div>
              <div className="confirm-actions">
                <Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button>
                <Button variant="primary" onClick={confirmDeleteGateway}>确认删除</Button>
              </div>
            </div>
          </div>
        ) : null}
        {disableCandidate ? (
          <div className="confirm-overlay" role="presentation" onMouseDown={() => setDisableCandidate(null)}>
            <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="disable-gateway-title" onMouseDown={(event) => event.stopPropagation()}>
              <h3 id="disable-gateway-title">停用网关</h3>
              <p>停用 {disableCandidate.name} 后，关联入口将不再承载流量。请确认关联路由和服务已迁移或可以暂停访问。</p>
              <div className="confirm-meta">
                <span>监听器</span><strong>{disableCandidate.listeners}</strong>
                <span>关联路由</span><strong>{disableCandidate.routeCount} 条</strong>
                <span>关联服务</span><strong>{disableCandidate.serviceCount} 个</strong>
              </div>
              <div className="confirm-actions">
                <Button variant="ghost" onClick={() => setDisableCandidate(null)}>取消</Button>
                <Button variant="primary" onClick={confirmDisableGateway}>确认停用</Button>
              </div>
            </div>
          </div>
        ) : null}
    </PageFrame>
  );
}

function GatewayFormPanel({
  mode,
  draft,
  validation,
  originalGateway,
  certificates,
  onDraftChange,
  onSubmit,
  onCancel,
}: {
  mode: GatewayPanelMode;
  draft: GatewayFormDraft;
  validation: GatewayValidationReport;
  originalGateway: Gateway | null;
  certificates: GatewayCertificateOption[];
  onDraftChange: (patch: Partial<GatewayFormDraft>) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const fieldErrors = gatewayFieldErrors(validation);

  return (
    <Panel title={mode === 'create' ? '新建网关' : '编辑网关'} subtitle={draft.name}>
      <div className="editor-grid form-only">
        <div className="editor-main-stack">
          <section className="form-section">
            <div className="form-section-title">
              <h3>基础信息</h3>
              <p>用于识别这个入口的业务用途。</p>
            </div>
            <div className="field-grid">
              <InputField label="网关名称" value={draft.name} error={fieldErrors.name} onChange={(value) => onDraftChange({ name: value })} />
              <InputField label="描述" value={draft.description} onChange={(value) => onDraftChange({ description: value })} />
            </div>
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>监听器</h3>
              <p>配置入口协议、端口和 HTTPS 证书。</p>
            </div>
            <GatewayListenerEditor
              value={draft.listeners}
              certificates={certificates}
              listenerError={fieldErrors.listeners}
              certificateError={fieldErrors.certificate}
              onChange={(listeners) => onDraftChange({ listeners })}
            />
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>Host 策略</h3>
              <p>决定网关是否先按请求 Host 做入口过滤。</p>
            </div>
            <GatewayHostnameEditor
              mode={draft.hostMode}
              value={draft.hostnames}
              error={fieldErrors.host}
              onModeChange={(hostMode) => onDraftChange({ hostMode, hostnames: hostMode === 'any' ? [] : draft.hostnames })}
              onChange={(hostnames) => onDraftChange({ hostnames, hostMode: 'specified' })}
            />
          </section>
        </div>
        <div className="form-actions">
          <Button variant="primary" disabled={!validation.valid} onClick={onSubmit}>保存网关</Button>
          <Button variant="ghost" onClick={onCancel}>取消</Button>
        </div>
      </div>
    </Panel>
  );
}

function gatewayFieldErrors(validation: GatewayValidationReport) {
  return {
    name: validation.items.find((item) => item.label === '网关名称' && item.status === 'critical')?.message,
    listeners: validation.items.find((item) => item.label === '监听器' && item.status === 'critical')?.message,
    certificate: validation.items.find((item) => item.label === 'HTTPS 证书' && item.status === 'critical')?.message,
    host: validation.items.find((item) => item.label === 'Host 策略' && item.status === 'critical')?.message,
  };
}

function GatewayListenerEditor({
  value,
  certificates,
  listenerError,
  certificateError,
  onChange,
}: {
  value: GatewayListener[];
  certificates: GatewayCertificateOption[];
  listenerError?: string;
  certificateError?: string;
  onChange: (listeners: GatewayListener[]) => void;
}) {
  const updateListener = (listenerId: string, patch: Partial<GatewayListener>) => {
    onChange(value.map((listener) => listener.id === listenerId ? { ...listener, ...patch } : listener));
  };

  const removeListener = (listenerId: string) => {
    onChange(value.filter((listener) => listener.id !== listenerId));
  };

  return (
    <div className="listener-editor">
      <div className="listener-grid listener-grid-head">
        <span>协议</span>
        <span>端口</span>
        <span>证书</span>
        <span>操作</span>
      </div>
      {value.map((listener) => (
        <div className="listener-grid" key={listener.id}>
          <select
            value={listener.protocol}
            onChange={(event) => {
              const protocol = event.target.value as GatewayListener['protocol'];
              updateListener(listener.id, {
                protocol,
                certificateId: protocol === 'HTTPS' ? listener.certificateId : undefined,
                certificateName: protocol === 'HTTPS' ? listener.certificateName : undefined,
              });
            }}
          >
            <option value="HTTP">HTTP</option>
            <option value="HTTPS">HTTPS</option>
          </select>
          <input className={!listener.port.trim() || listenerError ? 'invalid-control' : ''} value={listener.port} onChange={(event) => updateListener(listener.id, { port: event.target.value })} />
          <select
            value={listener.certificateId ?? ''}
            disabled={listener.protocol !== 'HTTPS'}
            className={listener.protocol === 'HTTPS' && !listener.certificateId ? 'invalid-control' : ''}
            onChange={(event) => {
              const certificate = certificates.find((item) => item.id === event.target.value);
              updateListener(listener.id, { certificateId: certificate?.id, certificateName: certificate?.name });
            }}
          >
            <option value="">选择证书</option>
            {certificates.map((certificate) => (
              <option key={certificate.id} value={certificate.id}>{certificate.name}</option>
            ))}
          </select>
          <button
            className="link-button danger"
            type="button"
            disabled={value.length <= 1}
            title={value.length <= 1 ? '至少保留一个监听器' : undefined}
            onClick={() => removeListener(listener.id)}
          >删除</button>
        </div>
      ))}
      {listenerError ? <div className="form-error">{listenerError}</div> : null}
      {certificateError ? <div className="form-error">{certificateError}</div> : null}
      <button className="link-button" type="button" onClick={() => onChange([...value, createGatewayListener('HTTP', '')])}>添加监听器</button>
    </div>
  );
}

function GatewayHostnameEditor({
  mode,
  value,
  error,
  onModeChange,
  onChange,
}: {
  mode: GatewayHostMode;
  value: string[];
  error?: string;
  onModeChange: (mode: GatewayHostMode) => void;
  onChange: (value: string[]) => void;
}) {
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
    <div className={`gateway-host-editor ${error ? 'invalid' : ''}`.trim()}>
      <div className="gateway-host-mode" role="group" aria-label="Host 策略">
        <button className={mode === 'any' ? 'active' : ''} type="button" onClick={() => onModeChange('any')}>
          不限制 Host
        </button>
        <button className={mode === 'specified' ? 'active' : ''} type="button" onClick={() => onModeChange('specified')}>
          指定 Host
        </button>
      </div>
      {mode === 'specified' ? (
        <>
          <div className="gateway-host-input">
            <input
              value={inputValue}
              placeholder="api.example.com 或 *.example.com"
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
          <div className="tag-list">
            {value.length === 0 ? (
              <span className="host-empty">请添加至少一个 Host</span>
            ) : value.map((hostname) => (
              <button key={hostname} className="tag-chip" type="button" onClick={() => removeHostname(hostname)} title="点击移除">
                {hostname}
                <span aria-hidden="true">×</span>
              </button>
            ))}
          </div>
          {error ? <div className="form-error">{error}</div> : null}
        </>
      ) : (
        <span className="host-empty">当前不校验请求 Host，直接进入路由匹配。</span>
      )}
    </div>
  );
}

function InputField({ label, value, error, onChange }: { label: string; value: string; error?: string; onChange: (value: string) => void }) {
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <label>{label}</label>
      <input value={value} onChange={(event) => onChange(event.target.value)} />
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function GatewayDetail({ gateway }: { gateway: Gateway }) {
  return (
    <div className="section-grid">
      <div className="detail-card">
        <h4>入口信息</h4>
        <div className="kv">
          {[
            ['描述', gateway.description],
            ['启用状态', gateway.enabled ? '启用' : '停用'],
            ['生效状态', runtimeSyncStatusLabel(gateway.runtimeStatus)],
            ['健康状态', healthLabel(gateway.healthStatus)],
            ['最近变更', gateway.lastChangedAt],
          ].flatMap(([label, value]) => [
            <div key={`${label}-label`}>{label}</div>,
            <div key={`${label}-value`}>{value}</div>,
          ])}
        </div>
      </div>
      <div className="detail-card">
        <h4>运行归属</h4>
        <div className="kv">
          <div>运行组</div><div>{gateway.runtimeGroupName}</div>
          <div>运行组 ID</div><div>{gateway.runtimeGroupId}</div>
        </div>
      </div>
      <div className="detail-card">
        <h4>Host 策略</h4>
        <div className="mini-card-title">{gateway.hostnames.length > 0 ? '指定 Host' : '不限制 Host'}</div>
        <div className="tag-list">
          {gateway.hostnames.length > 0 ? gateway.hostnames.map((hostname) => (
            <span className="tag-chip static" key={hostname}>{hostname}</span>
          )) : (
            <span className="host-empty">不校验请求 Host，直接进入路由匹配。</span>
          )}
        </div>
      </div>
      <div className="detail-card">
        <h4>监听器</h4>
        <div className="drawer-list">
          {gateway.listenerItems.map((listener) => (
            <div className="legend-row" key={listener.id}>
              <span>{listener.protocol}:{listener.port}</span>
              <span className="mini-card-meta">{listener.protocol === 'HTTPS' ? listener.certificateName ?? '未配置证书' : 'HTTP'}</span>
            </div>
          ))}
        </div>
      </div>
      <div className="detail-card">
        <h4>关联范围</h4>
        <div className="kv">
          <div>关联路由</div><div>{gateway.routeCount} 条</div>
          <div>关联服务</div><div>{gateway.serviceCount} 个</div>
        </div>
      </div>
    </div>
  );
}
