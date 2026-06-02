import { useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import { publishStatusLabel, statusTone } from '@/domain/common';
import type { PolicyResource } from '@/domain/policy';
import { riskLevelLabel, riskLevelStatus } from '@/domain/policy';

const loadPolicies = () => consoleRepository.listPolicies();
type PolicyMode = 'list' | 'detail' | 'create' | 'edit';
type PolicyDraft = Pick<PolicyResource, 'name' | 'type' | 'scope' | 'riskLevel'>;

export function PolicyPage() {
  const policies = useResource(loadPolicies);
  const [mode, setMode] = useState<PolicyMode>('list');
  const [selectedPolicyId, setSelectedPolicyId] = useState('api-key-auth');
  const [query, setQuery] = useState('');
  const [hiddenPolicyIds, setHiddenPolicyIds] = useState<string[]>([]);
  const [draft, setDraft] = useState<PolicyDraft>({ name: 'new-policy', type: '认证', scope: '全部网关', riskLevel: 'low' });

  if (policies.loading) {
    return (
      <PageFrame title="策略" subtitle="统一管理认证、限流、请求校验、AI 安全与配额策略">
        <ResourceStatePanel title="加载策略数据" message="正在读取策略列表和覆盖率。" />
      </PageFrame>
    );
  }

  if (policies.error || !policies.data) {
    return (
      <PageFrame title="策略" subtitle="统一管理认证、限流、请求校验、AI 安全与配额策略">
        <ResourceStatePanel title="策略数据加载失败" message={policies.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const selectedPolicy = policies.data.policies.find((policy) => policy.id === selectedPolicyId) ?? policies.data.policies[0];
  const visiblePolicies = policies.data.policies.filter((policy) => {
    const normalizedQuery = query.trim().toLowerCase();
    const matchedQuery = !normalizedQuery || [policy.name, policy.type, policy.scope].some((value) => value.toLowerCase().includes(normalizedQuery));

    return matchedQuery && !hiddenPolicyIds.includes(policy.id);
  });

  const openCreate = () => {
    setDraft({ name: 'new-policy', type: '认证', scope: '全部网关', riskLevel: 'low' });
    setMode('create');
  };

  const openEdit = (policy: PolicyResource) => {
    setSelectedPolicyId(policy.id);
    setDraft({ name: policy.name, type: policy.type, scope: policy.scope, riskLevel: policy.riskLevel });
    setMode('edit');
  };

  if (mode === 'detail') {
    return (
      <PageFrame
        title="策略详情"
        subtitle={selectedPolicy?.name ?? '未选择策略'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedPolicy ? <PolicyDetail policy={selectedPolicy} /> : null}
        </Panel>
      </PageFrame>
    );
  }

  if (mode === 'create' || mode === 'edit') {
    return (
      <PageFrame
        title={mode === 'create' ? '新建策略' : '编辑策略'}
        subtitle="定义策略意图和适用范围，具体执行能力由后端按资源类型落地"
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <Panel title={mode === 'create' ? '新建策略' : '编辑策略'} subtitle={draft.name}>
          <div className="editor-grid">
            <div className="field-grid">
              <InputField label="策略名称" value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
              <InputField label="策略类型" value={draft.type} onChange={(value) => setDraft({ ...draft, type: value })} />
              <InputField label="适用范围" value={draft.scope} onChange={(value) => setDraft({ ...draft, scope: value })} />
              <SelectField label="风险等级" value={draft.riskLevel} options={['low', 'medium', 'high']} onChange={(value) => setDraft({ ...draft, riskLevel: value as PolicyDraft['riskLevel'] })} />
            </div>
            <div className="detail-card editor-side-card">
              <h4>保存说明</h4>
              <div className="mini-card-meta">当前先保存策略的用户意图。后端接入后会根据策略类型选择内置能力、运行时能力或插件能力执行。</div>
            </div>
            <div className="form-actions">
              <Button variant="soft">保存草稿</Button>
              <Button variant="primary" disabled={!draft.name.trim()}>提交变更</Button>
              <Button variant="ghost" onClick={() => setMode('list')}>取消</Button>
            </div>
          </div>
        </Panel>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="策略"
      subtitle="统一管理认证、限流、请求校验、AI 安全与配额策略"
      actions={
        <>
          <Button variant="soft" onClick={() => setQuery('')}>重置筛选</Button>
          <Button variant="primary" onClick={openCreate}>新建策略</Button>
        </>
      }
    >
        <Panel
          title="策略列表"
          actions={
            <div className="table-toolbar">
              <input className="toolbar-input" value={query} placeholder="搜索策略名称 / 类型 / 范围" onChange={(event) => setQuery(event.target.value)} />
            </div>
          }
        >
          <div style={{ overflow: 'auto' }}>
            <table className="table">
              <thead>
                <tr>
                  <th>策略名称</th>
                  <th>类型</th>
                  <th>适用范围</th>
                  <th>已绑定路由</th>
                  <th>生效状态</th>
                  <th>风险等级</th>
                  <th>最近变更</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visiblePolicies.map((policy) => (
                  <tr key={policy.id} className={policy.id === selectedPolicyId ? 'selected' : ''} onClick={() => setSelectedPolicyId(policy.id)}>
                    <td>{policy.name}</td>
                    <td>{policy.type}</td>
                    <td>{policy.scope}</td>
                    <td>{policy.boundRoutes}</td>
                    <td>
                      <Badge tone={statusTone(policy.publishStatus)}>{publishStatusLabel(policy.publishStatus)}</Badge>
                    </td>
                    <td>
                      <Badge tone={statusTone(riskLevelStatus(policy.riskLevel))}>{riskLevelLabel(policy.riskLevel)}</Badge>
                    </td>
                    <td>{policy.lastChangedAt}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setSelectedPolicyId(policy.id);
                          setMode('detail');
                        }}>详情</button>
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          openEdit(policy);
                        }}>编辑</button>
                        <button className="link-button danger" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setHiddenPolicyIds((ids) => [...ids, policy.id]);
                        }}>删除</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
    </PageFrame>
  );
}

function PolicyDetail({ policy }: { policy: PolicyResource }) {
  const rows = [
    ['策略名称', policy.name],
    ['类型', policy.type],
    ['适用范围', policy.scope],
    ['已绑定路由', String(policy.boundRoutes)],
    ['生效状态', publishStatusLabel(policy.publishStatus)],
    ['风险等级', riskLevelLabel(policy.riskLevel)],
    ['最近变更', policy.lastChangedAt],
  ];

  return (
    <div className="detail-card">
      <div className="kv">
        {rows.flatMap(([label, value]) => [
          <div key={`${label}-label`}>{label}</div>,
          <div key={`${label}-value`}>{value}</div>,
        ])}
      </div>
    </div>
  );
}

function InputField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <input value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function SelectField({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option} value={option}>{riskLevelLabel(option as PolicyDraft['riskLevel'])}</option>
        ))}
      </select>
    </div>
  );
}
