import {
  Plus,
  ShieldCheck,
} from "lucide-react";
import { useState } from "react";
import {
  DeleteConfirm,
  Drawer,
  EmptyState,
  FormActions,
  PageHeader,
  PrimaryButton,
  RouteTypeBadge,
  RowActions,
  SearchField,
  StatusBadge,
  submitForm,
} from "../../components/ui";
import type {
  GatewayRoute,
  IdentitySource,
} from "../../data";
import { usePrototype } from "../../prototype-context";

export function IdentitySourcePage() {
  const {
    identitySources,
    routes,
    addIdentitySource,
    updateIdentitySource,
    deleteIdentitySource,
  } = usePrototype();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<IdentitySource | null>(null);
  const [editing, setEditing] = useState<IdentitySource | null>(null);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<IdentitySource | null>(null);
  const visible = identitySources.filter((source) =>
    `${source.name}${source.provider}${source.discoveryURL}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  return (
    <div className="page-stack page-enter">
      <PageHeader
        eyebrow="访问治理"
        title="身份源"
        description="统一管理路由信任的企业身份系统"
        actions={
          <PrimaryButton onClick={() => setCreating(true)}>
            <Plus />
            添加身份源
          </PrimaryButton>
        }
      />
      <section className="identity-source-intro">
        <span><ShieldCheck /></span>
        <div>
          <strong>路由选择身份源，身份源负责验证 Token 和登录会话</strong>
          <p>API Key 由调用方管理；JWT 访问令牌与浏览器登录才需要身份源。</p>
        </div>
      </section>
      <section className="card table-card">
        <header className="table-toolbar">
          <SearchField value={query} onChange={setQuery} placeholder="搜索名称、厂商或发现地址" />
          <span>{visible.length} 个身份源</span>
        </header>
        <div className="table-head identity-source-columns">
          <span>身份源</span><span>发现地址</span><span>Audience</span><span>引用路由</span><span>状态</span><span>操作</span>
        </div>
        {visible.map((source) => {
          const references = routes.filter((route) => route.identitySourceID === source.id);
          return (
            <div className="table-row identity-source-columns" key={source.id}>
              <div className="name-cell"><span><ShieldCheck /></span><div><strong>{source.name}</strong><small>{source.provider}</small></div></div>
              <code>{source.discoveryURL}</code>
              <span>{source.audiences.join("、")}</span>
              <strong>{references.length} 条</strong>
              <StatusBadge state={source.state} label={source.state === "healthy" ? "连接正常" : "需要检查"} />
              <RowActions onDetail={() => setSelected(source)} onEdit={() => setEditing(source)} onDelete={() => setDeleting(source)} />
            </div>
          );
        })}
      </section>
      {selected ? <IdentitySourceDetail source={selected} routes={routes.filter((route) => route.identitySourceID === selected.id)} onClose={() => setSelected(null)} /> : null}
      {creating || editing ? (
        <IdentitySourceForm
          initial={editing ?? undefined}
          onClose={() => { setCreating(false); setEditing(null); }}
          onSave={(source) => {
            if (editing) updateIdentitySource(source);
            else addIdentitySource(source);
            setCreating(false);
            setEditing(null);
            setSelected(source);
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteConfirm
          resourceType="身份源"
          resourceName={deleting.name}
          blockedReason={routes.some((route) => route.identitySourceID === deleting.id) ? "仍有路由引用该身份源，请先调整路由访问控制。" : undefined}
          onCancel={() => setDeleting(null)}
          onConfirm={() => { deleteIdentitySource(deleting.id); setDeleting(null); }}
        />
      ) : null}
    </div>
  );
}

export function IdentitySourceDetail({ source, routes, onClose }: { source: IdentitySource; routes: GatewayRoute[]; onClose: () => void }) {
  return (
    <Drawer title={source.name} description={source.provider} onClose={onClose} width="wide">
      <section className="detail-section">
        <header><h3>信任配置</h3><StatusBadge state={source.state} label="连接正常" /></header>
        <dl className="definition-list">
          <div><dt>OIDC Discovery</dt><dd><code>{source.discoveryURL}</code></dd></div>
          <div><dt>允许的 Audience</dt><dd>{source.audiences.join("、")}</dd></div>
          <div><dt>最近验证</dt><dd>{source.lastVerified}</dd></div>
        </dl>
      </section>
      <section className="detail-section">
        <header><h3>引用路由</h3><span>{routes.length} 条</span></header>
        {routes.length ? <div className="chip-list">{routes.map((route) => <span key={route.id}><RouteTypeBadge type={route.type} />{route.name} · {route.accessMode}</span>)}</div> : <EmptyState title="尚未被路由引用" description="创建或编辑路由时可选择该身份源。" />}
      </section>
    </Drawer>
  );
}

export function IdentitySourceForm({ initial, onClose, onSave }: { initial?: IdentitySource; onClose: () => void; onSave: (source: IdentitySource) => void }) {
  const [name, setName] = useState(initial?.name ?? "");
  const [provider, setProvider] = useState(initial?.provider ?? "Keycloak");
  const [discoveryURL, setDiscoveryURL] = useState(initial?.discoveryURL ?? "");
  const [audience, setAudience] = useState(initial?.audiences.join(", ") ?? "");
  return (
    <Drawer title={initial ? "编辑身份源" : "添加身份源"} description="OIDC / JWT 信任配置" onClose={onClose} width="wide">
      <form onSubmit={(event) => submitForm(event, () => onSave({ id: initial?.id ?? `idp-${Date.now()}`, name, provider, discoveryURL, audiences: audience.split(",").map((item) => item.trim()).filter(Boolean), state: "healthy", lastVerified: "刚刚" }))}>
        <div className="form-grid">
          <label><span>名称</span><input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如企业统一身份" /></label>
          <label><span>身份系统</span><input required value={provider} onChange={(event) => setProvider(event.target.value)} placeholder="Keycloak、Entra ID、Auth0" /></label>
          <label className="field-wide"><span>OIDC Discovery 地址</span><input required type="url" value={discoveryURL} onChange={(event) => setDiscoveryURL(event.target.value)} placeholder="https://.../.well-known/openid-configuration" /></label>
          <label className="field-wide"><span>允许的 Audience</span><input required value={audience} onChange={(event) => setAudience(event.target.value)} placeholder="多个值使用逗号分隔" /></label>
        </div>
        <div className="form-note"><ShieldCheck />保存时验证发现文档、JWKS 地址及 Audience 配置；客户端密钥等敏感信息不在此页展示。</div>
        <FormActions submitLabel="验证并保存" onCancel={onClose} />
      </form>
    </Drawer>
  );
}
