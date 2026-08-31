import {
  Download,
  FileClock,
} from "lucide-react";
import { useState } from "react";
import type { AuditRecord } from "../../data";
import {
  Drawer,
  EmptyState,
  FilterSelect,
  PageHeader,
  SearchField,
  StatusBadge,
} from "../../components/ui";
import { usePrototype } from "../../prototype-context";

export function AuditPage() {
  const { auditRecords } = usePrototype();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<AuditRecord | null>(null);
  const [result, setResult] = useState<"ALL" | AuditRecord["result"]>("ALL");
  const [resourceType, setResourceType] = useState<
    "ALL" | AuditRecord["resourceType"]
  >("ALL");
  const visible = auditRecords.filter(
    (record) =>
      (resourceType === "ALL" || record.resourceType === resourceType) &&
      (result === "ALL" || record.result === result) &&
      `${record.actor}${record.action}${record.resource}${record.detail}`.includes(
        query,
      ),
  );
  const exportRecords = () => {
    const rows = [
      ["记录编号", "时间", "操作者", "动作", "资源", "结果", "详情"],
      ...visible.map((record) => [
        record.id,
        record.time,
        record.actor,
        record.action,
        record.resource,
        record.result,
        record.detail,
      ]),
    ];
    const csv = rows
      .map((row) =>
        row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(","),
      )
      .join("\n");
    const url = URL.createObjectURL(
      new Blob([`\ufeff${csv}`], { type: "text/csv;charset=utf-8" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "ingate-audit-2026-08-12.csv";
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="系统管理"
        title="审计日志"
        description="配置、权限与凭据操作"
        actions={
          <button
            className="button button-secondary"
            type="button"
            onClick={exportRecords}
            disabled={!visible.length}
          >
            <Download />
            导出日志
          </button>
        }
      />
      <section className="card audit-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索操作者、动作或资源"
          />
          <FilterSelect
            label="资源类型"
            value={resourceType}
            onChange={setResourceType}
            options={[
            { value: "ALL", label: "全部", count: auditRecords.length },
              { value: "网关", label: "网关" },
              { value: "路由", label: "路由" },
              { value: "服务", label: "服务" },
              { value: "调用方", label: "调用方" },
              { value: "流量策略", label: "策略" },
            ]}
          />
          <FilterSelect
            label="操作结果"
            value={result}
            onChange={setResult}
            options={[
              { value: "ALL", label: "全部结果" },
              { value: "成功", label: "成功" },
              { value: "失败", label: "失败" },
            ]}
          />
        </header>
        <div className="audit-date">
          <span>当前记录 · {visible.length} 项</span>
          <i />
        </div>
        {visible.length ? (
          visible.map((record, index) => (
            <button
              className="audit-row"
              type="button"
              onClick={() => setSelected(record)}
              key={record.id}
            >
              <span>
                <FileClock />
              </span>
              <div>
                <div>
                  <strong>{record.action}</strong>
                  <StatusBadge
                    state={record.result === "成功" ? "healthy" : "error"}
                    label={record.result}
                  />
                </div>
                <p>{record.detail}</p>
                <small>
                  {record.resourceType} · {record.resource}
                </small>
              </div>
              <div>
                <strong>{record.actor}</strong>
                <time>{record.time}</time>
              </div>
              {index < visible.length - 1 ? <i /> : null}
            </button>
          ))
        ) : (
          <EmptyState
            title="没有匹配的审计记录"
            description="请调整搜索或资源类型。"
          />
        )}
      </section>
      {selected ? (
        <Drawer
          title={selected.action}
          description={`${selected.actor} · ${selected.time}`}
          onClose={() => setSelected(null)}
        >
          <div className="detail-hero">
            <span><FileClock /></span>
            <div>
              <StatusBadge state={selected.result === "成功" ? "healthy" : "error"} label={selected.result} />
              <h3>{selected.resource}</h3>
              <p>{selected.resourceType} · 记录编号 {selected.id}</p>
            </div>
          </div>
          <section className="detail-section">
            <header><h3>变更摘要</h3></header>
            <div className="detail-line">
              <span><FileClock /></span>
              <div><strong>{selected.detail}</strong><small>敏感凭据不会写入审计内容</small></div>
              <span>{selected.time}</span>
            </div>
          </section>
        </Drawer>
      ) : null}
    </div>
  );
}
