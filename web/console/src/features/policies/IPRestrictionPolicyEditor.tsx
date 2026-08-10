import type {
  IPRestrictionPolicy,
  IPRestrictionPolicyPayload,
  PolicyTargetOption,
  PolicyTargetRef,
} from '@/domain/policy';
import { PolicyInputField, PolicyTextareaField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

type RestrictionMode = 'allow' | 'deny';

export interface IPRestrictionPolicyDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  mode: RestrictionMode;
  ranges: string;
}

const fields = {
  name: 'name',
  ranges: 'allow',
} as const;

type FieldPath = typeof fields[keyof typeof fields];

export interface IPRestrictionPolicyValidation {
  valid: boolean;
  errors: Partial<Record<FieldPath, string>>;
}

export function IPRestrictionPolicyEditor({
  draft,
  targets,
  validation,
  onChange,
}: {
  draft: IPRestrictionPolicyDraft;
  targets: PolicyTargetOption[];
  validation: IPRestrictionPolicyValidation;
  onChange: (draft: IPRestrictionPolicyDraft) => void;
}) {
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title">
          <h3>基础信息</h3>
          <p>策略只判断客户端连接来源 IP，不读取请求头中的转发地址。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} error={validation.errors[fields.name]} onChange={(name) => onChange({ ...draft, name })} />
          {draft.id ? (
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
              <span>启用策略</span>
            </label>
          ) : null}
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>应用目标</h3>
          <p>可同时应用到多个网关或路由；留空时仅保存策略。</p>
        </div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect options={targets} value={draft.targets} onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })} />
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title">
          <h3>访问范围</h3>
          <p>允许列表只放行列出的地址；拒绝列表只拦截列出的地址。</p>
        </div>
        <div className="policy-restriction-mode" role="radiogroup" aria-label="访问限制方式">
          <button
            className={draft.mode === 'allow' ? 'selected' : ''}
            type="button"
            role="radio"
            aria-checked={draft.mode === 'allow'}
            onClick={() => onChange({ ...draft, mode: 'allow' })}
          >
            <strong>仅允许列表内地址</strong>
            <span>适合管理接口、内部服务和固定合作方出口</span>
          </button>
          <button
            className={draft.mode === 'deny' ? 'selected' : ''}
            type="button"
            role="radio"
            aria-checked={draft.mode === 'deny'}
            onClick={() => onChange({ ...draft, mode: 'deny' })}
          >
            <strong>拒绝列表内地址</strong>
            <span>适合屏蔽已知异常来源或临时封禁网段</span>
          </button>
        </div>
        <div className="policy-editor-grid">
          <PolicyTextareaField
            label={draft.mode === 'allow' ? '允许的 IP 或 CIDR' : '拒绝的 IP 或 CIDR'}
            value={draft.ranges}
            placeholder={'每行一个，例如：\n192.168.1.20\n10.0.0.0/8\n2001:db8::/32'}
            hint="支持 IPv4、IPv6 和 CIDR；单个 IP 保存后会自动转换为精确网段。"
            error={validation.errors[fields.ranges]}
            onChange={(ranges) => onChange({ ...draft, ranges })}
          />
        </div>
        <div className="mini-card policy-execution-note">
          <div className="mini-card-title">拒绝响应固定为 HTTP 403</div>
          <div className="mini-card-meta">响应内容由系统统一维护，策略只描述哪些客户端可以访问。</div>
        </div>
      </section>
    </div>
  );
}

export function createIPRestrictionPolicyDraft(policy?: IPRestrictionPolicy): IPRestrictionPolicyDraft {
  const mode: RestrictionMode = policy?.deny.length ? 'deny' : 'allow';
  const ranges = mode === 'allow' ? policy?.allow : policy?.deny;
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    mode,
    ranges: ranges?.join('\n') ?? '',
  };
}

export function validateIPRestrictionPolicyDraft(draft: IPRestrictionPolicyDraft): IPRestrictionPolicyValidation {
  const errors: IPRestrictionPolicyValidation['errors'] = {};
  if (!draft.name.trim()) {
    errors[fields.name] = '请输入策略名称';
  }
  if (parseRanges(draft.ranges).length === 0) {
    errors[fields.ranges] = draft.mode === 'allow' ? '请至少填写一个允许地址' : '请至少填写一个拒绝地址';
  }
  return { valid: Object.values(errors).every((message) => !message), errors };
}

export function ipRestrictionPolicyPayload(draft: IPRestrictionPolicyDraft): IPRestrictionPolicyPayload {
  const ranges = parseRanges(draft.ranges);
  const config = {
    name: draft.name.trim(),
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    allow: draft.mode === 'allow' ? ranges : [],
    deny: draft.mode === 'deny' ? ranges : [],
  };
  if (!draft.id) {
    return config;
  }
  if (!draft.version) {
    throw new Error('策略版本缺失，请刷新后重试');
  }
  return { ...config, id: draft.id, version: draft.version, enabled: draft.enabled };
}

function parseRanges(value: string) {
  return Array.from(new Set(value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean)));
}
