import { useState } from 'react';
import { deletePolicyBinding, savePolicyBinding, setPolicyBindingEnabled } from '@/api/policies';
import { Badge, Button, Toast } from '@/components/ui';
import type { GovernancePolicyKind, PolicyBinding, PolicyBindingPayload, PolicyTargetKind, PolicyWorkspace } from '@/domain/policy';
import { policyNamesForBinding, policyRefKey, policyStatusLabel, policyStatusTone, policyTargetKindLabel } from '@/domain/policy';
import { PolicyMultiSelect } from './PolicyMultiSelect';

interface GovernanceBindingDraft {
  id?: string;
  version?: string;
  name: string;
  description: string;
  enabled: boolean;
  policyKeys: string[];
}

interface Notice {
  message: string;
  tone: 'success' | 'error';
}

export function GovernanceBindingPanel({
  targetKind,
  targetID,
  targetName,
  ruleName,
  workspace,
  onChanged,
}: {
  targetKind: PolicyTargetKind;
  targetID: string;
  targetName: string;
  ruleName?: string;
  workspace: PolicyWorkspace;
  onChanged: () => Promise<void> | void;
}) {
  const [editorOpen, setEditorOpen] = useState(false);
  const [draft, setDraft] = useState<GovernanceBindingDraft>(() => newBindingDraft(targetName));
  const [notice, setNotice] = useState<Notice | null>(null);
  const bindings = workspace.bindings.filter((binding) => {
    if (binding.targetRef.kind !== targetKind || binding.targetRef.name !== targetID) {
      return false;
    }
    return !ruleName || !binding.targetRef.ruleName || binding.targetRef.ruleName === ruleName;
  });

  const openCreate = () => {
    setDraft(newBindingDraft(targetName));
    setEditorOpen(true);
  };

  const openEdit = (binding: PolicyBinding) => {
    setDraft({
      id: binding.id,
      version: binding.version,
      name: binding.name,
      description: binding.description ?? '',
      enabled: binding.enabled,
      policyKeys: binding.policies.map(policyRefKey),
    });
    setEditorOpen(true);
  };

  const saveBinding = async () => {
    try {
      const result = await savePolicyBinding(bindingPayload(draft, targetKind, targetID, ruleName));
      await onChanged();
      setNotice({ message: result.message, tone: 'success' });
      setEditorOpen(false);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存策略绑定失败', tone: 'error' });
    }
  };

  const toggleBinding = async (binding: PolicyBinding) => {
    try {
      const result = await setPolicyBindingEnabled(binding.id, !binding.enabled);
      await onChanged();
      setNotice({ message: result.message, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '更新策略绑定失败', tone: 'error' });
    }
  };

  const deleteBinding = async (binding: PolicyBinding) => {
    try {
      const result = await deletePolicyBinding(binding.id);
      await onChanged();
      setNotice({ message: result.message, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除策略绑定失败', tone: 'error' });
    }
  };

  return (
    <div className="detail-card governance-card">
      <div className="governance-card-head">
        <div>
          <h4>策略绑定</h4>
          <span>{policyTargetKindLabel(targetKind)} / {targetName}</span>
        </div>
        <Button variant="soft" type="button" onClick={openCreate}>新增绑定</Button>
      </div>
      <div className="governance-binding-list">
        {bindings.length > 0 ? bindings.map((binding) => (
          <div className="governance-binding-row" key={binding.id}>
            <div>
              <strong>{binding.name}</strong>
              <span>{policyNamesForBinding(binding, workspace.policies).join('、') || '未选择策略'}</span>
            </div>
            <div className="governance-binding-actions">
              <Badge tone={policyStatusTone(binding.enabled)}>{policyStatusLabel(binding.enabled)}</Badge>
              <button className="link-button" type="button" onClick={() => openEdit(binding)}>编辑</button>
              <button className="link-button" type="button" onClick={() => toggleBinding(binding)}>{binding.enabled ? '停用' : '启用'}</button>
              <button className="link-button danger" type="button" onClick={() => deleteBinding(binding)}>删除</button>
            </div>
          </div>
        )) : (
          <span className="mini-card-meta">暂无绑定策略</span>
        )}
      </div>
      {editorOpen ? (
        <div className="governance-binding-editor">
          <div className="policy-editor-grid">
            <label className="field">
              <span>绑定名称</span>
              <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
            </label>
            <label className="field">
              <span>描述</span>
              <input value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
            </label>
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} />
              <span>启用绑定</span>
            </label>
            <PolicyMultiSelect
              policies={workspace.policies}
              value={draft.policyKeys}
              onChange={(policyKeys) => setDraft({ ...draft, policyKeys })}
            />
          </div>
          <div className="form-actions">
            <Button variant="primary" type="button" onClick={saveBinding}>保存绑定</Button>
            <Button variant="ghost" type="button" onClick={() => setEditorOpen(false)}>取消</Button>
          </div>
        </div>
      ) : null}
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </div>
  );
}

function newBindingDraft(targetName: string): GovernanceBindingDraft {
  return {
    name: `${targetName} 策略绑定`,
    description: '',
    enabled: true,
    policyKeys: [],
  };
}

function bindingPayload(
  draft: GovernanceBindingDraft,
  targetKind: PolicyTargetKind,
  targetID: string,
  ruleName?: string,
): PolicyBindingPayload {
  return {
    id: draft.id,
    version: draft.version,
    name: draft.name,
    description: draft.description,
    enabled: draft.enabled,
    targetRef: {
      kind: targetKind,
      name: targetID,
      ...(targetKind === 'Route' && ruleName ? { ruleName } : {}),
    },
    policies: draft.policyKeys.map((key) => {
      const [kind, name] = key.split(':') as [GovernancePolicyKind, string];
      return { kind, name };
    }),
  };
}
