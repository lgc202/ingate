import { Field, Section } from '../../components/ui';
import { ResourcePicker } from '../../components/ResourcePicker';
import { ParentsFields } from './ParentsFields';
import type { MCPRoute, Spec } from '../../domain/resources';

export function MCPRouteFields({
  namespace,
  value,
  onChange,
}: {
  namespace: string;
  value: Spec<MCPRoute>;
  onChange: (value: Spec<MCPRoute>) => void;
}) {
  return (
    <>
      <ParentsFields
        namespace={namespace}
        parents={value.parentRefs ?? []}
        onChange={(parentRefs) => onChange({ ...value, parentRefs })}
      />
      <Section title="请求匹配">
        <Field label="入口路径" help="客户端访问的完整路径，当前采用精确匹配。留空不是有效路径。">
          <input
            aria-label="入口路径"
            required
            value={value.path ?? '/mcp'}
            onChange={(event) => onChange({ ...value, path: event.target.value })}
          />
        </Field>
      </Section>
      <Section title="转发目标">
        <Field label="目标服务">
          <ResourcePicker
            namespace={namespace}
            kind="backends"
            label="MCP 目标服务"
            value={value.backendRefs?.[0]?.name ?? ''}
            onChange={(name) =>
              onChange({
                ...value,
                backendRefs: [{ ...value.backendRefs?.[0], name }],
              })
            }
          />
        </Field>
        <Field
          label="上游 MCP 路径"
          help="发送到 MCP 服务的路径，可以与客户端入口路径不同。尾斜杠保留原样。"
        >
          <input
            aria-label="上游 MCP 路径"
            required
            value={value.backendRefs?.[0]?.path ?? '/mcp'}
            onChange={(event) =>
              onChange({
                ...value,
                backendRefs: [
                  {
                    name: value.backendRefs?.[0]?.name ?? '',
                    path: event.target.value,
                  },
                ],
              })
            }
          />
        </Field>
      </Section>
    </>
  );
}
