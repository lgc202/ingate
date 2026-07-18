import type {
  AccessControlAction,
  AccessControlCondition,
  AccessControlConditionType,
  AccessControlPolicy,
  AccessControlPolicyPayload,
  AccessControlRule,
} from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';

export interface AccessControlPolicyDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  defaultAction: AccessControlAction;
  ruleEnabled: boolean;
  ruleName: string;
  ruleAction: AccessControlAction;
  conditionType: AccessControlConditionType;
  conditionName: string;
  conditionValue: string;
  responseStatusCode: string;
  responseMessage: string;
  preservedConditions: AccessControlCondition[];
  preservedRules: AccessControlRule[];
}

const accessControlConditionTypes: AccessControlConditionType[] = ['IP', 'Header', 'Consumer', 'Tenant'];

export function AccessControlPolicyEditor({
  draft,
  onChange,
}: {
  draft: AccessControlPolicyDraft;
  onChange: (draft: AccessControlPolicyDraft) => void;
}) {
  const needsConditionName = draft.conditionType === 'Header';

  return (
    <div className="editor-main-stack">
      {draft.preservedConditions.length > 0 || draft.preservedRules.length > 0 ? (
        <div className="mini-card policy-preserved-note">
          <div className="mini-card-title">保留已有高级规则</div>
          <div className="mini-card-meta">当前页面编辑第一条规则的第一个匹配条件；其余 {draft.preservedConditions.length} 个条件和 {draft.preservedRules.length} 条规则会原样保存。</div>
        </div>
      ) : null}
      <section className="form-section">
        <div className="form-section-title">
          <h3>基础信息</h3>
          <p>默认动作决定没有命中规则的请求如何处理。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
          <PolicySelectField
            label="默认动作"
            value={draft.defaultAction}
            options={draft.ruleEnabled ? [['Allow', '默认放行'], ['Deny', '默认拒绝']] : [['Deny', '默认拒绝']]}
            onChange={(defaultAction) => onChange({ ...draft, defaultAction: defaultAction as AccessControlAction })}
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
          <h3>访问规则</h3>
          <p>配置当前主规则的动作和匹配条件。</p>
        </div>
        {draft.preservedRules.length === 0 ? (
          <label className="policy-check-row policy-rule-toggle">
            <input
              type="checkbox"
              checked={draft.ruleEnabled}
              onChange={(event) => onChange({
                ...draft,
                ruleEnabled: event.target.checked,
                defaultAction: event.target.checked ? draft.defaultAction : 'Deny',
              })}
            />
            <span>配置访问规则</span>
          </label>
        ) : null}
        {draft.ruleEnabled ? (
          <div className="policy-editor-grid">
          <PolicyInputField label="规则名称" value={draft.ruleName} onChange={(ruleName) => onChange({ ...draft, ruleName })} />
          <PolicySelectField
            label="规则动作"
            value={draft.ruleAction}
            options={[['Allow', '允许'], ['Deny', '拒绝']]}
            onChange={(ruleAction) => onChange({ ...draft, ruleAction: ruleAction as AccessControlAction })}
          />
          <PolicySelectField
            label="匹配类型"
            value={draft.conditionType}
            options={accessControlConditionTypes.map((type) => [type, accessControlConditionTypeLabel(type)])}
            onChange={(conditionType) => onChange({ ...draft, conditionType: conditionType as AccessControlConditionType })}
          />
          {needsConditionName ? <PolicyInputField label="匹配名称" value={draft.conditionName} onChange={(conditionName) => onChange({ ...draft, conditionName })} /> : null}
          <PolicyInputField label="匹配值" value={draft.conditionValue} onChange={(conditionValue) => onChange({ ...draft, conditionValue })} />
          </div>
        ) : (
          <div className="mini-card policy-execution-note">
            <div className="mini-card-title">默认拒绝全部请求</div>
            <div className="mini-card-meta">不配置规则时，默认拒绝所有请求。</div>
          </div>
        )}
      </section>
      <section className="form-section">
        <div className="form-section-title">
          <h3>拒绝响应</h3>
          <p>请求被拒绝时返回给调用方的 HTTP 状态码和消息。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="拒绝状态码" value={draft.responseStatusCode} type="number" onChange={(responseStatusCode) => onChange({ ...draft, responseStatusCode })} />
          <PolicyInputField label="拒绝消息" value={draft.responseMessage} onChange={(responseMessage) => onChange({ ...draft, responseMessage })} />
        </div>
      </section>
    </div>
  );
}

export function createAccessControlPolicyDraft(policy?: AccessControlPolicy): AccessControlPolicyDraft {
  const rule = policy?.rules?.[0];
  const condition = rule?.conditions?.[0];
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    enabled: policy?.enabled ?? true,
    defaultAction: policy?.defaultAction || 'Deny',
    ruleEnabled: policy ? Boolean(rule) : true,
    ruleName: rule?.name ?? 'default',
    ruleAction: rule?.action || 'Allow',
    conditionType: condition?.type ?? 'IP',
    conditionName: condition?.name ?? '',
    conditionValue: condition?.value ?? '',
    responseStatusCode: String(policy?.response?.statusCode ?? 403),
    responseMessage: policy?.response?.message ?? 'Forbidden',
    preservedConditions: condition ? rule?.conditions?.slice(1) ?? [] : [],
    preservedRules: policy?.rules?.slice(1) ?? [],
  };
}

export function accessControlPolicyPayload(draft: AccessControlPolicyDraft): AccessControlPolicyPayload {
  const condition = {
    type: draft.conditionType,
    ...(draft.conditionName ? { name: draft.conditionName } : {}),
    value: draft.conditionValue,
  };
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    defaultAction: draft.defaultAction,
    rules: draft.ruleEnabled ? [{
      name: draft.ruleName,
      action: draft.ruleAction,
      conditions: [condition, ...draft.preservedConditions],
    }, ...draft.preservedRules] : [],
    response: {
      statusCode: Number(draft.responseStatusCode || 403),
      message: draft.responseMessage,
    },
  };
}

function accessControlConditionTypeLabel(type: AccessControlConditionType) {
  const labels: Record<AccessControlConditionType, string> = {
    IP: '客户端 IP',
    Header: '请求头',
    Consumer: '调用方',
    Tenant: '租户',
  };
  return labels[type];
}
