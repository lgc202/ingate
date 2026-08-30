import { useEffect, useState, type FormEvent } from 'react';
import { CircleAlert, KeyRound, Link2, ShieldCheck } from 'lucide-react';
import { updateModelConnection } from '@/api/assistant';
import { errorMessage } from '@/api/errors';
import { Button, Drawer } from '@/components/ui';
import type {
  ModelConnection,
  ModelConnectionMode,
  ModelProtocol,
  UpdateModelConnectionInput,
} from '@/domain/assistant';

interface ModelConnectionDrawerProps {
  connection: ModelConnection;
  open: boolean;
  onClose: () => void;
  onSaved: (connection: ModelConnection) => void;
}

interface ConnectionForm {
  mode: ModelConnectionMode;
  protocol: ModelProtocol;
  endpoint: string;
  model: string;
  apiKey: string;
  clearApiKey: boolean;
  timeoutSeconds: number;
  maxOutputTokens: number;
  reasoningBudgetTokens: number;
}

export function ModelConnectionDrawer({ connection, open, onClose, onSaved }: ModelConnectionDrawerProps) {
  const [form, setForm] = useState(() => connectionForm(connection));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setForm(connectionForm(connection));
    setError('');
  }, [connection, open]);

  const setMode = (mode: ModelConnectionMode) => {
    setForm((current) => ({
      ...current,
      mode,
      protocol: mode === 'MODEL_CONNECTION_MODE_INGATE'
        ? 'MODEL_PROTOCOL_OPENAI_COMPATIBLE'
        : current.protocol,
      reasoningBudgetTokens: mode === 'MODEL_CONNECTION_MODE_INGATE' ? 0 : current.reasoningBudgetTokens,
    }));
  };

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (saving) return;
    const validationError = validateConnection(form);
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    setError('');
    try {
      const input: UpdateModelConnectionInput = {
        connectionMode: form.mode,
        protocol: form.mode === 'MODEL_CONNECTION_MODE_INGATE'
          ? 'MODEL_PROTOCOL_OPENAI_COMPATIBLE'
          : form.protocol,
        endpoint: form.endpoint.trim(),
        model: form.model.trim(),
        clearApiKey: form.clearApiKey,
        timeoutSeconds: form.timeoutSeconds,
        maxOutputTokens: form.maxOutputTokens,
        reasoningBudgetTokens: form.mode === 'MODEL_CONNECTION_MODE_DIRECT'
          && form.protocol === 'MODEL_PROTOCOL_ANTHROPIC'
          ? form.reasoningBudgetTokens
          : 0,
      };
      if (form.apiKey.trim()) input.apiKey = form.apiKey.trim();
      onSaved(await updateModelConnection(input));
      onClose();
    } catch (cause) {
      setError(errorMessage(cause, '保存模型连接失败'));
    } finally {
      setSaving(false);
    }
  };

  const isAnthropic = form.mode === 'MODEL_CONNECTION_MODE_DIRECT'
    && form.protocol === 'MODEL_PROTOCOL_ANTHROPIC';

  return (
    <Drawer
      title="模型连接"
      subtitle="运维助手使用的模型与访问凭据"
      isOpen={open}
      onClose={onClose}
    >
      <form className="assistant-connection-form" onSubmit={(event) => void save(event)}>
        <section className="assistant-form-section">
          <header>
            <Link2 aria-hidden="true" />
            <div>
              <h3>连接方式</h3>
              <p>选择模型请求的出口</p>
            </div>
          </header>
          <div className="assistant-mode-options">
            <button
              type="button"
              className={form.mode === 'MODEL_CONNECTION_MODE_INGATE' ? 'is-selected' : ''}
              onClick={() => setMode('MODEL_CONNECTION_MODE_INGATE')}
            >
              <ShieldCheck aria-hidden="true" />
              <strong>通过当前网关</strong>
              <span>使用已发布的模型路由，凭据与治理能力由网关统一处理</span>
            </button>
            <button
              type="button"
              className={form.mode === 'MODEL_CONNECTION_MODE_DIRECT' ? 'is-selected' : ''}
              onClick={() => setMode('MODEL_CONNECTION_MODE_DIRECT')}
            >
              <Link2 aria-hidden="true" />
              <strong>直连模型端点</strong>
              <span>直接连接兼容的模型服务，适合独立部署助手</span>
            </button>
          </div>
        </section>

        <section className="assistant-form-section">
          <header>
            <KeyRound aria-hidden="true" />
            <div>
              <h3>连接参数</h3>
              <p>密钥只会保存在服务端</p>
            </div>
          </header>
          <div className="assistant-form-grid">
            {form.mode === 'MODEL_CONNECTION_MODE_DIRECT' ? (
              <label>
                <span>接口协议</span>
                <select
                  className="select"
                  value={form.protocol}
                  onChange={(event) => setForm((current) => ({
                    ...current,
                    protocol: event.target.value as ModelProtocol,
                    reasoningBudgetTokens: event.target.value === 'MODEL_PROTOCOL_ANTHROPIC'
                      ? current.reasoningBudgetTokens
                      : 0,
                  }))}
                >
                  <option value="MODEL_PROTOCOL_OPENAI_COMPATIBLE">OpenAI 兼容</option>
                  <option value="MODEL_PROTOCOL_ANTHROPIC">Anthropic</option>
                </select>
              </label>
            ) : null}
            <label className={form.mode === 'MODEL_CONNECTION_MODE_INGATE' ? 'is-wide' : ''}>
              <span>模型名称</span>
              <input
                className="input"
                value={form.model}
                placeholder="例如 qwen-max"
                onChange={(event) => setForm((current) => ({ ...current, model: event.target.value }))}
              />
            </label>
            <label className="is-wide">
              <span>服务地址</span>
              <input
                className="input"
                type="url"
                value={form.endpoint}
                placeholder={form.mode === 'MODEL_CONNECTION_MODE_INGATE'
                  ? '例如 http://envoy:8080/v1'
                  : '模型服务的 API 地址'}
                onChange={(event) => setForm((current) => ({ ...current, endpoint: event.target.value }))}
              />
            </label>
            <label className="is-wide">
              <span>访问密钥</span>
              <input
                className="input"
                type="password"
                autoComplete="new-password"
                value={form.apiKey}
                disabled={form.clearApiKey}
                placeholder={connection.apiKeyConfigured ? '已配置，留空表示不修改' : '未配置'}
                onChange={(event) => setForm((current) => ({ ...current, apiKey: event.target.value }))}
              />
            </label>
            {connection.apiKeyConfigured ? (
              <label className="assistant-clear-secret is-wide">
                <input
                  type="checkbox"
                  checked={form.clearApiKey}
                  onChange={(event) => setForm((current) => ({
                    ...current,
                    clearApiKey: event.target.checked,
                    apiKey: event.target.checked ? '' : current.apiKey,
                  }))}
                />
                <span>清除已保存的访问密钥</span>
              </label>
            ) : null}
          </div>
        </section>

        <section className="assistant-form-section">
          <header>
            <span className="assistant-number-icon">#</span>
            <div>
              <h3>生成限制</h3>
              <p>约束单次回答的等待时间与输出长度</p>
            </div>
          </header>
          <div className="assistant-form-grid">
            <NumberField
              label="请求超时（秒）"
              value={form.timeoutSeconds}
              min={1}
              max={1800}
              onChange={(value) => setForm((current) => ({ ...current, timeoutSeconds: value }))}
            />
            <NumberField
              label="最大输出 Token"
              value={form.maxOutputTokens}
              min={1}
              max={1000000}
              onChange={(value) => setForm((current) => ({ ...current, maxOutputTokens: value }))}
            />
            {isAnthropic ? (
              <NumberField
                label="思考预算 Token"
                value={form.reasoningBudgetTokens}
                min={0}
                max={999999}
                onChange={(value) => setForm((current) => ({ ...current, reasoningBudgetTokens: value }))}
              />
            ) : null}
          </div>
        </section>

        {error ? (
          <div className="assistant-form-error" role="alert">
            <CircleAlert aria-hidden="true" />
            <span>{error}</span>
          </div>
        ) : null}
        <footer className="assistant-form-actions">
          <Button type="button" variant="ghost" onClick={onClose}>取消</Button>
          <Button type="submit" size="lg" disabled={saving}>{saving ? '保存中...' : '保存连接'}</Button>
        </footer>
      </form>
    </Drawer>
  );
}

