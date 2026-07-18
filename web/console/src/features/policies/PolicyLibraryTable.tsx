import { ChevronDown, Plus } from 'lucide-react';
import { Badge, EmptyState } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { AccessControlPolicy, GovernancePolicy, PolicyBinding, RateLimitPolicy } from '@/domain/policy';
import {
  governancePolicyKey,
  governancePolicyRef,
  policyKindLabel,
  policyStatusLabel,
  policyStatusTone,
} from '@/domain/policy';

export function CreatePolicyMenu({
  onCreateRateLimit,
  onCreateAccessControl,
}: {
  onCreateRateLimit: () => void;
  onCreateAccessControl: () => void;
}) {
  return (
    <details className="policy-create-menu">
      <summary className="button primary">
        <Plus size={15} aria-hidden="true" />
        新建策略
        <ChevronDown size={15} aria-hidden="true" />
      </summary>
      <div className="policy-create-menu-popover">
        <button type="button" onClick={onCreateRateLimit}>
          <strong>限流策略</strong>
          <span>控制请求速率，可在多个网关实例间统一计数</span>
        </button>
        <button type="button" onClick={onCreateAccessControl}>
          <strong>访问控制</strong>
          <span>按 IP、请求头、调用方或租户放行或拒绝</span>
        </button>
      </div>
    </details>
  );
}

export function PolicyLibraryTable({
  policies,
  bindings,
  onEdit,
  onToggle,
  onDelete,
}: {
  policies: GovernancePolicy[];
  bindings: PolicyBinding[];
  onEdit: (policy: GovernancePolicy) => void;
  onToggle: (policy: GovernancePolicy) => void;
  onDelete: (policy: GovernancePolicy) => void;
}) {
  if (policies.length === 0) {
    return <div className="table-empty"><EmptyState title="暂无策略" message="当前没有匹配的策略。" /></div>;
  }

  return (
    <div className="table-scroll policy-table-scroll">
      <table className="table policy-table">
        <thead>
          <tr>
            <th>策略名称</th>
            <th>执行方式</th>
            <th>被绑定</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {policies.map((policy) => {
            const editReason = policyEditReason(policy);
            return (
              <tr key={governancePolicyKey(policy)}>
                <td>
                  <div className="table-primary">{policy.name}</div>
                  <div className="table-secondary">{policyKindLabel(policy.kind)} · {policy.description || policy.id}</div>
                </td>
                <td>
                  <div className="table-primary">{policy.mode}</div>
                  <div className="table-secondary">{policy.ruleCount} 条规则{editReason ? ' · 控制台只读' : ''}</div>
                </td>
                <td>{policyBindingCount(policy, bindings)}</td>
                <td><Badge tone={policyStatusTone(policy.enabled)}>{policyStatusLabel(policy.enabled)}</Badge></td>
                <td>{formatDateTime(policy.createdAt ?? '')}</td>
                <td>
                  <div className="row-actions">
                    <button className="link-button" type="button" disabled={Boolean(editReason)} title={editReason || undefined} onClick={() => onEdit(policy)}>编辑</button>
                    <button className="link-button" type="button" onClick={() => onToggle(policy)}>{policy.enabled ? '停用' : '启用'}</button>
                    <button className="link-button danger" type="button" onClick={() => onDelete(policy)}>删除</button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function policyBindingCount(policy: GovernancePolicy, bindings: PolicyBinding[]) {
  const ref = governancePolicyRef(policy);
  return bindings.filter((binding) => binding.policies.some((policyRef) => policyRef.kind === ref.kind && policyRef.name === ref.name)).length;
}

function policyEditReason(policy: GovernancePolicy) {
  if (policy.kind === 'RateLimitPolicy') {
    const rateLimitPolicy = policy.raw as RateLimitPolicy;
    if (rateLimitPolicy.rules.length !== 1 || rateLimitPolicy.rules[0].key.parts.length !== 1) {
      return '当前控制台只支持编辑单规则、单计数维度的限流策略';
    }
    return '';
  }

  const accessControlPolicy = policy.raw as AccessControlPolicy;
  if ((accessControlPolicy.rules?.length ?? 0) > 1 || (accessControlPolicy.rules?.[0]?.conditions?.length ?? 0) > 1) {
    return '当前控制台只支持编辑单规则、单匹配条件的访问控制策略';
  }
  return '';
}
