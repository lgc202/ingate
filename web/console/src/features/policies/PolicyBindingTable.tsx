import { Badge, EmptyState } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { GovernancePolicy, PolicyBinding, PolicyWorkspace } from '@/domain/policy';
import {
  policyBindingTargetLabel,
  policyNamesForBinding,
  policyStatusLabel,
  policyStatusTone,
} from '@/domain/policy';

export function PolicyBindingTable({
  bindings,
  policies,
  targets,
  onEdit,
  onToggle,
  onDelete,
}: {
  bindings: PolicyBinding[];
  policies: GovernancePolicy[];
  targets: PolicyWorkspace['targets'];
  onEdit: (binding: PolicyBinding) => void;
  onToggle: (binding: PolicyBinding) => void;
  onDelete: (binding: PolicyBinding) => void;
}) {
  if (bindings.length === 0) {
    return <div className="table-empty"><EmptyState title="暂无策略绑定" message="当前没有匹配的绑定关系。" /></div>;
  }

  return (
    <div className="table-scroll policy-binding-table-scroll">
      <table className="table policy-binding-table">
        <thead>
          <tr>
            <th>绑定名称</th>
            <th>作用目标</th>
            <th>绑定策略</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {bindings.map((binding) => (
            <tr key={binding.id}>
              <td>
                <div className="table-primary">{binding.name}</div>
                <div className="table-secondary">{binding.description || binding.id}</div>
              </td>
              <td>{policyBindingTargetLabel(binding, targets)}</td>
              <td>{policyNamesForBinding(binding, policies).join('、')}</td>
              <td><Badge tone={policyStatusTone(binding.enabled)}>{policyStatusLabel(binding.enabled)}</Badge></td>
              <td>{formatDateTime(binding.createdAt ?? '')}</td>
              <td>
                <div className="row-actions">
                  <button className="link-button" type="button" onClick={() => onEdit(binding)}>编辑</button>
                  <button className="link-button" type="button" onClick={() => onToggle(binding)}>{binding.enabled ? '停用' : '启用'}</button>
                  <button className="link-button danger" type="button" onClick={() => onDelete(binding)}>删除</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
