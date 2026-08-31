import type { ServiceFormState } from "./use-service-form";

export function ServiceConnectionSection({ form }: { form: ServiceFormState }) {
  const {
    type,
    name,
    setName,
    provider,
    setProvider,
    protocol,
    setProtocol,
    authentication,
    setAuthentication,
    credential,
    setCredential,
    credentialName,
    setCredentialName,
    credentialRegion,
    setCredentialRegion,
    setClientCertificateID,
    models,
    setModels,
    setTested,
    protocolOptions,
    authenticationOptions,
    clientCertificates,
    effectiveClientCertificateID,
    canReuseStoredCredential,
  } = form;

  return (
    <>
      <section className="form-section">
        <header>
          <span>1</span>
          <div>
            <strong>连接信息</strong>
            <small>服务协议和上游身份认证</small>
          </div>
        </header>
        <div className="form-grid">
          <label>
            <span>服务名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <label>
            <span>服务提供方</span>
            <input
              required
              value={provider}
              onChange={(event) => setProvider(event.target.value)}
            />
          </label>
          <label>
            <span>{type === "MCP" ? "传输方式" : "请求协议"}</span>
            <select
              value={protocol}
              onChange={(event) => {
                setProtocol(event.target.value);
                setTested(false);
              }}
            >
              {protocolOptions.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </select>
          </label>
          <label>
            <span>认证方式</span>
            <select
              value={authentication}
              onChange={(event) => {
                setAuthentication(event.target.value);
                setCredential("");
                setCredentialName("");
                setCredentialRegion("");
                setClientCertificateID("");
                setTested(false);
              }}
            >
              {authenticationOptions.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </select>
          </label>
          {authentication === "mTLS" ? (
            <label className="field-wide">
              <span>客户端证书</span>
              <select
                required
                value={effectiveClientCertificateID}
                onChange={(event) => {
                  setClientCertificateID(event.target.value);
                  setTested(false);
                }}
              >
                <option value="">选择客户端证书</option>
                {clientCertificates.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name} · {item.issuer}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {authentication === "Basic" ? (
            <>
              <label>
                <span>用户名</span>
                <input
                  required={!canReuseStoredCredential}
                  value={credentialName}
                  onChange={(event) => {
                    setCredentialName(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的用户名"
                      : ""
                  }
                />
              </label>
              <label>
                <span>密码</span>
                <input
                  required={!canReuseStoredCredential}
                  type="password"
                  value={credential}
                  onChange={(event) => {
                    setCredential(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的密码"
                      : "保存后不再显示"
                  }
                />
              </label>
            </>
          ) : null}
          {authentication === "自定义请求头" ? (
            <>
              <label>
                <span>请求头名称</span>
                <input
                  required={!canReuseStoredCredential}
                  value={credentialName}
                  onChange={(event) => {
                    setCredentialName(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的名称"
                      : "例如 x-api-key"
                  }
                />
              </label>
              <label>
                <span>请求头值</span>
                <input
                  required={!canReuseStoredCredential}
                  type="password"
                  value={credential}
                  onChange={(event) => {
                    setCredential(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的值"
                      : "保存后不再显示"
                  }
                />
              </label>
            </>
          ) : null}
          {authentication === "AWS 签名" ? (
            <>
              <label>
                <span>Access Key ID</span>
                <input
                  required={!canReuseStoredCredential}
                  value={credentialName}
                  onChange={(event) => {
                    setCredentialName(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的 ID"
                      : ""
                  }
                />
              </label>
              <label>
                <span>Secret Access Key</span>
                <input
                  required={!canReuseStoredCredential}
                  type="password"
                  value={credential}
                  onChange={(event) => {
                    setCredential(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的密钥"
                      : ""
                  }
                />
              </label>
              <label className="field-wide">
                <span>AWS 区域</span>
                <input
                  required={!canReuseStoredCredential}
                  value={credentialRegion}
                  onChange={(event) => {
                    setCredentialRegion(event.target.value);
                    setTested(false);
                  }}
                  placeholder={
                    canReuseStoredCredential
                      ? "留空表示继续使用已保存的区域"
                      : "例如 us-east-1"
                  }
                />
              </label>
            </>
          ) : null}
          {authentication === "Bearer Token" ||
          authentication === "API Key" ? (
            <label className="field-wide">
              <span>{authentication}</span>
              <input
                required={!canReuseStoredCredential}
                type="password"
                value={credential}
                onChange={(event) => {
                  setCredential(event.target.value);
                  setTested(false);
                }}
                placeholder={
                  canReuseStoredCredential
                    ? "留空表示继续使用已保存的凭据"
                    : "保存后不再显示明文"
                }
              />
            </label>
          ) : null}
          {type === "MODEL" ? (
            <label className="field-wide">
              <span>实际模型 ID（可选）</span>
              <input
                value={models}
                onChange={(event) => {
                  setModels(event.target.value);
                  setTested(false);
                }}
                placeholder="多个模型用逗号分隔；也可在验证连接时读取"
              />
              <small>
                部分厂商不提供模型列表接口，此时需要手动填写
              </small>
            </label>
          ) : null}
        </div>
      </section>
    </>
  );
}
