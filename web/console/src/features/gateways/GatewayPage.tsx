import { useState } from 'react';
import { Link } from 'react-router-dom';
import { listCertificates } from '@/api/certificates';
import { deleteGateway, listGateways, saveGateway, setGatewayEnabled } from '@/api/gateways';
import { getPolicyWorkspace } from '@/api/policies';
import { useResource } from '@/api/useResource';
import { Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { Certificate } from '@/domain/certificate';
import type { Gateway, GatewayValidationReport } from '@/domain/gateway';
import type { PolicyWorkspace } from '@/domain/policy';
import { GovernanceBindingPanel } from '@/features/policies/GovernanceBindingPanel';
import type { GatewayFormDraft } from './form';
import {
  buildGatewayPayload,
  createGatewayDraft,
  GATEWAY_HTTP_PORT,
  GATEWAY_HTTPS_PORT,
  normalizeHostnames,
  parseHostnames,
  validateGatewayDraft,
} from './form';
import type { GatewayHostMode } from './form';

const loadGateways = () => listGateways();
const loadCertificates = () => listCertificates();
type GatewayPanelMode = 'list' | 'detail' | 'create' | 'edit';
type GatewayEnabledFilter = 'all' | 'enabled' | 'disabled';

interface GatewayFilters {
  keyword: string;
  host: string;
  enabled: GatewayEnabledFilter;
}

interface GatewayNotice {
  message: string;
  tone: 'success' | 'error';
}

const emptyGatewayFilters: GatewayFilters = {
  keyword: '',
  host: '',
  enabled: 'all',
};

export function GatewayPage() {
  const gateways = useResource(loadGateways);
  const certificates = useResource(loadCertificates);
  const policies = useResource(getPolicyWorkspace);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [panelMode, setPanelMode] = useState<GatewayPanelMode>('list');
  const [filterDraft, setFilterDraft] = useState<GatewayFilters>(emptyGatewayFilters);
  const [filters, setFilters] = useState<GatewayFilters>(emptyGatewayFilters);
  const [draftState, setDraftState] = useState<GatewayFormDraft | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [notice, setNotice] = useState<GatewayNotice | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Gateway | null>(null);
  const [disableCandidate, setDisableCandidate] = useState<Gateway | null>(null);

  if (gateways.loading) {
    return (
      <PageFrame title="网关" subtitle="定义对外访问入口和域名范围">
        <ResourceStatePanel title="加载网关数据" message="正在读取网关列表。" />
      </PageFrame>
    );
  }

  if (gateways.error || !gateways.data) {
    return (
      <PageFrame title="网关" subtitle="定义对外访问入口和域名范围">
        <ResourceStatePanel title="网关数据加载失败" message={gateways.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const availableGateways = gateways.data.gateways;
  const selectedGateway = availableGateways.find((gateway) => gateway.id === selectedGatewayId) ?? availableGateways[0] ?? null;
  const selectedGatewayView = selectedGateway;
  const visibleGateways = availableGateways.filter((gateway) => {
    const keyword = filters.keyword.trim().toLowerCase();
    const host = filters.host.trim().toLowerCase();
    const matchedKeyword = !keyword || [gateway.name, gateway.description].some((value) => value.toLowerCase().includes(keyword));
    const matchedHost = !host || [hostBindingSummary(gateway), ...gatewayHostnames(gateway)].some((value) => value.toLowerCase().includes(host));
    const matchedEnabled = filters.enabled === 'all' || (filters.enabled === 'enabled' ? gateway.enabled : !gateway.enabled);

    return matchedKeyword && matchedHost && matchedEnabled;
  });
  const hasActiveFilters = Boolean(filters.keyword.trim() || filters.host.trim() || filters.enabled !== 'all');
  const draft = draftState ?? createGatewayDraft(panelMode === 'edit' ? selectedGateway : null);
  const clientValidation = validateGatewayDraft(draft);
  const payload = buildGatewayPayload(draft);
  const openCreate = () => {
    setPanelMode('create');
    setDraftState(createGatewayDraft());
    setSubmitError(null);
    setNotice(null);
  };

  const openEdit = (gateway: Gateway) => {
    setSelectedGatewayId(gateway.id);
    setPanelMode('edit');
    setDraftState(createGatewayDraft(gateway));
    setSubmitError(null);
    setNotice(null);
  };

  const requestDeleteGateway = (gateway: Gateway) => {
    setDeleteCandidate(gateway);
  };

  const confirmDeleteGateway = async () => {
    if (!deleteCandidate) {
      return;
    }

    try {
      await deleteGateway(deleteCandidate.id);
      await gateways.reload();
      setSelectedGatewayId((current) => {
        if (current !== deleteCandidate.id) {
          return current;
        }
        return availableGateways.find((gateway) => gateway.id !== deleteCandidate.id)?.id ?? '';
      });
      setNotice({ message: `已删除网关：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除网关失败', tone: 'error' });
    }
  };

  const toggleGatewayEnabled = async (gateway: Gateway) => {
    if (gateway.enabled) {
      setDisableCandidate(gateway);
      return;
    }

    try {
      await setGatewayEnabled(gateway.id, true);
      await gateways.reload();
      setNotice({ message: `已启用网关：${gateway.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '启用网关失败', tone: 'error' });
    }
  };

  const confirmDisableGateway = async () => {
    if (!disableCandidate) {
      return;
    }

    try {
      await setGatewayEnabled(disableCandidate.id, false);
      await gateways.reload();
      setNotice({ message: `已停用网关：${disableCandidate.name}`, tone: 'success' });
      setDisableCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '停用网关失败', tone: 'error' });
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
    setSubmitError(null);
  };

  const handleGatewaySubmit = async () => {
    setSubmitError(null);

    if (!clientValidation.valid) {
      return;
    }

    try {
      const result = await saveGateway(payload);
      await gateways.reload();
      setSelectedGatewayId(result.changeId ?? payload.id ?? selectedGatewayId);
      setNotice({ message: `网关已保存：${payload.name}`, tone: 'success' });
      setPanelMode('list');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : '保存网关失败');
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
          {selectedGatewayView ? (
            <GatewayDetail
              gateway={selectedGatewayView}
              policyWorkspace={policies.data}
              onPolicyWorkspaceChanged={policies.reload}
            />
          ) : null}
        </Panel>
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  if (panelMode !== 'list') {
    return (
      <PageFrame
        title={panelMode === 'create' ? '新建网关' : '编辑网关'}
        actions={<Button variant="soft" onClick={() => setPanelMode('list')}>返回列表</Button>}
      >
        <section className="editor-layout">
          <GatewayFormPanel
            draft={draft}
            validation={clientValidation}
            certificates={certificates.data?.certificates ?? []}
            submitError={submitError}
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
      title="网关"
      subtitle="定义对外访问入口和域名范围"
      actions={
        <Button variant="primary" onClick={openCreate}>新建网关</Button>
      }
    >
        <Panel title="网关列表">
          <div className="gateway-query">
            <div className="gateway-query-grid gateway-query-grid-3">
              <label className="query-control">
                <span>网关名称</span>
                <input value={filterDraft.keyword} placeholder="请输入名称或描述" onChange={(event) => updateFilterDraft({ keyword: event.target.value })} />
              </label>
              <label className="query-control">
                <span>域名</span>
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
          <div className="table-scroll gateway-table-scroll">
            <table className="table gateway-table">
              <thead>
                <tr>
                  <th>网关名称</th>
                  <th>运行入口</th>
                  <th>域名范围</th>
                  <th>启用状态</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleGateways.map((gateway) => (
                  <tr key={gateway.id}>
                    <td>
                      <div className="table-primary">{gateway.name}</div>
                      <div className="table-secondary">{gateway.description}</div>
                    </td>
                    <td>
                      <div className="table-primary">{listenerSummary(gateway)}</div>
                      <div className="table-secondary">固定访问入口</div>
                    </td>
                    <td>
                      <div className="table-primary">{hostBindingSummary(gateway)}</div>
                      <div className="table-secondary">{gatewayHostnames(gateway).length > 0 ? `${gatewayHostnames(gateway).length} 个域名` : '不限制域名'}</div>
                    </td>
                    <td>
                      <div className={`gateway-status ${gateway.enabled ? 'on' : ''}`.trim()}>
                        <button
                          className="gateway-switch"
                          type="button"
                          role="switch"
                          aria-checked={gateway.enabled}
                          aria-label={`${gateway.name} ${gateway.enabled ? '已启用' : '已停用'}`}
                          onClick={(event) => {
                            event.stopPropagation();
                            toggleGatewayEnabled(gateway);
                          }}
                        >
                          <span aria-hidden="true" />
                        </button>
                        <strong>{gateway.enabled ? '启用' : '停用'}</strong>
                      </div>
                    </td>
                    <td>{formatDateTime(gateway.createdAt)}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setSelectedGatewayId(gateway.id);
                          setPanelMode('detail');
                        }}>详情</button>
                        <Link className="link-button" to={`/policies?tab=bindings&targetKind=Gateway&targetID=${encodeURIComponent(gateway.id)}`} onClick={(event) => event.stopPropagation()}>策略</Link>
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          openEdit(gateway);
                        }}>编辑</button>
                        <button className="link-button danger" type="button" onClick={(event) => {
                          event.stopPropagation();
                          requestDeleteGateway(gateway);
                        }}>删除</button>
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
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
        {deleteCandidate ? (
          <div className="confirm-overlay" role="presentation" onMouseDown={() => setDeleteCandidate(null)}>
            <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-gateway-title" onMouseDown={(event) => event.stopPropagation()}>
              <h3 id="delete-gateway-title">删除网关</h3>
              <p>确定删除 {deleteCandidate.name}？仍有关联路由时，系统会拒绝删除。</p>
              <div className="confirm-meta">
                <span>域名范围</span><strong>{hostBindingSummary(deleteCandidate)}</strong>
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
              <p>停用 {disableCandidate.name} 后，关联路由将不再承载流量。请确认业务可以暂停访问。</p>
              <div className="confirm-meta">
                <span>域名范围</span><strong>{hostBindingSummary(disableCandidate)}</strong>
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
  draft,
  validation,
  certificates,
  submitError,
  onDraftChange,
  onSubmit,
  onCancel,
}: {
  draft: GatewayFormDraft;
  validation: GatewayValidationReport;
  certificates: Certificate[];
  submitError: string | null;
  onDraftChange: (patch: Partial<GatewayFormDraft>) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const fieldErrors = gatewayFieldErrors(validation);

  return (
    <Panel>
      <div className="editor-grid form-only">
        <div className="editor-main-stack">
          <section className="form-section">
            <div className="form-section-title">
              <h3>基础信息</h3>
            </div>
            <div className="field-grid">
              <InputField label="网关名称" value={draft.name} error={fieldErrors.name} onChange={(value) => onDraftChange({ name: value })} />
              <InputField label="描述" value={draft.description} onChange={(value) => onDraftChange({ description: value })} />
            </div>
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>运行入口</h3>
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
              <h3>域名范围</h3>
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
          {submitError ? <div className="form-error submit-error" role="alert">{submitError}</div> : null}
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
    listeners: validation.items.find((item) => item.label === '运行入口' && item.status === 'critical')?.message,
    certificate: validation.items.find((item) => item.label === 'HTTPS 证书' && item.status === 'critical')?.message,
    host: validation.items.find((item) => item.label === '域名范围' && item.status === 'critical')?.message,
  };
}

function GatewayListenerEditor({
  value,
  certificates,
  listenerError,
  certificateError,
  onChange,
}: {
  value: GatewayFormDraft['listeners'];
  certificates: Certificate[];
  listenerError?: string;
  certificateError?: string;
  onChange: (listeners: GatewayFormDraft['listeners']) => void;
}) {
  const httpListener = value.find((listener) => listener.protocol === 'HTTP');
  const httpsListener = value.find((listener) => listener.protocol === 'HTTPS');
  const setEnabled = (protocol: 'HTTP' | 'HTTPS', enabled: boolean) => {
    if (!enabled) {
      onChange(value.filter((listener) => listener.protocol !== protocol));
      return;
    }
    const listener = protocol === 'HTTP'
      ? { protocol, port: GATEWAY_HTTP_PORT } as const
      : { protocol, port: GATEWAY_HTTPS_PORT, certificateID: '' } as const;
    onChange([...value, listener].sort((a, b) => a.protocol.localeCompare(b.protocol)));
  };

  return (
    <div className="gateway-listener-editor">
      <div className={`gateway-listener-card ${httpListener ? 'enabled' : ''}`.trim()}>
        <div className="gateway-listener-head">
          <strong>HTTP</strong>
          <label className="gateway-listener-toggle">
            <input type="checkbox" checked={Boolean(httpListener)} onChange={(event) => setEnabled('HTTP', event.target.checked)} />
            <span>{httpListener ? '已启用' : '未启用'}</span>
          </label>
        </div>
        <div className="gateway-listener-address">0.0.0.0:{GATEWAY_HTTP_PORT}</div>
      </div>

      <div className={`gateway-listener-card ${httpsListener ? 'enabled' : ''}`.trim()}>
        <div className="gateway-listener-head">
          <strong>HTTPS</strong>
          <label className="gateway-listener-toggle">
            <input type="checkbox" checked={Boolean(httpsListener)} onChange={(event) => setEnabled('HTTPS', event.target.checked)} />
            <span>{httpsListener ? '已启用' : '未启用'}</span>
          </label>
        </div>
        <div className="gateway-listener-address">0.0.0.0:{GATEWAY_HTTPS_PORT}</div>
        {httpsListener ? (
          <div className={`field gateway-certificate-field ${certificateError ? 'invalid' : ''}`.trim()}>
            <label htmlFor="gateway-https-certificate">证书</label>
            <select
              id="gateway-https-certificate"
              value={httpsListener.certificateID ?? ''}
              onChange={(event) => onChange(value.map((listener) => listener.protocol === 'HTTPS'
                ? { ...listener, certificateID: event.target.value }
                : listener))}
            >
              <option value="">请选择证书</option>
              {certificates.map((certificate) => (
                <option key={certificate.id} value={certificate.id}>{certificate.name}</option>
              ))}
            </select>
            {certificates.length === 0 ? <Link className="link-button gateway-certificate-link" to="/certificates">创建新证书</Link> : null}
          </div>
        ) : null}
      </div>
      {listenerError ? <div className="form-error">{listenerError}</div> : null}
      {certificateError ? <div className="form-error">{certificateError}</div> : null}
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
      <div className="gateway-host-mode" role="group" aria-label="域名范围">
        <button className={mode === 'any' ? 'active' : ''} type="button" onClick={() => onModeChange('any')}>
          不限制域名
        </button>
        <button className={mode === 'specified' ? 'active' : ''} type="button" onClick={() => onModeChange('specified')}>
          指定域名
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
              <span className="host-empty">请添加至少一个域名</span>
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
        <>
          <span className="host-empty">当前不限制请求域名，直接进入路由匹配。</span>
          {error ? <div className="form-error">{error}</div> : null}
        </>
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

function GatewayDetail({
  gateway,
  policyWorkspace,
  onPolicyWorkspaceChanged,
}: {
  gateway: Gateway;
  policyWorkspace: PolicyWorkspace | null | undefined;
  onPolicyWorkspaceChanged: () => Promise<void> | void;
}) {
  return (
    <div className="section-grid">
      <div className="detail-card">
        <h4>入口信息</h4>
        <div className="kv">
          {[
            ['描述', gateway.description],
            ['启用状态', gateway.enabled ? '启用' : '停用'],
            ['创建时间', formatDateTime(gateway.createdAt)],
          ].flatMap(([label, value]) => [
            <div key={`${label}-label`}>{label}</div>,
            <div key={`${label}-value`}>{value}</div>,
          ])}
        </div>
      </div>
      <div className="detail-card">
        <h4>运行入口</h4>
        <div className="drawer-list">
          {gateway.listeners.map((listener) => (
            <div className="legend-row" key={listener.protocol}>
              <span>{listener.protocol}</span>
              <strong>0.0.0.0:{listener.port}</strong>
            </div>
          ))}
        </div>
      </div>
      <div className="detail-card">
        <h4>域名范围</h4>
        <div className="mini-card-title">{gatewayHostnames(gateway).length > 0 ? '指定域名' : '不限制域名'}</div>
        <div className="tag-list">
          {gatewayHostnames(gateway).length > 0 ? gatewayHostnames(gateway).map((hostname) => (
            <span className="tag-chip static" key={hostname}>{hostname}</span>
          )) : (
            <span className="host-empty">不限制请求域名，直接进入路由匹配。</span>
          )}
        </div>
      </div>
      {policyWorkspace ? (
        <GovernanceBindingPanel
          targetKind="Gateway"
          targetID={gateway.id}
          targetName={gateway.name}
          workspace={policyWorkspace}
          onChanged={onPolicyWorkspaceChanged}
        />
      ) : (
        <div className="mini-card">
          <div className="mini-card-title">策略绑定暂不可用</div>
          <div className="mini-card-meta">网关本身可以继续查看和编辑；策略接口恢复后再管理绑定关系。</div>
        </div>
      )}
    </div>
  );
}

function gatewayHostnames(gateway: Gateway) {
  return gateway.hostnames;
}

function hostBindingSummary(gateway: Gateway) {
  const hostnames = gatewayHostnames(gateway);
  return hostnames.length > 0 ? hostnames.join('、') : '不限制域名';
}

function listenerSummary(gateway: Gateway) {
  if (gateway.listeners.length === 0) {
    return '-';
  }
  return gateway.listeners.map((listener) => `${listener.protocol}:${listener.port}`).join(' / ');
}
