import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui';
import type {
  MockResponsePolicy,
  MockResponsePolicyPayload,
  PolicyTargetOption,
  PolicyTargetRef,
} from '@/domain/policy';
import { PolicyInputField, PolicyTextareaField } from './PolicyEditorFields';
import { PolicyTargetSelect } from './PolicySelectors';

interface MockResponseHeaderDraft {
  id: string;
  name: string;
  value: string;
}

export interface MockResponsePolicyDraft {
  id?: string;
  version?: number;
  name: string;
  enabled: boolean;
  targets: PolicyTargetRef[];
  statusCode: string;
  contentType: string;
  headers: MockResponseHeaderDraft[];
  body: string;
}

export interface MockResponsePolicyValidation {
  valid: boolean;
  errors: Record<string, string>;
}

export function MockResponsePolicyEditor({
  draft,
  targets,
  validation,
  onChange,
}: {
  draft: MockResponsePolicyDraft;
  targets: PolicyTargetOption[];
  validation: MockResponsePolicyValidation;
  onChange: (draft: MockResponsePolicyDraft) => void;
}) {
  const routeTargets = targets.filter((target) => target.kind === 'Route');
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title"><h3>基础信息</h3></div>
        <div className="policy-editor-grid">
          <PolicyInputField label="策略名称" value={draft.name} error={validation.errors.name} onChange={(name) => onChange({ ...draft, name })} />
          {draft.id ? (
            <label className="policy-check-row">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
              <span>启用策略</span>
            </label>
          ) : null}
        </div>
      </section>

      <section className="form-section">
        <div className="form-section-title"><h3>应用路由</h3></div>
        <div className="policy-editor-grid">
          <PolicyTargetSelect
            label="路由"
            emptyMessage="暂无可选路由"
            options={routeTargets}
            value={draft.targets}
            onChange={(nextTargets) => onChange({ ...draft, targets: nextTargets })}
          />
        </div>
      </section>

      <section className="form-section space-y-4">
        <div className="form-section-title"><h3>响应内容</h3></div>
        <div className="policy-editor-grid">
          <PolicyInputField
            label="HTTP 状态码"
            type="number"
            value={draft.statusCode}
            error={validation.errors.statusCode}
            onChange={(statusCode) => onChange({ ...draft, statusCode })}
          />
          <div className={`field${validation.errors.contentType ? ' invalid' : ''}`}>
            <label htmlFor="mock-response-content-type">内容类型</label>
            <input
              id="mock-response-content-type"
              list="mock-response-content-types"
              value={draft.contentType}
              aria-invalid={Boolean(validation.errors.contentType)}
              onChange={(event) => onChange({ ...draft, contentType: event.target.value })}
            />
            <datalist id="mock-response-content-types">
              <option value="application/json" />
              <option value="text/plain; charset=utf-8" />
              <option value="text/html; charset=utf-8" />
            </datalist>
            {validation.errors.contentType ? <div className="form-error" role="alert">{validation.errors.contentType}</div> : null}
          </div>
          <PolicyTextareaField
            label="响应正文"
            value={draft.body}
            error={validation.errors.body}
            onChange={(body) => onChange({ ...draft, body })}
          />
        </div>
      </section>

      <section className="form-section space-y-4">
        <div className="form-section-title">
          <h3>响应 Header</h3>
          <Button variant="soft" onClick={() => onChange({ ...draft, headers: [...draft.headers, createHeader()] })}>
            <Plus className="h-3.5 w-3.5" />添加 Header
          </Button>
        </div>
        {draft.headers.length > 0 ? (
          <div className="mock-response-header-list">
            {draft.headers.map((header, index) => (
              <div className="mock-response-header-row" key={header.id}>
                <div className={`field${validation.errors[`headers.${index}.name`] ? ' invalid' : ''}`}>
                  <label>名称</label>
                  <input
                    className="font-mono"
                    value={header.name}
                    placeholder="例如：cache-control"
                    onChange={(event) => onChange({
                      ...draft,
                      headers: draft.headers.map((item) => item.id === header.id ? { ...item, name: event.target.value } : item),
                    })}
                  />
                  {validation.errors[`headers.${index}.name`] ? <div className="form-error" role="alert">{validation.errors[`headers.${index}.name`]}</div> : null}
                </div>
                <div className={`field${validation.errors[`headers.${index}.value`] ? ' invalid' : ''}`}>
                  <label>值</label>
                  <input
                    value={header.value}
                    onChange={(event) => onChange({
                      ...draft,
                      headers: draft.headers.map((item) => item.id === header.id ? { ...item, value: event.target.value } : item),
                    })}
                  />
                  {validation.errors[`headers.${index}.value`] ? <div className="form-error" role="alert">{validation.errors[`headers.${index}.value`]}</div> : null}
                </div>
                <button
                  type="button"
                  className="mock-response-header-delete"
                  aria-label={`删除响应 Header ${index + 1}`}
                  onClick={() => onChange({ ...draft, headers: draft.headers.filter((item) => item.id !== header.id) })}
                >
                  <Trash2 aria-hidden="true" />
                </button>
              </div>
            ))}
          </div>
        ) : <div className="policy-empty-rules">未配置额外响应 Header</div>}
      </section>
    </div>
  );
}

