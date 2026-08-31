import {
  CheckCircle2,
  CircleGauge,
} from "lucide-react";
import { useState } from "react";
import {
  Drawer,
  FormActions,
  submitForm,
} from "../../components/ui";
import type { Certificate } from "../../data";

export function CreateCertificate({
  initial,
  onClose,
  onSave,
}: {
  initial?: Certificate;
  onClose: () => void;
  onSave: (certificate: Certificate) => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [intendedUsage, setIntendedUsage] = useState<Certificate["usage"]>(
    initial?.usage ?? "服务器证书",
  );
  const [inputMode, setInputMode] = useState<"file" | "paste">("file");
  const [certificatePEM, setCertificatePEM] = useState("");
  const [privateKeyPEM, setPrivateKeyPEM] = useState("");
  const [certificateFileName, setCertificateFileName] = useState("");
  const [privateKeyFileName, setPrivateKeyFileName] = useState("");
  const [parsed, setParsed] = useState(false);
  const requiresPrivateKey = intendedUsage !== "信任证书";
  const validPaste =
    certificatePEM.includes("-----BEGIN CERTIFICATE-----") &&
    certificatePEM.includes("-----END CERTIFICATE-----") &&
    (!requiresPrivateKey ||
      privateKeyPEM.includes("-----BEGIN PRIVATE KEY-----") ||
      privateKeyPEM.includes("-----BEGIN RSA PRIVATE KEY-----"));
  const validFiles =
    Boolean(certificateFileName) &&
    (!requiresPrivateKey || Boolean(privateKeyFileName));
  const canParse = inputMode === "file" ? validFiles : validPaste;
  const identities =
    intendedUsage === "服务器证书"
      ? ["*.new.example.com", "new.example.com"]
      : intendedUsage === "客户端证书"
        ? ["ingate-client.internal"]
        : ["企业内部根 CA"];
  const issuer =
    intendedUsage === "服务器证书" ? "Let's Encrypt R13" : "企业内部根 CA";
  const resetParsing = () => setParsed(false);
  const save = () =>
    onSave({
      id: initial?.id ?? `cert-${Date.now()}`,
      name,
      identities,
      issuer,
      usage: intendedUsage,
      expiresAt: "2027-08-12",
      remainingDays: 365,
      sourceName: inputMode === "file" ? certificateFileName : undefined,
      state: "healthy",
    });
  return (
    <Drawer
      title={initial ? "更新证书" : "导入证书"}
      description={
        initial
          ? "上传新证书内容，资源引用将保持不变"
          : "系统读取用途、标识、签发机构和有效期"
      }
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="form-grid">
          <label>
            <span>证书名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如 example.com 生产证书"
            />
          </label>
          <label>
            <span>用于</span>
            <select
              value={intendedUsage}
              onChange={(event) => {
                setIntendedUsage(event.target.value as Certificate["usage"]);
                resetParsing();
              }}
            >
              <option>服务器证书</option>
              <option>客户端证书</option>
              <option>信任证书</option>
            </select>
            <small>解析时会校验证书是否符合这个用途</small>
          </label>
        </div>
        <div
          className="input-method-selector"
          role="tablist"
          aria-label="证书输入方式"
        >
          <button
            type="button"
            className={inputMode === "file" ? "is-selected" : ""}
            onClick={() => {
              setInputMode("file");
              resetParsing();
            }}
          >
            上传文件
          </button>
          <button
            type="button"
            className={inputMode === "paste" ? "is-selected" : ""}
            onClick={() => {
              setInputMode("paste");
              resetParsing();
            }}
          >
            手动粘贴
          </button>
        </div>
        {inputMode === "file" ? (
          <div className="form-grid certificate-files">
            <label className="field-wide">
              <span>证书链文件</span>
              <input
                required
                type="file"
                accept=".pem,.crt,.cer"
                onChange={(event) => {
                  setCertificateFileName(event.target.files?.[0]?.name ?? "");
                  resetParsing();
                }}
              />
              <small>{certificateFileName || "支持 PEM、CRT 或 CER"}</small>
            </label>
            {requiresPrivateKey ? (
              <label className="field-wide">
                <span>私钥文件</span>
                <input
                  required
                  type="file"
                  accept=".pem,.key"
                  onChange={(event) => {
                    setPrivateKeyFileName(event.target.files?.[0]?.name ?? "");
                    resetParsing();
                  }}
                />
                <small>
                  {privateKeyFileName || "支持 PEM 或 KEY；导入后不再显示内容"}
                </small>
              </label>
            ) : null}
          </div>
        ) : (
          <div className="form-grid">
            <label className="field-wide">
              <span>证书链 PEM</span>
              <textarea
                required
                value={certificatePEM}
                onChange={(event) => {
                  setCertificatePEM(event.target.value);
                  resetParsing();
                }}
                placeholder="-----BEGIN CERTIFICATE-----"
              />
            </label>
            {requiresPrivateKey ? (
              <label className="field-wide">
                <span>私钥 PEM</span>
                <textarea
                  required
                  value={privateKeyPEM}
                  onChange={(event) => {
                    setPrivateKeyPEM(event.target.value);
                    resetParsing();
                  }}
                  placeholder="-----BEGIN PRIVATE KEY-----"
                />
              </label>
            ) : null}
          </div>
        )}
        <button
          className={`connection-test ${parsed ? "is-success" : ""}`}
          type="button"
          disabled={!canParse}
          onClick={() => setParsed(true)}
        >
          {parsed ? <CheckCircle2 /> : <CircleGauge />}
          <div>
            <strong>
              {parsed
                ? requiresPrivateKey
                  ? "证书与私钥匹配"
                  : "信任证书校验通过"
                : "解析并校验证书"}
            </strong>
            <span>
              {parsed
                ? "用途、标识、签发机构和有效期已读取"
                : "保存前检查格式、用途、有效期和私钥匹配关系"}
            </span>
          </div>
        </button>
        {parsed ? (
          <dl className="definition-list certificate-preview">
            <div>
              <dt>用途</dt>
              <dd>{intendedUsage}</dd>
            </div>
            <div>
              <dt>
                {intendedUsage === "服务器证书"
                  ? "覆盖域名"
                  : intendedUsage === "客户端证书"
                    ? "客户端标识"
                    : "证书主体"}
              </dt>
              <dd>
                {identities.map((identity) => (
                  <code key={identity}>{identity}</code>
                ))}
              </dd>
            </div>
            <div>
              <dt>签发机构</dt>
              <dd>{issuer}</dd>
            </div>
            <div>
              <dt>有效期</dt>
              <dd>2026-08-12 至 2027-08-12</dd>
            </div>
            <div>
              <dt>指纹</dt>
              <dd>
                <code>SHA256 · A8:31:7F:••:9C</code>
              </dd>
            </div>
          </dl>
        ) : null}
        <FormActions
          submitLabel={initial ? "保存并替换" : "导入证书"}
          submitDisabled={!parsed || !canParse}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
