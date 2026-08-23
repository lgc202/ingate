import type {
  PolicyTargetOption,
  PolicyTargetRef,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  RateLimitSubjectType,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

type WindowUnit = 'Second' | 'Minute' | 'Hour' | 'Day';

const windowUnits: Array<{ value: WindowUnit; label: string; seconds: number }> = [
  { value: 'Second', label: '秒', seconds: 1 },
  { value: 'Minute', label: '分钟', seconds: 60 },
  { value: 'Hour', label: '小时', seconds: 3_600 },
  { value: 'Day', label: '天', seconds: 86_400 },
];

const subjectOptions: Array<{ value: RateLimitSubjectType; label: string; description: string }> = [
  { value: 'Shared', label: '目标共享', description: '网关或路由下的全部请求共同使用上限' },
  { value: 'IP', label: '按客户端 IP', description: '每个客户端连接来源地址分别计数' },
  { value: 'Header', label: '按请求头', description: '每个指定请求头值分别计数' },
];

const fields = {
  name: 'name',
  subject: 'subject',
  headerName: 'headerName',
  requests: 'requests',
  window: 'window',
} as const;

type FieldPath = typeof fields[keyof typeof fields];

export interface RateLimitPolicyDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  subject: RateLimitSubjectType | '';
  headerName: string;
  requests: string;
  windowValue: string;
  windowUnit: WindowUnit;
}

export interface RateLimitPolicyValidation {
  valid: boolean;
  errors: Partial<Record<FieldPath, string>>;
}

export function RateLimitPolicyEditor({
  draft,
  targets,
  validation,
  onChange,
}: {
  draft: RateLimitPolicyDraft;
  targets: PolicyTargetOption[];
  validation: RateLimitPolicyValidation;
  onChange: (draft: RateLimitPolicyDraft) => void;
}) {
  const trafficTargets = targets.filter((target) => target.kind === 'Gateway' || target.kind === 'Route');
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title"><h3>基础信息</h3></div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} error={validation.errors[fields.name]} onChange={(name) => onChange({ ...draft, name })} />
          {draft.id ? (
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
              <span>启用策略</span>
            </label>
          ) : null}
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>应用目标</h3></div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect
            options={trafficTargets}
            value={draft.targets}
            emptyMessage="暂无可选网关或路由"
            onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })}
          />
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>计数方式</h3></div>
        <div className="policy-restriction-mode policy-rate-subject" role="radiogroup" aria-label="限流计数方式">
          {subjectOptions.map((option) => (
            <button
              className={draft.subject === option.value ? 'selected' : ''}
              key={option.value}
              type="button"
              role="radio"
              aria-checked={draft.subject === option.value}
              onClick={() => onChange({
                ...draft,
                subject: option.value,
                headerName: option.value === 'Header' ? draft.headerName : '',
              })}
            >
              <strong>{option.label}</strong>
              <span>{option.description}</span>
            </button>
          ))}
        </div>
        {validation.errors[fields.subject] ? <div className="form-error" role="alert">{validation.errors[fields.subject]}</div> : null}
        {draft.subject === 'Header' ? (
          <div className="policy-editor-grid">
            <PolicyInputField
              label="请求头名称"
              value={draft.headerName}
              placeholder="例如：x-api-key"
              error={validation.errors[fields.headerName]}
              onChange={(headerName) => onChange({ ...draft, headerName })}
            />
          </div>
        ) : null}
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>请求上限</h3></div>
        <div className="policy-editor-grid">
          <PolicyInputField
            label="最大请求数"
            type="number"
            min="1"
            max="2147483647"
            step="1"
            value={draft.requests}
            placeholder="例如：100"
            error={validation.errors[fields.requests]}
            onChange={(requests) => onChange({ ...draft, requests })}
          />
          <PolicyInputField
            label="统计周期"
            type="number"
            min="1"
            max="2147483647"
            step="1"
            value={draft.windowValue}
            placeholder="例如：1"
            error={validation.errors[fields.window]}
            onChange={(windowValue) => onChange({ ...draft, windowValue })}
          />
          <PolicySelectField
            label="周期单位"
            value={draft.windowUnit}
            options={windowUnits.map((unit): [string, string] => [unit.value, unit.label])}
            onChange={(windowUnit) => onChange({ ...draft, windowUnit: windowUnit as WindowUnit })}
          />
        </div>
        <div className="mini-card policy-execution-note">
          <div className="mini-card-title">达到上限后返回 HTTP 429</div>
          <div className="mini-card-meta">进入下一个统计周期后自动恢复接收请求</div>
        </div>
      </section>
    </div>
  );
}

export function createRateLimitPolicyDraft(policy?: RateLimitPolicy): RateLimitPolicyDraft {
  const window = splitWindow(policy?.limit.windowSeconds);
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    subject: policy?.subject.type ?? '',
    headerName: policy?.subject.headerName ?? '',
    requests: policy ? String(policy.limit.requests) : '',
    windowValue: window.value,
    windowUnit: window.unit,
  };
}

export function validateRateLimitPolicyDraft(draft: RateLimitPolicyDraft): RateLimitPolicyValidation {
  const errors: RateLimitPolicyValidation['errors'] = {};
  if (!draft.name.trim()) errors[fields.name] = '请输入策略名称';
  if (!draft.subject) {
    errors[fields.subject] = '请选择计数方式';
  } else if (draft.subject === 'Header' && !validHeaderName(draft.headerName.trim())) {
    errors[fields.headerName] = '请输入有效的请求头名称';
  }
  if (!validPositiveInteger(draft.requests)) errors[fields.requests] = '请求数必须是大于 0 的整数';
  const windowSeconds = durationSeconds(draft.windowValue, draft.windowUnit);
  if (!validPositiveInteger(draft.windowValue) || windowSeconds > 2_147_483_647) {
    errors[fields.window] = '统计周期必须是有效的正整数';
  }
  return { valid: Object.values(errors).every((message) => !message), errors };
}

export function rateLimitPolicyPayload(draft: RateLimitPolicyDraft): RateLimitPolicyPayload {
  if (!draft.subject) throw new Error('限流计数方式缺失');
  const config = {
    name: draft.name.trim(),
    enabled: draft.enabled,
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    subject: {
      type: draft.subject,
      headerName: draft.subject === 'Header' ? draft.headerName.trim().toLowerCase() : '',
    },
    limit: {
      requests: Number(draft.requests),
      windowSeconds: durationSeconds(draft.windowValue, draft.windowUnit),
    },
  };
  if (!draft.id) return config;
  if (!draft.version) throw new Error('策略版本缺失，请刷新后重试');
  return { ...config, id: draft.id, version: draft.version };
}

function durationSeconds(value: string, unit: WindowUnit) {
  return Number(value) * (windowUnits.find((item) => item.value === unit)?.seconds ?? 1);
}

function splitWindow(seconds?: number): { value: string; unit: WindowUnit } {
  if (!seconds) return { value: '', unit: 'Minute' };
  const unit = [...windowUnits].reverse().find((item) => seconds % item.seconds === 0) ?? windowUnits[0];
  return { value: String(seconds / unit.seconds), unit: unit.value };
}

function validPositiveInteger(value: string) {
  const number = Number(value);
  return Number.isSafeInteger(number) && number > 0 && number <= 2_147_483_647;
}

function validHeaderName(value: string) {
  return /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(value) && !value.startsWith(':');
}
