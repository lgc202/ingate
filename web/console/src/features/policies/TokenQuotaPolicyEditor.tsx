import type {
  PolicyTargetOption,
  PolicyTargetRef,
  TokenQuotaFailurePolicy,
  TokenQuotaPolicy,
  TokenQuotaPolicyPayload,
  TokenQuotaSubjectType,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

export interface TokenQuotaPolicyDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  subjectType: TokenQuotaSubjectType;
  headerName: string;
  tokens: string;
  windowValue: string;
  windowUnit: TokenQuotaWindowUnit;
  failurePolicy: TokenQuotaFailurePolicy;
  responseMessage: string;
}

type TokenQuotaWindowUnit = 'second' | 'minute' | 'hour' | 'day';

const tokenQuotaPolicyFields = {
  name: 'name',
  headerName: 'subject.headerName',
  tokens: 'quota.tokens',
  windowSeconds: 'quota.windowSeconds',
} as const;

type TokenQuotaPolicyFieldPath = typeof tokenQuotaPolicyFields[keyof typeof tokenQuotaPolicyFields];

export interface TokenQuotaPolicyValidation {
  valid: boolean;
  errors: Partial<Record<TokenQuotaPolicyFieldPath, string>>;
}

const tokenQuotaSubjectTypes: TokenQuotaSubjectType[] = ['Shared', 'IP', 'Header'];
const maxWindowSeconds = 2_147_483_647;
const windowUnitSeconds: Record<TokenQuotaWindowUnit, number> = {
  second: 1,
  minute: 60,
  hour: 3_600,
  day: 86_400,
};
const httpHeaderNamePattern = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/;

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
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title">
          <h3>基础信息</h3>
          <p>Token 配额用于保护模型服务预算，不影响普通 HTTP 请求。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField
            label="策略名称"
            value={draft.name}
            error={validation.errors[tokenQuotaPolicyFields.name]}
            onChange={(name) => onChange({ ...draft, name })}
          />
          <PolicyInputField label="描述" value={draft.description} onChange={(description) => onChange({ ...draft, description })} />
          <label className="policy-check-row">
            <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
            <span>启用策略</span>
          </label>
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>应用目标</h3>
          <p>可应用到模型路由或承载模型路由的网关；留空时仅保存策略。普通 HTTP 请求不会消耗 Token 额度。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect
            options={targets}
            value={draft.targets}
            requireTokenQuotaSupport
            onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })}
          />
        </div>
        <div className="mini-card policy-execution-note">
          <div className="mini-card-title">多个目标共享同一预算池</div>
          <div className="mini-card-meta">需要独立预算时，请分别创建 Token 配额策略。</div>
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>Token 配额</h3>
          <p>按输入与输出 Token 总量累计，额度耗尽后拒绝后续请求。请求完成后按实际用量记账，并发中的请求可能使最终用量略高于额度。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicySelectField
            label="额度划分方式"
            value={draft.subjectType}
            options={tokenQuotaSubjectTypes.map((type) => [type, tokenQuotaSubjectLabel(type)])}
            onChange={(subjectType) => {
              const nextSubjectType = subjectType as TokenQuotaSubjectType;
              onChange({
                ...draft,
                subjectType: nextSubjectType,
                headerName: nextSubjectType === 'Header' ? draft.headerName : '',
              });
            }}
          />
          {draft.subjectType === 'Header' ? (
            <PolicyInputField
              label="请求头名称"
              value={draft.headerName}
              placeholder="例如 x-customer-id"
              error={validation.errors[tokenQuotaPolicyFields.headerName]}
              onChange={(headerName) => onChange({ ...draft, headerName })}
            />
          ) : null}
          <PolicyInputField
            label="Token 额度"
            value={draft.tokens}
            type="number"
            error={validation.errors[tokenQuotaPolicyFields.tokens]}
            onChange={(tokens) => onChange({ ...draft, tokens })}
          />
          <PolicyInputField
            label="统计周期"
            value={draft.windowValue}
            type="number"
            error={validation.errors[tokenQuotaPolicyFields.windowSeconds]}
            onChange={(windowValue) => onChange({ ...draft, windowValue })}
          />
          <PolicySelectField
            label="周期单位"
            value={draft.windowUnit}
            options={[['second', '秒'], ['minute', '分钟'], ['hour', '小时'], ['day', '天']]}
            onChange={(windowUnit) => onChange({ ...draft, windowUnit: windowUnit as TokenQuotaWindowUnit })}
          />
        </div>
        {draft.subjectType === 'Header' ? (
          <div className="mini-card policy-trust-note">
            <div className="mini-card-title">请求头必须由可信认证层写入</div>
            <div className="mini-card-meta">如果客户端可以自行修改该请求头，就能通过更换值绕过额度；缺失该请求头的请求会共用同一个未标识预算池。</div>
          </div>
        ) : null}
        {draft.subjectType === 'IP' ? (
          <div className="mini-card policy-trust-note">
            <div className="mini-card-title">按网关看到的来源 IP 划分额度</div>
            <div className="mini-card-meta">如果前面还有负载均衡或反向代理且未保留真实来源地址，不同客户端可能共用同一个预算池。</div>
          </div>
        ) : null}
        <div className="mini-card policy-execution-note">
          <div className="mini-card-title">用于预算保护，不用于精确计费</div>
          <div className="mini-card-meta">流式请求在完成标记到达前被中断时可能不会记账。</div>
        </div>
      </section>

      <details className="policy-advanced">
        <summary>高级设置</summary>
        <div className="policy-advanced-body">
          <div className="form-section-title">
            <h3>执行与响应</h3>
            <p>配置配额状态不可用时的处理方式，以及额度耗尽后的响应。</p>
          </div>
          <div className="policy-editor-grid">
            <PolicySelectField
              label="配额状态不可用时"
              value={draft.failurePolicy}
              options={[['FailOpen', '放行请求'], ['FailClose', '拒绝请求']]}
              onChange={(failurePolicy) => onChange({ ...draft, failurePolicy: failurePolicy as TokenQuotaFailurePolicy })}
            />
            <PolicyInputField label="超额消息" value={draft.responseMessage} onChange={(responseMessage) => onChange({ ...draft, responseMessage })} />
          </div>
        </div>
      </details>
    </div>
  );
}

