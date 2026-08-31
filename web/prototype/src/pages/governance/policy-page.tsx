import {
  Plus,
  ShieldCheck,
} from "lucide-react";
import { useState } from "react";
import {
  CompactTagList,
  ConfigBadge,
  DeleteConfirm,
  Drawer,
  EmptyState,
  FilterSelect,
  PageHeader,
  PrimaryButton,
  RowActions,
  SearchField,
  Toast,
} from "../../components/ui";
import type { Policy } from "../../data";
import { usePrototype } from "../../prototype-context";
import { CreatePolicy } from "./policy-form";
import {
  policyGroup,
  type PolicyGroup,
} from "./policy-model";

export function PolicyPage() {
  const { policies, routes, gateways, addPolicy, updatePolicy, deletePolicy } =
    usePrototype();
  const [filter, setFilter] = useState<PolicyGroup>("ALL");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Policy | null>(null);
  const [editing, setEditing] = useState<Policy | null>(null);
  const [deleting, setDeleting] = useState<Policy | null>(null);
  const [creating, setCreating] = useState(false);
  const [toast, setToast] = useState("");
  const visible = policies.filter(
    (policy) =>
      (filter === "ALL" || policyGroup(policy.type) === filter) &&
      `${policy.name}${policy.type}${policy.targets.map((target) => target.name).join("")}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const groups: Array<{ value: PolicyGroup; label: string; count?: number }> = [
    { value: "ALL", label: "全部", count: policies.length },
    ...(["访问控制", "流量控制"] as const).map((value) => ({
      value,
      label: value,
      count: policies.filter((policy) => policyGroup(policy.type) === value)
        .length,
    })),
  ];
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="访问治理"
        title="流量策略"
        description="可复用于网关或路由的访问与流量规则"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            创建流量策略
          </PrimaryButton>
        }
      />
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索策略或生效范围"
          />
          <FilterSelect
            label="策略分类"
            value={filter}
            onChange={setFilter}
            options={groups}
          />
        </header>
        <div className="table-head policy-columns">
          <span>策略</span>
          <span>分类</span>
          <span>生效范围</span>
          <span>规则</span>
          <span>配置状态</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((policy) => (
            <div key={policy.id} className="table-row policy-columns">
              <div className="name-cell">
                <span>
                  <ShieldCheck />
                </span>
                <div>
                  <strong>{policy.name}</strong>
                  <small>{policy.type}</small>
                </div>
              </div>
              <span>{policyGroup(policy.type)}</span>
              <CompactTagList
                items={policy.targets.map((target) => target.name)}
                empty="尚未应用"
              />
              <span>{policy.rule}</span>
              <ConfigBadge
                state={
                  policy.configState ??
                  (policy.targets.length ? "active" : "not-applied")
                }
              />
              <RowActions
                onDetail={() => setSelected(policy)}
                onEdit={() => setEditing(policy)}
                onDelete={() => setDeleting(policy)}
              />
            </div>
          ))
        ) : (
          <EmptyState title="没有匹配的策略" description="请调整筛选条件。" />
        )}
      </section>
      {selected ? (
        <Drawer
          title={selected.name}
          description={`${policyGroup(selected.type)} · ${selected.type}`}
          onClose={() => setSelected(null)}
        >
          <div className="detail-hero">
            <span>
              <ShieldCheck />
            </span>
            <div>
              <ConfigBadge
                state={
                  selected.configState ??
                  (selected.targets.length ? "active" : "not-applied")
                }
              />
              <h3>{selected.rule}</h3>
              <p>执行结果与拒绝明细可在请求记录中查看</p>
            </div>
          </div>
          <section className="detail-section">
            <header>
              <h3>生效范围</h3>
            </header>
            {selected.targets.length ? (
              selected.targets.map((target) => (
                <div
                  className="detail-line"
                  key={`${target.kind}-${target.id}`}
                >
                  <span>
                    <ShieldCheck />
                  </span>
                  <div>
                    <strong>{target.name}</strong>
                    <small>{target.kind} · 同一策略的各目标独立执行</small>
                  </div>
                  <ConfigBadge
                    state={
                      selected.configState ??
                      (selected.targets.length ? "active" : "not-applied")
                    }
                  />
                </div>
              ))
            ) : (
              <EmptyState
                title="尚未应用"
                description="策略已保存，但当前不影响任何流量。"
              />
            )}
          </section>
        </Drawer>
      ) : null}
      {creating ? (
        <CreatePolicy
          gateways={gateways}
          routes={routes}
          onClose={() => setCreating(false)}
          onSave={(policy) => {
            addPolicy(policy);
            setCreating(false);
            setToast(
              policy.targets.length
                ? "流量策略已保存"
                : "流量策略已保存，尚未应用",
            );
          }}
        />
      ) : null}
      {editing ? (
        <CreatePolicy
          initial={editing}
          gateways={gateways}
          routes={routes}
          onClose={() => setEditing(null)}
          onSave={(policy) => {
            updatePolicy(policy);
            setEditing(null);
            setToast(
              policy.targets.length
                ? "策略修改已保存"
                : "策略修改已保存，当前未应用",
            );
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="流量策略"
          resourceName={deleting.name}
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deletePolicy(deleting.id);
            setDeleting(null);
            setToast("流量策略已删除");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
