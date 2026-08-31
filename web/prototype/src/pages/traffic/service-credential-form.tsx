import {
  CheckCircle2,
  CircleGauge,
  KeyRound,
} from "lucide-react";
import { useState } from "react";
import {
  Drawer,
  FormActions,
  submitForm,
} from "../../components/ui";
import type {
  Certificate,
  Service,
} from "../../data";

export function RotateServiceCredential({
  service,
  certificates,
  onClose,
  onSave,
}: {
  service: Service;
  certificates: Certificate[];
  onClose: () => void;
  onSave: (clientCertificateID?: string) => void;
}) {
  const isMTLS = service.authentication.startsWith("mTLS");
  const isAWS = service.authentication === "AWS 签名";
  const isBasic = service.authentication === "Basic";
  const [identity, setIdentity] = useState("");
  const [secret, setSecret] = useState("");
  const [region, setRegion] = useState("");
  const [clientCertificateID, setClientCertificateID] = useState(
    service.clientCertificateID ?? "",
  );
  const [tested, setTested] = useState(false);
  const clientCertificates = certificates.filter(
    (certificate) =>
      certificate.usage === "客户端证书" && certificate.state !== "error",
  );
  const complete = isMTLS
    ? Boolean(clientCertificateID)
    : isAWS
      ? Boolean(identity && secret && region)
      : isBasic
        ? Boolean(identity && secret)
        : Boolean(secret);
  return (
    <Drawer
      title="更新服务凭据"
      description={`${service.name} · ${service.authentication}`}
      onClose={onClose}
    >
      <form
        onSubmit={(event) =>
          submitForm(event, () =>
            onSave(isMTLS ? clientCertificateID : undefined),
          )
        }
      >
        <div className="form-grid">
          {isMTLS ? (
            <label className="field-wide">
              <span>客户端证书</span>
              <select
                required
                value={clientCertificateID}
                onChange={(event) => {
                  setClientCertificateID(event.target.value);
                  setTested(false);
                }}
              >
                <option value="">选择客户端证书</option>
                {clientCertificates.map((certificate) => (
                  <option key={certificate.id} value={certificate.id}>
                    {certificate.name}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <>
              {isAWS || isBasic ? (
                <label>
                  <span>{isAWS ? "Access Key ID" : "用户名"}</span>
                  <input
                    required
                    value={identity}
                    onChange={(event) => {
                      setIdentity(event.target.value);
                      setTested(false);
                    }}
                  />
                </label>
              ) : null}
              <label className={isAWS || isBasic ? "" : "field-wide"}>
                <span>新凭据</span>
                <input
                  required
                  type="password"
                  value={secret}
                  onChange={(event) => {
                    setSecret(event.target.value);
                    setTested(false);
                  }}
                  placeholder="保存后不再显示明文"
                />
              </label>
              {isAWS ? (
                <label className="field-wide">
                  <span>AWS 区域</span>
                  <input
                    required
                    value={region}
                    onChange={(event) => {
                      setRegion(event.target.value);
                      setTested(false);
                    }}
                  />
                </label>
              ) : null}
            </>
          )}
        </div>
        <button
          className={`connection-test ${tested ? "is-success" : ""}`}
          type="button"
          disabled={!complete}
          onClick={() => setTested(true)}
        >
          {tested ? <CheckCircle2 /> : <CircleGauge />}
          <div>
            <strong>{tested ? "新凭据验证通过" : "验证新凭据"}</strong>
            <span>新凭据验证并保存前继续使用旧凭据</span>
          </div>
        </button>
        <div className="form-note">
          <KeyRound />
          保存后立即切换凭据；敏感值仅提交一次，且不写入操作日志。
        </div>
        <FormActions
          submitLabel="保存并切换"
          submitDisabled={!complete || !tested}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
