import type {
  PolicyTargetOption,
  PolicyTargetRef,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  RateLimitSubjectType,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

export interface RateLimitPolicyDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  subjectType: RateLimitSubjectType;
  headerName: string;
  requests: string;
  windowSeconds: string;
}

const fields = {
  name: 'name',
  headerName: 'subject.headerName',
  requests: 'limit.requests',
  windowSeconds: 'limit.windowSeconds',
} as const;

type FieldPath = typeof fields[keyof typeof fields];

export interface RateLimitPolicyValidation {
  valid: boolean;
  errors: Partial<Record<FieldPath, string>>;
}

const maxPluginInteger = 2_147_483_647;
const httpHeaderNamePattern = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/;

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
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title">
          <h3>基础信息</h3>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} error={validation.errors[fields.name]} onChange={(name) => onChange({ ...draft, name })} />
          <label className="policy-check-row">
            <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
            <span>启用策略</span>
          </label>
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>应用目标</h3>
        </div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect options={targets} value={draft.targets} onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })} />
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>请求额度</h3>
        </div>
        <div className="policy-editor-grid">
          <PolicySelectField
            label="计数对象"
            value={draft.subjectType}
            options={[
              ['Shared', '所有请求共用'],
              ['IP', '每个客户端 IP 独立'],
              ['Header', '每个请求头值独立'],
            ]}
            onChange={(subjectType) => onChange({
              ...draft,
              subjectType: subjectType as RateLimitSubjectType,
              headerName: subjectType === 'Header' ? draft.headerName : '',
            })}
          />
          {draft.subjectType === 'Header' ? (
            <PolicyInputField
              label="请求头名称"
              value={draft.headerName}
              placeholder="例如 x-client-id"
              error={validation.errors[fields.headerName]}
              onChange={(headerName) => onChange({ ...draft, headerName })}
            />
          ) : null}
          <PolicyInputField label="请求数" value={draft.requests} type="number" error={validation.errors[fields.requests]} onChange={(requests) => onChange({ ...draft, requests })} />
          <PolicyInputField label="时间窗口（秒）" value={draft.windowSeconds} type="number" error={validation.errors[fields.windowSeconds]} onChange={(windowSeconds) => onChange({ ...draft, windowSeconds })} />
        </div>
      </section>
    </div>
  );
}

export function createRateLimitPolicyDraft(policy?: RateLimitPolicy): RateLimitPolicyDraft {
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    subjectType: policy?.subject.type ?? 'Shared',
    headerName: policy?.subject.headerName ?? '',
    requests: String(policy?.limit.requests ?? 100),
    windowSeconds: String(policy?.limit.windowSeconds ?? 60),
  };
}

export function validateRateLimitPolicyDraft(draft: RateLimitPolicyDraft): RateLimitPolicyValidation {
  const errors: RateLimitPolicyValidation['errors'] = {};
  if (!draft.name.trim()) {
    errors[fields.name] = '请输入策略名称';
  }
  if (draft.subjectType === 'Header') {
    const headerName = draft.headerName.trim();
    if (!headerName || !httpHeaderNamePattern.test(headerName)) {
      errors[fields.headerName] = '请输入正确的请求头名称';
    }
  }
  errors[fields.requests] = integerFieldError(draft.requests, '请求数');
  errors[fields.windowSeconds] = integerFieldError(draft.windowSeconds, '时间窗口');
  return { valid: Object.values(errors).every((message) => !message), errors };
}

export function rateLimitPolicyPayload(draft: RateLimitPolicyDraft): RateLimitPolicyPayload {
  const config = {
    name: draft.name.trim(),
    enabled: draft.enabled,
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    subject: {
      type: draft.subjectType,
      ...(draft.subjectType === 'Header' ? { headerName: draft.headerName.trim().toLowerCase() } : {}),
    },
    limit: {
      requests: Number(draft.requests),
      windowSeconds: Number(draft.windowSeconds),
    },
  };
  if (!draft.id) {
    return config;
  }
  if (!draft.version) {
    throw new Error('策略版本缺失，请刷新后重试');
  }
  return { ...config, id: draft.id, version: draft.version };
}

function integerFieldError(value: string, label: string) {
  const parsed = Number(value.trim());
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > maxPluginInteger) {
    return `${label}必须是 1 到 ${maxPluginInteger.toLocaleString('zh-CN')} 之间的整数`;
  }
  return undefined;
}
