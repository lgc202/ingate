import { useCallback, useMemo } from 'react';
import { AlertCircle, Clock3 } from 'lucide-react';
import { getCallerTokenQuotaUsage } from '@/api/policies';
import { useResource } from '@/api/useResource';
import { formatDateTime } from '@/domain/common';
import type { CallerTokenQuotaUsage as TokenQuotaUsage, TokenQuotaPeriod } from '@/domain/policy';

const periodLabels: Record<TokenQuotaPeriod, string> = {
  Day: '每日额度',
  Week: '每周额度',
  Month: '每月额度',
};

export function CallerTokenQuotaUsage({ callerID }: { callerID: string }) {
  const load = useCallback(() => getCallerTokenQuotaUsage(callerID), [callerID]);
  const usage = useResource(load);
  const policies = useMemo(() => groupByPolicy(usage.data ?? []), [usage.data]);

  if (usage.loading && !usage.data) {
    return <div className="token-quota-usage-state">正在读取实时额度...</div>;
  }
  if (usage.error) {
    return (
      <div className="token-quota-usage-state is-error">
        <AlertCircle />
        <span>{usage.error.message}</span>
        <button type="button" onClick={() => void usage.reload()}>重新加载</button>
      </div>
    );
  }
  if (policies.length === 0) {
    return <div className="token-quota-usage-state">当前没有正在执行的 Token 额度</div>;
  }

  return (
    <div className="token-quota-usage-list">
      {policies.map((policy) => (
        <article key={policy.id} className="token-quota-usage-card">
          <header>
            <div><strong>{policy.name}</strong><span>实时用量</span></div>
            <Clock3 />
          </header>
          <div className="token-quota-usage-periods">
            {policy.usages.map((item) => <QuotaPeriod key={item.period} usage={item} />)}
          </div>
        </article>
      ))}
    </div>
  );
}

function QuotaPeriod({ usage }: { usage: TokenQuotaUsage }) {
  const ratio = usage.limitTokens > 0 ? usage.usedTokens / usage.limitTokens : 0;
  const percentage = Math.round(ratio * 100);
  const tone = ratio >= 1 ? 'danger' : ratio >= 0.8 ? 'warning' : 'normal';
  return (
    <div className={`token-quota-usage-period is-${tone}`}>
      <div className="token-quota-usage-summary">
        <div><span>{periodLabels[usage.period]}</span><strong>{formatTokens(usage.usedTokens)} / {formatTokens(usage.limitTokens)}</strong></div>
        <div><span>剩余</span><strong>{formatTokens(usage.remainingTokens)}</strong></div>
      </div>
      <div className="token-quota-usage-track" aria-label={`已使用 ${percentage}%`}>
        <i style={{ width: `${Math.min(100, percentage)}%` }} />
      </div>
      <div className="token-quota-usage-reset">
        <span>{percentage}%</span>
        <time dateTime={usage.resetsAt}>{formatDateTime(usage.resetsAt)} 重置</time>
      </div>
    </div>
  );
}

function groupByPolicy(usages: TokenQuotaUsage[]) {
  const policies = new Map<string, { id: string; name: string; usages: TokenQuotaUsage[] }>();
  for (const usage of usages) {
    const policy = policies.get(usage.policyID);
    if (policy) {
      policy.usages.push(usage);
      continue;
    }
    policies.set(usage.policyID, { id: usage.policyID, name: usage.policyName, usages: [usage] });
  }
  return [...policies.values()];
}

function formatTokens(tokens: number) {
  return new Intl.NumberFormat('zh-CN', {
    notation: tokens >= 10_000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(tokens);
}
