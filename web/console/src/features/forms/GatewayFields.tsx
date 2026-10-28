import { Plus, Trash2 } from 'lucide-react';
import { Field, Section } from '../../components/ui';
import type { Gateway, Listener, Spec } from '../../domain/resources';

export function GatewayFields({
  value,
  onChange,
}: {
  value: Spec<Gateway>;
  onChange: (value: Spec<Gateway>) => void;
}) {
  const listeners = value.listeners ?? [];
  const update = (index: number, changes: Partial<Listener>) =>
    onChange({
      ...value,
      listeners: listeners.map((item, current) =>
        current === index ? { ...item, ...changes } : item,
      ),
    });

  return (
    <Section
      title="监听入口"
      action={
        <button
          className="button"
          type="button"
          onClick={() =>
            onChange({
              ...value,
              listeners: [
                ...listeners,
                {
                  name: '',
                  protocol: 'LISTENER_PROTOCOL_HTTP',
                  port: 8080,
                  hostname: '',
                },
              ],
            })
          }
        >
          <Plus />
          添加入口
        </button>
      }
    >
      {listeners.map((listener, index) => (
        <section className="listener" key={index}>
          <header>
            <span>入口 {index + 1}</span>
            <button
              className="icon-button"
              type="button"
              aria-label={`删除入口 ${index + 1}`}
              disabled={listeners.length <= 1}
              onClick={() =>
                onChange({
                  ...value,
                  listeners: listeners.filter((_, current) => current !== index),
                })
              }
            >
              <Trash2 />
            </button>
          </header>
          <div className="listener-grid">
            <Field
              label="入口名称"
              help="同一网关内必须唯一。路由通过这个名称选择入口；修改名称不会自动修改已有路由引用。"
            >
              <input
                aria-label={`入口名称 ${index + 1}`}
                required
                maxLength={63}
                value={listener.name}
                onChange={(event) => update(index, { name: event.target.value })}
              />
            </Field>
            <Field label="协议">
              <select
                aria-label={`协议 ${index + 1}`}
                value={listener.protocol}
                onChange={() => {}}
              >
                <option value="LISTENER_PROTOCOL_HTTP">HTTP</option>
              </select>
            </Field>
            <Field
              label="端口"
              help="客户端连接的数据面监听端口。不同精确主机名可以共用端口；同端口的空主机名与其他入口冲突。"
            >
              <input
                aria-label={`监听端口 ${index + 1}`}
                required
                type="number"
                min={1}
                max={65535}
                value={listener.port || ''}
                onChange={(event) => update(index, { port: Number(event.target.value) })}
              />
            </Field>
          </div>
          <Field
            label="主机名"
            help="每个入口填一个精确主机名；留空匹配全部主机名。多个主机名请添加多个入口，可使用相同端口。当前不支持通配符或逗号分隔。"
          >
            <input
              aria-label={`主机名 ${index + 1}`}
              maxLength={253}
              placeholder="api.example.com；留空匹配全部主机名"
              value={listener.hostname ?? ''}
              onChange={(event) => update(index, { hostname: event.target.value })}
            />
          </Field>
        </section>
      ))}
    </Section>
  );
}
