import { Badge } from '@/components/ui';
import { formatDateTime, resourceStateLabel, resourceStateTone } from '@/domain/common';
import type { Certificate } from '@/domain/certificate';

export interface CertificateReference {
  gatewayID: string;
  gatewayName: string;
  listenerName: string;
}

export function CertificateDetail({ certificate, references }: { certificate: Certificate; references: CertificateReference[] }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{certificate.name}</h3></div>
        <Badge tone={resourceStateTone(certificate.state)}>{resourceStateLabel(certificate.state)}</Badge>
      </section>
      <section className="resource-detail-section">
        <h3>使用位置</h3>
        {references.length > 0 ? <div className="resource-detail-list">{references.map((reference) => <article key={`${reference.gatewayID}:${reference.listenerName}`}><div><strong>{reference.gatewayName}</strong><small>入口：{reference.listenerName}</small></div><Badge tone="accent">HTTPS</Badge></article>)}</div> : <p className="text-xs text-slate-500">当前没有 HTTPS 入口使用此证书</p>}
      </section>
      <section className="resource-detail-section">
        <h3>证书范围</h3>
        <div className="resource-detail-list">
          {certificate.dnsNames.map((dnsName) => <article key={dnsName}><div><strong>{dnsName}</strong><small>HTTPS DNS 域名</small></div><Badge tone="accent">TLS</Badge></article>)}
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>有效期</h3>
        <div className="resource-detail-grid">
          <div><span>开始时间</span><strong>{formatDateTime(certificate.notBefore)}</strong></div>
          <div><span>截止时间</span><strong>{formatDateTime(certificate.notAfter)}</strong></div>
          <div><span>录入时间</span><strong>{formatDateTime(certificate.createdAt)}</strong></div>
          <div><span>更新时间</span><strong>{formatDateTime(certificate.updatedAt || certificate.createdAt)}</strong></div>
        </div>
      </section>
      <section className="resource-detail-section">
        <h3>资源信息</h3>
        <div className="resource-detail-grid">
          <div><span>生效状态</span><strong>{certificate.message || resourceStateLabel(certificate.state)}</strong></div>
        </div>
      </section>
    </div>
  );
}
