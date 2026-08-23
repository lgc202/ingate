import { useCallback, useState } from 'react';
import { CircleAlert, Plus } from 'lucide-react';
import {
  createPluginSource,
  deletePluginSource,
  listPluginSourcePage,
  syncPluginSource,
  updatePluginSource,
} from '@/api/plugins';
import { useCursorResource } from '@/api/useResource';
import { Button, Drawer, Modal, ResourcePagination, ResourceStatePanel, Toast } from '@/components/ui';
import type { PluginSource, PluginSourceInput } from '@/domain/plugin';
import {
  emptyPluginSourceFilters,
  PluginSourceDetail,
  PluginSourceEditor,
  PluginSources,
  type PluginSourceFilters,
} from './PluginSourceViews';

const emptyPluginSourceInput = (): PluginSourceInput => ({ name: '', url: '', enabled: true });

// PluginSourceTab 集中管理插件发现来源；插件市场只消费同步结果，不承担来源生命周期操作
export function PluginSourceTab() {
  const [filterDraft, setFilterDraft] = useState<PluginSourceFilters>(emptyPluginSourceFilters);
  const [filters, setFilters] = useState<PluginSourceFilters>(emptyPluginSourceFilters);
  const [pageSize, setPageSize] = useState(10);
  const [busy, setBusy] = useState(false);
  const [syncingSourceID, setSyncingSourceID] = useState('');
  const [editor, setEditor] = useState<{ source?: PluginSource; input: PluginSourceInput } | null>(null);
  const [editorError, setEditorError] = useState('');
  const [detail, setDetail] = useState<PluginSource | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<PluginSource | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [notice, setNotice] = useState<{ message: string; tone: 'success' | 'error' } | null>(null);

  const loadPage = useCallback((cursor: string) => listPluginSourcePage({
    limit: pageSize,
    cursor,
    query: filters.query.trim() || undefined,
    syncState: filters.state === 'all' ? undefined : filters.state,
  }), [filters, pageSize]);
  const sources = useCursorResource(loadPage);
  const currentSources = sources.data?.items ?? [];

  if (sources.loading && !sources.data) {
    return <ResourceStatePanel title="正在加载插件源" message="正在读取插件目录来源与同步状态" />;
  }
  if (sources.error || !sources.data) {
    return <ResourceStatePanel title="插件源加载失败" message={sources.error?.message ?? '请稍后重试'} />;
  }

  const save = async () => {
    if (!editor || busy) return;
    const name = editor.input.name.trim();
    const url = editor.input.url.trim();
    if (!name || !url) {
      setEditorError('请填写插件源名称和目录地址');
      return;
    }
    setEditorError('');
    setBusy(true);
    try {
      const input = { ...editor.input, name, url };
      const saved = editor.source
        ? await updatePluginSource(editor.source, input)
        : await createPluginSource(input);
      await sources.reload();
      setEditor(null);
      setNotice({
        message: saved.syncState === 'PLUGIN_SOURCE_SYNC_STATE_ERROR'
          ? `插件源已保存，但首次同步失败：${saved.name}`
          : `插件源已保存：${saved.name}`,
        tone: saved.syncState === 'PLUGIN_SOURCE_SYNC_STATE_ERROR' ? 'error' : 'success',
      });
    } catch (error) {
      setEditorError(error instanceof Error ? error.message : '保存插件源失败');
    } finally {
      setBusy(false);
    }
  };

  const sync = async (source: PluginSource) => {
    if (syncingSourceID) return;
    setSyncingSourceID(source.id);
    try {
      await syncPluginSource(source.id);
      await sources.reload();
      setNotice({ message: `插件源已同步：${source.name}`, tone: 'success' });
    } catch (error) {
      await sources.reload();
      setNotice({ message: error instanceof Error ? error.message : '同步插件源失败', tone: 'error' });
    } finally {
      setSyncingSourceID('');
    }
  };

  const remove = async () => {
    if (!deleteCandidate || busy) return;
    setBusy(true);
    setDeleteError('');
    try {
      await deletePluginSource(deleteCandidate.id, deleteCandidate.version);
      await sources.reload();
      setDeleteCandidate(null);
      setNotice({ message: `插件源已删除：${deleteCandidate.name}`, tone: 'success' });
    } catch (error) {
      setDeleteError(error instanceof Error ? error.message : '删除插件源失败');
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      <div className="plugin-source-tab-action">
        <Button onClick={() => { setEditorError(''); setEditor({ input: emptyPluginSourceInput() }); }}><Plus className="h-4 w-4" />添加插件源</Button>
      </div>
      <PluginSources
        sources={currentSources}
        resultCount={currentSources.length}
        filters={filterDraft}
        appliedFilters={filters}
        busySourceID={syncingSourceID}
        onFiltersChange={setFilterDraft}
        onSearch={() => { sources.reset(); setFilters({ ...filterDraft }); }}
        onReset={() => { const next = emptyPluginSourceFilters(); setFilterDraft(next); setFilters(next); sources.reset(); }}
        onDetail={setDetail}
        onEdit={(source) => { setEditorError(''); setEditor({ source, input: { name: source.name, url: source.url, enabled: source.enabled } }); }}
        onSync={(source) => void sync(source)}
        onDelete={(source) => { setDeleteError(''); setDeleteCandidate(source); }}
      />
      {currentSources.length > 0 ? <ResourcePagination page={sources.page} pageSize={pageSize} itemCount={currentSources.length} hasNext={sources.hasNext} onPageChange={(nextPage) => nextPage > sources.page ? sources.next() : sources.previous()} onPageSizeChange={(size) => { sources.reset(); setPageSize(size); }} /> : null}

      <Drawer title={editor?.source ? '编辑插件源' : '添加插件源'} subtitle="管理公开插件目录的同步地址" isOpen={Boolean(editor)} onClose={() => { setEditorError(''); setEditor(null); }}>
        {editor ? <><PluginSourceEditor draft={editor.input} error={editorError} onChange={(input) => setEditor({ ...editor, input })} /><div className="plugin-editor-footer"><Button variant="ghost" onClick={() => setEditor(null)}>取消</Button><Button size="lg" disabled={busy} onClick={() => void save()}>{busy ? '保存中...' : '保存插件源'}</Button></div></> : null}
      </Drawer>

      <Drawer title="插件源详情" subtitle={detail?.name} isOpen={Boolean(detail)} onClose={() => setDetail(null)}>{detail ? <PluginSourceDetail source={detail} /> : null}</Drawer>

      <Modal title="删除插件源" isOpen={Boolean(deleteCandidate)} onClose={() => { setDeleteError(''); setDeleteCandidate(null); }}>
        <div className="space-y-5">
          <p className="text-sm leading-6 text-slate-600">删除后将不再从“<strong className="text-slate-900">{deleteCandidate?.name}</strong>”发现或升级插件，已经安装的插件仍会继续运行。</p>
          {deleteError ? <div className="flex items-start gap-2.5 rounded-lg border border-rose-200 bg-rose-50 px-3.5 py-3 text-sm leading-5 text-rose-800" role="alert"><CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" /><span>{deleteError}</span></div> : null}
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4"><Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" disabled={busy} onClick={() => void remove()}>{busy ? '删除中...' : '确认删除'}</Button></div>
        </div>
      </Modal>
    </>
  );
}
