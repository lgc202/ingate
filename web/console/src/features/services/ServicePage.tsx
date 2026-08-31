import { useCallback, useState } from 'react';
import { BrainCircuit, Plus, Server } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { deleteService, listServicePage, saveService } from '@/api/services';
import { listRoutes } from '@/api/routes';
import { useCursorResource, useResource } from '@/api/useResource';
import {
  Badge,
  Button,
  Drawer,
  EmptyState,
  Modal,
  PageFrame,
  Panel,
  ResourceFilterField,
  ResourceListFilters,
  ResourcePagination,
  ResourceStatePanel,
  RowActions,
  SearchField,
  Toast,
} from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone, type ResourceState } from '@/domain/common';
import type { Service } from '@/domain/service';
import { modelProtocolLabel, serviceLoadBalancingLabel } from '@/domain/service';
import { ResourceTrafficSignal, useResourceTrafficOverview } from '@/features/traffic/ResourceTrafficSignal';
import { buildServicePayload, createServiceDraft, validateServiceDraft, type ServiceDraft } from './form';
import { ServiceDetail } from './ServiceDetail';
import { ServiceEditor } from './ServiceEditor';

type ServiceTypeFilter = 'all' | ServiceDraft['type'];
type ServiceStateFilter = 'all' | Exclude<ResourceState, 'Disabled'>;

interface ServiceFilters {
  query: string;
  type: ServiceTypeFilter;
  state: ServiceStateFilter;
}

const emptyServiceFilters = (): ServiceFilters => ({ query: '', type: 'all', state: 'all' });

