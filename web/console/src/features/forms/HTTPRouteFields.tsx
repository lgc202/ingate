import { Plus, Trash2, ArrowUp, ArrowDown } from 'lucide-react';
import { Field, Section } from '../../components/ui';
import { ResourcePicker } from '../../components/ResourcePicker';
import { ParentsFields } from './ParentsFields';
import type { HTTPRoute, HTTPRule, HTTPMatch, Spec } from '../../domain/resources';

export function HTTPRouteFields({
  namespace,
  value,
  onChange,
}: {
  namespace: string;
  value: Spec<HTTPRoute>;
  onChange: (value: Spec<HTTPRoute>) => void;
}) {
  const rules = value.rules ?? [];
  const update = (index: number, rule: HTTPRule) =>
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
          title={`规则 ${index + 1}`}
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
                aria-label={`删除规则 ${index + 1}`}
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
          <div className="subheading">
            <strong>请求匹配</strong>
            <button
              type="button"
              className="text-button"
              onClick={() =>
                update(index, {
                  ...rule,
                  matches: [
                    ...(rule.matches ?? []),
                    { path: { type: 'PATH_MATCH_TYPE_PREFIX', value: '/' } },
                  ],
                })
              }
            >
              添加条件
            </button>
          </div>
          {!rule.matches?.length && <p className="muted">匹配所有路径（PathPrefix /）</p>}
          {rule.matches?.map((match, matchIndex) => (
            <div className="match-row" key={matchIndex}>
              <Field label="路径匹配">
                <select
                  aria-label={`路径匹配 ${index + 1}-${matchIndex + 1}`}
                  value={
                    match.path?.type === 'PATH_MATCH_TYPE_EXACT'
                      ? 'PATH_MATCH_TYPE_EXACT'
                      : 'PATH_MATCH_TYPE_PREFIX'
                  }
                  onChange={(event) => {
                    const next: HTTPMatch = {
                      ...match,
                      path: {
                        value: match.path?.value ?? '/',
                        type: event.target.value as
                          | 'PATH_MATCH_TYPE_EXACT'
                          | 'PATH_MATCH_TYPE_PREFIX',
                      },
                    };
                    update(index, {
                      ...rule,
                      matches: rule.matches!.map((item, current) =>
                        current === matchIndex ? next : item,
                      ),
                    });
                  }}
                >
                  <option value="PATH_MATCH_TYPE_PREFIX">前缀匹配</option>
                  <option value="PATH_MATCH_TYPE_EXACT">精确匹配</option>
                </select>
              </Field>
              <Field label="请求路径">
                <input
                  aria-label={`请求路径 ${index + 1}-${matchIndex + 1}`}
                  required
                  value={match.path?.value ?? '/'}
                  onChange={(event) =>
                    update(index, {
                      ...rule,
                      matches: rule.matches!.map((item, current) =>
                        current === matchIndex
                          ? {
                              ...match,
                              path: {
                                type: match.path?.type ?? 'PATH_MATCH_TYPE_PREFIX',
                                value: event.target.value,
                              },
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
                aria-label={`删除匹配 ${index + 1}-${matchIndex + 1}`}
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
          <Field label="目标服务">
            <ResourcePicker
              namespace={namespace}
              kind="backends"
              label={`目标服务 ${index + 1}`}
              value={rule.backendRefs?.[0]?.name ?? ''}
              onChange={(name) => update(index, { ...rule, backendRefs: [{ name }] })}
            />
          </Field>
        </Section>
      ))}
      <button
        className="button add-section"
        type="button"
        onClick={() =>
          onChange({
            ...value,
            rules: [
              ...rules,
              {
                matches: [{ path: { type: 'PATH_MATCH_TYPE_PREFIX', value: '/' } }],
                backendRefs: [{ name: '' }],
              },
            ],
          })
        }
      >
        <Plus />
        添加规则
      </button>
    </>
  );
}
