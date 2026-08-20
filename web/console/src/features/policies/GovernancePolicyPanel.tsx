import { useState } from 'react';
import { updateGovernancePolicyTargets } from '@/api/policies';
import { Badge, Button, Toast } from '@/components/ui';
import type { ResourceStatus } from '@/domain/common';
import type {
  GovernancePolicy,
  PolicyTargetKind,
  PolicyWorkspace,
} from '@/domain/policy';
import {
  governancePolicyKey,
  policyKindLabel,
  policyStatusLabel,
  policyStatusTone,
  policyTargetKindLabel,
  policyTargetsResource,
} from '@/domain/policy';
import { GovernancePolicySelect } from './PolicySelectors';

interface Notice {
  message: string;
  tone: 'success' | 'error';
}

export function GovernancePolicyPanel({
  targetKind,
  targetID,
  targetName,
  inheritedGatewayIDs = [],
  workspace,
  onChanged,
}: {
  targetKind: PolicyTargetKind;
  targetID: string;
  targetName: string;
  inheritedGatewayIDs?: string[];
  workspace: PolicyWorkspace;
  onChanged: () => Promise<void> | void;
}) {
  const [editorOpen, setEditorOpen] = useState(false);
  const [selectedPolicyKeys, setSelectedPolicyKeys] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const directPolicies = workspace.policies.filter((policy) => policyTargetsResource(policy, targetKind, targetID));
  const inheritedPolicies = targetKind === 'Route'
    ? workspace.policies.flatMap((policy) => {
      const gatewayTargets = policy.targets.filter((target) => target.kind === 'Gateway' && inheritedGatewayIDs.includes(target.id));
      const gateways = gatewayTargets.map((target) => (
        workspace.targets.find((option) => option.kind === 'Gateway' && option.id === target.id)?.name ?? target.displayName ?? target.id
      ));
      return gateways.length > 0 ? [{
        policy,
        gateways,
        directlyApplied: policyTargetsResource(policy, 'Route', targetID),
        status: mostImportantStatus(gatewayTargets.map((target) => target.status), policy.status),
      }] : [];
    })
    : [];
	const candidates = workspace.policies.filter((policy) => (
		!policyTargetsResource(policy, targetKind, targetID)
	));

  const openEditor = () => {
    setSelectedPolicyKeys([]);
    setEditorOpen(true);
  };

  const applyPolicies = async () => {
    const selectedPolicies = candidates.filter((policy) => selectedPolicyKeys.includes(governancePolicyKey(policy)));
    if (selectedPolicies.length === 0) {
      return;
    }

    setSubmitting(true);
    try {
      const results = await Promise.allSettled(selectedPolicies.map((policy) => (
        updateGovernancePolicyTargets(policy, [
          ...policy.targets,
          { kind: targetKind, id: targetID, displayName: targetName },
        ])
      )));
      await onChanged();
      const failedIndexes = results.flatMap((result, index) => result.status === 'rejected' ? [index] : []);
      if (failedIndexes.length === 0) {
        setNotice({ message: `已应用 ${selectedPolicies.length} 条策略`, tone: 'success' });
        setEditorOpen(false);
        setSelectedPolicyKeys([]);
        return;
      }
      const succeeded = selectedPolicies.length - failedIndexes.length;
      const firstFailure = results[failedIndexes[0]];
      const reason = firstFailure.status === 'rejected' && firstFailure.reason instanceof Error
        ? firstFailure.reason.message
        : '请刷新后重试';
      setSelectedPolicyKeys(failedIndexes.map((index) => governancePolicyKey(selectedPolicies[index])));
      setNotice({ message: `${succeeded} 条成功，${failedIndexes.length} 条失败：${reason}`, tone: 'error' });
    } catch (error) {
      await onChanged();
      setNotice({ message: error instanceof Error ? error.message : '应用策略失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const removePolicy = async (policy: GovernancePolicy) => {
    setSubmitting(true);
    try {
      const remainsInherited = targetKind === 'Route'
		&& policy.targets.some((target) => (
          target.kind === 'Gateway' && inheritedGatewayIDs.includes(target.id)
        ));
      const result = await updateGovernancePolicyTargets(
        policy,
        policy.targets.filter((target) => target.kind !== targetKind || target.id !== targetID),
      );
      await onChanged();
      setNotice({
        message: remainsInherited ? `${result.message}；该策略仍通过所属网关生效` : result.message,
        tone: 'success',
      });
    } catch (error) {
      await onChanged();
      setNotice({ message: error instanceof Error ? error.message : '移除策略失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="detail-card governance-card">
      <div className="governance-card-head">
        <div>
          <h4>应用策略</h4>
          <span>{policyTargetKindLabel(targetKind)} / {targetName}</span>
        </div>
        <Button variant="soft" type="button" disabled={submitting} onClick={openEditor}>应用已有策略</Button>
      </div>

      <PolicySection
        title="直接应用"
        empty="当前没有直接应用的策略"
        policies={directPolicies}
        disabled={submitting}
        statusForPolicy={(policy) => policy.targets.find((target) => target.kind === targetKind && target.id === targetID)?.status ?? policy.status}
        onRemove={removePolicy}
      />

      {targetKind === 'Route' ? (
        <div className="governance-policy-section">
          <div className="governance-policy-section-title">
            <strong>通过网关继承</strong>
            <span>{inheritedPolicies.length} 条</span>
          </div>
          <div className="governance-policy-list">
            {inheritedPolicies.length > 0 ? inheritedPolicies.map(({ policy, gateways, directlyApplied, status }) => (
              <div className="governance-policy-row" key={`inherited:${governancePolicyKey(policy)}`}>
                <div>
                  <strong>{policy.name}</strong>
                  <span>
                    {policyKindLabel(policy.kind)} · 来自 {gateways.join('、')}
                    {directlyApplied ? ' · 与直接应用重叠时只执行一次' : ''}
                  </span>
                  <span>{status.message}</span>
                </div>
                <Badge tone={policyStatusTone(status)}>{policyStatusLabel(status)}</Badge>
              </div>
            )) : <span className="mini-card-meta">当前没有从所属网关继承的策略</span>}
          </div>
        </div>
      ) : null}

      {editorOpen ? (
        <div className="governance-policy-editor">
          <div>
            <strong>选择要应用的策略</strong>
            <p>策略原有的其他应用目标会保留。</p>
          </div>
          {candidates.length > 0 ? (
            <GovernancePolicySelect policies={candidates} value={selectedPolicyKeys} onChange={setSelectedPolicyKeys} />
          ) : (
            <span className="mini-card-meta">当前没有可应用到该{policyTargetKindLabel(targetKind)}的已有策略。</span>
          )}
          <div className="form-actions">
            <Button variant="primary" type="button" disabled={submitting || selectedPolicyKeys.length === 0} onClick={applyPolicies}>确认应用</Button>
            <Button variant="ghost" type="button" disabled={submitting} onClick={() => setEditorOpen(false)}>取消</Button>
          </div>
        </div>
      ) : null}
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </div>
  );
}

function PolicySection({
  title,
  empty,
  policies,
  disabled,
  statusForPolicy,
  onRemove,
}: {
  title: string;
  empty: string;
  policies: GovernancePolicy[];
  disabled: boolean;
  statusForPolicy: (policy: GovernancePolicy) => ResourceStatus;
  onRemove: (policy: GovernancePolicy) => void;
}) {
  return (
    <div className="governance-policy-section">
      <div className="governance-policy-section-title">
        <strong>{title}</strong>
        <span>{policies.length} 条</span>
      </div>
      <div className="governance-policy-list">
        {policies.length > 0 ? policies.map((policy) => {
          const status = statusForPolicy(policy);
          return (
            <div className="governance-policy-row" key={governancePolicyKey(policy)}>
              <div>
                <strong>{policy.name}</strong>
                <span>{policyKindLabel(policy.kind)} · {policy.summary}</span>
                <span>{status.message}</span>
              </div>
              <div className="governance-policy-actions">
                <Badge tone={policyStatusTone(status)}>{policyStatusLabel(status)}</Badge>
                <button className="link-button danger" type="button" disabled={disabled} onClick={() => onRemove(policy)}>移除</button>
              </div>
            </div>
          );
        }) : <span className="mini-card-meta">{empty}</span>}
      </div>
    </div>
  );
}

function mostImportantStatus(statuses: Array<ResourceStatus | undefined>, fallback: ResourceStatus) {
  const priority = { Error: 0, Pending: 1, Ready: 2, Disabled: 3 };
  return statuses.filter((status): status is ResourceStatus => Boolean(status)).sort((left, right) => (
    priority[left.state] - priority[right.state]
  ))[0] ?? fallback;
}