export function ServicePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterDraft, setFilterDraft] = useState<ServiceFilters>(emptyServiceFilters);
  const [filters, setFilters] = useState<ServiceFilters>(emptyServiceFilters);
  const [pageSize, setPageSize] = useState(10);
  const [draft, setDraft] = useState<ServiceDraft>(() => createServiceDraft());
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteCandidate, setDeleteCandidate] = useState<Service | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{
    message: string;
    tone: 'success' | 'error';
  } | null>(null);
  const loadPage = useCallback(
    (cursor: string) => listServicePage({
      limit: pageSize,
      cursor,
      query: filters.query.trim() || undefined,
      type: filters.type === 'all' ? undefined : `SERVICE_TYPE_${filters.type}`,
      state: filters.state === 'all' ? undefined : filters.state.toUpperCase(),
    }),
    [filters, pageSize],
  );
  const resource = useCursorResource(loadPage, {
    autoRefreshWhen: (data) => data.items.some((service) => service.state === 'Pending'),
  });
  const list = resource.data?.items ?? [];
  const detail = list.find((service) => service.id === searchParams.get('detail')) ?? null;
  const routes = useResource(listRoutes, { enabled: Boolean(detail) });
  const trafficOverview = useResourceTrafficOverview(
    'service',
    list.map((service) => service.id),
  );

  if (resource.loading && !resource.data) {
    return (
      <PageFrame title="服务">
        <ResourceStatePanel title="正在加载服务" message="正在读取当前服务配置" />
      </PageFrame>
    );
  }
  if (resource.error || !resource.data) {
    return (
      <PageFrame title="服务">
        <ResourceStatePanel
          title="服务加载失败"
          message={resource.error?.message ?? '请稍后重试'}
        />
      </PageFrame>
    );
  }

  const openEditor = (service?: Service) => {
    setDraft(createServiceDraft(service));
    setEditorOpen(true);
  };

  const referencingRoutes = (serviceID: string) => (
    routes.data?.routes.filter((route) => (
      route.services.some((target) => target.serviceID === serviceID)
      || route.ai?.models.some((model) => (
        model.targets.some((target) => target.serviceID === serviceID)
      ))
    )) ?? []
  );
  const setDetail = (service?: Service) => {
    const next = new URLSearchParams(searchParams);
    if (service) next.set('detail', service.id);
    else next.delete('detail');
    setSearchParams(next);
  };
  const save = async () => {
    const errors = validateServiceDraft(draft);
    if (errors.length > 0) {
      setNotice({ message: errors[0], tone: 'error' });
      return;
    }
    setBusy(true);
    try {
      const saved = await saveService(buildServicePayload(draft));
      await resource.reload();
      setEditorOpen(false);
      setNotice({ message: `服务已保存：${saved.name}`, tone: 'success' });
    } catch (error) {
      setNotice({
        message: error instanceof Error ? error.message : '保存服务失败',
        tone: 'error',
      });
    } finally {
      setBusy(false);
    }
  };
  const remove = async () => {
    if (!deleteCandidate) return;
    setBusy(true);
    try {
      await deleteService(deleteCandidate.id, deleteCandidate.version);
      await resource.reload();
      setNotice({ message: `服务已删除：${deleteCandidate.name}`, tone: 'success' });
      setDeleteCandidate(null);
    } catch (error) {
      setNotice({
        message: error instanceof Error ? error.message : '删除服务失败',
        tone: 'error',
      });
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageFrame
      title="服务"
      actions={(
        <Button onClick={() => openEditor()}>
          <Plus className="w-4 h-4" />
          创建服务
        </Button>
      )}
    >
      <Panel>
        <ResourceListFilters
          summary={serviceFilterSummary(filters)}
          resultLabel={`本页 ${list.length} 个服务`}
          onSearch={() => {
            resource.reset();
            setFilters({ ...filterDraft });
          }}
          onReset={() => {
            const next = emptyServiceFilters();
            setFilterDraft(next);
            setFilters(next);
            resource.reset();
          }}
        >
          <ResourceFilterField label="关键词">
            <SearchField
              value={filterDraft.query}
              onChange={(query) => setFilterDraft((current) => ({ ...current, query }))}
              placeholder="搜索服务、地址或端口"
            />
          </ResourceFilterField>
          <ResourceFilterField label="服务类型">
            <select
              className="select"
              value={filterDraft.type}
              onChange={(event) => setFilterDraft((current) => ({
                ...current,
                type: event.target.value as ServiceTypeFilter,
              }))}
            >
              <option value="all">全部类型</option>
              <option value="HTTP">HTTP 服务</option>
              <option value="MODEL">模型服务</option>
            </select>
          </ResourceFilterField>
          <ResourceFilterField label="生效状态">
            <select
              className="select"
              value={filterDraft.state}
              onChange={(event) => setFilterDraft((current) => ({
                ...current,
                state: event.target.value as ServiceStateFilter,
              }))}
            >
              <option value="all">全部生效状态</option>
              <option value="Ready">已生效</option>
              <option value="Pending">待生效</option>
              <option value="Error">生效失败</option>
            </select>
          </ResourceFilterField>
        </ResourceListFilters>
        {list.length === 0 ? (
          <div className="p-5">
            <EmptyState
              title={hasActiveFilters(filters) ? '没有匹配的服务' : '暂无服务'}
              message={hasActiveFilters(filters)
                ? '请调整搜索条件'
                : '创建服务后即可在路由中选择转发目标'}
            />
          </div>
        ) : (
          <div className="table-scroll resource-table-scroll">
            <table className="table resource-table resource-service-table">
              <thead>
                <tr>
                  <th>服务</th>
                  <th>服务地址</th>
                  <th>连接方式</th>
                  <th>最近 1 小时</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {list.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <div className="resource-table-name">
                        {item.model
                          ? <BrainCircuit className="text-violet-600" />
                          : <Server className="text-blue-600" />}
                        <strong>{item.name}</strong>
                      </div>
                      <div className="table-secondary mt-1">
                        {item.model
                          ? `${modelProtocolLabel(item.model.protocol)}模型服务`
                          : 'HTTP 服务'}
                      </div>
                    </td>
                    <td className="font-mono text-[11px]">
                      {item.endpoints
                        .map((endpoint) => `${endpoint.address}:${endpoint.port}`)
                        .join('、')}
                    </td>
                    <td>
                      <div className="table-primary">{item.tls ? 'HTTPS' : 'HTTP'}</div>
                      <div className="table-secondary">
                        {serviceLoadBalancingLabel(item.loadBalancing)}
                        {item.healthCheck ? ' · 已配置健康检查' : ''}
                      </div>
                    </td>
                    <td>
                      <ResourceTrafficSignal resourceID={item.id} overview={trafficOverview} />
                    </td>
                    <td>
                      <Badge tone={resourceStateTone(item.state)}>
                        {resourceStateLabel(item.state)}
                      </Badge>
                      <div className="table-secondary mt-1">
                        {formatDateTime(item.updatedAt || item.createdAt)}
                      </div>
                    </td>
                    <td>
                      <RowActions
                        onDetail={() => setDetail(item)}
                        onEdit={() => openEditor(item)}
                        onDelete={() => setDeleteCandidate(item)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {list.length > 0 ? (
          <ResourcePagination
            page={resource.page}
            pageSize={pageSize}
            itemCount={list.length}
            hasNext={resource.hasNext}
            onPageChange={(nextPage) => (
              nextPage > resource.page ? resource.next() : resource.previous()
            )}
            onPageSizeChange={(size) => {
              resource.reset();
              setPageSize(size);
            }}
          />
        ) : null}
      </Panel>

      <Drawer
        title="服务详情"
        subtitle={detail?.name}
        isOpen={Boolean(detail)}
        onClose={() => setDetail()}
      >
        {detail ? <ServiceDetail service={detail} routes={referencingRoutes(detail.id)} /> : null}
      </Drawer>

      <ServiceEditor
        draft={draft}
        open={editorOpen}
        busy={busy}
        onChange={setDraft}
        onClose={() => setEditorOpen(false)}
        onSave={() => void save()}
      />

      <Modal
        title="删除服务"
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
      >
        <div className="space-y-5 p-6">
          <p className="text-sm">确定删除服务“{deleteCandidate?.name}”吗？</p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDeleteCandidate(null)}>取消</Button>
            <Button variant="danger" disabled={busy} onClick={remove}>确认删除</Button>
          </div>
        </div>
      </Modal>
      <Toast
        message={notice?.message ?? null}
        tone={notice?.tone}
        onClose={() => setNotice(null)}
      />
    </PageFrame>
  );
}

function hasActiveFilters(filters: ServiceFilters): boolean {
  return Boolean(filters.query.trim())
    || filters.type !== 'all'
    || filters.state !== 'all';
}

function serviceFilterSummary(filters: ServiceFilters): string {
  const conditions = [];
  if (filters.query.trim()) {
    conditions.push(`关键词“${filters.query.trim()}”`);
  }
  if (filters.type !== 'all') {
    conditions.push(`类型：${filters.type === 'MODEL' ? '模型服务' : 'HTTP 服务'}`);
  }
  if (filters.state !== 'all') {
    conditions.push(`生效状态：${resourceStateLabel(filters.state)}`);
  }
  return conditions.join(' · ') || '全部服务';
}
