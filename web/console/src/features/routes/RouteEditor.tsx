import { useEffect, useRef, useState } from 'react';
import { Badge, Button } from '@/components/ui';
import type { HeaderMatch, HttpMethod, ModelRoute, RouteGatewayOption, UpstreamOption, WeightedUpstream } from '@/domain/route';
import { upstreamTypeLabel } from '@/domain/upstream';
import type { RouteComposerDraft, RouteDraftValidation, RouteForwardMode } from './composer';
import {
  changeRouteForwardMode,
  createModelRoute,
  formatModelRoutes,
  formatWeightedUpstreams,
  forwardControlCount,
  modelRoutePath,
  normalizeHostnames,
  parseHostnames,
  upstreamWeightSum,
} from './composer';
import { RouteForwardControls } from './RouteForwardControls';
import { formatGatewayIDs, formatHostnames, formatMethods } from './routeView';

const httpMethods: HttpMethod[] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'];

export function RouteEditor({
  draft,
  validation,
  gateways,
  upstreams,
  submitting,
  onDraftChange,
  onCancel,
  onSave,
}: {
  draft: RouteComposerDraft;
  validation: RouteDraftValidation;
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
  submitting: boolean;
  onDraftChange: (draft: RouteComposerDraft) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  const updateDraft = (patch: Partial<RouteComposerDraft>) => {
    onDraftChange({ ...draft, ...patch });
  };
  const controlCount = forwardControlCount(draft);

  return (
    <section className="route-workbench">
      <div className="route-workbench-top">
        <div>
          <h2>{draft.id ? '编辑路由' : '创建路由'}</h2>
          <p>{draft.name.trim() || '未命名路由'} · {formatMethods(draft.methods)} {draft.path || '/'}</p>
        </div>
        <div className="route-workbench-meta">
          <Badge tone={draft.enabled ? 'accent' : 'neutral'}>{draft.enabled ? '启用' : '停用'}</Badge>
          <Badge tone={draft.forwardMode === 'model' ? 'accent' : 'neutral'}>{draft.forwardMode === 'model' ? '模型路由' : '普通转发'}</Badge>
          {draft.preservedRules.length > 0 ? <Badge tone="warning">保留 {draft.preservedRules.length} 条附加规则</Badge> : null}
          <span>{controlCount > 0 ? `${controlCount} 项转发控制` : '使用默认转发行为'}</span>
        </div>
      </div>

      <div className="route-workbench-grid">
        <div className="route-form-layout">
          <nav className="route-form-nav" aria-label="路由配置导航">
            <a href="#route-basic">基础信息</a>
            <a href="#route-match">匹配条件</a>
            <a href="#route-upstreams">转发目标</a>
            <a href="#route-controls">转发控制</a>
          </nav>

          <div className="route-form-sections">
            {draft.preservedRules.length > 0 ? (
              <aside className="mini-card">
                <strong>附加规则保持不变</strong>
                <div className="mini-card-meta">
                  当前编辑器只修改第 1 条规则；其余 {draft.preservedRules.length} 条规则会在保存时按原配置一并提交
                </div>
              </aside>
            ) : null}
            <section id="route-basic" className="detail-card composer-card route-form-section">
              <SectionTitle number="01" title="基础信息" description="定义路由的展示名称和启用状态" />
              <div className="field-grid">
                <InputField
                  label="路由名称"
                  value={draft.name}
                  required
                  maxLength={64}
                  placeholder="例如：模型对话接口"
                  info="用于在控制台中识别路由，同类路由名称不能重复"
                  error={validation.errors.name}
                  onChange={(name) => updateDraft({ name })}
                />
                <div className="field">
                  <FieldLabel label="启用状态" info="停用后资源仍保留，但不会下发到网关实例" />
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
              <SectionTitle number="02" title="匹配条件" description="选择承载网关，并定义请求方法、路径、域名和请求头条件" />
              <div className="field-grid">
                <GatewayMultiSelect
                  options={gateways}
                  value={draft.gatewayIDs}
                  error={validation.errors.gateways}
                  onChange={(gatewayIDs) => updateDraft({ gatewayIDs })}
                />
                <InputField
                  label="规则名称"
                  value={draft.ruleName}
                  required
                  maxLength={63}
                  placeholder="main"
                  info="用于识别这条匹配规则，也可单独关联策略"
                  error={validation.errors.ruleName}
                  onChange={(ruleName) => updateDraft({ ruleName })}
                />
                {draft.forwardMode === 'model' ? (
                  <div className="field">
                    <FieldLabel label="请求方法" info="模型路由固定接收 POST 请求" />
                    <div className="readonly-control"><strong>POST</strong><span>模型路由固定方法</span></div>
                  </div>
                ) : <MethodSelector value={draft.methods} onChange={(methods) => updateDraft({ methods })} />}
                {draft.forwardMode === 'model' ? (
                  <div className="field">
                    <FieldLabel label="请求路径" info="第一阶段固定提供 OpenAI Chat Completions 接口" />
                    <div className="readonly-control"><strong>{modelRoutePath}</strong><span>模型路由固定路径</span></div>
                  </div>
                ) : (
                  <InputField
                    label="路径前缀"
                    value={draft.path}
                    required
                    maxLength={256}
                    placeholder="/api"
                    info="必须以 / 开头；/ 表示匹配所有路径"
                    error={validation.errors.path}
                    onChange={(path) => updateDraft({ path })}
                  />
                )}
                <HostnameEditor
                  value={draft.hostnames}
                  error={validation.errors.hostnames}
                  onChange={(hostnames) => updateDraft({ hostnames })}
                />
                <HeaderMatchEditor
                  value={draft.headers}
                  error={validation.errors.headers}
                  onChange={(headers) => updateDraft({ headers })}
                />
              </div>
            </section>

            <section id="route-upstreams" className="detail-card composer-card route-form-section">
              <SectionTitle number="03" title="转发目标" description="选择普通服务，或连接一个模型服务并配置模型别名" />
              <ForwardModeSelector
                value={draft.forwardMode}
                onChange={(forwardMode) => onDraftChange(changeRouteForwardMode(draft, forwardMode))}
              />
              {draft.forwardMode === 'model' ? (
                <ModelRouteEditor
                  upstreams={upstreams}
                  upstreamID={draft.modelUpstreamID}
                  value={draft.modelRoutes}
                  error={validation.errors.models}
                  onUpstreamChange={(modelUpstreamID) => updateDraft({ modelUpstreamID })}
                  onChange={(modelRoutes) => updateDraft({ modelRoutes })}
                />
              ) : (
                <UpstreamSelector
                  upstreams={upstreams}
                  selected={draft.weightedUpstreams}
                  error={validation.errors.upstreams}
                  onChange={(weightedUpstreams) => updateDraft({ weightedUpstreams })}
                />
              )}
            </section>

            <section id="route-controls" className="detail-card composer-card route-form-section">
              <SectionTitle number="04" title="转发控制" description="按需配置请求头、超时和重试能力" />
              <RouteForwardControls
                draft={draft}
                errors={validation.errors}
                modelRouting={draft.forwardMode === 'model'}
                onChange={updateDraft}
              />
            </section>
          </div>
        </div>

        <RouteEditorSummary draft={draft} validation={validation} gateways={gateways} upstreams={upstreams} />
      </div>

      <div className="route-workbench-actions">
        <Button variant="ghost" disabled={submitting} onClick={onCancel}>取消</Button>
        <Button variant="primary" disabled={!validation.valid || submitting} onClick={onSave}>
          {submitting ? '保存中...' : '保存路由'}
        </Button>
      </div>
    </section>
  );
}

function RouteEditorSummary({
  draft,
  validation,
  gateways,
  upstreams,
}: {
  draft: RouteComposerDraft;
  validation: RouteDraftValidation;
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
}) {
  const errors = Object.values(validation.errors).filter(Boolean);
  const modelRouting = draft.forwardMode === 'model';
  const items = [
    { label: '入口网关', value: formatGatewayIDs(draft.gatewayIDs, gateways) },
    { label: '匹配请求', value: `${formatMethods(draft.methods)} ${draft.path || '/'}` },
    { label: '域名', value: formatHostnames(draft.hostnames) },
    { label: '转发方式', value: modelRouting ? '模型服务代理' : '普通服务转发' },
    {
      label: modelRouting ? '模型映射' : '目标服务',
      value: modelRouting
        ? formatModelRoutes(draft.modelUpstreamID, draft.modelRoutes, upstreams)
        : formatWeightedUpstreams(draft.weightedUpstreams, upstreams),
    },
    ...(modelRouting ? [{ label: '模型数量', value: `${draft.modelRoutes.length} 个` }] : [{ label: '总权重', value: String(upstreamWeightSum(draft.weightedUpstreams)) }]),
    { label: '转发控制', value: `${forwardControlCount(draft)} 项` },
    ...(draft.preservedRules.length > 0 ? [{ label: '附加规则', value: `保留 ${draft.preservedRules.length} 条` }] : []),
  ];

  return (
    <aside className="route-editor-summary" aria-label="路由配置摘要">
      <div className="route-summary-block">
        <div className="route-summary-title">请求链路</div>
        <dl>
          {items.map((item) => (
            <div key={item.label}>
              <dt>{item.label}</dt>
              <dd>{item.value}</dd>
            </div>
          ))}
        </dl>
      </div>
      <div className="route-summary-block">
        <div className="route-summary-title">保存检查</div>
        <div className={`route-summary-status ${validation.valid ? 'ok' : 'bad'}`.trim()}>{validation.summary}</div>
        {errors.length > 0 ? (
          <div className="route-summary-errors">
            {errors.map((error, index) => <div key={`${error}-${index}`}>{error}</div>)}
          </div>
        ) : null}
      </div>
    </aside>
  );
}

function ForwardModeSelector({
  value,
  onChange,
}: {
  value: RouteForwardMode;
  onChange: (value: RouteForwardMode) => void;
}) {
  const options: Array<{ value: RouteForwardMode; title: string; description: string }> = [
    { value: 'service', title: '普通服务转发', description: '选择一个或多个服务，并按权重分配请求' },
    { value: 'model', title: '模型路由', description: '连接一个大模型服务，并按请求中的 model 改写目标模型名称' },
  ];

  return (
    <div className="route-forward-mode" role="radiogroup" aria-label="转发方式">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={value === option.value}
          className={value === option.value ? 'active' : ''}
          onClick={() => onChange(option.value)}
        >
          <span className="route-forward-mode-dot" aria-hidden="true" />
          <span><strong>{option.title}</strong><small>{option.description}</small></span>
        </button>
      ))}
    </div>
  );
}

