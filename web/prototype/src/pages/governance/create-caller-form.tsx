import {
  ChevronRight,
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
  GatewayRoute,
} from "../../data";
import type { PermissionDraft } from "./caller-types";
import {
  permissionRecords,
  PermissionSelector,
} from "./caller-permission-form";

export function CreateCaller({
  routes,
  onClose,
  onSave,
}: {
  routes: GatewayRoute[];
  onClose: () => void;
  onSave: (caller: Caller) => void;
}) {
  const [name, setName] = useState("");
  const [owner, setOwner] = useState("");
  const [purpose, setPurpose] = useState("");
  const [permissions, setPermissions] = useState<PermissionDraft>({});
  const [keyName, setKeyName] = useState("");
  const [validityDays, setValidityDays] = useState("90");
  const [createdKey, setCreatedKey] = useState<CallerAccessKey | null>(null);
  const [secret, setSecret] = useState("");
  const [activeSection, setActiveSection] = useState<"identity" | "permissions" | "key">(
    "identity",
  );
  const save = () => {
    const granted = permissionRecords(permissions);
    const grantedTypes = new Set(
      granted.map(
        (permission) =>
          routes.find((route) => route.id === permission.routeID)?.type,
      ),
    );
    const metrics = [
      grantedTypes.has("API")
        ? { label: "API 请求", value: "0", note: "今天" }
        : null,
      grantedTypes.has("AI")
        ? { label: "AI Token", value: "0", note: "本月" }
        : null,
      grantedTypes.has("MCP")
        ? { label: "MCP 工具调用", value: "0", note: "今天" }
        : null,
    ].filter((item): item is { label: string; value: string; note: string } =>
      Boolean(item),
    );
    while (metrics.length < 3)
      metrics.push({
        label: metrics.length ? "策略拒绝" : "请求",
        value: "0",
        note: "今天",
      });
    const value = `ig_live_${crypto.randomUUID().replaceAll("-", "")}`;
    const createdAt = new Date();
    const expiresAt = new Date(createdAt);
    expiresAt.setDate(expiresAt.getDate() + Number(validityDays));
    const key: CallerAccessKey = {
      id: `key-${crypto.randomUUID()}`,
      name: keyName || `${name}访问密钥`,
      prefix: `${value.slice(0, 16)}…`,
      createdAt: createdAt.toISOString().slice(0, 10),
      expiresAt: expiresAt.toISOString().slice(0, 10),
      lastUsed: "尚未使用",
      state: "healthy",
    };
    onSave({
      id: `caller-${Date.now()}`,
      name,
      slug: `caller-${crypto.randomUUID().slice(0, 8)}`,
      owner,
      purpose,
      enabled: true,
      keys: [key],
      permissions: granted,
      metrics,
      quotas: [],
      state: "healthy",
      lastActive: "从未调用",
    });
    setCreatedKey(key);
    setSecret(value);
  };

  if (createdKey)
    return (
      <Drawer title="调用方已创建" description={name} onClose={onClose}>
        <div className="secret-result">
          <span><KeyRound /></span>
          <div>
            <strong>复制访问密钥</strong>
            <p>完整密钥仅显示一次，关闭后无法再次查看。</p>
          </div>
        </div>
        <div className="secret-value">
          <code>{secret}</code>
          <CopyButton value={secret} />
        </div>
        <dl className="definition-list secret-metadata">
          <div><dt>调用方</dt><dd>{name}</dd></div>
          <div><dt>访问权限</dt><dd>{permissionRecords(permissions).length} 项</dd></div>
          <div><dt>密钥到期</dt><dd>{createdKey.expiresAt}</dd></div>
        </dl>
        <footer className="form-actions">
          <PrimaryButton onClick={onClose}>我已保存密钥</PrimaryButton>
        </footer>
      </Drawer>
    );

  return (
    <Drawer
      title="创建调用方"
      description="身份、访问权限与首个密钥"
      onClose={onClose}
      width="wide"
    >
      <form
        className="caller-create-form"
        onSubmit={(event) => submitForm(event, save)}
      >
        <section className="form-section caller-create-section">
          <button
            className="caller-create-section-toggle"
            type="button"
            onClick={() => setActiveSection("identity")}
            aria-expanded={activeSection === "identity"}
          >
            <span>1</span>
            <div>
              <strong>基本信息</strong>
              <small>
                {name && owner ? `${name} · ${owner}` : "用于识别实际发起请求的应用或服务"}
              </small>
            </div>
            <ChevronRight />
          </button>
          <div
            className="caller-create-section-body form-grid"
            hidden={activeSection !== "identity"}
          >
            <label>
              <span>调用方名称</span>
              <input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="例如客服助手"
              />
            </label>
            <label>
              <span>负责人</span>
              <input
                value={owner}
                onChange={(event) => setOwner(event.target.value)}
                placeholder="团队或负责人"
              />
            </label>
            <label className="field-wide">
              <span>用途</span>
              <textarea
                value={purpose}
                onChange={(event) => setPurpose(event.target.value)}
                placeholder="例如客服工作台查询订单并生成辅助回复"
              />
            </label>
          </div>
        </section>

        <PermissionSelector
          sectionNumber="2"
          routes={routes}
          value={permissions}
          onChange={setPermissions}
          expanded={activeSection === "permissions"}
          onToggle={() => setActiveSection("permissions")}
        />

        <section className="form-section caller-create-section">
          <button
            className="caller-create-section-toggle"
            type="button"
            onClick={() => setActiveSection("key")}
            aria-expanded={activeSection === "key"}
          >
            <span>3</span>
            <div>
              <strong>访问密钥</strong>
              <small>{keyName || `有效期 ${validityDays} 天 · 创建后仅显示一次`}</small>
            </div>
            <ChevronRight />
          </button>
          <div
            className="caller-create-section-body form-grid"
            hidden={activeSection !== "key"}
          >
            <label>
              <span>密钥名称</span>
              <input
                value={keyName}
                onChange={(event) => setKeyName(event.target.value)}
                placeholder={`${name || "调用方"}生产环境`}
              />
            </label>
            <label>
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
            </label>
          </div>
        </section>

        <FormActions
          submitLabel="创建并签发密钥"
          submitDisabled={!name.trim() || !owner.trim() || !purpose.trim()}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
