import { Badge, EmptyState } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { GovernancePolicy, PolicyTargetOption } from '@/domain/policy';
import {
  governancePolicyKey,
  governancePolicyStatusLabel,
  policyKindLabel,
  policyStatusTone,
  policyTargetLabel,
} from '@/domain/policy';

export function PolicyLibraryTable({
  policies,
  targets,
  onDetail,
  onEdit,
  onToggle,
  onDelete,
}: {
  policies: GovernancePolicy[];
  targets: PolicyTargetOption[];
  onDetail: (policy: GovernancePolicy) => void;
  onEdit: (policy: GovernancePolicy) => void;
  onToggle: (policy: GovernancePolicy) => void;
  onDelete: (policy: GovernancePolicy) => void;
}) {
  if (policies.length === 0) {
    return <div className="table-empty"><EmptyState title="暂无策略" message="当前没有匹配的策略。" /></div>;
  }

  return (
    <div className="table-scroll resource-table-scroll policy-table-scroll">
      <table className="table resource-table policy-table">
        <thead>
          <tr>
            <th>策略名称</th>
            <th>策略内容</th>
            <th>应用目标</th>
            <th>状态</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {policies.map((policy) => {
            const unapplied = policy.enabled && policy.targets.length === 0 && policy.status.state === 'Ready';
            const readyTargets = policy.targets.filter((target) => target.status?.state === 'Ready').length;
            const errorTargets = policy.targets.filter((target) => target.status?.state === 'Error').length;
            const partiallyApplied = policy.enabled && policy.status.state !== 'Error' && readyTargets > 0 && readyTargets < policy.targets.length;
            const statusLabel = partiallyApplied ? errorTargets > 0 ? '部分异常' : '部分生效' : governancePolicyStatusLabel(policy);
            const statusMessage = partiallyApplied
              ? `${readyTargets}/${policy.targets.length} 个目标已生效${errorTargets > 0 ? `，${errorTargets} 个异常` : ''}`
              : policy.status.message;
            return (
            <tr key={governancePolicyKey(policy)}>
              <td>
                <div className="table-primary">{policy.name}</div>
                <div className="table-secondary">{policyKindLabel(policy.kind)}{policy.description ? ` · ${policy.description}` : ''}</div>
              </td>
              <td>
                <div className="table-primary">{policy.summary}</div>
                <div className="table-secondary">{policyContentCount(policy)}</div>
              </td>
              <td>
                <div className="table-primary">{policy.targets.length > 0 ? `${policy.targets.length} 个` : '未应用'}</div>
                <div className="table-secondary policy-target-summary">
                  {policy.targets.length > 0
                    ? policy.targets.map((target) => policyTargetLabel(target, targets)).join('、')
                    : '可编辑策略后选择网关或路由'}
                </div>
              </td>
              <td>
                <Badge tone={unapplied ? 'neutral' : partiallyApplied ? errorTargets > 0 ? 'danger' : 'warning' : policyStatusTone(policy.status)}>
                  {statusLabel}
                </Badge>
                {statusMessage && statusMessage !== statusLabel ? <div className="table-secondary policy-status-message">{statusMessage}</div> : null}
              </td>
              <td className="resource-table-time">{formatDateTime(policy.updatedAt ?? policy.createdAt ?? '')}</td>
              <td>
                <div className="row-actions">
                  <button className="link-button" type="button" onClick={() => onDetail(policy)}>详情</button>
                  <button className="link-button" type="button" onClick={() => onEdit(policy)}>编辑</button>
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

function policyContentCount(policy: GovernancePolicy) {
  if (policy.kind === 'IPRestrictionPolicy') {
    return `${policy.ruleCount} 个地址或网段`;
  }
  return '1 项请求额度';
}
