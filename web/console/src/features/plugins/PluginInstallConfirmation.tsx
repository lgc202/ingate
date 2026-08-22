import { ExternalLink, PackagePlus } from 'lucide-react';
import { Badge } from '@/components/ui';
import type { PluginCatalogItem, WasmPlugin } from '@/domain/plugin';

export function PluginInstallConfirmation({
  item,
  installed,
}: {
  item: PluginCatalogItem;
  installed?: WasmPlugin;
}) {
  const upgrading = Boolean(installed);
  return (
    <div className="plugin-install-confirmation">
      <section className="plugin-install-hero">
        <div className="plugin-market-icon"><PackagePlus aria-hidden="true" /></div>
        <div>
          <div className="plugin-market-title">
            <h2>{item.name}</h2>
            <Badge tone="accent">{item.category}</Badge>
          </div>
          <p>{item.description}</p>
        </div>
      </section>

      <section className="plugin-install-facts">
        <div><span>发布者</span><strong>{item.provider}</strong></div>
        <div><span>{upgrading ? '目标版本' : '安装版本'}</span><strong>v{item.pluginVersion}</strong></div>
        <div><span>许可证</span><strong>{item.license}</strong></div>
        <div><span>源代码</span><a href={item.sourceURL} target="_blank" rel="noreferrer">查看源码<ExternalLink aria-hidden="true" /></a></div>
      </section>

      {installed ? (
        <div className="plugin-upgrade-summary">
          <span>当前版本</span>
          <strong>v{installed.pluginVersion}</strong>
          <span aria-hidden="true">→</span>
          <span>目标版本</span>
          <strong>v{item.pluginVersion}</strong>
        </div>
      ) : null}
    </div>
  );
}
