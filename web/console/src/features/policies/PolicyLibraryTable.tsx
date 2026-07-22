import { ChevronDown, Plus } from 'lucide-react';
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

export function CreatePolicyMenu({
  onCreateRateLimit,
  onCreateAccessControl,
  onCreateTokenQuota,
}: {
  onCreateRateLimit: () => void;
  onCreateAccessControl: () => void;
  onCreateTokenQuota: () => void;
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
          <span>控制请求速率，并在当前环境内共享计数</span>
        </button>
        <button type="button" onClick={onCreateAccessControl}>
          <strong>访问控制</strong>
          <span>按 IP 或请求特征放行、拒绝访问</span>
        </button>
        <button type="button" onClick={onCreateTokenQuota}>
          <strong>Token 配额</strong>
          <span>限制模型请求在统计周期内可消耗的输入与输出 Token</span>
        </button>
      </div>
    </details>
  );
}

export function PolicyLibraryTable({
  policies,
  targets,
  onEdit,
  onToggle,
  onDelete,
}: {
  policies: GovernancePolicy[];
  targets: PolicyTargetOption[];
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
            <th>策略内容</th>
            <th>应用目标</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {policies.map((policy) => {
            const unapplied = policy.enabled && policy.targets.length === 0 && policy.status.state === 'Ready';
            const readyTargets = policy.targets.filter((target) => target.status?.state === 'Ready').length;
            const errorTargets = policy.targets.filter((target) => target.status?.state === 'Error').length;
            const partiallyApplied = policy.enabled && policy.status.state !== 'Error' && readyTargets > 0 && readyTargets < policy.targets.length;
            return (
            <tr key={governancePolicyKey(policy)}>
              <td>
                <div className="table-primary">{policy.name}</div>
                <div className="table-secondary">{policyKindLabel(policy.kind)} · {policy.description || '暂无描述'}</div>
              </td>
              <td>
                <div className="table-primary">{policy.summary}</div>
                <div className="table-secondary">{policy.kind === 'TokenQuotaPolicy' ? '1 个预算池' : `${policy.ruleCount} 条规则`}</div>
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
                  {partiallyApplied ? errorTargets > 0 ? '部分异常' : '部分生效' : governancePolicyStatusLabel(policy)}
                </Badge>
                <div className="table-secondary policy-status-message">
                  {partiallyApplied
                    ? `${readyTargets}/${policy.targets.length} 个目标已生效${errorTargets > 0 ? `，${errorTargets} 个异常` : ''}`
                    : policy.status.message}
                </div>
              </td>
              <td>{formatDateTime(policy.createdAt ?? '')}</td>
              <td>
                <div className="row-actions">
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
