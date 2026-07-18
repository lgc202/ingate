import { useState } from 'react';
import { listUpstreamCredentials } from '@/api/credentials';
import { deleteUpstream, listUpstreams, saveUpstream } from '@/api/upstreams';
import { useResource } from '@/api/useResource';
import { Badge, Button, EmptyState, PageFrame, Panel, ResourceStatePanel, Toast } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { UpstreamCredential } from '@/domain/credential';
import type { Upstream, UpstreamEndpoint, UpstreamType } from '@/domain/upstream';
import {
  upstreamLoadBalancePolicyLabel,
  upstreamLoadBalancePolicyOptions,
  upstreamProtocolLabel,
  upstreamTypeLabel,
  upstreamTypeOptions,
} from '@/domain/upstream';
import type { UpstreamFormDraft, UpstreamFormValidation } from './form';
import {
  buildUpstreamPayload,
  changeUpstreamType,
  createUpstreamDraft,
  createUpstreamEndpoint,
  formatEndpointSummary,
  formatInstanceSummary,
  validateUpstreamDraft,
} from './form';

const loadUpstreamWorkspace = async () => {
  const [upstreamList, credentialList] = await Promise.all([
    listUpstreams(),
    listUpstreamCredentials(),
  ]);
  return {
    upstreams: upstreamList.upstreams,
    credentials: credentialList.credentials,
  };
};
type UpstreamPanelMode = 'list' | 'detail' | 'create' | 'edit';

interface UpstreamNotice {
  message: string;
  tone: 'success' | 'error';
}

