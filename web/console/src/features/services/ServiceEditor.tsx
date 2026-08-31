import type { ReactNode } from 'react';
import { KeyRound, Trash2 } from 'lucide-react';
import { Button, Drawer } from '@/components/ui';
import type { ServiceEndpoint } from '@/domain/service';
import { serviceLoadBalancingOptions } from '@/domain/service';
import type { ServiceDraft } from './form';

interface ServiceEditorProps {
  draft: ServiceDraft;
  open: boolean;
  busy: boolean;
  onChange: (draft: ServiceDraft) => void;
  onClose: () => void;
  onSave: () => void;
}

export function ServiceEditor({
  draft,
  open,
  busy,
  onChange,
  onClose,
  onSave,
}: ServiceEditorProps) {
  const updateEndpoint = (index: number, endpoint: ServiceEndpoint) => {
    onChange({
      ...draft,
      endpoints: draft.endpoints.map((item, current) => (
        current === index ? endpoint : item
      )),
    });
  };

  return (
    <Drawer
      title={draft.id ? `编辑服务：${draft.name}` : '创建服务'}
      isOpen={open}
      onClose={() => onClose()}
    >
      <div className="space-y-5">
        <Field label="服务名称">
          <input
            className="input"
            value={draft.name}
            onChange={(event) => onChange({ ...draft, name: event.target.value })}
          />
        </Field>
        {!draft.id ? (
          <Field label="服务类型">
            <select
              className="select"
              value={draft.type}
              onChange={(event) => onChange({
                ...draft,
                type: event.target.value as ServiceDraft['type'],
              })}
            >
              <option value="HTTP">HTTP 服务</option>
              <option value="MODEL">模型服务</option>
            </select>
          </Field>
        ) : null}
        {draft.type === 'MODEL' ? (
          <section className="rounded-xl border border-violet-100 bg-violet-50/40 p-4 space-y-4">
            <Field label="模型接口协议">
              <select
                className="select"
                value={draft.modelProtocol}
                onChange={(event) => onChange({
                  ...draft,
                  modelProtocol: event.target.value as ServiceDraft['modelProtocol'],
                })}
              >
                <option value="MODEL_PROTOCOL_OPENAI">OpenAI 兼容</option>
                <option value="MODEL_PROTOCOL_ANTHROPIC">Anthropic Messages</option>
              </select>
            </Field>
            <Field label="API Key">
              <div className="relative">
                <KeyRound className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                <input
                  className="input input-leading-icon"
                  type="password"
                  autoComplete="new-password"
                  value={draft.apiKey}
                  onChange={(event) => onChange({
                    ...draft,
                    apiKey: event.target.value,
                    clearApiKey: false,
                  })}
                  placeholder={draft.apiKeyConfigured
                    ? '已配置，留空保持不变'
                    : '未配置时适用于无需认证的模型服务'}
                />
              </div>
            </Field>
            {draft.apiKeyConfigured ? (
              <label className="flex items-center gap-2 text-xs text-slate-600">
                <input
                  type="checkbox"
                  checked={draft.clearApiKey}
                  onChange={(event) => onChange({
                    ...draft,
                    clearApiKey: event.target.checked,
                    apiKey: '',
                  })}
                />
                删除已配置的 API Key
              </label>
            ) : null}
          </section>
        ) : null}
        <Field label="负载均衡">
          <select
            className="select"
            value={draft.loadBalancing}
            onChange={(event) => onChange({
              ...draft,
              loadBalancing: event.target.value as ServiceDraft['loadBalancing'],
            })}
          >
            {serviceLoadBalancingOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </Field>
        <div className="space-y-3">
          <div className="flex items-start justify-between gap-4">
            <strong className="text-xs">服务地址</strong>
            <Button
              variant="soft"
              size="sm"
              onClick={() => onChange({
                ...draft,
                endpoints: [
                  ...draft.endpoints,
                  { address: '', port: 0, weight: 1 },
                ],
              })}
            >
              添加地址
            </Button>
          </div>
          <div className="overflow-x-auto">
            <div className="grid min-w-[480px] gap-2">
              <div className="grid grid-cols-[minmax(0,1fr)_90px_90px_36px] gap-2 px-1 text-[11px] font-medium text-slate-500" aria-hidden="true">
                <span>地址</span>
                <span>端口</span>
                <span>权重</span>
                <span />
              </div>
              {draft.endpoints.map((endpoint, index) => (
                <div key={index} className="grid grid-cols-[minmax(0,1fr)_90px_90px_36px] gap-2">
                  <input
                    className="input font-mono"
                    aria-label={`地址 ${index + 1}`}
                    placeholder="service.example.com"
                    value={endpoint.address}
                    onChange={(event) => updateEndpoint(index, {
                      ...endpoint,
                      address: event.target.value,
                    })}
                  />
                  <input
                    className="input"
                    aria-label={`端口 ${index + 1}`}
                    placeholder={draft.httpsEnabled ? '443' : '80'}
                    type="number"
                    min="1"
                    max="65535"
                    value={endpoint.port || ''}
                    onChange={(event) => updateEndpoint(index, {
                      ...endpoint,
                      port: Number(event.target.value),
                    })}
                  />
                  <input
                    className="input"
                    aria-label={`权重 ${index + 1}`}
                    type="number"
                    min="1"
                    max="1000"
                    value={endpoint.weight}
                    onChange={(event) => updateEndpoint(index, {
                      ...endpoint,
                      weight: Number(event.target.value),
                    })}
                  />
                  <Button
                    variant="ghost"
                    size="sm"
                    aria-label={`删除地址 ${index + 1}`}
                    onClick={() => onChange({
                      ...draft,
                      endpoints: draft.endpoints.filter((_, current) => current !== index),
                    })}
                  >
                    <Trash2 className="h-3.5 w-3.5 text-rose-600" />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        </div>
        <label className="flex items-center gap-2 text-xs">
          <input
            type="checkbox"
            checked={draft.httpsEnabled}
            onChange={(event) => onChange({
              ...draft,
              httpsEnabled: event.target.checked,
            })}
          />
          使用 HTTPS
        </label>
        {draft.httpsEnabled ? (
          <Field label="证书服务名称">
            <input
              className="input"
              value={draft.serverName}
              onChange={(event) => onChange({ ...draft, serverName: event.target.value })}
            />
          </Field>
        ) : null}
        <label className="flex items-center gap-2 text-xs">
          <input
            type="checkbox"
            checked={draft.healthCheckEnabled}
            onChange={(event) => onChange({
              ...draft,
              healthCheckEnabled: event.target.checked,
            })}
          />
          启用主动健康检查
        </label>
        {draft.healthCheckEnabled ? (
          <div className="grid grid-cols-3 gap-3">
            <Field label="检查路径">
              <input
                className="input"
                value={draft.healthCheckPath}
                onChange={(event) => onChange({
                  ...draft,
                  healthCheckPath: event.target.value,
                })}
              />
            </Field>
            <Field label="间隔（秒）">
              <input
                className="input"
                type="number"
                value={draft.healthCheckInterval}
                onChange={(event) => onChange({
                  ...draft,
                  healthCheckInterval: Number(event.target.value),
                })}
              />
            </Field>
            <Field label="超时（秒）">
              <input
                className="input"
                type="number"
                value={draft.healthCheckTimeout}
                onChange={(event) => onChange({
                  ...draft,
                  healthCheckTimeout: Number(event.target.value),
                })}
              />
            </Field>
          </div>
        ) : null}
        <div className="flex justify-end gap-2 border-t border-slate-200 pt-3">
          <Button variant="ghost" onClick={() => onClose()}>取消</Button>
          <Button disabled={busy} onClick={onSave}>
            {busy ? '保存中...' : '保存服务'}
          </Button>
        </div>
      </div>
    </Drawer>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-1">
      <span className="text-xs font-medium text-slate-700">{label}</span>
      {children}
    </label>
  );
}
