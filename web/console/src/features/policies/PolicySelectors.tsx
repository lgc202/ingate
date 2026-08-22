import { SelectPopover } from '@/components/ui';
import type {
  GovernancePolicy,
  PolicyTargetOption,
  PolicyTargetRef,
} from '@/domain/policy';
import {
  governancePolicyKey,
  governancePolicyStatusLabel,
  policyKindLabel,
  policyTargetKey,
  policyTargetKindLabel,
  policyTargetLabel,
} from '@/domain/policy';

export function PolicyTargetSelect({
  options,
  value,
  label = '应用目标',
  emptySummary = '暂不应用',
  emptyMessage = '暂无可选目标',
  onChange,
}: {
  options: PolicyTargetOption[];
  value: PolicyTargetRef[];
  label?: string;
  emptySummary?: string;
  emptyMessage?: string;
  onChange: (value: PolicyTargetRef[]) => void;
}) {
  const selected = new Set(value.map(policyTargetKey));
  const labels = value.map((target) => `${policyTargetKindLabel(target.kind)} / ${policyTargetLabel(target, options)}`);
  const missingTargets = value.filter((target) => !options.some((option) => policyTargetKey(option) === policyTargetKey(target)));
  const targetKinds = Array.from(new Set(options.map((option) => option.kind)));

  return (
    <SelectPopover
      label={label}
      summary={labels.length > 0 ? labels.join('、') : emptySummary}
      emptyMessage={emptyMessage}
      hasOptions={options.length > 0 || missingTargets.length > 0}
    >
      <>
        {targetKinds.map((kind) => {
          const group = options.filter((option) => option.kind === kind);
          return (
            <div className="resource-select-group" key={kind}>
              <div className="resource-select-group-title">{policyTargetKindLabel(kind)}</div>
              {group.map((option) => {
                const key = policyTargetKey(option);
                const checked = selected.has(key);
                return (
                  <button
                    key={key}
                    className={`resource-select-option${checked ? ' selected' : ''}`}
                    type="button"
                    role="option"
                    aria-selected={checked}
                    aria-pressed={checked}
                    onClick={() => onChange(checked
                      ? value.filter((target) => policyTargetKey(target) !== key)
                      : [...value, { kind: option.kind, id: option.id, displayName: option.name }])}
                  >
                    <span className="multi-check">{checked ? '✓' : ''}</span>
                    <strong>{option.name}</strong>
                    <small>{policyTargetKindLabel(option.kind)}</small>
                  </button>
                );
              })}
            </div>
          );
        })}
        {missingTargets.length > 0 ? (
          <div className="resource-select-group">
            <div className="resource-select-group-title">已失效目标</div>
            {missingTargets.map((target) => {
              const key = policyTargetKey(target);
              return (
                <button
                  key={key}
                  className="resource-select-option selected"
                  type="button"
                  role="option"
                  aria-selected="true"
                  aria-pressed="true"
                  onClick={() => onChange(value.filter((item) => policyTargetKey(item) !== key))}
                >
                  <span className="multi-check">✓</span>
                  <strong>{target.displayName ?? target.id}</strong>
                  <small>点击移除</small>
                </button>
              );
            })}
          </div>
        ) : null}
      </>
    </SelectPopover>
  );
}

export function GovernancePolicySelect({
  policies,
  value,
  onChange,
}: {
  policies: GovernancePolicy[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const selected = new Set(value);
  const labels = policies.filter((policy) => selected.has(governancePolicyKey(policy))).map((policy) => policy.name);

  return (
    <SelectPopover
      label="选择策略"
      summary={labels.length > 0 ? labels.join('、') : '请选择要应用的策略'}
      emptyMessage="没有可应用的策略"
      hasOptions={policies.length > 0}
    >
      {policies.map((policy) => {
        const key = governancePolicyKey(policy);
        const checked = selected.has(key);
        return (
          <button
            key={key}
            className={`resource-select-option${checked ? ' selected' : ''}`}
            type="button"
            role="option"
            aria-selected={checked}
            aria-pressed={checked}
            onClick={() => onChange(checked ? value.filter((item) => item !== key) : [...value, key])}
          >
            <span className="multi-check">{checked ? '✓' : ''}</span>
            <strong>{policy.name}</strong>
            <small>{policyKindLabel(policy.kind)} · {policy.summary} · {governancePolicyStatusLabel(policy)}</small>
          </button>
        );
      })}
    </SelectPopover>
  );
}
