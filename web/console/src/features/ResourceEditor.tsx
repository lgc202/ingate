import { useId, useState, type FormEvent } from 'react';
import { Drawer, Field, Section } from '../components/ui';
import { saveResource } from '../api/resources';
import { errorMessage } from '../api/client';
import {
  labels,
  newSpec,
  resourceSpec,
  type AnyResource,
  type Kind,
  type Gateway,
  type Backend,
  type AIBackend,
  type HTTPRoute,
  type ModelRoute,
  type MCPRoute,
  type Spec,
} from '../domain/resources';
import { GatewayFields } from './forms/GatewayFields';
import { AIBackendFields, BackendFields } from './forms/ServiceFields';
import { HTTPRouteFields } from './forms/HTTPRouteFields';
import { ModelRouteFields } from './forms/ModelRouteFields';
import { MCPRouteFields } from './forms/MCPRouteFields';

export function ResourceEditor({
  namespace,
  kind,
  observed,
  onClose,
  onSaved,
}: {
  namespace: string;
  kind: Kind;
  observed?: AnyResource;
  onClose: () => void;
  onSaved: (resource: AnyResource) => void;
}) {
  const [draft, setDraft] = useState(() =>
    observed ? resourceSpec(kind, observed) : newSpec(kind),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const formID = useId();
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      const resource = await saveResource(namespace, kind, draft, observed);
      onSaved(resource);
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  };
  const close = () => {
    if (!busy) onClose();
  };
  let fields;
  switch (kind) {
    case 'gateways':
      fields = <GatewayFields value={draft as Spec<Gateway>} onChange={setDraft} />;
      break;
    case 'backends':
      fields = <BackendFields value={draft as Spec<Backend>} onChange={setDraft} />;
      break;
    case 'ai-backends':
      fields = (
        <AIBackendFields
          namespace={namespace}
          value={draft as Spec<AIBackend>}
          onChange={setDraft}
        />
      );
      break;
    case 'http-routes':
      fields = (
        <HTTPRouteFields
          namespace={namespace}
          value={draft as Spec<HTTPRoute>}
          onChange={setDraft}
        />
      );
      break;
    case 'model-routes':
      fields = (
        <ModelRouteFields
          namespace={namespace}
          value={draft as Spec<ModelRoute>}
          onChange={setDraft}
        />
      );
      break;
    case 'mcp-routes':
      fields = (
        <MCPRouteFields namespace={namespace} value={draft as Spec<MCPRoute>} onChange={setDraft} />
      );
      break;
  }

  return (
    <Drawer
      namespace={namespace}
      title={(observed ? '编辑' : '创建') + labels[kind]}
      onClose={close}
      footer={
        <>
          <button className="button" disabled={busy} onClick={close}>
            取消
          </button>
          <button className="button primary" form={formID} disabled={busy} type="submit">
            {busy ? '保存中…' : observed ? '保存修改' : '创建' + labels[kind]}
          </button>
        </>
      }
    >
      <form id={formID} onSubmit={(event) => void submit(event)}>
        <fieldset disabled={busy} className="editor-fields">
          {error && (
            <div role="alert" className="notice error">
              {error}
            </div>
          )}
          <Section title="基本信息">
            <Field label="名称">
              <input
                aria-label="资源名称"
                value={draft.displayName}
                onChange={(event) => setDraft({ ...draft, displayName: event.target.value })}
                placeholder="便于辨认的展示名称"
              />
            </Field>
          </Section>
          {fields}
        </fieldset>
      </form>
    </Drawer>
  );
}
