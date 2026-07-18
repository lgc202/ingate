import type {
  RateLimitAlgorithm,
  RateLimitFailurePolicy,
  RateLimitKeyPart,
  RateLimitKeyType,
  RateLimitMode,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  RateLimitRule,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';

export interface RateLimitPolicyDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  mode: RateLimitMode;
  ruleName: string;
  keyType: RateLimitKeyType;
  keyName: string;
  requests: string;
  windowSeconds: string;
  burst: string;
  algorithm: RateLimitAlgorithm;
  failurePolicy: RateLimitFailurePolicy;
  responseStatusCode: string;
  responseMessage: string;
  quotaHeaderEnabled: boolean;
  preservedKeyParts: RateLimitKeyPart[];
  preservedRules: RateLimitRule[];
}

const rateLimitKeyTypes: RateLimitKeyType[] = [
  'IP',
  'Header',
  'Query',
  'Cookie',
  'Consumer',
  'Route',
  'Gateway',
  'RouteRule',
  'JWTClaim',
  'APIKey',
  'Tenant',
];

export function RateLimitPolicyEditor({
  draft,
  onChange,
}: {
  draft: RateLimitPolicyDraft;
  onChange: (draft: RateLimitPolicyDraft) => void;
}) {
  const needsKeyName = ['Header', 'Query', 'Cookie', 'JWTClaim'].includes(draft.keyType);

  return (
    <div className="editor-main-stack">
      {draft.preservedKeyParts.length > 0 || draft.preservedRules.length > 0 ? (
        <div className="mini-card policy-preserved-note">
          <div className="mini-card-title">保留已有高级规则</div>
          <div className="mini-card-meta">当前页面编辑第一条规则的第一个计数维度；其余 {draft.preservedKeyParts.length} 个维度和 {draft.preservedRules.length} 条规则会原样保存。</div>
        </div>
      ) : null}
      <section className="form-section">
        <div className="form-section-title">
          <h3>基础信息</h3>
          <p>定义策略名称、启用状态和计数范围。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
          <PolicySelectField
            label="限流模式"
            value={draft.mode}
            options={[['Local', '单实例计数'], ['Global', '全局共享计数']]}
            onChange={(mode) => onChange({ ...draft, mode: mode as RateLimitMode })}
          />
          <PolicyInputField label="描述" value={draft.description} onChange={(description) => onChange({ ...draft, description })} />
          {draft.mode === 'Global' ? (
            <div className="mini-card field-wide policy-execution-note">
              <div className="mini-card-meta">共享状态</div>
              <div className="mini-card-title">所有网关实例自动共享计数状态，无需额外配置</div>
            </div>
          ) : null}
          <label className="policy-check-row">
            <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
            <span>启用策略</span>
          </label>
        </div>
      </section>
      <section className="form-section">
        <div className="form-section-title">
          <h3>计数规则</h3>
          <p>配置当前主规则的计数维度、算法和额度。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="规则名称" value={draft.ruleName} onChange={(ruleName) => onChange({ ...draft, ruleName })} />
          <PolicySelectField
            label="计数维度"
            value={draft.keyType}
            options={rateLimitKeyTypes.map((type) => [type, rateLimitKeyLabel(type)])}
            onChange={(keyType) => onChange({ ...draft, keyType: keyType as RateLimitKeyType })}
          />
          {needsKeyName ? <PolicyInputField label="维度名称" value={draft.keyName} onChange={(keyName) => onChange({ ...draft, keyName })} /> : null}
          <PolicySelectField
            label="算法"
            value={draft.algorithm}
            options={[['FixedWindow', '固定窗口'], ['SlidingWindow', '滑动窗口'], ['TokenBucket', '令牌桶']]}
            onChange={(algorithm) => onChange({ ...draft, algorithm: algorithm as RateLimitAlgorithm })}
          />
          <PolicyInputField label="请求数" value={draft.requests} type="number" onChange={(requests) => onChange({ ...draft, requests })} />
          <PolicyInputField label="窗口秒数" value={draft.windowSeconds} type="number" onChange={(windowSeconds) => onChange({ ...draft, windowSeconds })} />
          <PolicyInputField label="突发额度" value={draft.burst} type="number" onChange={(burst) => onChange({ ...draft, burst })} />
        </div>
      </section>
      <section className="form-section">
        <div className="form-section-title">
          <h3>失败与超限响应</h3>
          <p>定义限流执行异常和请求超过额度时的处理方式。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicySelectField
            label="失败策略"
            value={draft.failurePolicy}
            options={[['FailOpen', '失败放行'], ['FailClose', '失败拒绝']]}
            onChange={(failurePolicy) => onChange({ ...draft, failurePolicy: failurePolicy as RateLimitFailurePolicy })}
          />
          <PolicyInputField label="超限状态码" value={draft.responseStatusCode} type="number" onChange={(responseStatusCode) => onChange({ ...draft, responseStatusCode })} />
          <PolicyInputField label="超限消息" value={draft.responseMessage} onChange={(responseMessage) => onChange({ ...draft, responseMessage })} />
          <label className="policy-check-row">
            <input type="checkbox" checked={draft.quotaHeaderEnabled} onChange={(event) => onChange({ ...draft, quotaHeaderEnabled: event.target.checked })} />
            <span>返回限流配额响应头</span>
          </label>
        </div>
      </section>
    </div>
  );
}

