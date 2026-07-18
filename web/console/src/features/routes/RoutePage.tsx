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
  const [mode, setMode] = useState<RouteMode>('list');
  const [selectedRouteID, setSelectedRouteID] = useState('');
  const [draft, setDraft] = useState<RouteComposerDraft | null>(null);
  const [notice, setNotice] = useState<RouteNotice | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<RouteResource | null>(null);
  const [disableCandidate, setDisableCandidate] = useState<RouteResource | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [toggling, setToggling] = useState(false);

  if (workspace.loading) {
    return (
      <PageFrame title="路由" subtitle="定义请求匹配、目标服务和转发规则">
        <ResourceStatePanel title="加载路由数据" message="正在读取路由、网关和目标服务。" />
      </PageFrame>
    );
  }

  if (workspace.error || !workspace.data) {
    return (
      <PageFrame title="路由" subtitle="定义请求匹配、目标服务和转发规则">
        <ResourceStatePanel title="路由数据加载失败" message={workspace.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const { routes, gateways, upstreams } = workspace.data;
  const selectedRoute = routes.find((route) => route.id === selectedRouteID) ?? routes[0] ?? null;
  const activeDraft = draft ?? createRouteComposerDraft();
  const validation = validateRouteComposerDraft(activeDraft);

  const openCreate = () => {
    setMode('editor');
    setDraft(createRouteComposerDraft());
    setNotice(null);
  };

  const openEdit = (route: RouteResource) => {
    setSelectedRouteID(route.id);
    setMode('editor');
    setDraft(createRouteComposerDraft(route));
    setNotice(null);
  };

  const closeEditor = () => {
    setMode('list');
    setDraft(null);
    setSubmitting(false);
  };

  const handleSave = async () => {
    if (!validation.valid) {
      setNotice({ message: validation.summary, tone: 'error' });
      return;
    }

    const payload = buildRouteMutationPayload(activeDraft);
    setSubmitting(true);
    try {
      const result = await saveRoute(payload);
      await workspace.reload();
      setSelectedRouteID(result.changeId ?? payload.id ?? '');
      setNotice({ message: result.message, tone: 'success' });
      setMode('list');
      setDraft(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '保存路由失败', tone: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteCandidate) {
      return;
    }

    setDeleting(true);
    try {
      await deleteRoute(deleteCandidate.id);
      await workspace.reload();
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
      await workspace.reload();
      setNotice({ message: `已启用路由：${route.name}`, tone: 'success' });
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '启用路由失败', tone: 'error' });
    } finally {
      setToggling(false);
    }
  };

  const confirmDisable = async () => {
    if (!disableCandidate) {
      return;
    }

    setToggling(true);
    try {
      await setRouteEnabled(disableCandidate.id, false);
      await workspace.reload();
      setNotice({ message: `已停用路由：${disableCandidate.name}`, tone: 'success' });
      setDisableCandidate(null);
    } catch (error) {
      setNotice({ message: error instanceof Error ? error.message : '停用路由失败', tone: 'error' });
    } finally {
      setToggling(false);
    }
  };

  if (mode === 'detail' && selectedRoute) {
    return (
      <>
        <RouteDetail
          route={selectedRoute}
          gateways={gateways}
          upstreams={upstreams}
          policyWorkspace={policyWorkspace.data}
          onPolicyWorkspaceChanged={policyWorkspace.reload}
          onBack={() => setMode('list')}
        />
        <Toast message={notice?.message ?? null} tone={notice?.tone} onClose={() => setNotice(null)} />
      </>
    );
  }

  if (mode === 'editor') {
    return (
      <PageFrame
        title="路由"
        subtitle="一条路由描述从网关到目标服务的完整请求链路"
        actions={<Button variant="soft" disabled={submitting} onClick={closeEditor}>返回列表</Button>}
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
      subtitle="定义请求匹配、目标服务和转发规则"
      actions={<Button variant="primary" onClick={openCreate}>新建路由</Button>}
    >
      <RouteList
        routes={routes}
        gateways={gateways}
        upstreams={upstreams}
        policyWorkspace={policyWorkspace.data}
        toggling={toggling}
        onDetail={(route) => {
          setSelectedRouteID(route.id);
          setMode('detail');
        }}
        onEdit={openEdit}
        onDelete={setDeleteCandidate}
        onToggleEnabled={toggleEnabled}
      />

      {deleteCandidate ? (
        <RouteConfirmDialog
          title="删除路由"
          message={`确定删除 ${deleteCandidate.name}？删除后匹配请求将不再转发到目标服务。`}
          details={[
            { label: '所属网关', value: formatGatewayIDs(deleteCandidate.gatewayIDs, gateways) },
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
