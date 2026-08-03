import { Badge, Button } from '@/components/ui';
import type { HeaderModifier, HeaderValue, RouteRetry, RouteTimeout } from '@/domain/route';
import type { RouteComposerDraft, RouteDraftErrors } from './composer';
import { forwardControlCount } from './composer';

const defaultTimeout: RouteTimeout = { requestMillis: 30000 };
const defaultRetry: RouteRetry = { attempts: 2, perTryTimeoutMillis: 1000 };

export function RouteForwardControls({
  draft,
  errors,
  modelRouting = false,
  onChange,
}: {
  draft: RouteComposerDraft;
  errors: RouteDraftErrors;
  modelRouting?: boolean;
  onChange: (patch: Partial<RouteComposerDraft>) => void;
}) {
  const enabledCount = forwardControlCount(draft);

  return (
    <div className="route-forward-controls">
      <div className="forward-controls-head">
        <div>
          <h4>转发控制</h4>
          <p>这些能力由网关直接执行，不需要安装额外插件。</p>
        </div>
        <Badge tone={enabledCount > 0 ? 'accent' : 'neutral'}>已启用 {enabledCount} 项</Badge>
      </div>

      <div className="route-policy-capability-list">
        <HeaderModifierControl
          title="请求头改写"
          description="请求转发到目标服务前，写入或删除请求头"
          value={draft.requestHeaderModifier}
          error={errors.requestHeaderModifier}
          onChange={(requestHeaderModifier) => onChange({ requestHeaderModifier })}
        />
        <HeaderModifierControl
          title="响应头改写"
          description="响应返回给调用方前，写入或删除响应头"
          value={draft.responseHeaderModifier}
          error={errors.responseHeaderModifier}
          onChange={(responseHeaderModifier) => onChange({ responseHeaderModifier })}
        />
        <TimeoutControl
          value={draft.timeout}
          error={errors.timeout}
          onChange={(timeout) => onChange({ timeout })}
        />
        {modelRouting ? (
          <div className="route-control-note">
            <strong>模型路由暂不配置失败重试</strong>
            <span>避免同一次模型请求被重复发送；请求超时仍可单独设置。</span>
          </div>
        ) : (
          <RetryControl
            value={draft.retry}
            error={errors.retry}
            onChange={(retry) => onChange({ retry })}
          />
        )}
      </div>
    </div>
  );
}