export function createRateLimitPolicyDraft(policy?: RateLimitPolicy): RateLimitPolicyDraft {
  const rule = policy?.rules[0];
  const keyPart = rule?.key.parts[0];
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    enabled: policy?.enabled ?? true,
    mode: policy?.mode ?? 'Local',
    ruleName: rule?.name ?? 'default',
    keyType: keyPart?.type ?? 'IP',
    keyName: keyPart?.name ?? '',
    requests: String(rule?.limit.requests ?? 100),
    windowSeconds: String(rule?.limit.windowSeconds ?? 60),
    burst: String(rule?.limit.burst ?? 0),
    algorithm: rule?.algorithm ?? 'FixedWindow',
    failurePolicy: policy?.failurePolicy ?? 'FailOpen',
    responseStatusCode: String(policy?.response?.statusCode ?? 429),
    responseMessage: policy?.response?.message ?? 'Too many requests',
    quotaHeaderEnabled: policy?.response?.quotaHeaderEnabled ?? true,
    preservedKeyParts: rule?.key.parts.slice(1) ?? [],
    preservedRules: policy?.rules.slice(1) ?? [],
  };
}

export function rateLimitPolicyPayload(draft: RateLimitPolicyDraft): RateLimitPolicyPayload {
  const keyPart = { type: draft.keyType, ...(draft.keyName ? { name: draft.keyName } : {}) };
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    mode: draft.mode,
    rules: [
      {
        name: draft.ruleName,
        key: { parts: [keyPart, ...draft.preservedKeyParts] },
        limit: {
          requests: Number(draft.requests),
          windowSeconds: Number(draft.windowSeconds),
          burst: Number(draft.burst || 0),
        },
        algorithm: draft.algorithm,
      },
      ...draft.preservedRules,
    ],
    response: {
      statusCode: Number(draft.responseStatusCode || 429),
      message: draft.responseMessage,
      quotaHeaderEnabled: draft.quotaHeaderEnabled,
    },
    failurePolicy: draft.failurePolicy,
  };
}

function rateLimitKeyLabel(type: RateLimitKeyType) {
  const labels: Record<RateLimitKeyType, string> = {
    IP: '客户端 IP',
    Header: '请求头',
    Query: '查询参数',
    Cookie: 'Cookie',
    Consumer: '调用方',
    Route: '路由',
    Gateway: '网关',
    RouteRule: '路由规则',
    JWTClaim: 'JWT 字段',
    APIKey: 'API 密钥',
    Tenant: '租户',
  };
  return labels[type];
}
