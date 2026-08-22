import type { ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui';
import type {
  HeaderTransformationOperation,
  HeaderTransformationPolicy,
  HeaderTransformationPolicyPayload,
  PolicyTargetOption,
  PolicyTargetRef,
} from '@/domain/policy';
import { PolicyInputField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

type TransformationDirection = 'request' | 'response';

interface TransformationRuleDraft {
  id: string;
  direction: TransformationDirection;
  operation: HeaderTransformationOperation;
  name: string;
  value: string;
}

export interface HeaderTransformationPolicyDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  rules: TransformationRuleDraft[];
}

export interface HeaderTransformationPolicyValidation {
  valid: boolean;
  errors: Record<string, string>;
}

export function HeaderTransformationPolicyEditor({
  draft,
  targets,
  validation,
  onChange,
}: {
  draft: HeaderTransformationPolicyDraft;
  targets: PolicyTargetOption[];
  validation: HeaderTransformationPolicyValidation;
  onChange: (draft: HeaderTransformationPolicyDraft) => void;
}) {
  const routeTargets = targets.filter((target) => target.kind === 'Route');
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title"><h3>基础信息</h3></div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} error={validation.errors.name} onChange={(name) => onChange({ ...draft, name })} />
          {draft.id ? (
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
              <span>启用策略</span>
            </label>
          ) : null}
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>应用路由</h3></div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect
            label="路由"
            emptyMessage="暂无可选路由"
            options={routeTargets}
            value={draft.targets}
            onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })}
          />
        </div>
      </section>

      <section className="form-section space-y-4">
        <div className="form-section-title">
          <h3>转换规则</h3>
          <p>规则按照列表顺序执行。</p>
        </div>
        <div className="transformation-rule-list">
          {draft.rules.map((rule, index) => (
            <TransformationRuleEditor
              key={rule.id}
              index={index}
              rule={rule}
              error={validation.errors[`rules.${index}`]}
              onChange={(next) => onChange({
                ...draft,
                rules: draft.rules.map((item) => item.id === rule.id ? next : item),
              })}
              onDelete={() => onChange({ ...draft, rules: draft.rules.filter((item) => item.id !== rule.id) })}
            />
          ))}
        </div>
        {validation.errors.rules ? <div className="form-error" role="alert">{validation.errors.rules}</div> : null}
        <div className="transformation-rule-actions">
          <Button variant="soft" onClick={() => onChange({ ...draft, rules: [...draft.rules, createRule('request')] })}><Plus className="h-3.5 w-3.5" />添加请求规则</Button>
          <Button variant="outline" onClick={() => onChange({ ...draft, rules: [...draft.rules, createRule('response')] })}><Plus className="h-3.5 w-3.5" />添加响应规则</Button>
        </div>
      </section>
    </div>
  );
}

export function createHeaderTransformationPolicyDraft(policy?: HeaderTransformationPolicy): HeaderTransformationPolicyDraft {
  const requests = policy?.requestRules.map((rule) => ({ ...rule, id: crypto.randomUUID(), direction: 'request' as const })) ?? [];
  const responses = policy?.responseRules.map((rule) => ({ ...rule, id: crypto.randomUUID(), direction: 'response' as const })) ?? [];
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    rules: policy ? [...requests, ...responses] : [createRule('request')],
  };
}

export function validateHeaderTransformationPolicyDraft(draft: HeaderTransformationPolicyDraft): HeaderTransformationPolicyValidation {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) errors.name = '请输入策略名称';
  if (draft.rules.length === 0) errors.rules = '请至少添加一条请求或响应 Header 规则';
  draft.rules.forEach((rule, index) => {
    if (!validHeaderName(rule.name.trim())) {
      errors[`rules.${index}`] = '请输入有效的 Header 名称';
    } else if (rule.operation === 'rename' && !validHeaderName(rule.value.trim())) {
      errors[`rules.${index}`] = '请输入有效的新 Header 名称';
    }
  });
  return { valid: Object.keys(errors).length === 0, errors };
}

