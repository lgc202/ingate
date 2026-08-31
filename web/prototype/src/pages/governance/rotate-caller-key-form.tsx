import {
  RotateCw,
  ShieldCheck,
} from "lucide-react";
import { useState } from "react";
import {
  CopyButton,
  Drawer,
  FormActions,
  PrimaryButton,
  submitForm,
} from "../../components/ui";
import type {
  Caller,
  CallerAccessKey,
} from "../../data";

export function RotateCallerKey({
  caller,
  currentKey,
  onClose,
  onRotate,
}: {
  caller: Caller;
  currentKey: CallerAccessKey;
  onClose: () => void;
  onRotate: (key: CallerAccessKey, graceUntil: string) => void;
}) {
  const [graceDays, setGraceDays] = useState("7");
  const [secret, setSecret] = useState("");
  const [newKey, setNewKey] = useState<CallerAccessKey | null>(null);
  const [graceUntil, setGraceUntil] = useState("");
  const rotate = () => {
    const value = `ig_live_${crypto.randomUUID().replaceAll("-", "")}`;
    const created = new Date();
    const expires = new Date(created);
    expires.setDate(expires.getDate() + 90);
    const graceEnd = new Date(created);
    graceEnd.setDate(graceEnd.getDate() + Number(graceDays));
    const createdKey: CallerAccessKey = {
      id: `key-${crypto.randomUUID()}`,
      name: `${currentKey.name} · 新密钥`,
      prefix: `${value.slice(0, 16)}…`,
      createdAt: created.toISOString().slice(0, 10),
      expiresAt: expires.toISOString().slice(0, 10),
      lastUsed: "尚未使用",
      state: "healthy",
    };
    const graceDate = graceEnd.toISOString().slice(0, 10);
    onRotate(createdKey, graceDate);
    setNewKey(createdKey);
    setGraceUntil(graceDate);
    setSecret(value);
  };

  if (secret && newKey)
    return (
      <Drawer
        title="密钥轮换已开始"
        description={`${caller.name} · ${currentKey.name}`}
        onClose={onClose}
      >
        <div className="secret-result">
          <span>
            <RotateCw />
          </span>
          <div>
            <strong>先部署新密钥，再停用旧密钥</strong>
            <p>
              新旧密钥同时有效至 {graceUntil}。完整新密钥仅显示一次。
            </p>
          </div>
        </div>
        <div className="secret-value">
          <code>{secret}</code>
          <CopyButton value={secret} />
        </div>
        <dl className="definition-list secret-metadata">
          <div>
            <dt>新密钥</dt>
            <dd>{newKey.name}</dd>
          </div>
          <div>
            <dt>旧密钥宽限期</dt>
            <dd>截至 {graceUntil}</dd>
          </div>
          <div>
            <dt>下一步</dt>
            <dd>确认调用已切换后，可提前停用旧密钥</dd>
          </div>
        </dl>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>我已保存新密钥</PrimaryButton>
        </footer>
      </Drawer>
    );

  return (
    <Drawer
      title="轮换访问密钥"
      description={`${caller.name} · ${currentKey.name}`}
      onClose={onClose}
    >
      <form onSubmit={(event) => submitForm(event, rotate)}>
        <div className="rotation-path">
          <div>
            <span>1</span>
            <strong>生成新密钥</strong>
          </div>
          <i />
          <div>
            <span>2</span>
            <strong>迁移调用方</strong>
          </div>
          <i />
          <div>
            <span>3</span>
            <strong>停用旧密钥</strong>
          </div>
        </div>
        <div className="form-grid">
          <label className="field-wide">
            <span>旧密钥宽限期</span>
            <select
              value={graceDays}
              onChange={(event) => setGraceDays(event.target.value)}
            >
              <option value="1">1 天</option>
              <option value="7">7 天</option>
              <option value="14">14 天</option>
              <option value="30">30 天</option>
            </select>
            <small>宽限期内新旧密钥同时有效</small>
          </label>
        </div>
        <div className="form-note">
          <ShieldCheck />
          访问权限和用量归属保持不变；宽限期结束后旧密钥自动失效。
        </div>
        <FormActions submitLabel="生成新密钥" onCancel={onClose} />
      </form>
    </Drawer>
  );
}