export function createMockResponsePolicyDraft(policy?: MockResponsePolicy): MockResponsePolicyDraft {
  return {
    id: policy?.id,
    version: policy?.version,
    name: policy?.name ?? '',
    enabled: policy?.enabled ?? true,
    targets: policy?.targets ?? [],
    statusCode: String(policy?.statusCode ?? 200),
    contentType: policy?.contentType ?? 'application/json',
    headers: policy?.headers.map((header) => ({ ...header, id: crypto.randomUUID() })) ?? [],
    body: policy?.body ?? '{\n  "message": "mock response"\n}',
  };
}

export function validateMockResponsePolicyDraft(draft: MockResponsePolicyDraft): MockResponsePolicyValidation {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) errors.name = '请输入策略名称';
  const statusCode = Number(draft.statusCode);
  if (!Number.isInteger(statusCode) || statusCode < 200 || statusCode > 599) errors.statusCode = '请输入 200 到 599 之间的整数';
  if (!/^[^/\s]+\/[^;\s]+(?:\s*;.*)?$/.test(draft.contentType.trim())) errors.contentType = '请输入有效的内容类型';
  if (new TextEncoder().encode(draft.body).length > 1_048_576) errors.body = '响应正文不能超过 1 MiB';

  const names = new Set<string>();
  draft.headers.forEach((header, index) => {
    const name = header.name.trim().toLowerCase();
    if (!/^[!#$%&'*+.^_`|~0-9a-z-]+$/.test(name)) {
      errors[`headers.${index}.name`] = '请输入有效的 Header 名称';
    } else if (name === 'content-type') {
      errors[`headers.${index}.name`] = 'Content-Type 请在上方配置';
    } else if (names.has(name)) {
      errors[`headers.${index}.name`] = 'Header 名称不能重复';
    }
    names.add(name);
    if (/\r|\n/.test(header.value)) errors[`headers.${index}.value`] = 'Header 值不能包含换行符';
  });
  return { valid: Object.keys(errors).length === 0, errors };
}

export function mockResponsePolicyPayload(draft: MockResponsePolicyDraft): MockResponsePolicyPayload {
  const config = {
    name: draft.name.trim(),
    targets: draft.targets.map((target) => ({ kind: target.kind, id: target.id })),
    statusCode: Number(draft.statusCode),
    contentType: draft.contentType.trim(),
    headers: draft.headers.map((header) => ({ name: header.name.trim().toLowerCase(), value: header.value })),
    body: draft.body,
  };
  if (!draft.id) return config;
  if (!draft.version) throw new Error('策略版本缺失，请刷新后重试');
  return { ...config, id: draft.id, version: draft.version, enabled: draft.enabled };
}

function createHeader(): MockResponseHeaderDraft {
  return { id: crypto.randomUUID(), name: '', value: '' };
}
