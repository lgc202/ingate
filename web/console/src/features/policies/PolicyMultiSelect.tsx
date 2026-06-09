import { useEffect, useRef, useState } from 'react';
import type { GovernancePolicy } from '@/domain/policy';
import { governancePolicyKey, policyKindLabel } from '@/domain/policy';

export function PolicyMultiSelect({
  policies,
  value,
  onChange,
}: {
  policies: GovernancePolicy[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const selectedPolicies = new Set(value);
  const selectedLabels = policies.filter((policy) => selectedPolicies.has(governancePolicyKey(policy))).map((policy) => policy.name);

  useEffect(() => {
    if (!open) {
      return;
    }

    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', closeOnOutsideClick);
    return () => document.removeEventListener('mousedown', closeOnOutsideClick);
  }, [open]);

  const togglePolicy = (policy: GovernancePolicy) => {
    const key = governancePolicyKey(policy);
    onChange(selectedPolicies.has(key) ? value.filter((item) => item !== key) : [...value, key]);
  };

  return (
    <div className="field field-wide">
      <label>绑定策略</label>
      <div ref={rootRef} className={`policy-select ${open ? 'open' : ''}`.trim()}>
        <button className="policy-select-trigger" type="button" onClick={() => setOpen(!open)}>
          <span>{selectedLabels.length > 0 ? selectedLabels.join('、') : '请选择策略'}</span>
          <span aria-hidden="true">⌄</span>
        </button>
        {open ? (
          <div className="policy-select-menu">
            {policies.map((policy) => {
              const selected = selectedPolicies.has(governancePolicyKey(policy));
              return (
                <button key={governancePolicyKey(policy)} className={selected ? 'selected' : ''} type="button" onClick={() => togglePolicy(policy)}>
                  <span className="multi-check">{selected ? '✓' : ''}</span>
                  <strong>{policy.name}</strong>
                  <small>{policyKindLabel(policy.kind)} · {policy.mode}</small>
                </button>
              );
            })}
          </div>
        ) : null}
      </div>
    </div>
  );
}
