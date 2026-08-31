import {
  CheckCircle2,
  CircleGauge,
} from "lucide-react";
import {
  CompactTagList,
  SearchField,
} from "../../components/ui";
import type { Service } from "../../data";
import type { ServiceFormState } from "./use-service-form";

export function ServiceCapabilitiesSection({ form }: { form: ServiceFormState }) {
  const {
    type,
    transportSecurity,
    setTransportSecurity,
    serverName,
    setServerName,
    trustMode,
    setTrustMode,
    setTrustCertificateID,
    endpoints,
    healthMode,
    setHealthMode,
    healthPath,
    setHealthPath,
    healthInterval,
    setHealthInterval,
    modelPrices,
    setModelPrices,
    modelPriceQuery,
    setModelPriceQuery,
    discoveredTools,
    tested,
    setTested,
    trustCertificates,
    effectiveTrustCertificateID,
    capabilities,
    visiblePriceModels,
    testConnection,
    canTest,
  } = form;

  return (
    <>
      <section className="form-section">
        <header>
          <span>3</span>
          <div>
            <strong>传输安全</strong>
            <small>TLS 服务端校验与可选的客户端证书是两件事</small>
          </div>
        </header>
        <div className="form-grid">
          <label>
            <span>连接加密</span>
            <select
              value={transportSecurity}
              onChange={(event) => {
                setTransportSecurity(
                  event.target.value as Service["transportSecurity"],
                );
                setTested(false);
              }}
            >
              <option>明文连接</option>
              <option>TLS</option>
            </select>
          </label>
          {transportSecurity === "TLS" ? (
            <>
              <label>
                <span>服务端名称</span>
                <input
                  value={serverName}
                  onChange={(event) => {
                    setServerName(event.target.value);
                    setTested(false);
                  }}
                  placeholder="默认从端点地址推导"
                />
              </label>
              <label>
                <span>证书信任</span>
                <select
                  value={trustMode}
                  onChange={(event) => {
                    setTrustMode(
                      event.target.value as "系统信任" | "指定信任证书",
                    );
                    setTrustCertificateID("");
                    setTested(false);
                  }}
                >
                  <option>系统信任</option>
                  <option>指定信任证书</option>
                </select>
              </label>
              {trustMode === "指定信任证书" ? (
                <label>
                  <span>信任证书</span>
                  <select
                    required
                    value={effectiveTrustCertificateID}
                    onChange={(event) => {
                      setTrustCertificateID(event.target.value);
                      setTested(false);
                    }}
                  >
                    <option value="">选择 CA 证书</option>
                    {trustCertificates.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}
            </>
          ) : null}
        </div>
      </section>
      <details className="advanced-config">
        <summary>健康检查</summary>
        <div className="form-grid">
          <label>
            <span>检查方式</span>
            <select
              value={healthMode}
              onChange={(event) => setHealthMode(event.target.value)}
            >
              <option>HTTP</option>
              <option>TCP</option>
              <option>被动检查</option>
            </select>
          </label>
          {healthMode !== "被动检查" ? (
            <label>
              <span>检查周期（秒）</span>
              <input
                type="number"
                min="2"
                value={healthInterval}
                onChange={(event) => setHealthInterval(event.target.value)}
              />
            </label>
          ) : null}
          {healthMode === "HTTP" ? (
            <label className="field-wide">
              <span>检查路径</span>
              <input
                value={healthPath}
                onChange={(event) => setHealthPath(event.target.value)}
              />
            </label>
          ) : null}
        </div>
      </details>
      <button
        className={`connection-test ${tested ? "is-success" : ""}`}
        type="button"
        disabled={!canTest}
        onClick={testConnection}
      >
        {tested ? <CheckCircle2 /> : <CircleGauge />}
        <div>
          <strong>
            {tested
              ? "配置校验通过"
              : type === "MCP"
                ? "测试连接并发现工具"
                : type === "MODEL"
                  ? "测试连接并读取模型 ID"
                  : "测试连接"}
          </strong>
          <span>
            {tested
              ? type === "MCP"
                ? `已发现 ${discoveredTools.length} 个工具`
                : type === "MODEL"
                  ? capabilities.length
                    ? `已确认 ${capabilities.length} 个实际模型 ID`
                    : "连接通过，请手动填写实际模型 ID"
                  : `已校验 ${endpoints.length} 个端点、认证和 TLS`
              : "校验端点、认证、TLS 和协议兼容性"}
          </span>
        </div>
      </button>
      {tested && type !== "HTTP" ? (
        <div className="discovery-result">
          <strong>{type === "MODEL" ? "实际模型 ID" : "已发现工具"}</strong>
          <CompactTagList items={capabilities} limit={5} />
        </div>
      ) : null}
      {tested && type === "MODEL" ? (
        <section className="model-price-editor">
          <header>
            <div>
              <strong>模型单价</strong>
              <small>
                仅对返回 Token 用量的调用预估成本
              </small>
            </div>
            <span>人民币 / 百万 Token</span>
          </header>
          <div className="model-price-toolbar">
            <SearchField
              value={modelPriceQuery}
              onChange={setModelPriceQuery}
              placeholder="搜索实际模型"
            />
            <span>
              显示 {visiblePriceModels.length} / {capabilities.length}
            </span>
          </div>
          <div className="model-price-list">
            {visiblePriceModels.map((model) => (
              <div key={model}>
                <code>{model}</code>
                <label>
                  <span>输入</span>
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    value={modelPrices[model]?.input ?? 0}
                    onChange={(event) =>
                      setModelPrices((prices) => ({
                        ...prices,
                        [model]: {
                          input: Number(event.target.value),
                          cachedInput: prices[model]?.cachedInput,
                          output: prices[model]?.output ?? 0,
                        },
                      }))
                    }
                  />
                </label>
                <label>
                  <span>缓存输入</span>
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    value={modelPrices[model]?.cachedInput ?? 0}
                    onChange={(event) =>
                      setModelPrices((prices) => ({
                        ...prices,
                        [model]: {
                          input: prices[model]?.input ?? 0,
                          cachedInput: Number(event.target.value),
                          output: prices[model]?.output ?? 0,
                        },
                      }))
                    }
                  />
                </label>
                <label>
                  <span>输出</span>
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    value={modelPrices[model]?.output ?? 0}
                    onChange={(event) =>
                      setModelPrices((prices) => ({
                        ...prices,
                        [model]: {
                          input: prices[model]?.input ?? 0,
                          cachedInput: prices[model]?.cachedInput,
                          output: Number(event.target.value),
                        },
                      }))
                    }
                  />
                </label>
              </div>
            ))}
          </div>
        </section>
      ) : null}
      {!tested ? (
        <div className="form-note">
          <CircleGauge />
          未验证的服务可保存为“待验证”，但不能被新路由引用。
        </div>
      ) : null}
    </>
  );
}
