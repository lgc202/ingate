import { useEffect, useState } from 'react';
import { getResource } from '../api/resources';
import { title, type Kind } from '../domain/resources';

export function ResourceName({
  namespace,
  kind,
  name,
}: {
  namespace: string;
  kind: Kind;
  name: string;
}) {
  const key = JSON.stringify([namespace, kind, name]);
  const [resolved, setResolved] = useState<{ key: string; label: string }>();

  useEffect(() => {
    setResolved(undefined);
    if (!name) return;
    const controller = new AbortController();
    getResource(namespace, kind, name, controller.signal).then(
      (resource) => {
        if (!controller.signal.aborted) setResolved({ key, label: title(resource) });
      },
      () => {
        /* 不把无法解析的引用伪装成已解析；保留标识供人工辨认。 */
      },
    );

    return () => controller.abort();
  }, [namespace, kind, name, key]);

  return <span title={name}>{resolved?.key === key ? resolved.label : name || '未配置引用'}</span>;
}