export function createTokenQuotaPolicyDraft(policy?: TokenQuotaPolicy): TokenQuotaPolicyDraft {
  const window = tokenQuotaWindowDraft(policy?.quota.windowSeconds ?? 3_600);
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    subjectType: policy?.subject.type ?? 'Shared',
    headerName: policy?.subject.headerName ?? '',
    tokens: String(policy?.quota.tokens ?? 100_000),
    windowValue: window.value,
    windowUnit: window.unit,
    failurePolicy: policy?.failurePolicy ?? 'FailClose',
    responseMessage: policy?.response.message ?? 'Token quota exceeded',
  };
}

export function validateTokenQuotaPolicyDraft(draft: TokenQuotaPolicyDraft): TokenQuotaPolicyValidation {
  const errors: TokenQuotaPolicyValidation['errors'] = {};
  if (!draft.name.trim()) {
    errors[tokenQuotaPolicyFields.name] = '请输入策略名称';
  }
  if (draft.subjectType === 'Header') {
    const headerName = draft.headerName.trim();
    if (!headerName) {
      errors[tokenQuotaPolicyFields.headerName] = '请输入请求头名称';
    } else if (!httpHeaderNamePattern.test(headerName)) {
      errors[tokenQuotaPolicyFields.headerName] = '请求头名称格式不正确';
    }
  }
  errors[tokenQuotaPolicyFields.tokens] = positiveSafeIntegerFieldError(draft.tokens, 'Token 额度');
  errors[tokenQuotaPolicyFields.windowSeconds] = tokenQuotaWindowFieldError(draft);
  return {
    valid: Object.values(errors).every((message) => !message),
    errors,
  };
}

export function tokenQuotaPolicyPayload(draft: TokenQuotaPolicyDraft): TokenQuotaPolicyPayload {
  const config = {
    name: draft.name.trim(),
    description: draft.description,
    enabled: draft.enabled,
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    subject: {
      type: draft.subjectType,
      ...(draft.subjectType === 'Header' ? { headerName: draft.headerName.trim() } : {}),
    },
    quota: {
      tokens: Number(draft.tokens),
      windowSeconds: tokenQuotaWindowSeconds(draft),
    },
    failurePolicy: draft.failurePolicy,
    response: {
      message: draft.responseMessage,
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

function positiveSafeIntegerFieldError(value: string, label: string) {
  const normalized = value.trim();
  if (!normalized) {
    return `请输入${label}`;
  }
  const parsed = Number(normalized);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    return `${label}必须是大于 0 的整数`;
  }
  if (!Number.isSafeInteger(parsed)) {
    return `${label}超出支持范围`;
  }
  return undefined;
}

function tokenQuotaWindowFieldError(draft: TokenQuotaPolicyDraft) {
  const normalized = draft.windowValue.trim();
  if (!normalized) {
    return '请输入统计周期';
  }
  const value = Number(normalized);
  if (!Number.isFinite(value) || value <= 0) {
    return '统计周期必须大于 0';
  }
  const seconds = tokenQuotaWindowSecondsValue(draft);
  const roundedSeconds = Math.round(seconds);
  const roundingTolerance = Number.EPSILON * Math.max(1, Math.abs(seconds)) * 4;
  if (Math.abs(seconds - roundedSeconds) > roundingTolerance) {
    return '统计周期换算后必须是整数秒';
  }
  if (roundedSeconds > maxWindowSeconds) {
    return `统计周期不能超过 ${maxWindowSeconds} 秒`;
  }
  return undefined;
}

function tokenQuotaWindowSeconds(draft: Pick<TokenQuotaPolicyDraft, 'windowValue' | 'windowUnit'>) {
  return Math.round(tokenQuotaWindowSecondsValue(draft));
}

function tokenQuotaWindowSecondsValue(draft: Pick<TokenQuotaPolicyDraft, 'windowValue' | 'windowUnit'>) {
  return Number(draft.windowValue) * windowUnitSeconds[draft.windowUnit];
}

function tokenQuotaWindowDraft(windowSeconds: number): { value: string; unit: TokenQuotaWindowUnit } {
  if (windowSeconds % windowUnitSeconds.day === 0) {
    return { value: String(windowSeconds / windowUnitSeconds.day), unit: 'day' };
  }
  if (windowSeconds % windowUnitSeconds.hour === 0) {
    return { value: String(windowSeconds / windowUnitSeconds.hour), unit: 'hour' };
  }
  if (windowSeconds % windowUnitSeconds.minute === 0) {
    return { value: String(windowSeconds / windowUnitSeconds.minute), unit: 'minute' };
  }
  return { value: String(windowSeconds), unit: 'second' };
}

function tokenQuotaSubjectLabel(type: TokenQuotaSubjectType) {
  const labels: Record<TokenQuotaSubjectType, string> = {
    Shared: '所有命中请求共用',
    IP: '每个来源 IP 独立',
    Header: '每个请求头值独立',
  };
  return labels[type];
}
