import { useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type {
  ServiceEndpointPayload,
  ServiceResource,
  ServiceType,
  ServiceValidationReport,
} from '@/domain/service';
import { serviceLoadBalancePolicyLabel, serviceLoadBalancePolicyOptions, serviceTypeLabel, serviceTypeOptions } from '@/domain/service';
import type { ServiceFormDraft } from './form';
import { buildServicePayload, createServiceDraft, createServiceEndpoint, formatEndpointSummary, formatInstanceSummary, validateServiceDraft } from './form';

const loadServices = () => consoleRepository.listServices();
type ServicePanelMode = 'list' | 'detail' | 'create' | 'edit';

export function ServicePage() {
  const services = useResource(loadServices);
  const [selectedServiceId, setSelectedServiceId] = useState('');
  const [panelMode, setPanelMode] = useState<ServicePanelMode>('list');
  const [query, setQuery] = useState('');
  const [draftState, setDraftState] = useState<ServiceFormDraft | null>(null);
  const [serverValidation, setServerValidation] = useState<ServiceValidationReport | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<ServiceResource | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (services.loading) {
    return (
      <PageFrame title="流量 / 服务" subtitle="第一阶段只管理应用服务，对应 Upstream。模型、Agent 和 MCP 服务后续扩展。">
        <ResourceStatePanel title="加载服务数据" message="正在读取服务列表、实例状态和最近异常。" />
      </PageFrame>
    );
  }

  if (services.error || !services.data) {
    return (
      <PageFrame title="流量 / 服务" subtitle="第一阶段只管理应用服务，对应 Upstream。模型、Agent 和 MCP 服务后续扩展。">
        <ResourceStatePanel title="服务数据加载失败" message={services.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const availableServices = services.data.upstreams;
  const selectedService = availableServices.find((service) => service.id === selectedServiceId) ?? availableServices[0] ?? null;
  const visibleServices = availableServices.filter((service) => {
    const normalizedQuery = query.trim().toLowerCase();
    return !normalizedQuery || [service.name, service.type, serviceTypeLabel(service.type), formatEndpointSummary(service.endpoints)].some((value) => value.toLowerCase().includes(normalizedQuery));
  });
  const hasActiveFilters = Boolean(query.trim());
  const draft = draftState ?? createServiceDraft(panelMode === 'edit' ? selectedService : null);
  const clientValidation = validateServiceDraft(draft);
  const activeValidation = serverValidation ?? clientValidation;
  const payload = buildServicePayload(draft);

  const openCreate = () => {
    setPanelMode('create');
    setDraftState(createServiceDraft(null));
    setServerValidation(null);
    setNotice(null);
    setSubmitting(false);
  };

  const openEdit = (service: ServiceResource) => {
    setSelectedServiceId(service.id);
    setPanelMode('edit');
    setDraftState(createServiceDraft(service));
    setServerValidation(null);
    setNotice(null);
    setSubmitting(false);
  };

  const deleteService = (service: ServiceResource) => {
    setDeleteCandidate(service);
  };

  const confirmDeleteService = async () => {
    if (!deleteCandidate) {
      return;
    }

    setDeleting(true);
    try {
      await consoleRepository.deleteService(deleteCandidate.id);
      await services.reload();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '删除服务失败');
      setDeleteCandidate(null);
      setDeleting(false);
      return;
    }

    setSelectedServiceId((current) => {
      if (current !== deleteCandidate.id) {
        return current;
      }

      return availableServices.find((service) => service.id !== deleteCandidate.id)?.id ?? '';
    });
    setNotice(`已删除服务：${deleteCandidate.name}`);
    setDeleteCandidate(null);
    setDeleting(false);
  };

  const updateDraft = (patch: Partial<ServiceFormDraft>) => {
    setDraftState({ ...draft, ...patch });
    setServerValidation(null);
  };

  const handleServiceSubmit = async () => {
    const validation = await consoleRepository.validateServiceDraft(payload);
    setServerValidation(validation);

    if (!validation.valid) {
      setNotice(validation.summary);
      return;
    }

    setSubmitting(true);
    try {
      const result = await consoleRepository.saveServiceDraft(payload);
      await services.reload();
      setSelectedServiceId(result.changeId ?? payload.id ?? '');
      setNotice(result.message);
      setPanelMode('list');
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '保存服务失败');
    } finally {
      setSubmitting(false);
    }
  };

  if (panelMode === 'detail') {
    return (
      <PageFrame
        title="服务详情"
        subtitle={selectedService?.name ?? '未选择服务'}
        actions={<Button variant="soft" onClick={() => setPanelMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedService ? <ServiceDetail service={selectedService} /> : null}
        </Panel>
      </PageFrame>
    );
  }

  const closeEditor = () => {
    setPanelMode('list');
    setDraftState(null);
    setServerValidation(null);
    setSubmitting(false);
  };

  if (panelMode !== 'list') {
    return (
      <PageFrame
        title={panelMode === 'create' ? '新建服务' : '编辑服务'}
        subtitle={panelMode === 'create' ? '创建路由可以选择的后端服务' : '调整服务端点、负载均衡和健康检查'}
        actions={<Button variant="soft" onClick={closeEditor} disabled={submitting}>返回列表</Button>}
      >
        <section className="editor-layout">
          <ServiceFormPanel
            mode={panelMode}
            draft={draft}
            validation={activeValidation}
            submitting={submitting}
            onDraftChange={updateDraft}
            onSubmit={handleServiceSubmit}
            onCancel={closeEditor}
          />
        </section>
        <Toast message={notice} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="流量 / 服务"
      subtitle="第一阶段只管理应用服务，对应 Upstream。模型、Agent 和 MCP 服务后续扩展。"
      actions={
        <>
          <Button variant="soft" onClick={() => {
            setQuery('');
          }}>重置筛选</Button>
          <Button variant="primary" onClick={openCreate}>新建服务</Button>
        </>
      }
    >
        <Panel
          title="服务列表"
          actions={
            <div className="table-toolbar">
              <input className="toolbar-input" value={query} placeholder="搜索服务名称 / 地址" onChange={(event) => setQuery(event.target.value)} />
            </div>
          }
        >
          <div style={{ overflow: 'auto' }}>
            <table className="table">
              <thead>
                <tr>
                  <th>服务名称</th>
                  <th>类型</th>
                  <th>端点</th>
                  <th>启用端点</th>
                  <th>负载均衡</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleServices.map((service) => (
                  <tr
                    key={service.id}
                    className={service.id === selectedService?.id ? 'selected' : ''}
                    onClick={() => setSelectedServiceId(service.id)}
                  >
                    <td>{service.name}</td>
                    <td>{serviceTypeLabel(service.type)}</td>
                    <td>{formatEndpointSummary(service.endpoints)}</td>
                    <td>{formatInstanceSummary(service.endpoints)}</td>
                    <td>{serviceLoadBalancePolicyLabel(service.loadBalancePolicy)}</td>
                    <td>{formatDateTime(service.createdAt)}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setSelectedServiceId(service.id);
                          setPanelMode('detail');
                        }}>详情</button>
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          openEdit(service);
                        }}>编辑</button>
                        <button className="link-button danger" type="button" onClick={(event) => {
                          event.stopPropagation();
                          deleteService(service);
                        }}>删除</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {visibleServices.length === 0 ? (
              <div className="table-empty">
                <EmptyState
                  title={hasActiveFilters ? '没有匹配的服务' : '暂无服务'}
                  message={hasActiveFilters ? '调整查询条件后再试，或重置筛选查看全部服务。' : '当前还没有后端服务，可以先新建一个服务。'}
                />
              </div>
            ) : null}
          </div>
        </Panel>
        <Toast message={notice} onClose={() => setNotice(null)} />
        {deleteCandidate ? (
          <div className="confirm-overlay" role="presentation" onMouseDown={() => {
            if (!deleting) {
              setDeleteCandidate(null);
            }
          }}>
            <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-service-title" onMouseDown={(event) => event.stopPropagation()}>
              <h3 id="delete-service-title">删除服务</h3>
              <p>确定删除 {deleteCandidate.name}？删除后引用该服务的路由将无法选择它作为目标。</p>
              <div className="confirm-meta">
                <span>服务端点</span><strong>{formatEndpointSummary(deleteCandidate.endpoints)}</strong>
                <span>启用端点</span><strong>{formatInstanceSummary(deleteCandidate.endpoints)}</strong>
              </div>
              <div className="confirm-actions">
                <Button variant="ghost" onClick={() => setDeleteCandidate(null)} disabled={deleting}>取消</Button>
                <Button variant="primary" onClick={confirmDeleteService} disabled={deleting}>{deleting ? '删除中...' : '确认删除'}</Button>
              </div>
            </div>
          </div>
        ) : null}
    </PageFrame>
  );
}

function ServiceFormPanel({
  mode,
  draft,
  validation,
  submitting,
  onDraftChange,
  onSubmit,
  onCancel,
}: {
  mode: ServicePanelMode;
  draft: ServiceFormDraft;
  validation: ServiceValidationReport;
  submitting: boolean;
  onDraftChange: (patch: Partial<ServiceFormDraft>) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const fieldErrors = serviceFieldErrors(validation);

  return (
    <Panel title={mode === 'create' ? '新建服务' : '编辑服务'} subtitle={mode === 'create' ? '填写基础信息和服务端点' : draft.name}>
      <div className="editor-grid form-only">
        <div className="editor-main-stack">
          <section className="form-section">
            <div className="form-section-title">
              <h3>基础信息</h3>
              <p>服务是路由的目标对象，可以是应用、模型、Agent 或 MCP 服务。</p>
            </div>
            <div className="field-grid">
              <InputField label="服务名称" value={draft.name} error={fieldErrors.name} disabled={mode === 'edit'} onChange={(value) => onDraftChange({ name: value })} />
              <SelectField
                label="服务类型"
                value={draft.type}
                options={serviceTypeOptions}
                onChange={(value) => onDraftChange({ type: value as ServiceType })}
              />
              <SelectField
                label="负载均衡"
                value={draft.loadBalancePolicy}
                options={serviceLoadBalancePolicyOptions}
                error={fieldErrors.loadBalance}
                onChange={(value) => onDraftChange({ loadBalancePolicy: value as ServiceFormDraft['loadBalancePolicy'] })}
              />
            </div>
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>服务端点</h3>
              <p>配置实际可转发的后端地址。第一阶段使用静态端点，后续可扩展服务发现。</p>
            </div>
            <ServiceEndpointEditor
              value={draft.endpoints}
              error={fieldErrors.endpoints}
              onChange={(endpoints) => onDraftChange({ endpoints })}
            />
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>健康检查</h3>
              <p>健康检查可以关闭；关闭后服务健康状态可能显示为未知。</p>
            </div>
            <HealthCheckEditor
              draft={draft}
              error={fieldErrors.healthCheck}
              onChange={onDraftChange}
            />
          </section>
        </div>
        <div className="form-actions">
          <Button variant="primary" disabled={submitting} onClick={onSubmit}>{submitting ? '保存中...' : '保存服务'}</Button>
          <Button variant="ghost" disabled={submitting} onClick={onCancel}>取消</Button>
        </div>
      </div>
    </Panel>
  );
}

function serviceFieldErrors(validation: ServiceValidationReport) {
  return {
    name: validation.items.find((item) => item.label === '服务名称' && item.status === 'critical')?.message,
    endpoints: validation.items.find((item) => item.label === '服务端点' && item.status !== 'healthy')?.message,
    loadBalance: validation.items.find((item) => item.label === '负载均衡' && item.status !== 'healthy')?.message,
    healthCheck: validation.items.find((item) => item.label === '健康检查' && item.status !== 'healthy')?.message,
  };
}

function ServiceEndpointEditor({
  value,
  error,
  onChange,
}: {
  value: ServiceEndpointPayload[];
  error?: string;
  onChange: (value: ServiceEndpointPayload[]) => void;
}) {
  const updateEndpoint = (endpointId: string, patch: Partial<ServiceEndpointPayload>) => {
    onChange(value.map((endpoint) => endpoint.id === endpointId ? { ...endpoint, ...patch } : endpoint));
  };

  const removeEndpoint = (endpointId: string) => {
    onChange(value.filter((endpoint) => endpoint.id !== endpointId));
  };

  return (
    <div className="service-endpoint-editor">
      <div className="service-endpoint-grid service-endpoint-grid-head">
        <span>地址</span>
        <span>端口</span>
        <span>权重</span>
        <span>启用</span>
        <span>操作</span>
      </div>
      {value.map((endpoint) => (
        <div className="service-endpoint-grid" key={endpoint.id}>
          <input
            className={!endpoint.address.trim() ? 'invalid-control' : ''}
            value={endpoint.address}
            placeholder="order-svc.cluster.local"
            onChange={(event) => updateEndpoint(endpoint.id, { address: event.target.value })}
          />
          <input
            className={!isValidPort(endpoint.port) ? 'invalid-control' : ''}
            value={endpoint.port}
            inputMode="numeric"
            onChange={(event) => updateEndpoint(endpoint.id, { port: Number(event.target.value) })}
          />
          <input
            className={!isValidWeight(endpoint.weight) ? 'invalid-control' : ''}
            value={endpoint.weight}
            inputMode="numeric"
            onChange={(event) => updateEndpoint(endpoint.id, { weight: Number(event.target.value) })}
          />
          <button
            className={`gateway-switch ${endpoint.enabled ? 'on' : ''}`.trim()}
            type="button"
            role="switch"
            aria-checked={endpoint.enabled}
            aria-label={`${endpoint.address || '端点'} ${endpoint.enabled ? '已启用' : '已停用'}`}
            onClick={() => updateEndpoint(endpoint.id, { enabled: !endpoint.enabled })}
          >
            <span aria-hidden="true" />
          </button>
          <button
            className="link-button danger"
            type="button"
            disabled={value.length <= 1}
            title={value.length <= 1 ? '至少保留一个端点' : undefined}
            onClick={() => removeEndpoint(endpoint.id)}
          >删除</button>
        </div>
      ))}
      {error ? <div className="form-error">{error}</div> : null}
      <button className="link-button" type="button" onClick={() => onChange([...value, createServiceEndpoint()])}>添加端点</button>
    </div>
  );
}

function HealthCheckEditor({
  draft,
  error,
  onChange,
}: {
  draft: ServiceFormDraft;
  error?: string;
  onChange: (patch: Partial<ServiceFormDraft>) => void;
}) {
  return (
    <div className="health-check-editor">
      <label className="toggle">
        <span
          className={`switch ${draft.healthCheck.enabled ? 'on' : ''}`}
          onClick={() => onChange({ healthCheck: { ...draft.healthCheck, enabled: !draft.healthCheck.enabled } })}
          role="switch"
          aria-checked={draft.healthCheck.enabled}
          tabIndex={0}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              onChange({ healthCheck: { ...draft.healthCheck, enabled: !draft.healthCheck.enabled } });
            }
          }}
        />
        启用健康检查
      </label>
      {draft.healthCheck.enabled ? (
        <div className="field-grid">
          <InputField label="探活路径" value={draft.healthCheck.path} error={error && !draft.healthCheck.path.startsWith('/') ? error : undefined} onChange={(value) => onChange({ healthCheck: { ...draft.healthCheck, path: value } })} />
          <InputField label="检查间隔（秒）" value={String(draft.healthCheck.intervalSeconds)} error={error && !isValidInterval(draft.healthCheck.intervalSeconds) ? error : undefined} onChange={(value) => onChange({ healthCheck: { ...draft.healthCheck, intervalSeconds: Number(value) } })} />
          <InputField label="超时时间（秒）" value={String(draft.healthCheck.timeoutSeconds)} error={error && !isValidTimeout(draft.healthCheck.timeoutSeconds, draft.healthCheck.intervalSeconds) ? error : undefined} onChange={(value) => onChange({ healthCheck: { ...draft.healthCheck, timeoutSeconds: Number(value) } })} />
        </div>
      ) : (
        <span className="host-empty">关闭后仍可保存服务，但健康状态会依赖请求结果或显示为未知。</span>
      )}
    </div>
  );
}

