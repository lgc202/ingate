import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import type {
  GovernancePolicy,
  PolicyTargetKind,
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
	onChange,
}: {
  options: PolicyTargetOption[];
  value: PolicyTargetRef[];
  onChange: (value: PolicyTargetRef[]) => void;
}) {
  const selected = new Set(value.map(policyTargetKey));
  const labels = value.map((target) => `${policyTargetKindLabel(target.kind)} / ${policyTargetLabel(target, options)}`);
  const missingTargets = value.filter((target) => !options.some((option) => policyTargetKey(option) === policyTargetKey(target)));

  return (
    <SelectPopover
      label="应用目标"
      summary={labels.length > 0 ? labels.join('、') : '暂不应用到网关或路由'}
      emptyMessage="暂无可选网关或路由"
      hasOptions={options.length > 0 || missingTargets.length > 0}
    >
      <>
        {(['Gateway', 'Route'] as PolicyTargetKind[]).map((kind) => {
          const group = options.filter((option) => option.kind === kind);
          if (group.length === 0) {
            return null;
          }
          return (
            <div className="policy-select-group" key={kind}>
              <div className="policy-select-group-title">{policyTargetKindLabel(kind)}</div>
              {group.map((option) => {
                const key = policyTargetKey(option);
                const checked = selected.has(key);
				return (
                  <button
                    key={key}
					className={checked ? 'selected' : ''}
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
          <div className="policy-select-group">
            <div className="policy-select-group-title">已失效目标</div>
            {missingTargets.map((target) => {
              const key = policyTargetKey(target);
              return (
                <button
                  key={key}
                  className="selected"
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
            className={checked ? 'selected' : ''}
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

function SelectPopover({
  label,
  summary,
  emptyMessage,
  hasOptions,
  children,
}: {
  label: string;
  summary: string;
  emptyMessage: string;
  hasOptions: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const labelID = useId();

  useEffect(() => {
    if (!open) {
      return;
    }
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  return (
    <div className="field field-wide">
      <label id={labelID}>{label}</label>
      <div ref={rootRef} className={`policy-select ${open ? 'open' : ''}`.trim()}>
        <button
          className="policy-select-trigger"
          type="button"
          aria-labelledby={labelID}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => setOpen((current) => !current)}
        >
          <span>{summary}</span>
          <span aria-hidden="true">⌄</span>
        </button>
        {open ? (
          <div className="policy-select-menu" role="listbox" aria-label={label}>
            {hasOptions ? children : <div className="policy-select-empty">{emptyMessage}</div>}
          </div>
        ) : null}
      </div>
    </div>
  );
}