function HeaderModifierControl({
  title,
  description,
  value,
  error,
  onChange,
}: {
  title: string;
  description: string;
  value?: HeaderModifier;
  error?: string;
  onChange: (value: HeaderModifier | undefined) => void;
}) {
  const enabled = Boolean(value);
  const modifier = value ?? { set: [], remove: [] };

  const updateSetHeader = (index: number, patch: Partial<HeaderValue>) => {
    onChange({
      ...modifier,
      set: modifier.set.map((header, currentIndex) => (currentIndex === index ? { ...header, ...patch } : header)),
    });
  };

  const updateRemoveHeader = (index: number, name: string) => {
    onChange({
      ...modifier,
      remove: modifier.remove.map((headerName, currentIndex) => (currentIndex === index ? name : headerName)),
    });
  };

  return (
    <article className={`route-policy-capability ${enabled ? 'enabled' : ''}`.trim()}>
      <ControlToggle
        title={title}
        description={description}
        enabled={enabled}
        category="请求头"
        onToggle={() => onChange(enabled ? undefined : { set: [{ name: '', value: '' }], remove: [] })}
      />
      {enabled ? (
        <div className="policy-param-grid">
          <div className="field field-wide header-match-editor">
            <div className="field-row-label">
              <FieldHeading title="写入请求头" hint="名称相同时覆盖现有值" />
              <Button
                variant="soft"
                type="button"
                onClick={() => onChange({ ...modifier, set: [...modifier.set, { name: '', value: '' }] })}
              >添加</Button>
            </div>
            {modifier.set.length === 0 ? (
              <span className="mini-card-meta">未配置写入动作</span>
            ) : (
              <div className="header-match-list">
                {modifier.set.map((header, index) => (
                  <div key={index} className="header-match-row">
                    <input
                      value={header.name}
                      placeholder="请求头名称"
                      onChange={(event) => updateSetHeader(index, { name: event.target.value })}
                    />
                    <input
                      value={header.value}
                      placeholder="请求头值"
                      onChange={(event) => updateSetHeader(index, { value: event.target.value })}
                    />
                    <button
                      className="link-button danger"
                      type="button"
                      onClick={() => onChange({ ...modifier, set: modifier.set.filter((_, currentIndex) => currentIndex !== index) })}
                    >删除</button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="field field-wide header-match-editor">
            <div className="field-row-label">
              <FieldHeading title="删除请求头" hint="转发前移除指定请求头" />
              <Button
                variant="soft"
                type="button"
                onClick={() => onChange({ ...modifier, remove: [...modifier.remove, ''] })}
              >添加</Button>
            </div>
            {modifier.remove.length === 0 ? (
              <span className="mini-card-meta">未配置删除动作</span>
            ) : (
              <div className="header-match-list">
                {modifier.remove.map((headerName, index) => (
                  <div key={index} className="header-match-row">
                    <input
                      value={headerName}
                      placeholder="请求头名称，例如 x-internal-token"
                      onChange={(event) => updateRemoveHeader(index, event.target.value)}
                    />
                    <span aria-hidden="true" />
                    <button
                      className="link-button danger"
                      type="button"
                      onClick={() => onChange({ ...modifier, remove: modifier.remove.filter((_, currentIndex) => currentIndex !== index) })}
                    >删除</button>
                  </div>
                ))}
              </div>
            )}
          </div>
          {error ? <div className="form-error">{error}</div> : null}
        </div>
      ) : null}
    </article>
  );
}

function TimeoutControl({
  value,
  error,
  onChange,
}: {
  value?: RouteTimeout;
  error?: string;
  onChange: (value: RouteTimeout | undefined) => void;
}) {
  const enabled = Boolean(value);

  return (
    <article className={`route-policy-capability ${enabled ? 'enabled' : ''}`.trim()}>
      <ControlToggle
        title="请求总超时"
        description="限制一条请求从匹配路由到目标服务响应的总时间"
        enabled={enabled}
        category="可靠性"
        onToggle={() => onChange(enabled ? undefined : { ...defaultTimeout })}
      />
      {value ? (
        <div className="policy-param-grid">
          <label className={`policy-param-field ${error ? 'invalid' : ''}`.trim()}>
            <FieldHeading title="总超时" hint="100-300000ms" />
            <div className="unit-input">
              <input
                type="number"
                min={100}
                max={300000}
                value={value.requestMillis}
                onChange={(event) => onChange({ requestMillis: Number(event.target.value) })}
              />
              <span>ms</span>
            </div>
            {error ? <span className="form-error">{error}</span> : null}
          </label>
        </div>
      ) : null}
    </article>
  );
}

function RetryControl({
  value,
  error,
  onChange,
}: {
  value?: RouteRetry;
  error?: string;
  onChange: (value: RouteRetry | undefined) => void;
}) {
  const enabled = Boolean(value);

  return (
    <article className={`route-policy-capability ${enabled ? 'enabled' : ''}`.trim()}>
      <ControlToggle
        title="失败重试"
        description="目标服务请求失败时按固定次数重试"
        enabled={enabled}
        category="可靠性"
        onToggle={() => onChange(enabled ? undefined : { ...defaultRetry })}
      />
      {value ? (
        <div className="policy-param-grid">
          <label className={`policy-param-field ${error ? 'invalid' : ''}`.trim()}>
            <FieldHeading title="重试次数" hint="1-5 次" />
            <div className="unit-input">
              <input
                type="number"
                min={1}
                max={5}
                value={value.attempts}
                onChange={(event) => onChange({ ...value, attempts: Number(event.target.value) })}
              />
              <span>次</span>
            </div>
          </label>
          <label className={`policy-param-field ${error ? 'invalid' : ''}`.trim()}>
            <FieldHeading title="单次超时" hint="不能超过请求总超时" />
            <div className="unit-input">
              <input
                type="number"
                min={100}
                max={60000}
                value={value.perTryTimeoutMillis}
                onChange={(event) => onChange({ ...value, perTryTimeoutMillis: Number(event.target.value) })}
              />
              <span>ms</span>
            </div>
          </label>
          {error ? <div className="form-error">{error}</div> : null}
        </div>
      ) : null}
    </article>
  );
}

function ControlToggle({
  title,
  description,
  category,
  enabled,
  onToggle,
}: {
  title: string;
  description: string;
  category: string;
  enabled: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="route-policy-capability-head">
      <button className="route-policy-toggle" type="button" role="switch" aria-checked={enabled} onClick={onToggle}>
        <span className={`switch ${enabled ? 'on' : ''}`} aria-hidden="true" />
        <span>
          <strong>{title}</strong>
          <small>{description}</small>
        </span>
      </button>
      <Badge tone="neutral">{category}</Badge>
    </div>
  );
}

function FieldHeading({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="space-y-0.5 mb-1">
      <span className="block text-xs font-semibold text-slate-700">{title}</span>
      {hint ? <p className="text-[11px] text-slate-400 font-normal leading-normal">{hint}</p> : null}
    </div>
  );
}
