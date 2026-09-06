import { Badge, EmptyState } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { GovernancePolicy, PolicyTargetOption, PolicyTargetRef } from '@/domain/policy';
import {
  governancePolicyStatusLabel,
  policyKindLabel,
  policyStatusTone,
  policyTargetKindLabel,
  policyTargetLabel,
} from '@/domain/policy';

export function PolicyDetail({ policy, targets }: { policy: GovernancePolicy; targets: PolicyTargetOption[] }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div>
          <h3>{policy.name}</h3>
          {policy.status.state === 'Error' || policy.status.state === 'Pending'
            ? <p>{policy.status.message}</p>
            : null}
        </div>
        <Badge tone={policyStatusTone(policy.status)}>{governancePolicyStatusLabel(policy)}</Badge>
      </section>
      <section className="resource-detail-section">
        <h3>策略规则</h3>
        <div className="resource-detail-grid">
          <div><span>策略类型</span><strong>{policyKindLabel(policy.kind)}</strong></div>
          <div><span>规则摘要</span><strong>{policy.summary}</strong></div>
          <div><span>启用状态</span><strong>{policy.enabled ? '已启用' : '已停用'}</strong></div>
          <div><span>生效状态</span><strong>{governancePolicyStatusLabel(policy)}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(policy.createdAt ?? '')}</strong></div>
          <PolicyRuleDetails policy={policy} />
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>应用目标</h3>
        {policy.targets.length > 0 ? (
          <div className="resource-detail-list">
            {policy.targets.map((target) => (
              <article key={`${target.kind}:${target.id}`}>
                <div>
                  <strong>{policyTargetLabel(target, targets)}</strong>
                  <small>{policyTargetKindLabel(target.kind)} · {target.status?.message || '等待系统反馈执行状态'}</small>
                </div>
                <Badge tone={target.status ? policyStatusTone(target.status) : 'neutral'}>
                  {policyTargetStatusLabel(policy, target)}
                </Badge>
              </article>
            ))}
          </div>
        ) : <EmptyState title="尚未应用" message="策略已保存，但当前不影响任何流量" />}
      </section>
    </div>
  );
}

function PolicyRuleDetails({ policy }: { policy: GovernancePolicy }) {
  if (policy.kind === 'IPRestrictionPolicy') {
    return <><div><span>允许地址</span><strong>{policy.raw.allow.join('、') || '未配置'}</strong></div><div><span>拒绝地址</span><strong>{policy.raw.deny.join('、') || '未配置'}</strong></div></>;
  }
  if (policy.kind === 'RateLimitPolicy') {
    const subject = policy.raw.subject.type === 'Shared'
      ? '目标共享'
      : policy.raw.subject.type === 'IP'
        ? '按客户端 IP'
        : `按请求头 ${policy.raw.subject.headerName}`;
    return <><div><span>计数方式</span><strong>{subject}</strong></div><div><span>请求上限</span><strong>{policy.summary}</strong></div></>;
  }
  if (policy.kind === 'TokenQuotaPolicy') {
    return <><div><span>周期时区</span><strong>{policy.raw.timeZone}</strong></div><div><span>额度上限</span><strong>{policy.summary}</strong></div></>;
  }
  if (policy.kind === 'HeaderTransformationPolicy') {
    return <><div><span>请求规则</span><strong>{policy.raw.requestRules.length} 条</strong></div><div><span>响应规则</span><strong>{policy.raw.responseRules.length} 条</strong></div></>;
  }
  return <><div><span>HTTP 状态码</span><strong>{policy.raw.statusCode}</strong></div><div><span>内容类型</span><strong>{policy.raw.contentType}</strong></div><div><span>响应 Header</span><strong>{policy.raw.headers.length} 个</strong></div><div className="resource-detail-grid-wide"><span>响应正文</span><pre className="mock-response-detail-body">{policy.raw.body || '空正文'}</pre></div></>;
}

function policyTargetStatusLabel(policy: GovernancePolicy, target: PolicyTargetRef): string {
  if (!target.status) return '未知';
  if (target.status.state === 'Ready') return policy.kind === 'TokenQuotaPolicy' ? '已启用' : '已生效';
  if (target.status.state === 'Error') return '生效失败';
  return '待生效';
}