function ModelRouteEditor({
  upstreams,
  upstreamID,
  value,
  error,
  onUpstreamChange,
  onChange,
}: {
  upstreams: UpstreamOption[];
  upstreamID: string;
  value: ModelRoute[];
  error?: string;
  onUpstreamChange: (value: string) => void;
  onChange: (value: ModelRoute[]) => void;
}) {
  const modelUpstreams = upstreams.filter((upstream) => (
    upstream.type === 'model' && upstream.protocol === 'OpenAI'
  ));
  const selectedAvailable = modelUpstreams.some((upstream) => upstream.id === upstreamID);

  const update = (index: number, patch: Partial<ModelRoute>) => {
    onChange(value.map((model, currentIndex) => currentIndex === index ? { ...model, ...patch } : model));
  };

  return (
    <div className="model-route-editor">
      <label className="field model-route-service-field">
        <span>模型服务</span>
        <select value={upstreamID} onChange={(event) => onUpstreamChange(event.target.value)}>
          <option value="">请选择模型服务</option>
          {!selectedAvailable && upstreamID ? <option value={upstreamID}>当前服务不可用</option> : null}
          {modelUpstreams.map((upstream) => (
            <option key={upstream.id} value={upstream.id}>{upstream.name}</option>
          ))}
        </select>
        <small className="field-hint">一条模型路由固定连接一个模型服务，下面配置客户端模型别名。</small>
      </label>
      <div className="model-route-head">
        <div>
          <strong>模型映射</strong>
          <span>客户端模型名称用于匹配请求；目标模型名称留空时保持原值。</span>
        </div>
        <Button variant="soft" type="button" onClick={() => onChange([...value, createModelRoute()])}>添加模型</Button>
      </div>

      {modelUpstreams.length === 0 ? (
        <div className="model-route-empty">当前没有可用的大模型服务，请先在“服务”中创建 OpenAI 兼容的大模型服务。</div>
      ) : null}

      <div className="model-route-grid model-route-grid-head">
        <span>客户端模型</span>
        <span>目标模型（可选）</span>
        <span>操作</span>
      </div>
      {value.map((model, index) => (
        <div className="model-route-grid" key={`${index}-${model.model}`}>
          <input
            value={model.model}
            placeholder="例如 gpt-4o-mini"
            onChange={(event) => update(index, { model: event.target.value })}
          />
          <input
            value={model.upstreamModel ?? ''}
            placeholder="留空则与客户端模型相同"
            onChange={(event) => update(index, { upstreamModel: event.target.value })}
          />
          <button
            className="link-button danger"
            type="button"
            disabled={value.length <= 1}
            title={value.length <= 1 ? '至少保留一个模型' : undefined}
            onClick={() => onChange(value.filter((_, currentIndex) => currentIndex !== index))}
          >删除</button>
        </div>
      ))}
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function UpstreamSelector({
  upstreams,
  selected,
  error,
  onChange,
}: {
  upstreams: UpstreamOption[];
  selected: WeightedUpstream[];
  error?: string;
  onChange: (value: WeightedUpstream[]) => void;
}) {
  const [candidateID, setCandidateID] = useState('');
  const selectedIDs = new Set(selected.map((upstream) => upstream.upstreamID));
  const available = upstreams.filter((upstream) => (
    upstream.protocol === 'HTTP' && !selectedIDs.has(upstream.id)
  ));
  const selectedCandidateID = available.some((upstream) => upstream.id === candidateID) ? candidateID : '';

  const addUpstream = () => {
    if (!selectedCandidateID) {
      return;
    }
    onChange([...selected, { upstreamID: selectedCandidateID, weight: 100 }]);
    setCandidateID('');
  };

  return (
    <div className="upstream-config">
      <div className="field upstream-picker">
        <FieldLabel label="添加目标服务" required info="一个路由可以选择多个目标服务，并使用权重控制流量比例" />
        <div className="inline-input">
          <select value={selectedCandidateID} disabled={available.length === 0} onChange={(event) => setCandidateID(event.target.value)}>
            <option value="">{available.length === 0 ? '所有目标服务都已选择' : '请选择目标服务'}</option>
            {available.map((upstream) => (
              <option key={upstream.id} value={upstream.id}>{upstream.name} · {upstreamTypeLabel(upstream.type)}</option>
            ))}
          </select>
          <Button variant="soft" type="button" disabled={!selectedCandidateID} onClick={addUpstream}>添加目标服务</Button>
        </div>
      </div>

      <div className="upstream-card upstream-card-list">
        <div className="upstream-card-head">
          <div>
            <span className="mini-card-meta">已选目标服务</span>
            <strong>{selected.length} 个 / 总权重 {upstreamWeightSum(selected)}</strong>
          </div>
          <Badge tone={selected.length > 1 ? 'accent' : 'neutral'}>{selected.length > 1 ? '加权分流' : '单一目标服务'}</Badge>
        </div>
        <div className="upstream-list">
          {selected.length === 0 ? (
            <div className={`upstream-empty ${error ? 'error' : ''}`.trim()}>{error || '请选择至少一个目标服务'}</div>
          ) : selected.map((weightedUpstream) => {
            const upstream = upstreams.find((item) => item.id === weightedUpstream.upstreamID);
            return (
              <div key={weightedUpstream.upstreamID} className="upstream-row">
                <div className="upstream-main">
                  <strong>{upstream?.name ?? weightedUpstream.upstreamID}</strong>
                  <span>{upstream ? upstreamTypeLabel(upstream.type) : '未知类型'} · {upstream?.endpoint ?? '未配置端点'}</span>
                </div>
                <Badge tone="neutral">{upstream?.meta ?? '未配置端点'}</Badge>
                <label className="upstream-weight-field">
                  <span>权重</span>
                  <input
                    value={weightedUpstream.weight}
                    type="number"
                    min={1}
                    max={100}
                    onChange={(event) => onChange(selected.map((item) => (
                      item.upstreamID === weightedUpstream.upstreamID
                        ? { ...item, weight: Number(event.target.value) }
                        : item
                    )))}
                  />
                </label>
                <button
                  className="link-button danger"
                  type="button"
                  onClick={() => onChange(selected.filter((item) => item.upstreamID !== weightedUpstream.upstreamID))}
                >删除</button>
              </div>
            );
          })}
        </div>
        {error && selected.length > 0 ? <div className="form-error">{error}</div> : null}
      </div>
    </div>
  );
}

function MethodSelector({ value, onChange }: { value: HttpMethod[]; onChange: (value: HttpMethod[]) => void }) {
  return (
    <MultiSelectDropdown
      label="请求方法"
      emptyLabel="全部方法"
      info="不选择时匹配所有 HTTP 方法"
      options={httpMethods.map((method) => ({ value: method, label: method }))}
      value={value}
      onChange={(methods) => onChange(methods as HttpMethod[])}
    />
  );
}

function GatewayMultiSelect({
  options,
  value,
  error,
  onChange,
}: {
  options: RouteGatewayOption[];
  value: string[];
  error?: string;
  onChange: (value: string[]) => void;
}) {
  return (
    <div className={`field field-wide route-relation-field ${error ? 'invalid' : ''}`.trim()}>
      <MultiSelectDropdown
        label="所属网关"
        emptyLabel="请选择网关"
        required
        info="同一条路由可以同时关联多个网关"
        options={options.map((gateway) => ({ value: gateway.id, label: gateway.name }))}
        value={value}
        onChange={onChange}
      />
      {error ? <div className="form-error">{error}</div> : null}
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
            <button className={value.length === 0 ? 'active' : ''} type="button" onClick={() => onChange([])}>{emptyLabel}</button>
            {options.map((option) => (
              <button
                key={option.value}
                className={value.includes(option.value) ? 'active' : ''}
                type="button"
                onClick={() => onChange(value.includes(option.value)
                  ? value.filter((item) => item !== option.value)
                  : [...value, option.value])}
              >
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
    onChange(normalizeHostnames([...value, ...parseHostnames(inputValue)]));
    setInputValue('');
  };

  return (
    <div className={`field field-wide ${error ? 'invalid' : ''}`.trim()}>
      <FieldLabel label="匹配域名" info="留空表示不限制域名；支持 *.example.com" />
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
      <div className="tag-list">
        {value.length === 0 ? (
          <span className="mini-card-meta">不限制域名</span>
        ) : value.map((hostname) => (
          <button key={hostname} className="tag-chip" type="button" onClick={() => onChange(value.filter((item) => item !== hostname))}>
            {hostname}<span aria-hidden="true">×</span>
          </button>
        ))}
      </div>
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function HeaderMatchEditor({ value, error, onChange }: { value: HeaderMatch[]; error?: string; onChange: (value: HeaderMatch[]) => void }) {
  return (
    <div className={`field field-wide header-match-editor ${error ? 'invalid' : ''}`.trim()}>
      <div className="field-row-label">
        <FieldLabel label="请求头匹配" info="可选；当前使用精确匹配" />
        <Button variant="soft" type="button" onClick={() => onChange([...value, { name: '', value: '' }])}>添加条件</Button>
      </div>
      {value.length === 0 ? (
        <span className="mini-card-meta">不限制请求头</span>
      ) : (
        <div className="header-match-list">
          {value.map((header, index) => (
            <div key={index} className="header-match-row">
              <input
                value={header.name}
                placeholder="请求头名称"
                onChange={(event) => onChange(value.map((item, currentIndex) => (
                  currentIndex === index ? { ...item, name: event.target.value } : item
                )))}
              />
              <input
                value={header.value}
                placeholder="请求头值"
                onChange={(event) => onChange(value.map((item, currentIndex) => (
                  currentIndex === index ? { ...item, value: event.target.value } : item
                )))}
              />
              <button className="link-button danger" type="button" onClick={() => onChange(value.filter((_, currentIndex) => currentIndex !== index))}>删除</button>
            </div>
          ))}
        </div>
      )}
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}

function SectionTitle({ number, title, description }: { number: string; title: string; description: string }) {
  return (
    <div className="route-section-title">
      <span className="route-section-number">{number}</span>
      <div><h3>{title}</h3><p>{description}</p></div>
    </div>
  );
}

function FieldLabel({ label, required, info }: { label: string; required?: boolean; info?: string }) {
  return (
    <span className="field-label">
      <span>{required ? <span className="required-mark">*</span> : null}{label}</span>
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
  onChange,
}: {
  label: string;
  value: string;
  error?: string;
  required?: boolean;
  info?: string;
  maxLength?: number;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <FieldLabel label={label} required={required} info={info} />
      <input value={value} maxLength={maxLength} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} />
      {typeof maxLength === 'number' ? <div className="field-counter">{value.length}/{maxLength}</div> : null}
      {error ? <div className="form-error">{error}</div> : null}
    </div>
  );
}
