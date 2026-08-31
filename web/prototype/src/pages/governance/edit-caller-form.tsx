import { useState } from "react";
import {
  Drawer,
  FormActions,
  submitForm,
} from "../../components/ui";
import type { Caller } from "../../data";

export function EditCaller({
  caller,
  onClose,
  onSave,
}: {
  caller: Caller;
  onClose: () => void;
  onSave: (caller: Caller) => void;
}) {
  const [name, setName] = useState(caller.name);
  const [owner, setOwner] = useState(caller.owner);
  const [purpose, setPurpose] = useState(caller.purpose);
  const [enabled, setEnabled] = useState(caller.enabled);
  return (
    <Drawer title="编辑调用方" description={caller.name} onClose={onClose}>
      <form
        onSubmit={(event) =>
          submitForm(event, () => onSave({ ...caller, name, owner, purpose, enabled }))
        }
      >
        <div className="form-grid">
          <label className="field-wide">
            <span>调用方名称</span>
            <input required value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="field-wide">
            <span>负责人</span>
            <input required value={owner} onChange={(event) => setOwner(event.target.value)} />
          </label>
          <label className="field-wide">
            <span>用途</span>
            <textarea required value={purpose} onChange={(event) => setPurpose(event.target.value)} />
          </label>
          <label className="field-wide toggle-line">
            <input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
            <span><strong>启用调用方</strong><small>停用后其全部 API Key 立即拒绝访问，不删除凭据、授权和用量记录</small></span>
          </label>
        </div>
        <FormActions submitLabel="保存" onCancel={onClose} />
      </form>
    </Drawer>
  );
}
