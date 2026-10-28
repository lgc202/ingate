import { Plus, Trash2, ArrowUp, ArrowDown } from 'lucide-react';
import { Field, Section } from '../../components/ui';
import { ResourcePicker } from '../../components/ResourcePicker';
import { ParentsFields } from './ParentsFields';
import type { ModelRoute, ModelRule, Spec } from '../../domain/resources';

export function ModelRouteFields({
  namespace,
  value,
  onChange,
}: {
  namespace: string;
  value: Spec<ModelRoute>;
  onChange: (value: Spec<ModelRoute>) => void;
}) {
  const rules = value.rules ?? [];
  const update = (index: number, rule: ModelRule) =>
    onChange({
      ...value,
      rules: rules.map((item, current) => (current === index ? rule : item)),
    });
  const move = (index: number, direction: number) => {
    const next = [...rules];
    [next[index], next[index + direction]] = [next[index + direction], next[index]];
    onChange({ ...value, rules: next });
  };

  return (
    <>
      <ParentsFields
        namespace={namespace}
        parents={value.parentRefs ?? []}
        onChange={(parentRefs) => onChange({ ...value, parentRefs })}
      />
      {rules.map((rule, index) => (
        <Section
          key={index}
          title={`模型规则 ${index + 1}`}
          action={
            <div className="inline-actions">
              <button
                type="button"
                className="icon-button"
                aria-label={`上移规则 ${index + 1}`}
                disabled={index === 0}
                onClick={() => move(index, -1)}
              >
                <ArrowUp />
              </button>
              <button
                type="button"
                className="icon-button"
                aria-label={`下移规则 ${index + 1}`}
                disabled={index === rules.length - 1}
                onClick={() => move(index, 1)}
              >
                <ArrowDown />
              </button>
              <button
                type="button"
                className="icon-button"
                aria-label={`删除模型规则 ${index + 1}`}
                disabled={rules.length <= 1}
                onClick={() =>
                  onChange({
                    ...value,
                    rules: rules.filter((_, current) => current !== index),
                  })
                }
              >
                <Trash2 />
              </button>
            </div>
          }
        >
          <Field label="规则名称（可选）">
            <input
              aria-label={`规则名称 ${index + 1}`}
              value={rule.name ?? ''}
              onChange={(event) => update(index, { ...rule, name: event.target.value })}
            />
          </Field>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={!rule.matches?.length}
              onChange={(event) =>
                update(index, {
                  ...rule,
                  matches: event.target.checked
                    ? []
                    : [
                        {
                          headers: [{ name: 'x-ai-eg-model', type: 'Exact', value: '' }],
                        },
                      ],
                })
              }
            />
            所有模型
          </label>
          {rule.matches?.map((match, matchIndex) => (
            <div className="model-match" key={matchIndex}>
              <Field
                label="客户端模型名"
                help="精确匹配请求正文里的 model，区分大小写且不自动删除空白。空匹配对象表示所有模型。"
              >
                <input
                  aria-label={`客户端模型名 ${index + 1}-${matchIndex + 1}`}
                  required={Boolean(match.headers?.length)}
                  value={match.headers?.[0]?.value ?? ''}
                  placeholder={
                    match.headers?.length ? '如 coding' : '空匹配：所有模型；填写后改为精确匹配'
                  }
                  onChange={(event) =>
                    update(index, {
                      ...rule,
                      matches: rule.matches!.map((item, current) =>
                        current === matchIndex
                          ? {
                              headers: [
                                {
                                  name: 'x-ai-eg-model',
                                  type: 'Exact',
                                  value: event.target.value,
                                },
                              ],
                            }
                          : item,
                      ),
                    })
                  }
                />
              </Field>
              <button
                className="icon-button"
                type="button"
                aria-label={`删除模型匹配 ${index + 1}-${matchIndex + 1}`}
                onClick={() =>
                  update(index, {
                    ...rule,
                    matches: rule.matches!.filter((_, current) => current !== matchIndex),
                  })
                }
              >
                <Trash2 />
              </button>
            </div>
          ))}
          {Boolean(rule.matches?.length) && (
            <button
              className="text-button"
              type="button"
              disabled={rule.matches!.length >= 128}
              onClick={() =>
                update(index, {
                  ...rule,
                  matches: [
                    ...rule.matches!,
                    {
                      headers: [{ name: 'x-ai-eg-model', type: 'Exact', value: '' }],
                    },
                  ],
                })
              }
            >
              添加模型匹配
            </button>
          )}
          <Field label="模型服务">
            <ResourcePicker
              namespace={namespace}
              kind="ai-backends"
              label={`模型服务 ${index + 1}`}
              value={rule.backendRefs?.[0]?.name ?? ''}
              onChange={(name) =>
                update(index, {
                  ...rule,
                  backendRefs: [{ ...rule.backendRefs?.[0], name }],
                })
              }
            />
          </Field>
          <Field
            label="上游模型名"
            help="将客户端模型名改写成上游接收的名称，例如 coding → code-v2。留空保留客户端模型名。"
          >
            <input
              aria-label={`上游模型名 ${index + 1}`}
              value={rule.backendRefs?.[0]?.modelNameOverride ?? ''}
              placeholder="留空保留客户端模型名"
              onChange={(event) =>
                update(index, {
                  ...rule,
                  backendRefs: [
                    {
                      name: rule.backendRefs?.[0]?.name ?? '',
                      modelNameOverride: event.target.value,
                    },
                  ],
                })
              }
            />
          </Field>
        </Section>
      ))}
      <button
        type="button"
        className="button add-section"
        disabled={rules.length >= 15}
        onClick={() =>
          onChange({
            ...value,
            rules: [
              ...rules,
              {
                matches: [
                  {
                    headers: [{ name: 'x-ai-eg-model', type: 'Exact', value: '' }],
                  },
                ],
                backendRefs: [{ name: '' }],
              },
            ],
          })
        }
      >
        <Plus />
        添加模型规则
      </button>
    </>
  );
}
