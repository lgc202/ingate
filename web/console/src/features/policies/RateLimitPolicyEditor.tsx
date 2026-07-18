import type {
  PolicyTargetOption,
  PolicyTargetRef,
  RateLimitFailurePolicy,
  RateLimitKeyPart,
  RateLimitKeyType,
  RateLimitPolicy,
  RateLimitPolicyPayload,
  RateLimitRule,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

export interface RateLimitPolicyDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  ruleName: string;
  keyType: RateLimitKeyType;
  keyName: string;
  requests: string;
  windowSeconds: string;
  burst: string;
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
  'Route',
  'Gateway',
  'RouteRule',
];

const namedRateLimitKeyTypes: RateLimitKeyType[] = ['Header', 'Query', 'Cookie'];

export function RateLimitPolicyEditor({
  draft,
  targets,
  onChange,
}: {
  draft: RateLimitPolicyDraft;
  targets: PolicyTargetOption[];
  onChange: (draft: RateLimitPolicyDraft) => void;
}) {
  const needsKeyName = rateLimitKeyNeedsName(draft.keyType);

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
          <p>策略保存后，在当前环境内统一共享计数状态。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
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
          <p>可同时应用到多个网关或路由；留空时仅保存策略，不会处理请求。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect options={targets} value={draft.targets} onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })} />
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>请求额度</h3>
          <p>按指定维度统计请求，并在时间窗口内执行共享限流。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicySelectField
            label="计数维度"
            value={draft.keyType}
            options={rateLimitKeyTypes.map((type) => [type, rateLimitKeyLabel(type)])}
            onChange={(keyType) => {
              const nextKeyType = keyType as RateLimitKeyType;
              onChange({
                ...draft,
                keyType: nextKeyType,
                keyName: rateLimitKeyNeedsName(nextKeyType) ? draft.keyName : '',
              });
            }}
          />
          {needsKeyName ? <PolicyInputField label="维度名称" value={draft.keyName} onChange={(keyName) => onChange({ ...draft, keyName })} /> : null}
          <PolicyInputField label="请求数" value={draft.requests} type="number" onChange={(requests) => onChange({ ...draft, requests })} />
          <PolicyInputField label="时间窗口（秒）" value={draft.windowSeconds} type="number" onChange={(windowSeconds) => onChange({ ...draft, windowSeconds })} />
        </div>
      </section>

      <details className="policy-advanced">
        <summary>高级设置</summary>
        <div className="policy-advanced-body">
          <div className="form-section-title">
            <h3>执行与响应</h3>
            <p>仅在需要自定义故障处理、突发额度或超限响应时调整。</p>
          </div>
          <div className="policy-editor-grid">
            <PolicyInputField label="规则名称" value={draft.ruleName} onChange={(ruleName) => onChange({ ...draft, ruleName })} />
            <PolicyInputField label="突发额度" value={draft.burst} type="number" onChange={(burst) => onChange({ ...draft, burst })} />
            <PolicySelectField
              label="执行异常时"
              value={draft.failurePolicy}
              options={[['FailOpen', '放行请求'], ['FailClose', '拒绝请求']]}
              onChange={(failurePolicy) => onChange({ ...draft, failurePolicy: failurePolicy as RateLimitFailurePolicy })}
            />
            <PolicyInputField label="超限状态码" value={draft.responseStatusCode} type="number" onChange={(responseStatusCode) => onChange({ ...draft, responseStatusCode })} />
            <PolicyInputField label="超限消息" value={draft.responseMessage} onChange={(responseMessage) => onChange({ ...draft, responseMessage })} />
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.quotaHeaderEnabled} onChange={(event) => onChange({ ...draft, quotaHeaderEnabled: event.target.checked })} />
              <span>返回配额响应头</span>
            </label>
          </div>
        </div>
      </details>
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
    targets: policy?.targets ?? [],
    ruleName: rule?.name ?? 'default',
    keyType: keyPart?.type ?? 'IP',
    keyName: keyPart?.name ?? '',
    requests: String(rule?.limit.requests ?? 100),
    windowSeconds: String(rule?.limit.windowSeconds ?? 60),
    burst: String(rule?.limit.burst ?? 0),
    failurePolicy: policy?.failurePolicy ?? 'FailOpen',
    responseStatusCode: String(policy?.response?.statusCode ?? 429),
    responseMessage: policy?.response?.message ?? 'Too many requests',
    quotaHeaderEnabled: policy?.response?.quotaHeaderEnabled ?? true,
    preservedKeyParts: rule?.key.parts.slice(1) ?? [],
    preservedRules: policy?.rules.slice(1) ?? [],
  };
}

export function rateLimitPolicyPayload(draft: RateLimitPolicyDraft): RateLimitPolicyPayload {
  const keyPart = {
    type: draft.keyType,
    ...(rateLimitKeyNeedsName(draft.keyType) && draft.keyName ? { name: draft.keyName } : {}),
  };
  const config = {
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    rules: [
      {
        name: draft.ruleName,
        key: { parts: [keyPart, ...draft.preservedKeyParts] },
        limit: {
          requests: Number(draft.requests),
          windowSeconds: Number(draft.windowSeconds),
          burst: Number(draft.burst || 0),
        },
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
  if (!draft.id) {
    return config;
  }
  if (!draft.version) {
    throw new Error('策略版本缺失，请刷新后重试');
  }
  return { ...config, id: draft.id, version: draft.version };
}

function rateLimitKeyNeedsName(type: RateLimitKeyType) {
  return namedRateLimitKeyTypes.includes(type);
}

function rateLimitKeyLabel(type: RateLimitKeyType) {
  const labels: Record<RateLimitKeyType, string> = {
    IP: '客户端 IP',
    Header: '请求头',
    Query: '查询参数',
    Cookie: 'Cookie',
    Route: '路由',
    Gateway: '网关',
    RouteRule: '路由规则',
  };
  return labels[type];
}
