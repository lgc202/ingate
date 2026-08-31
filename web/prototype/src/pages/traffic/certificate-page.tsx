import {
  CheckCircle2,
  Clock3,
  KeyRound,
  LockKeyhole,
  Network,
  Plus,
} from "lucide-react";
import { useState } from "react";
import {
  CompactTagList,
  ConfigBadge,
  DeleteConfirm,
  Drawer,
  EmptyState,
  PageHeader,
  PrimaryButton,
  RowActions,
  SearchField,
  StatusBadge,
  Toast,
} from "../../components/ui";
import type { Certificate } from "../../data";
import { usePrototype } from "../../prototype-context";
import { CreateCertificate } from "./certificate-form";
import { summarizeItems } from "./service-model";

export function CertificatePage() {
  const {
    certificates,
    gateways,
    services,
    addCertificate,
    updateCertificate,
    deleteCertificate,
  } = usePrototype();
  const [selected, setSelected] = useState<
    (typeof certificates)[number] | null
  >(null);
  const [editing, setEditing] = useState<Certificate | null>(null);
  const [deleting, setDeleting] = useState<Certificate | null>(null);
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState("");
  const [toast, setToast] = useState("");
  const visible = certificates.filter((certificate) =>
    `${certificate.name}${certificate.identities.join("")}${certificate.issuer}${certificate.usage}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  const referencesFor = (certificateID: string) => [
    ...gateways
      .filter((gateway) =>
        gateway.listeners.some((listener) =>
          listener.bindings.some(
            (binding) => binding.certificateID === certificateID,
          ),
        ),
      )
      .map((gateway) => ({
        name: gateway.name,
        detail: "网关 HTTPS 域名绑定",
      })),
    ...services
      .filter((service) => service.clientCertificateID === certificateID)
      .map((service) => ({
        name: service.name,
        detail: "上游 mTLS 客户端身份",
      })),
    ...services
      .filter((service) => service.trustCertificateID === certificateID)
      .map((service) => ({
        name: service.name,
        detail: "上游 TLS 服务端信任",
      })),
  ];
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="流量配置"
        title="证书"
        description="网关 HTTPS 与上游 TLS 证书"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            导入证书
          </PrimaryButton>
        }
      />
      <section className="certificate-ribbon">
        <div>
          <LockKeyhole />
          <span>
            <strong>{certificates.length}</strong> 张证书
          </span>
        </div>
        <div>
          <CheckCircle2 />
          <span>
            <strong>
              {certificates.filter((item) => item.state === "healthy").length}
            </strong>{" "}
            张状态正常
          </span>
        </div>
        <div className="warning">
          <Clock3 />
          <span>
            <strong>
              {
                certificates.filter(
                  (item) =>
                    item.remainingDays < 30 && referencesFor(item.id).length,
                ).length
              }
            </strong>{" "}
            张将在 30 天内过期
          </span>
        </div>
      </section>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索证书、标识或签发机构"
          />
          <span>{visible.length} 张证书</span>
        </header>
        <div className="table-head certificate-columns">
          <span>证书</span>
          <span>证书标识</span>
          <span>签发机构</span>
          <span>到期时间</span>
          <span>资源引用</span>
          <span>有效性 / 配置</span>
          <span>操作</span>
        </div>
        {visible.length ? (
          visible.map((certificate) => {
            const references = referencesFor(certificate.id);
            return (
              <div
                key={certificate.id}
                className="table-row certificate-columns"
              >
                <div className="name-cell">
                  <span>
                    <KeyRound />
                  </span>
                  <div>
                    <strong>{certificate.name}</strong>
                    <small>{certificate.usage}</small>
                  </div>
                </div>
                <CompactTagList items={certificate.identities} />
                <span>{certificate.issuer}</span>
                <div>
                  <strong>{certificate.expiresAt}</strong>
                  <small>剩余 {certificate.remainingDays} 天</small>
                </div>
                <CompactTagList
                  items={references.map((reference) => reference.name)}
                  empty="未引用"
                />
                <div className="certificate-state"><StatusBadge state={!references.length ? "disabled" : certificate.state} label={!references.length ? "未引用" : certificate.state === "warning" ? "即将到期" : certificate.state === "error" ? "不可用" : "有效"} /><ConfigBadge state={certificate.configState} /></div>
                <RowActions
                  onDetail={() => setSelected(certificate)}
                  onEdit={() => setEditing(certificate)}
                  onDelete={() => setDeleting(certificate)}
                />
              </div>
            );
          })
        ) : (
          <EmptyState title="没有匹配的证书" description="请调整搜索条件。" />
        )}
      </section>
      {selected ? (
        <Drawer
          title={selected.name}
          description={selected.usage}
          onClose={() => setSelected(null)}
        >
          <div className="detail-hero">
            <span>
              <KeyRound />
            </span>
            <div>
              <div className="certificate-state">
                <StatusBadge
                  state={referencesFor(selected.id).length ? selected.state : "disabled"}
                  label={
                    !referencesFor(selected.id).length
                      ? "未引用"
                      : selected.state === "warning"
                      ? "即将到期"
                      : selected.state === "error"
                        ? "不可用"
                        : "有效"
                  }
                />
                <ConfigBadge state={selected.configState} />
              </div>
              <h3>{summarizeItems(selected.identities, 4)}</h3>
              <p>
                {selected.issuer} · {selected.expiresAt} 到期
              </p>
              <div className="certificate-identities">
                {selected.identities.map((identity) => (
                  <code key={identity}>{identity}</code>
                ))}
              </div>
            </div>
          </div>
          <section className="detail-section">
            <header>
              <h3>引用关系</h3>
            </header>
            {referencesFor(selected.id).length ? (
              referencesFor(selected.id).map((reference) => (
                <div
                  className="detail-line"
                  key={`${reference.name}-${reference.detail}`}
                >
                  <span>
                    <Network />
                  </span>
                  <div>
                    <strong>{reference.name}</strong>
                    <small>{reference.detail}</small>
                  </div>
                  <span>引用中</span>
                </div>
              ))
            ) : (
              <EmptyState
                title="证书未被引用"
                description="当前可以安全删除；需要时也可在网关或服务配置中重新选择。"
              />
            )}
          </section>
          <div className="form-note">
            <LockKeyhole />
            {selected.usage === "信任证书"
              ? "信任证书不包含私钥。"
              : "私钥仅在导入时提交，不再显示且不写入审计详情。"}
          </div>
        </Drawer>
      ) : null}
      {creating ? (
        <CreateCertificate
          onClose={() => setCreating(false)}
          onSave={(certificate) => {
            addCertificate(certificate);
            setCreating(false);
            setToast("证书已导入");
          }}
        />
      ) : null}
      {editing ? (
        <CreateCertificate
          initial={editing}
          onClose={() => setEditing(null)}
          onSave={(certificate) => {
            updateCertificate(certificate);
            setEditing(null);
            setToast("证书已更新");
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="证书"
          resourceName={deleting.name}
          blockedReason={
            referencesFor(deleting.id).length
              ? `该证书仍被 ${referencesFor(deleting.id)
                  .map((reference) => reference.name)
                  .join("、")} 引用。请先解除引用。`
              : undefined
          }
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            deleteCertificate(deleting.id);
            setDeleting(null);
            setToast("证书已删除");
          }}
        />
      ) : null}
      {toast ? <Toast message={toast} onDone={() => setToast("")} /> : null}
    </div>
  );
}
