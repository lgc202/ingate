import type {
  GovernancePolicyKind,
  PolicyBinding,
  PolicyBindingPayload,
  PolicyTargetKind,
  PolicyWorkspace,
} from '@/domain/policy';
import { policyRefKey, policyTargetKindLabel } from '@/domain/policy';
import { PolicyInputField, PolicySelectField } from './PolicyEditorFields';
import { PolicyMultiSelect } from './PolicyMultiSelect';

export interface PolicyBindingDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  targetKind: PolicyTargetKind;
  targetID: string;
  ruleName: string;
  policyKeys: string[];
}

const targetKinds: PolicyTargetKind[] = ['Gateway', 'Route'];

export function PolicyBindingEditor({
  draft,
  workspace,
  onChange,
}: {
  draft: PolicyBindingDraft;
  workspace: PolicyWorkspace;
  onChange: (draft: PolicyBindingDraft) => void;
}) {
  const targets = workspace.targets.filter((target) => target.kind === draft.targetKind);
  const selectedTarget = targets.find((target) => target.id === draft.targetID);

  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title">
          <h3>基础信息</h3>
          <p>策略关联用于把已创建的治理策略应用到指定资源。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="关联名称" value={draft.name} onChange={(name) => onChange({ ...draft, name })} />
          <PolicyInputField label="描述" value={draft.description} onChange={(description) => onChange({ ...draft, description })} />
          <label className="policy-check-row">
            <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
            <span>启用关联</span>
          </label>
        </div>
      </section>
      <section className="form-section">
        <div className="form-section-title">
          <h3>作用范围</h3>
          <p>选择策略生效的网关、路由或具体路由规则。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicySelectField
            label="关联范围"
            value={draft.targetKind}
            options={targetKinds.map((kind) => [kind, policyTargetKindLabel(kind)])}
            onChange={(targetKind) => onChange({ ...draft, targetKind: targetKind as PolicyTargetKind, targetID: '' })}
          />
          <PolicySelectField
            label={draft.targetKind === 'Gateway' ? '网关' : '路由'}
            value={draft.targetID}
            options={[['', draft.targetKind === 'Gateway' ? '选择网关' : '选择路由'], ...targets.map((target) => [target.id, target.name] as [string, string])]}
            onChange={(targetID) => onChange({ ...draft, targetID, ruleName: '' })}
          />
          {draft.targetKind === 'Route' ? (
            <PolicySelectField
              label="路由规则"
              value={draft.ruleName}
              options={[['', '整条路由'], ...(selectedTarget?.ruleNames ?? []).map((ruleName) => [ruleName, ruleName] as [string, string])]}
              onChange={(ruleName) => onChange({ ...draft, ruleName })}
            />
          ) : null}
        </div>
      </section>
      <section className="form-section">
        <div className="form-section-title">
          <h3>关联策略</h3>
          <p>选择已经创建的限流或访问控制策略。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyMultiSelect
            policies={workspace.policies}
            value={draft.policyKeys}
            onChange={(policyKeys) => onChange({ ...draft, policyKeys })}
          />
        </div>
      </section>
    </div>
  );
}

export function createPolicyBindingDraft(
  workspace: PolicyWorkspace,
  source?: PolicyBinding | { targetKind: PolicyTargetKind; targetID: string; ruleName?: string },
): PolicyBindingDraft {
  if (source && 'targetRef' in source) {
    return {
      id: source.id,
      version: source.version,
      name: source.name,
      description: source.description ?? '',
      enabled: source.enabled,
      targetKind: source.targetRef.kind,
      targetID: source.targetRef.name,
      ruleName: source.targetRef.ruleName ?? '',
      policyKeys: source.policies.map(policyRefKey),
    };
  }

  const targetKind = source?.targetKind ?? 'Gateway';
  const targetID = source?.targetID ?? workspace.targets.find((target) => target.kind === targetKind)?.id ?? '';
  const target = workspace.targets.find((item) => item.kind === targetKind && item.id === targetID);
  return {
    name: target ? `${target.name} 策略关联` : '',
    description: '',
    enabled: true,
    targetKind,
    targetID,
    ruleName: source?.ruleName ?? '',
    policyKeys: [],
  };
}

export function policyBindingPayload(draft: PolicyBindingDraft): PolicyBindingPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    targetRef: {
      kind: draft.targetKind,
      name: draft.targetID,
      ...(draft.targetKind === 'Route' && draft.ruleName ? { ruleName: draft.ruleName } : {}),
    },
    policies: draft.policyKeys.map((key) => {
      const [kind, name] = key.split(':') as [GovernancePolicyKind, string];
      return { kind, name };
    }),
  };
}
