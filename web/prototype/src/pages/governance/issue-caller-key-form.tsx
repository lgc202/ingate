import {
  CalendarDays,
  KeyRound,
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

export function IssueCallerKey({
  caller,
  onClose,
  onIssue,
}: {
  caller: Caller;
  onClose: () => void;
  onIssue: (key: CallerAccessKey) => void;
}) {
  const [name, setName] = useState(`${caller.name}访问密钥`);
  const [validityDays, setValidityDays] = useState("90");
  const [secret, setSecret] = useState("");
  const [issuedKey, setIssuedKey] = useState<CallerAccessKey | null>(null);
  const issue = () => {
    const value = `ig_live_${crypto.randomUUID().replaceAll("-", "")}`;
    const created = new Date();
    const expires = new Date(created);
    expires.setDate(expires.getDate() + Number(validityDays));
    const key: CallerAccessKey = {
      id: `key-${crypto.randomUUID()}`,
      name,
      prefix: `${value.slice(0, 16)}…`,
      createdAt: created.toISOString().slice(0, 10),
      expiresAt: expires.toISOString().slice(0, 10),
      lastUsed: "尚未使用",
      state: "healthy",
    };
    onIssue(key);
    setIssuedKey(key);
    setSecret(value);
  };

  if (secret && issuedKey)
    return (
      <Drawer
        title="访问密钥已签发"
        description={caller.name}
        onClose={onClose}
      >
        <div className="secret-result">
          <span>
            <KeyRound />
          </span>
          <div>
            <strong>立即复制并妥善保存</strong>
            <p>
              完整密钥仅显示一次。系统只保留密钥标识和管理信息，无法再次查看明文。
            </p>
          </div>
        </div>
        <div className="secret-value">
          <code>{secret}</code>
          <CopyButton value={secret} />
        </div>
        <dl className="definition-list secret-metadata">
          <div>
            <dt>密钥名称</dt>
            <dd>{issuedKey.name}</dd>
          </div>
          <div>
            <dt>到期时间</dt>
            <dd>{issuedKey.expiresAt}</dd>
          </div>
          <div>
            <dt>保存记录</dt>
            <dd>
              <code>{issuedKey.prefix}</code>
            </dd>
          </div>
        </dl>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>完成</PrimaryButton>
        </footer>
      </Drawer>
    );

  return (
    <Drawer title="签发访问密钥" description={caller.name} onClose={onClose}>
      <form onSubmit={(event) => submitForm(event, issue)}>
        <div className="form-grid">
          <label className="field-wide">
            <span>密钥名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如生产环境、自动化任务"
            />
          </label>
          <label className="field-wide">
            <span>有效期</span>
            <select
              value={validityDays}
              onChange={(event) => setValidityDays(event.target.value)}
            >
              <option value="30">30 天</option>
              <option value="90">90 天</option>
              <option value="180">180 天</option>
              <option value="365">365 天</option>
            </select>
            <small>到期后自动失效，可提前停用</small>
          </label>
        </div>
        <div className="form-note">
          <CalendarDays />
          签发后生成新的访问密钥；完整密钥仅显示一次，之后只能查看名称、标识和使用状态。
        </div>
        <FormActions submitLabel="签发密钥" onCancel={onClose} />
      </form>
    </Drawer>
  );
}
