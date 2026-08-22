import type {
  PolicyTargetOption,
  PolicyTargetRef,
  TokenQuotaPeriod,
  TokenQuotaPolicy,
  TokenQuotaPolicyPayload,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

const periods: Array<{ period: TokenQuotaPeriod; label: string }> = [
  { period: 'Day', label: '每日额度' },
  { period: 'Week', label: '每周额度' },
  { period: 'Month', label: '每月额度' },
];

const fields = {
  name: 'name',
  limits: 'limits',
} as const;

type FieldPath = typeof fields[keyof typeof fields];

export interface TokenQuotaPolicyDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  timeZone: string;
  limits: Record<TokenQuotaPeriod, { enabled: boolean; tokens: string }>;
}

export interface TokenQuotaPolicyValidation {
  valid: boolean;
  errors: Partial<Record<FieldPath, string>>;
}

export function TokenQuotaPolicyEditor({
  draft,
  targets,
  validation,
  onChange,
}: {
  draft: TokenQuotaPolicyDraft;
  targets: PolicyTargetOption[];
  validation: TokenQuotaPolicyValidation;
  onChange: (draft: TokenQuotaPolicyDraft) => void;
}) {
  const callerTargets = targets.filter((target) => target.kind === 'Caller');
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title"><h3>基础信息</h3></div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} error={validation.errors[fields.name]} onChange={(name) => onChange({ ...draft, name })} />
          <PolicySelectField label="周期时区" value={draft.timeZone} options={timeZoneOptions(draft.timeZone)} onChange={(timeZone) => onChange({ ...draft, timeZone })} />
          {draft.id ? (
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
              <span>启用策略</span>
            </label>
          ) : null}
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>应用调用方</h3></div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect
            label="调用方"
            emptyMessage="暂无可选调用方"
            options={callerTargets}
            value={draft.targets}
            onChange={(targets) => onChange({ ...draft, targets })}
          />
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>Token 上限</h3></div>
        <div className="policy-quota-list">
          {periods.map(({ period, label }) => {
            const limit = draft.limits[period];
            return (
              <label className={`policy-quota-row ${limit.enabled ? 'selected' : ''}`.trim()} key={period}>
                <input
                  type="checkbox"
                  checked={limit.enabled}
                  onChange={(event) => onChange({
                    ...draft,
                    limits: { ...draft.limits, [period]: { ...limit, enabled: event.target.checked } },
                  })}
                />
                <strong>{label}</strong>
                <input
                  type="number"
                  min="1"
                  step="1"
                  disabled={!limit.enabled}
                  value={limit.tokens}
                  aria-label={`${label} Token 数`}
                  placeholder="Token 数"
                  onChange={(event) => onChange({
                    ...draft,
                    limits: { ...draft.limits, [period]: { ...limit, tokens: event.target.value } },
                  })}
                />
                <span>Token</span>
              </label>
            );
          })}
        </div>
        {validation.errors[fields.limits] ? <div className="form-error" role="alert">{validation.errors[fields.limits]}</div> : null}
        <div className="mini-card policy-execution-note">
          <div className="mini-card-title">按模型返回的实际 Token 结算</div>
          <div className="mini-card-meta">当前调用可能越过额度；达到上限后，后续调用会被拒绝。</div>
        </div>
      </section>
    </div>
  );
}

export function createTokenQuotaPolicyDraft(policy?: TokenQuotaPolicy): TokenQuotaPolicyDraft {
  const configured = new Map(policy?.limits.map((limit) => [limit.period, String(limit.tokens)]));
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    timeZone: policy?.timeZone ?? browserTimeZone(),
    limits: {
      Day: { enabled: configured.has('Day'), tokens: configured.get('Day') ?? '' },
      Week: { enabled: configured.has('Week') || !policy, tokens: configured.get('Week') ?? '' },
      Month: { enabled: configured.has('Month'), tokens: configured.get('Month') ?? '' },
    },
  };
}

export function validateTokenQuotaPolicyDraft(draft: TokenQuotaPolicyDraft): TokenQuotaPolicyValidation {
  const errors: TokenQuotaPolicyValidation['errors'] = {};
  if (!draft.name.trim()) {
    errors[fields.name] = '请输入策略名称';
  }
  const selected = periods.filter(({ period }) => draft.limits[period].enabled);
  if (selected.length === 0) {
    errors[fields.limits] = '请至少启用一个额度周期';
  } else if (selected.some(({ period }) => !validTokenLimit(draft.limits[period].tokens))) {
    errors[fields.limits] = '额度必须是大于 0 的整数';
  }
  return { valid: Object.values(errors).every((message) => !message), errors };
}

export function tokenQuotaPolicyPayload(draft: TokenQuotaPolicyDraft): TokenQuotaPolicyPayload {
  const config = {
    name: draft.name.trim(),
    enabled: draft.enabled,
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    timeZone: draft.timeZone,
    limits: periods.flatMap(({ period }) => draft.limits[period].enabled
      ? [{ period, tokens: Number(draft.limits[period].tokens) }]
      : []),
  };
  if (!draft.id) return config;
  if (!draft.version) throw new Error('策略版本缺失，请刷新后重试');
  return { ...config, id: draft.id, version: draft.version };
}

function validTokenLimit(value: string) {
  const number = Number(value);
  return Number.isSafeInteger(number) && number > 0;
}

function browserTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

function timeZoneOptions(current: string): Array<[string, string]> {
  const values = Array.from(new Set([current, browserTimeZone(), 'UTC', 'Asia/Shanghai', 'Asia/Tokyo', 'Europe/London', 'America/New_York']));
  return values.map((value) => [value, value]);
}
