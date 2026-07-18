import { useState } from 'react';
import { Button, PageFrame, Panel, Tabs } from '@/components/ui';
import type { PolicyWorkspace } from '@/domain/policy';
import type { RouteGatewayOption, RouteResource, UpstreamOption } from '@/domain/route';
import { GovernancePolicyPanel } from '@/features/policies/GovernancePolicyPanel';
import { routeDetailItems } from './routeView';

const detailTabs = [
  { key: 'overview', label: '概览' },
  { key: 'match', label: '匹配规则' },
  { key: 'upstream', label: '转发目标' },
  { key: 'controls', label: '转发控制' },
];

export function RouteDetail({
  route,
  gateways,
  upstreams,
  policyWorkspace,
  policyError,
  onPolicyWorkspaceChanged,
  onBack,
}: {
  route: RouteResource;
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
  policyWorkspace: PolicyWorkspace | null | undefined;
  policyError?: string;
  onPolicyWorkspaceChanged: () => Promise<void> | void;
  onBack: () => void;
}) {
  const [tab, setTab] = useState('overview');
  const details = routeDetailItems(route, tab, gateways, upstreams);

  return (
    <PageFrame
      title="路由详情"
      subtitle={route.name}
      actions={<Button variant="soft" onClick={onBack}>返回列表</Button>}
    >
      <Panel title="请求转发配置" subtitle="从网关匹配请求，并转发到普通服务或模型服务">
        <div className="route-detail-tabs">
          <Tabs tabs={detailTabs} active={tab} onChange={setTab} />
        </div>
        <div className="detail-card">
          <div className="kv">
            {details.flatMap((item) => [
              <div key={`${item.label}-label`}>{item.label}</div>,
              <div key={`${item.label}-value`}>{item.value}</div>,
            ])}
          </div>
        </div>
        {policyWorkspace ? (
          <GovernancePolicyPanel
            targetKind="Route"
            targetID={route.id}
            targetName={route.name}
            inheritedGatewayIDs={route.gatewayIDs}
            workspace={policyWorkspace}
            onChanged={onPolicyWorkspaceChanged}
          />
        ) : (
          <div className="mini-card policy-execution-note">
            <div className="mini-card-title">策略暂不可用</div>
            <div className="mini-card-meta">{policyError ?? '策略数据正在加载，请稍后刷新。'}</div>
          </div>
        )}
      </Panel>
    </PageFrame>
  );
}
