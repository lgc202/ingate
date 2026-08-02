import { useState } from 'react';
import { getPolicyWorkspace } from '@/api/policies';
import { deleteRoute, getRouteWorkspace, saveRoute, setRouteEnabled } from '@/api/routes';
import { useResource } from '@/api/useResource';
import { Button, PageFrame, ResourceStatePanel, Toast } from '@/components/ui';
import type { RouteResource } from '@/domain/route';
import type { RouteComposerDraft } from './composer';
import { buildRouteMutationPayload, createRouteComposerDraft, validateRouteComposerDraft } from './composer';
import { RouteConfirmDialog } from './RouteConfirmDialog';
import { RouteDetail } from './RouteDetail';
import { RouteEditor } from './RouteEditor';
import { RouteList } from './RouteList';
import { formatGatewayIDs, formatHostnames, routeUpstreamSummary } from './routeView';

type RouteMode = 'list' | 'detail' | 'editor';

interface RouteNotice {
  message: string;
  tone: 'success' | 'error';
}

export function RoutePage() {
  const workspace = useResource(getRouteWorkspace);
  const policyWorkspace = useResource(getPolicyWorkspace);

  const [selectedRouteID, setSelectedRouteID] = useState<string>('');
  const [mode, setMode] = useState<RouteMode>('list');
  const [draft, setDraft] = useState<RouteComposerDraft | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [disableCandidate, setDisableCandidate] = useState<RouteResource | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<RouteResource | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [toggling, setToggling] = useState(false);

  const [notice, setNotice] = useState<RouteNotice | null>(null);

  if (workspace.loading && !workspace.data) {
    return (
      <PageFrame title="路由">
        <ResourceStatePanel title="正在加载路由数据..." message="系统正在连接管理 API 获取当前系统的路由定义。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="路由">
        <ResourceStatePanel title="路由数据加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const { routes, gateways, upstreams } = workspace.data;
  const activeRouteID = selectedRouteID || routes[0]?.id || '';
  const activeRoute = routes.find((route) => route.id === activeRouteID) ?? null;

  const openCreate = () => {
    setDraft(createRouteComposerDraft());
    setMode('editor');
  };

  const openEdit = (route: RouteResource) => {
    setSelectedRouteID(route.id);
    setDraft(createRouteComposerDraft(route));
    setMode('editor');
  };

  const closeEditor = () => {
    setDraft(null);
    setMode(activeRoute ? 'detail' : 'list');
  };

  const handleSave = async () => {
    if (!draft || submitting) {
      return;
    }

    const validation = validateRouteComposerDraft(draft);
    if (!validation.valid) {
      setNotice({ message: validation.summary, tone: 'error' });
      return;
    }

    setSubmitting(true);
    try {
      const payload = buildRouteMutationPayload(draft);
      const result = await saveRoute(payload);
      await Promise.all([workspace.reload(), policyWorkspace.reload()]);
      setSelectedRouteID(result.changeId ?? payload.id ?? activeRouteID);
      setNotice({ message: result.message, tone: 'success' });
      closeEditor();
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存路由失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteCandidate || deleting) {
      return;
    }

    setDeleting(true);
    try {
      await deleteRoute(deleteCandidate.id);
      await Promise.all([workspace.reload(), policyWorkspace.reload()]);
      setSelectedRouteID((current) => (
        current === deleteCandidate.id
          ? routes.find((route) => route.id !== deleteCandidate.id)?.id ?? ''
          : current
      ));
      setNotice({ message: `已删除路由：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '删除路由失败', tone: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const toggleEnabled = async (route: RouteResource) => {
    if (route.enabled) {
      setDisableCandidate(route);
      return;
    }

    setToggling(true);
    try {
      await setRouteEnabled(route.id, true);
      await Promise.all([workspace.reload(), policyWorkspace.reload()]);
      setNotice({ message: '路由已启用', tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '启用路由失败', tone: 'error' });
    } finally {
      setToggling(false);
    }
  };

  const confirmDisable = async () => {
    if (!disableCandidate || toggling) {
      return;
    }

    setToggling(true);
    try {
      await setRouteEnabled(disableCandidate.id, false);
      await Promise.all([workspace.reload(), policyWorkspace.reload()]);
      setNotice({ message: '路由已停用', tone: 'success' });
      setDisableCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '停用路由失败', tone: 'error' });
    } finally {
      setToggling(false);
    }
  };

  if (mode === 'editor' && draft) {
    const activeDraft = draft;
    const validation = validateRouteComposerDraft(activeDraft);

    return (
      <PageFrame
        title={activeDraft.id ? `编辑路由：${activeDraft.name}` : '新建路由'}
        subtitle="集中配置域名、匹配条件、高级转发和 AI 模型能力"
      >
        <RouteEditor
          draft={activeDraft}
          validation={validation}
          gateways={gateways}
          upstreams={upstreams}
          submitting={submitting}
          onDraftChange={setDraft}
          onCancel={closeEditor}
          onSave={handleSave}
        />
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="路由"
      subtitle="定义请求匹配、AI 路由分发、目标服务和转发规则"
      actions={<Button variant="primary" onClick={openCreate}>+ 新建路由</Button>}
    >
      <RouteList
        routes={routes}
        gateways={gateways}
        upstreams={upstreams}
        selectedRouteID={activeRouteID}
        policyWorkspace={policyWorkspace.data}
        onSelect={(id) => {
          setSelectedRouteID(id);
          setMode('detail');
        }}
        onCreate={openCreate}
        onEdit={openEdit}
        onRequestDisable={setDisableCandidate}
        onRequestDelete={setDeleteCandidate}
        onToggleEnabled={toggleEnabled}
      />

      {activeRoute && mode === 'detail' && (
        <div className="mt-6">
          <RouteDetail
            route={activeRoute}
            gateways={gateways}
            upstreams={upstreams}
            policyWorkspace={policyWorkspace.data}
            policyError={policyWorkspace.error?.message}
            onBack={() => setMode('list')}
            onPolicyWorkspaceChanged={() => policyWorkspace.reload()}
          />
        </div>
      )}

      {deleteCandidate ? (
        <RouteConfirmDialog
          title="删除路由"
          message={`确定删除 ${deleteCandidate.name}？删除后匹配请求将不再转发到目标服务。`}
          details={[
            { label: '所属 Gateway', value: formatGatewayIDs(deleteCandidate.gatewayIDs, gateways) },
            { label: '匹配域名', value: formatHostnames(deleteCandidate.hostnames) },
            { label: '目标服务', value: routeUpstreamSummary(deleteCandidate, upstreams) },
          ]}
          confirmLabel="确认删除"
          busy={deleting}
          onCancel={() => setDeleteCandidate(null)}
          onConfirm={confirmDelete}
        />
      ) : null}

      {disableCandidate ? (
        <RouteConfirmDialog
          title="停用路由"
          message={`停用 ${disableCandidate.name} 后，命中该路由的请求将不再转发。`}
          details={[
            { label: '匹配域名', value: formatHostnames(disableCandidate.hostnames) },
            { label: '目标服务', value: routeUpstreamSummary(disableCandidate, upstreams) },
          ]}
          confirmLabel="确认停用"
          busy={toggling}
          onCancel={() => setDisableCandidate(null)}
          onConfirm={confirmDisable}
        />
      ) : null}

      <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}