function NumberField({
  label,
  value,
  min,
  max,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
}) {
  return (
    <label>
      <span>{label}</span>
      <input
        className="input"
        type="number"
        value={value}
        min={min}
        max={max}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}

function connectionForm(connection: ModelConnection): ConnectionForm {
  return {
    mode: connection.connectionMode || 'MODEL_CONNECTION_MODE_INGATE',
    protocol: connection.protocol || 'MODEL_PROTOCOL_OPENAI_COMPATIBLE',
    endpoint: connection.endpoint,
    model: connection.model,
    apiKey: '',
    clearApiKey: false,
    timeoutSeconds: connection.timeoutSeconds || 120,
    maxOutputTokens: connection.maxOutputTokens || 4096,
    reasoningBudgetTokens: connection.reasoningBudgetTokens || 0,
  };
}

function validateConnection(form: ConnectionForm): string {
  if (!form.model.trim()) return '请输入模型名称';
  if (!form.endpoint.trim()) return '请输入服务地址';
  try {
    new URL(form.endpoint.trim());
  } catch {
    return '服务地址格式不正确';
  }
  if (form.timeoutSeconds < 1 || form.timeoutSeconds > 1800) return '请求超时必须在 1 到 1800 秒之间';
  if (form.maxOutputTokens < 1 || form.maxOutputTokens > 1000000) return '最大输出 Token 超出有效范围';
  if (form.mode === 'MODEL_CONNECTION_MODE_DIRECT'
    && form.protocol === 'MODEL_PROTOCOL_ANTHROPIC'
    && form.reasoningBudgetTokens !== 0
    && (form.reasoningBudgetTokens < 1024 || form.reasoningBudgetTokens >= form.maxOutputTokens)) {
    return '思考预算应为 0，或至少 1024 且小于最大输出 Token';
  }
  return '';
}