export function headerTransformationPolicyPayload(draft: HeaderTransformationPolicyDraft): HeaderTransformationPolicyPayload {
  const rules = draft.rules.map(({ direction, operation, name, value }) => ({
    direction,
    rule: { operation, name: name.trim(), value: operation === 'remove' ? '' : value },
  }));
  const config = {
    name: draft.name.trim(),
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    requestRules: rules.filter((item) => item.direction === 'request').map((item) => item.rule),
    responseRules: rules.filter((item) => item.direction === 'response').map((item) => item.rule),
  };
  if (!draft.id) return config;
  if (!draft.version) throw new Error('策略版本缺失，请刷新后重试');
  return { ...config, id: draft.id, version: draft.version, enabled: draft.enabled };
}

function TransformationRuleEditor({
  index,
  rule,
  error,
  onChange,
  onDelete,
}: {
  index: number;
  rule: TransformationRuleDraft;
  error?: string;
  onChange: (rule: TransformationRuleDraft) => void;
  onDelete: () => void;
}) {
  return (
    <article className="transformation-rule-card">
      <header><strong>规则 {index + 1}</strong><button type="button" aria-label={`删除规则 ${index + 1}`} onClick={onDelete}><Trash2 aria-hidden="true" /></button></header>
      <div className={`transformation-rule-fields${rule.operation === 'remove' ? ' is-remove' : ''}`}>
        <RuleField label="处理阶段">
          <select className="select" value={rule.direction} onChange={(event) => onChange({ ...rule, direction: event.target.value as TransformationDirection })}>
            <option value="request">请求 Header</option>
            <option value="response">响应 Header</option>
          </select>
        </RuleField>
        <RuleField label="操作">
          <select className="select" value={rule.operation} onChange={(event) => onChange({ ...rule, operation: event.target.value as HeaderTransformationOperation, name: '', value: '' })}>
            {(['remove', 'rename', 'replace', 'add', 'append'] as const).map((operation) => <option key={operation} value={operation}>{operationLabel(operation)}</option>)}
          </select>
        </RuleField>
        <RuleField label={nameLabel(rule.operation)}>
          <input className="input font-mono" value={rule.name} placeholder="例如：x-request-id" onChange={(event) => onChange({ ...rule, name: event.target.value })} />
        </RuleField>
        {rule.operation !== 'remove' ? (
          <RuleField label={valueLabel(rule.operation)}>
            <input className="input font-mono" value={rule.value} placeholder={rule.operation === 'rename' ? '例如：x-trace-id' : '输入 Header 值'} onChange={(event) => onChange({ ...rule, value: event.target.value })} />
          </RuleField>
        ) : null}
      </div>
      {error ? <div className="form-error" role="alert">{error}</div> : null}
    </article>
  );
}

function RuleField({ label, children }: { label: string; children: ReactNode }) {
  return <label className="plugin-field"><span>{label}</span>{children}</label>;
}

function createRule(direction: TransformationDirection): TransformationRuleDraft {
  return { id: crypto.randomUUID(), direction, operation: 'add', name: '', value: '' };
}

function validHeaderName(value: string): boolean {
  return /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(value) && !value.startsWith(':');
}

function operationLabel(operation: HeaderTransformationOperation): string {
  const labels: Record<HeaderTransformationOperation, string> = {
    remove: '删除 Header', rename: '重命名 Header', replace: '替换 Header', add: '添加 Header', append: '追加 Header',
  };
  return labels[operation];
}

function nameLabel(operation: HeaderTransformationOperation): string {
  return operation === 'rename' ? '原 Header 名称' : 'Header 名称';
}

function valueLabel(operation: HeaderTransformationOperation): string {
  if (operation === 'rename') return '新 Header 名称';
  if (operation === 'replace') return '新值';
  if (operation === 'append') return '追加值';
  return 'Header 值';
}
