import { useMemo, useState } from 'react';
import { Blocks, CircleAlert, Database, Sparkles } from 'lucide-react';
import {
  deleteWasmPlugin,
  getWasmPlugin,
  installWasmPlugin,
  listPluginSources,
  listWasmPluginCatalog,
  listWasmPlugins,
  upgradeWasmPlugin,
} from '@/api/plugins';
import { useResource } from '@/api/useResource';
import { Button, Drawer, Modal, PageFrame, Panel, ResourcePagination, ResourceStatePanel, Toast } from '@/components/ui';
import type { PluginCatalogItem, WasmPlugin } from '@/domain/plugin';
import { PluginInstallConfirmation } from './PluginInstallConfirmation';
import { PluginSourceTab } from './PluginSourcePage';
import { emptyPluginFilters, InstalledPlugins, PluginDetail, PluginMarket, type PluginFilters } from './PluginViews';

type PluginTab = 'market' | 'installed' | 'sources';

export function PluginPage() {
  const plugins = useResource(listWasmPlugins, { autoRefreshWhen: (items) => items.some((item) => item.state === 'Pending') });
  const catalog = useResource(listWasmPluginCatalog);
  const sources = useResource(listPluginSources);
  const [tab, setTab] = useState<PluginTab>('market');
  const [filterDraft, setFilterDraft] = useState<PluginFilters>(emptyPluginFilters);
  const [filters, setFilters] = useState<PluginFilters>(emptyPluginFilters);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [change, setChange] = useState<{ item: PluginCatalogItem; installed?: WasmPlugin } | null>(null);
  const [changeError, setChangeError] = useState('');
  const [detail, setDetail] = useState<WasmPlugin | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<WasmPlugin | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  const visiblePlugins = useMemo(() => {
    const normalized = filters.query.trim().toLowerCase();
    return (plugins.data ?? []).filter((plugin) => (
      (filters.state === 'all' || plugin.state === filters.state)
      && `${plugin.name} ${plugin.sourceName} ${plugin.package} ${plugin.pluginVersion} ${plugin.url}`.toLowerCase().includes(normalized)
    ));
  }, [filters, plugins.data]);

  if ((plugins.loading && !plugins.data) || (catalog.loading && !catalog.data)) {
    return <PageFrame title="插件"><ResourceStatePanel title="正在加载插件" message="正在读取插件目录与安装状态" /></PageFrame>;
  }
  if (plugins.error || !plugins.data || catalog.error || !catalog.data) {
    return <PageFrame title="插件"><ResourceStatePanel title="插件加载失败" message={plugins.error?.message ?? catalog.error?.message ?? '请稍后重试'} /></PageFrame>;
  }

  const pageCount = Math.max(1, Math.ceil(visiblePlugins.length / pageSize));
  const currentPage = Math.min(page, pageCount);
  const pagedPlugins = visiblePlugins.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const loadPlugin = async (plugin: WasmPlugin): Promise<WasmPlugin | null> => {
    try {
      return await getWasmPlugin(plugin.id);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '加载插件详情失败', tone: 'error' });
      return null;
    }
  };

  const openDetail = async (plugin: WasmPlugin) => {
    const current = await loadPlugin(plugin);
    if (current) setDetail(current);
  };

  const openUninstall = async (plugin: WasmPlugin) => {
    setDeleteError('');
    const current = await loadPlugin(plugin);
    if (current) setDeleteCandidate(current);
  };

  const manageInstalledPlugin = () => {
    const nextFilters = emptyPluginFilters();
    setFilterDraft(nextFilters);
    setFilters(nextFilters);
    setPage(1);
    setTab('installed');
  };

  const closeUninstall = () => {
    setDeleteError('');
    setDeleteCandidate(null);
  };

  const save = async () => {
    if (!change || busy) return;
    setChangeError('');
    setBusy(true);
    try {
      const saved = change.installed
        ? await upgradeWasmPlugin(change.installed)
        : await installWasmPlugin(change.item.sourceID, change.item.package);
      await plugins.reload();
      setChange(null);
      setTab('installed');
      setNotice({ message: `${change.installed ? '插件升级已提交' : '插件安装已提交'}：${saved.name}`, tone: 'success' });
    } catch (error) {
      setChangeError(error instanceof Error ? error.message : '保存插件失败');
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!deleteCandidate || busy) return;
    setBusy(true);
    try {
      await deleteWasmPlugin(deleteCandidate.id, deleteCandidate.version);
      await plugins.reload();
      closeUninstall();
      setNotice({ message: `插件已卸载：${deleteCandidate.name}`, tone: 'success' });
    } catch (error) {
      setDeleteError(error instanceof Error ? error.message : '卸载插件失败');
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageFrame title="插件">
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      <Panel>
        <nav className="resource-kind-tabs plugin-tabs" aria-label="插件页面">
          <button type="button" className={tab === 'market' ? 'is-active' : ''} onClick={() => setTab('market')}><Sparkles aria-hidden="true" />插件市场<span>{catalog.data.plugins.length}</span></button>
          <button type="button" className={tab === 'installed' ? 'is-active' : ''} onClick={() => setTab('installed')}><Blocks aria-hidden="true" />已安装<span>{plugins.data.length}</span></button>
          <button type="button" className={tab === 'sources' ? 'is-active' : ''} onClick={() => setTab('sources')}><Database aria-hidden="true" />插件源<span>{sources.data?.length ?? '—'}</span></button>
        </nav>
        {tab === 'market' ? (
          <PluginMarket
            items={catalog.data.plugins}
            installed={plugins.data}
            onInstall={(item) => { setChangeError(''); setChange({ item }); }}
            onManage={manageInstalledPlugin}
            onManageSources={() => setTab('sources')}
          />
        ) : tab === 'installed' ? (
          <InstalledPlugins
            allPlugins={plugins.data}
            plugins={pagedPlugins}
            total={visiblePlugins.length}
            filters={filterDraft}
            appliedFilters={filters}
            onFiltersChange={setFilterDraft}
            onSearch={() => { setPage(1); setFilters({ ...filterDraft }); }}
            onReset={() => { const next = emptyPluginFilters(); setFilterDraft(next); setFilters(next); setPage(1); }}
            catalog={catalog.data.plugins}
            onDetail={(plugin) => void openDetail(plugin)}
            onUpgrade={(plugin, item) => { setChangeError(''); setChange({ item, installed: plugin }); }}
            onDelete={(plugin) => void openUninstall(plugin)}
          />
        ) : <PluginSourceTab sources={sources} />}
        {tab === 'installed' && visiblePlugins.length > 0 ? <ResourcePagination page={currentPage} pageSize={pageSize} total={visiblePlugins.length} onPageChange={setPage} onPageSizeChange={(size) => { setPage(1); setPageSize(size); }} /> : null}
      </Panel>

      <Drawer title={change ? `${change.installed ? '升级' : '安装'}插件` : ''} subtitle={change?.installed ? '升级到当前来源的推荐版本' : `来源：${change?.item.sourceName ?? ''}`} isOpen={Boolean(change)} onClose={() => { setChangeError(''); setChange(null); }}>
        {change ? <><PluginInstallConfirmation item={change.item} installed={change.installed} />{changeError ? <div className="plugin-change-error" role="alert"><CircleAlert aria-hidden="true" /><span>{changeError}</span></div> : null}<div className="plugin-editor-footer"><Button variant="ghost" onClick={() => setChange(null)}>取消</Button><Button size="lg" disabled={busy} onClick={() => void save()}>{busy ? '提交中...' : change.installed ? '确认升级' : '确认安装'}</Button></div></> : null}
      </Drawer>

      <Drawer title="插件详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>{detail ? <PluginDetail plugin={detail} catalogItem={catalog.data.plugins.find((item) => item.sourceID === detail.sourceID && item.package === detail.package)} /> : null}</Drawer>

      <Modal title="卸载插件" isOpen={Boolean(deleteCandidate)} onClose={closeUninstall}>
        <div className="space-y-5">
          <p className="text-sm leading-6 text-slate-600">
            卸载后将无法创建依赖该能力的策略。确定卸载“<strong className="text-slate-900">{deleteCandidate?.name}</strong>”吗？
          </p>
          {deleteCandidate?.usages.length ? (
            <div className="plugin-uninstall-usages" role="alert">
              <CircleAlert aria-hidden="true" />
              <div>
                <strong>当前有 {deleteCandidate.usages.length} 条策略依赖该插件</strong>
                <p>请先删除这些策略，再卸载插件。</p>
                <ul>
                  {deleteCandidate.usages.map((usage) => <li key={`${usage.policyKind}:${usage.policyID}`}>{usage.policyName}</li>)}
                </ul>
              </div>
            </div>
          ) : null}
          {deleteError ? (
            <div className="flex items-start gap-2.5 rounded-lg border border-rose-200 bg-rose-50 px-3.5 py-3 text-sm leading-5 text-rose-800" role="alert">
              <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span>{deleteError}</span>
            </div>
          ) : null}
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
            <Button variant="ghost" onClick={closeUninstall}>取消</Button>
            <Button variant="danger" disabled={busy || Boolean(deleteCandidate?.usages.length)} onClick={() => void remove()}>{busy ? '卸载中...' : '确认卸载'}</Button>
          </div>
        </div>
      </Modal>
    </PageFrame>
  );
}
