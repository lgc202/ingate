import {
  Globe2,
  KeyRound,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react";
import type { RouteAccessMode } from "../../data";
import type { RouteFormState } from "./use-route-form";

export function RouteAccessSection({ form }: { form: RouteFormState }) {
  const {
    identitySources,
    type,
    accessMode,
    setAccessMode,
    identitySourceID,
    setIdentitySourceID,
  } = form;

  return (
    <section className="form-section route-access-section">
      <header>
        <span>2</span>
        <div>
          <strong>访问控制</strong>
          <small>决定客户端如何证明身份；公开接口无需身份</small>
        </div>
      </header>
      <div className="route-access-options">
        {([
          ["公开访问", "无需凭据", "适合公开 API 与 Webhook"],
          ["API Key", "识别客户端应用", "适合服务、合作方和 AI SDK"],
          ["JWT 访问令牌", "验证 Bearer Token", "适合 SPA、移动端和企业 API"],
          ["浏览器登录", "网关发起 OIDC 登录", "适合需要登录态的 Web 页面"],
          ["客户端证书", "校验 mTLS 证书", "适合高信任服务间调用"],
        ] as Array<[RouteAccessMode, string, string]>).map(
          ([mode, title, description]) => (
            <button
              type="button"
              key={mode}
              className={accessMode === mode ? "is-selected" : ""}
              onClick={() => setAccessMode(mode)}
            >
              <span>{mode === "公开访问" ? <Globe2 /> : mode === "API Key" ? <KeyRound /> : mode === "客户端证书" ? <LockKeyhole /> : <ShieldCheck />}</span>
              <strong>{mode}</strong>
              <small>{title}</small>
              <p>{description}</p>
            </button>
          ),
        )}
      </div>
      {accessMode === "API Key" ? (
        <div className="route-access-config">
          <div>
            <strong>请求凭据</strong>
            <span>{type === "AI" ? "Authorization: Bearer <API_KEY>" : "X-API-Key: <API_KEY>"}</span>
          </div>
          <p>请求通过密钥识别调用方，再按本路由下的接口、模型或工具授权进行校验。</p>
        </div>
      ) : null}
      {accessMode === "JWT 访问令牌" || accessMode === "浏览器登录" ? (
        <div className="form-grid route-identity-config">
          <label className="field-wide">
            <span>身份源</span>
            <select
              value={identitySourceID}
              onChange={(event) => setIdentitySourceID(event.target.value)}
              required
            >
              {identitySources.map((source) => (
                <option key={source.id} value={source.id}>
                  {source.name} · {source.provider}
                </option>
              ))}
            </select>
            <small>
              {accessMode === "JWT 访问令牌"
                ? "验证签名、签发方、有效期和 Audience；授权可继续使用 Scope 或角色"
                : "未登录浏览器将跳转到身份源，登录成功后由网关维护会话"}
            </small>
          </label>
        </div>
      ) : null}
      {accessMode === "客户端证书" ? (
        <div className="route-access-config">
          <div><strong>客户端身份</strong><span>从通过校验的证书 Subject / SAN 提取</span></div>
          <p>证书信任链在网关监听入口配置；本路由可继续按证书身份执行授权。</p>
        </div>
      ) : null}
    </section>
  );
}