function InputField({
  label,
  value,
  error,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  error?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <label>{label}</label>
      <input value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} />
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function SelectField({
  label,
  value,
  options,
  error,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  error?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <label>{label}</label>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function isValidPort(port: number) {
  return Number.isInteger(port) && port >= 1 && port <= 65535;
}

function isValidWeight(weight: number) {
  return Number.isInteger(weight) && weight >= 1 && weight <= 100;
}

function isValidInterval(interval: number) {
  return Number.isInteger(interval) && interval >= 1 && interval <= 300;
}

function isValidTimeout(timeout: number, interval: number) {
  return Number.isInteger(timeout) && timeout >= 1 && timeout <= 60 && timeout < interval;
}

function ServiceDetail({ service }: { service: ServiceResource }) {
  const rows = [
    ['服务类型', serviceTypeLabel(service.type)],
    ['服务端点', formatEndpointSummary(service.endpoints)],
    ['启用端点', formatInstanceSummary(service.endpoints)],
    ['负载均衡', serviceLoadBalancePolicyLabel(service.loadBalancePolicy)],
    ['健康检查', service.healthCheck?.enabled ? `${service.healthCheck.path ?? '-'} / ${service.healthCheck.intervalSeconds ?? '-'}s / ${service.healthCheck.timeoutSeconds ?? '-'}s` : '未启用'],
    ['创建时间', formatDateTime(service.createdAt)],
  ];

  return (
    <div className="detail-card">
      <div className="kv">
        {rows.flatMap(([label, value]) => [
          <div key={`${label}-label`}>{label}</div>,
          <div key={`${label}-value`}>{value}</div>,
        ])}
      </div>
    </div>
  );
}