export function UpstreamPage() {
  const upstreams = useResource(loadUpstreamWorkspace);
  const [selectedUpstreamId, setSelectedUpstreamId] = useState('');
  const [panelMode, setPanelMode] = useState<UpstreamPanelMode>('list');
  const [query, setQuery] = useState('');
  const [draftState, setDraftState] = useState<UpstreamFormDraft | null>(null);
  const [notice, setNotice] = useState<UpstreamNotice | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Upstream | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (upstreams.loading) {
    return (
      <PageFrame title="服务" subtitle="统一管理应用、大模型、Agent 和 MCP 的可转发端点">
        <ResourceStatePanel title="加载服务数据" message="正在读取服务列表和端点配置。" />
      </PageFrame>
    );
  }

  if (upstreams.error || !upstreams.data) {
    return (
      <PageFrame title="服务" subtitle="统一管理应用、大模型、Agent 和 MCP 的可转发端点">
        <ResourceStatePanel title="服务数据加载失败" message={upstreams.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const availableUpstreams = upstreams.data.upstreams;
  const availableCredentials = upstreams.data.credentials;
  const selectedUpstream = availableUpstreams.find((upstream) => upstream.id === selectedUpstreamId) ?? availableUpstreams[0] ?? null;
  const visibleUpstreams = availableUpstreams.filter((upstream) => {
    const normalizedQuery = query.trim().toLowerCase();
    return !normalizedQuery || [
      upstream.name,
      upstream.type,
      upstream.protocol,
      upstreamTypeLabel(upstream.type),
      upstreamProtocolLabel(upstream.protocol),
      formatEndpointSummary(upstream.endpoints),
    ]
      .some((value) => value.toLowerCase().includes(normalizedQuery));
  });
  const hasActiveFilters = Boolean(query.trim());
  const draft = draftState ?? createUpstreamDraft(panelMode === 'edit' ? selectedUpstream : null);
  const validation = validateUpstreamDraft(draft);
  const payload = buildUpstreamPayload(draft);

  const openCreate = () => {
    setPanelMode('create');
    setDraftState(createUpstreamDraft(null));
    setNotice(null);
    setSubmitting(false);
  };

  const openEdit = (upstream: Upstream) => {
    setSelectedUpstreamId(upstream.id);
    setPanelMode('edit');
    setDraftState(createUpstreamDraft(upstream));
    setNotice(null);
    setSubmitting(false);
  };

  const confirmDeleteUpstream = async () => {
    if (!deleteCandidate) {
      return;
    }

    setDeleting(true);
    try {
      await deleteUpstream(deleteCandidate.id);
      await upstreams.reload();
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除服务失败', tone: 'error' });
      setDeleteCandidate(null);
      setDeleting(false);
      return;
    }

    setSelectedUpstreamId((current) => {
      if (current !== deleteCandidate.id) {
        return current;
      }

      return availableUpstreams.find((upstream) => upstream.id !== deleteCandidate.id)?.id ?? '';
    });
    setNotice({ message: `已删除服务：${deleteCandidate.name}`, tone: 'success' });
    setDeleteCandidate(null);
    setDeleting(false);
  };

  const updateDraft = (patch: Partial<UpstreamFormDraft>) => {
    setDraftState({ ...draft, ...patch });
  };

  const handleUpstreamSubmit = async () => {
    if (!validation.valid) {
      setNotice({ message: validation.summary, tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const result = await saveUpstream(payload);
      await upstreams.reload();
      setSelectedUpstreamId(result.changeId ?? payload.id ?? '');
      setNotice({ message: result.message, tone: 'success' });
      setPanelMode('list');
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存服务失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  if (panelMode === 'detail') {
    return (
      <PageFrame
        title="服务详情"
        subtitle={selectedUpstream?.name ?? '未选择服务'}
        actions={<Button variant="soft" onClick={() => setPanelMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedUpstream ? <UpstreamDetail upstream={selectedUpstream} credentials={availableCredentials} /> : null}
        </Panel>
      </PageFrame>
    );
  }

  const closeEditor = () => {
    setPanelMode('list');
    setDraftState(null);
    setSubmitting(false);
  };

  if (panelMode !== 'list') {
    return (
      <PageFrame
        title={panelMode === 'create' ? '新建服务' : '编辑服务'}
        subtitle={panelMode === 'create' ? '创建路由可以引用的目标服务' : '调整服务类型、端点、负载均衡和健康检查'}
        actions={<Button variant="soft" onClick={closeEditor} disabled={submitting}>返回列表</Button>}
      >
        <section className="editor-layout">
          <UpstreamFormPanel
            draft={draft}
            validation={validation}
            credentials={availableCredentials}
            submitting={submitting}
            onDraftChange={updateDraft}
            onSubmit={handleUpstreamSubmit}
            onCancel={closeEditor}
          />
        </section>
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="服务"
      subtitle="统一管理应用、大模型、Agent 和 MCP 的可转发端点"
      actions={<Button variant="primary" onClick={openCreate}>新建服务</Button>}
    >
      <Panel
        title="服务列表"
        actions={
          <div className="table-toolbar">
            <input
              className="toolbar-input"
              value={query}
              placeholder="搜索服务名称 / 地址 / 类型"
              onChange={(event) => setQuery(event.target.value)}
            />
            {query ? <Button variant="soft" onClick={() => setQuery('')}>重置</Button> : null}
          </div>
        }
      >
        <div className="table-scroll service-table-scroll">
          <table className="table service-table">
            <thead>
              <tr>
                <th>服务名称</th>
                <th>连接方式</th>
                <th>转发端点</th>
                <th>负载均衡</th>
                <th>健康检查</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {visibleUpstreams.map((upstream) => (
                <tr key={upstream.id}>
                  <td>
                    <div className="table-primary">{upstream.name}</div>
                    <div className="table-secondary">{upstreamTypeLabel(upstream.type)}</div>
                  </td>
                  <td>
                    <div className="table-primary">{upstreamConnectionSummary(upstream)}</div>
                    <div className="table-secondary">
                      {upstream.credentialID ? credentialName(upstream.credentialID, availableCredentials) : '无需访问凭据'}
                    </div>
                  </td>
                  <td>
                    <div className="table-primary">{formatEndpointSummary(upstream.endpoints)}</div>
                    <div className="table-secondary">{formatInstanceSummary(upstream.endpoints)}</div>
                  </td>
                  <td>{upstreamLoadBalancePolicyLabel(upstream.loadBalancePolicy)}</td>
                  <td><Badge tone={upstream.healthCheck?.enabled ? 'accent' : 'neutral'}>{upstream.healthCheck?.enabled ? '已启用' : '未启用'}</Badge></td>
                  <td>{formatDateTime(upstream.createdAt)}</td>
                  <td>
                    <div className="row-actions">
                      <button className="link-button" type="button" onClick={(event) => {
                        event.stopPropagation();
                        setSelectedUpstreamId(upstream.id);
                        setPanelMode('detail');
                      }}>详情</button>
                      <button className="link-button" type="button" onClick={(event) => {
                        event.stopPropagation();
                        openEdit(upstream);
                      }}>编辑</button>
                      <button className="link-button danger" type="button" onClick={(event) => {
                        event.stopPropagation();
                        setDeleteCandidate(upstream);
                      }}>删除</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {visibleUpstreams.length === 0 ? (
            <div className="table-empty">
              <EmptyState
                title={hasActiveFilters ? '没有匹配的服务' : '暂无服务'}
                message={hasActiveFilters ? '调整查询条件后再试，或重置筛选查看全部服务。' : '当前还没有服务，可以先创建一个服务。'}
              />
            </div>
          ) : null}
        </div>
      </Panel>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      {deleteCandidate ? (
        <div className="confirm-overlay" role="presentation" onMouseDown={() => {
          if (!deleting) {
            setDeleteCandidate(null);
          }
        }}>
          <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-upstream-title" onMouseDown={(event) => event.stopPropagation()}>
            <h3 id="delete-upstream-title">删除服务</h3>
            <p>确定删除 {deleteCandidate.name}？如果仍有路由引用该服务，后端会拒绝删除。</p>
            <div className="confirm-meta">
              <span>服务端点</span><strong>{formatEndpointSummary(deleteCandidate.endpoints)}</strong>
              <span>启用端点</span><strong>{formatInstanceSummary(deleteCandidate.endpoints)}</strong>
            </div>
            <div className="confirm-actions">
              <Button variant="ghost" onClick={() => setDeleteCandidate(null)} disabled={deleting}>取消</Button>
              <Button variant="primary" onClick={confirmDeleteUpstream} disabled={deleting}>{deleting ? '删除中...' : '确认删除'}</Button>
            </div>
          </div>
        </div>
      ) : null}
    </PageFrame>
  );
}

function UpstreamFormPanel({
  draft,
  validation,
  credentials,
  submitting,
  onDraftChange,
  onSubmit,
  onCancel,
}: {
  draft: UpstreamFormDraft;
  validation: UpstreamFormValidation;
  credentials: UpstreamCredential[];
  submitting: boolean;
  onDraftChange: (patch: Partial<UpstreamFormDraft>) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  return (
    <Panel>
      <div className="editor-grid form-only">
        <div className="editor-main-stack">
          <section className="form-section">
            <div className="form-section-title">
              <h3>基础信息</h3>
              <p>服务可以表示应用、大模型、Agent 或 MCP，类型用于区分业务语义。</p>
            </div>
            <div className="field-grid">
              <InputField label="服务名称" value={draft.name} error={validation.errors.name} onChange={(value) => onDraftChange({ name: value })} />
              <SelectField
                label="服务类型"
                value={draft.type}
                options={upstreamTypeOptions}
                onChange={(value) => onDraftChange(changeUpstreamType(draft, value as UpstreamType))}
              />
              <SelectField
                label="负载均衡"
                value={draft.loadBalancePolicy}
                options={upstreamLoadBalancePolicyOptions}
                error={validation.errors.loadBalancePolicy}
                onChange={(value) => onDraftChange({ loadBalancePolicy: value as UpstreamFormDraft['loadBalancePolicy'] })}
              />
            </div>
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>连接方式</h3>
              <p>{draft.type === 'model' ? '模型服务使用 OpenAI 兼容接口，并可选择访问凭据。' : '普通服务使用 HTTP 接口，可按需启用 HTTPS。'}</p>
            </div>
            <UpstreamConnectionEditor
              draft={draft}
              credentials={credentials}
              protocolError={validation.errors.protocol}
              tlsError={validation.errors.tls}
              onChange={onDraftChange}
            />
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>服务端点</h3>
              <p>配置网关访问服务时使用的地址、端口和流量权重。</p>
            </div>
            <UpstreamEndpointEditor
              model={draft.type === 'model'}
              value={draft.endpoints}
              error={validation.errors.endpoints}
              onChange={(endpoints) => onDraftChange({ endpoints })}
            />
          </section>

          <section className="form-section">
            <div className="form-section-title">
              <h3>健康检查</h3>
              <p>启用后，网关会主动检查服务端点是否可用。</p>
            </div>
            <HealthCheckEditor
              draft={draft}
              error={validation.errors.healthCheck}
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

function UpstreamEndpointEditor({
  model,
  value,
  error,
  onChange,
}: {
  model: boolean;
  value: UpstreamEndpoint[];
  error?: string;
  onChange: (value: UpstreamEndpoint[]) => void;
}) {
  const updateEndpoint = (endpointId: string, patch: Partial<UpstreamEndpoint>) => {
    onChange(value.map((endpoint) => endpoint.id === endpointId ? { ...endpoint, ...patch } : endpoint));
  };

  const removeEndpoint = (endpointId: string) => {
    onChange(value.filter((endpoint) => endpoint.id !== endpointId));
  };

  return (
    <div className="upstream-endpoint-editor">
      <div className="upstream-endpoint-grid upstream-endpoint-grid-head">
        <span>地址</span>
        <span>端口</span>
        <span>权重</span>
        <span>启用</span>
        <span>操作</span>
      </div>
      {value.map((endpoint) => (
        <div className="upstream-endpoint-grid" key={endpoint.id}>
          <input
            className={!endpoint.address.trim() ? 'invalid-control' : ''}
            value={endpoint.address}
            placeholder={model ? 'api.example.com' : 'order-svc.internal'}
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
      <button className="link-button" type="button" onClick={() => onChange([...value, createUpstreamEndpoint()])}>添加端点</button>
    </div>
  );
}

function UpstreamConnectionEditor({
  draft,
  credentials,
  protocolError,
  tlsError,
  onChange,
}: {
  draft: UpstreamFormDraft;
  credentials: UpstreamCredential[];
  protocolError?: string;
  tlsError?: string;
  onChange: (patch: Partial<UpstreamFormDraft>) => void;
}) {
  const selectedCredentialAvailable = credentials.some((credential) => credential.id === draft.credentialID);
  return (
    <div className="upstream-connection-grid">
      <div className={`field ${protocolError ? 'invalid' : ''}`.trim()}>
        <label>接口协议</label>
        <div className="readonly-control">
          <strong>{upstreamProtocolLabel(draft.protocol)}</strong>
          <span>{draft.type === 'model' ? '兼容 OpenAI Chat Completions 请求' : '标准 HTTP 服务'}</span>
        </div>
        {protocolError ? <div className="form-error">{protocolError}</div> : null}
      </div>

      {draft.type === 'model' ? (
        <label className="field">
          <span>访问凭据</span>
          <select value={draft.credentialID} onChange={(event) => onChange({ credentialID: event.target.value })}>
            <option value="">不使用访问凭据</option>
            {!selectedCredentialAvailable && draft.credentialID ? <option value={draft.credentialID}>当前凭据不可用</option> : null}
            {credentials.map((credential) => (
              <option key={credential.id} value={credential.id}>
                {credential.name}{credential.configured ? '' : '（未配置密钥）'}
              </option>
            ))}
          </select>
          <small className="field-hint">请求转发时自动携带所选密钥；使用凭据时必须开启 HTTPS。</small>
        </label>
      ) : null}

      <div className={`field field-wide ${tlsError ? 'invalid' : ''}`.trim()}>
        <label className="connection-toggle-row">
          <input
            type="checkbox"
            checked={draft.httpsEnabled}
            onChange={(event) => onChange({ httpsEnabled: event.target.checked })}
          />
          <span>
            <strong>使用 HTTPS 连接</strong>
            <small>启用后会校验目标服务证书</small>
          </span>
        </label>
        {draft.httpsEnabled ? (
          <input
            value={draft.serverName}
            placeholder="证书中的服务名称，例如 api.example.com"
            onChange={(event) => onChange({ serverName: event.target.value })}
          />
        ) : null}
        {tlsError ? <div className="form-error">{tlsError}</div> : null}
      </div>
    </div>
  );
}

function HealthCheckEditor({
  draft,
  error,
  onChange,
}: {
  draft: UpstreamFormDraft;
  error?: string;
  onChange: (patch: Partial<UpstreamFormDraft>) => void;
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
          <InputField
            label="探活路径"
            value={draft.healthCheck.path}
            error={error && !draft.healthCheck.path.startsWith('/') ? error : undefined}
            onChange={(value) => onChange({ healthCheck: { ...draft.healthCheck, path: value } })}
          />
          <InputField
            label="检查间隔（秒）"
            value={String(draft.healthCheck.intervalSeconds)}
            error={error && !isValidInterval(draft.healthCheck.intervalSeconds) ? error : undefined}
            onChange={(value) => onChange({ healthCheck: { ...draft.healthCheck, intervalSeconds: Number(value) } })}
          />
          <InputField
            label="超时时间（秒）"
            value={String(draft.healthCheck.timeoutSeconds)}
            error={error && !isValidTimeout(draft.healthCheck.timeoutSeconds, draft.healthCheck.intervalSeconds) ? error : undefined}
            onChange={(value) => onChange({ healthCheck: { ...draft.healthCheck, timeoutSeconds: Number(value) } })}
          />
        </div>
      ) : (
        <span className="host-empty">关闭后不执行主动健康检查。</span>
      )}
    </div>
  );
}

function InputField({
  label,
  value,
  error,
  onChange,
}: {
  label: string;
  value: string;
  error?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <label>{label}</label>
      <input value={value} onChange={(event) => onChange(event.target.value)} />
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

function UpstreamDetail({ upstream, credentials }: { upstream: Upstream; credentials: UpstreamCredential[] }) {
  const rows = [
    ['服务类型', upstreamTypeLabel(upstream.type)],
    ['接口协议', upstreamProtocolLabel(upstream.protocol)],
    ['安全连接', upstream.tls ? `HTTPS / ${upstream.tls.serverName}` : 'HTTP'],
    ['访问凭据', upstream.credentialID ? credentialName(upstream.credentialID, credentials) : '未配置'],
    ['服务端点', formatEndpointSummary(upstream.endpoints)],
    ['启用端点', formatInstanceSummary(upstream.endpoints)],
    ['负载均衡', upstreamLoadBalancePolicyLabel(upstream.loadBalancePolicy)],
    ['健康检查', upstream.healthCheck?.enabled ? `${upstream.healthCheck.path ?? '-'} / ${upstream.healthCheck.intervalSeconds ?? '-'}s / ${upstream.healthCheck.timeoutSeconds ?? '-'}s` : '未启用'],
    ['创建时间', formatDateTime(upstream.createdAt)],
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

function credentialName(credentialID: string, credentials: UpstreamCredential[]) {
  return credentials.find((credential) => credential.id === credentialID)?.name ?? '凭据不可用';
}

function upstreamConnectionSummary(upstream: Upstream) {
  const transport = upstream.tls ? 'HTTPS' : 'HTTP';
  return upstream.protocol === 'OpenAI'
    ? `${transport} · ${upstreamProtocolLabel(upstream.protocol)}`
    : transport;
}
