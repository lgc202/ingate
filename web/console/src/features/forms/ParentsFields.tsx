import { Plus, Trash2 } from 'lucide-react';
import { Field, Section } from '../../components/ui';
import { ListenerPicker, ResourcePicker } from '../../components/ResourcePicker';
import type { ParentRef } from '../../domain/resources';

export function ParentsFields({
  namespace,
  parents,
  onChange,
}: {
  namespace: string;
  parents: ParentRef[];
  onChange: (parents: ParentRef[]) => void;
}) {
  const update = (index: number, parent: ParentRef) =>
    onChange(parents.map((item, current) => (current === index ? parent : item)));

  return (
    <Section
      title="入口配置"
      action={
        <button
          type="button"
          className="button"
          disabled={parents.length >= 16}
          onClick={() => onChange([...parents, { name: '' }])}
        >
          <Plus />
          添加网关
        </button>
      }
    >
      {parents.map((parent, index) => (
        <div className="parent" key={index}>
          <Field label="网关">
            <ResourcePicker
              namespace={namespace}
              kind="gateways"
              label={`入口网关 ${index + 1}`}
              value={parent.name}
              onChange={(name) => update(index, { name, sectionName: '' })}
            />
          </Field>
          <Field
            label="监听入口"
            help="选择具体入口，或选择该网关的全部合格 HTTP 入口。引用不跨 Namespace。"
          >
            <ListenerPicker
              namespace={namespace}
              gatewayName={parent.name}
              value={parent.sectionName ?? ''}
              onChange={(sectionName) => update(index, { ...parent, sectionName })}
            />
          </Field>
          <button
            className="icon-button"
            type="button"
            aria-label={`删除附着入口 ${index + 1}`}
            disabled={parents.length <= 1}
            onClick={() => onChange(parents.filter((_, current) => current !== index))}
          >
            <Trash2 />
          </button>
        </div>
      ))}
    </Section>
  );
}
