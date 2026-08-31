import {
  AlertTriangle,
  Plus,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import { SearchField } from "../../components/ui";
import type { RouteForwarding } from "../../data";
import { newModelMapping } from "./route-model";
import type { RouteFormState } from "./use-route-form";

export function RouteDestinationSection({ form }: { form: RouteFormState }) {
  const {
    type,
    setServiceID,
    setSelectedTools,
    toolQuery,
    setToolQuery,
    secondLineEnabled,
    setSecondLineEnabled,
    setSecondServiceID,
    strategy,
    setStrategy,
    primaryWeight,
    setPrimaryWeight,
    setModelMappings,
    timeout,
    setTimeout,
    retries,
    setRetries,
    pathHandling,
    setPathHandling,
    hostRewrite,
    setHostRewrite,
    customHostname,
    setCustomHostname,
    rewritePath,
    setRewritePath,
    addHeaderName,
    setAddHeaderName,
    addHeaderValue,
    setAddHeaderValue,
    removeHeaders,
    setRemoveHeaders,
    failoverOn,
    setFailoverOn,
    availableServices,
    compatible,
    effectiveServiceID,
    primaryService,
    effectiveTools,
    visibleTools,
    secondCandidates,
    effectiveSecondServiceID,
    normalizedMappings,
    duplicateModelNames,
    updateMapping,
  } = form;

  return (
    <>
      <section className="form-section">
        <header>
          <span>3</span>
          <div>
            <strong>{type === "AI" ? "模型映射" : "目标服务"}</strong>
            <small>
              {type === "AI"
                ? "每个客户端模型名拥有独立的主线路和可选备用线路"
                : type === "MCP"
                  ? "开放已从 MCP 服务发现的工具"
                  : "配置主线路及可选的第二线路"}
            </small>
          </div>
        </header>
        {type === "AI" ? (
          <div className="model-mapping-editor">
            {normalizedMappings.map((mapping, index) => {
              const primary = availableServices.find(
                (service) => service.id === mapping.primaryServiceID,
              );
              const backupCandidates = availableServices.filter(
                (service) =>
                  service.type === "MODEL" &&
                  service.id !== mapping.primaryServiceID,
              );
              const backup = backupCandidates.find(
                (service) => service.id === mapping.backupServiceID,
              );
              return (
                <article key={mapping.id}>
                  <header>
                    <strong>模型映射 {index + 1}</strong>
                    {normalizedMappings.length > 1 ? (
                      <button
                        type="button"
                        onClick={() =>
                          setModelMappings((items) =>
                            items.filter((item) => item.id !== mapping.id),
                          )
                        }
                      >
                        <Trash2 />
                        移除
                      </button>
                    ) : null}
                  </header>
                  <div className="form-grid">
                    <label>
                      <span>客户端模型名</span>
                      <input
                        required
                        value={mapping.published}
                        onChange={(event) =>
                          updateMapping(mapping.id, {
                            published: event.target.value,
                          })
                        }
                        placeholder="例如 reasoning-pro"
                      />
                    </label>
                    <label>
                      <span>主线路服务</span>
                      <select
                        value={mapping.primaryServiceID}
                        onChange={(event) =>
                          updateMapping(mapping.id, {
                            primaryServiceID: event.target.value,
                            primaryModel: "",
                            backupServiceID: "",
                            backupModel: "",
                          })
                        }
                      >
                        {availableServices
                          .filter((service) => service.type === "MODEL")
                          .map((service) => (
                            <option key={service.id} value={service.id}>
                              {service.name}
                            </option>
                          ))}
                      </select>
                    </label>
                    <label className="field-wide">
                      <span>主线路真实模型</span>
                      <select
                        value={mapping.primaryModel}
                        onChange={(event) =>
                          updateMapping(mapping.id, {
                            primaryModel: event.target.value,
                          })
                        }
                      >
                        {primary?.capabilities.map((model) => (
                          <option key={model}>{model}</option>
                        ))}
                      </select>
                    </label>
                  </div>
                  <button
                    className="text-action"
                    type="button"
                    onClick={() =>
                      updateMapping(mapping.id, {
                        backupEnabled: !mapping.backupEnabled,
                        backupServiceID: "",
                        backupModel: "",
                      })
                    }
                  >
                    {mapping.backupEnabled
                      ? "移除备用线路"
                      : "+ 添加备用线路"}
                  </button>
                  {mapping.backupEnabled ? (
                    <div className="form-grid">
                      <label>
                        <span>备用线路服务</span>
                        <select
                          value={mapping.backupServiceID}
                          onChange={(event) =>
                            updateMapping(mapping.id, {
                              backupServiceID: event.target.value,
                              backupModel: "",
                            })
                          }
                        >
                          <option value="">选择备用服务</option>
                          {backupCandidates.map((service) => (
                            <option key={service.id} value={service.id}>
                              {service.name}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        <span>备用真实模型</span>
                        <select
                          value={mapping.backupModel}
                          onChange={(event) =>
                            updateMapping(mapping.id, {
                              backupModel: event.target.value,
                            })
                          }
                        >
                          <option value="">选择真实模型</option>
                          {backup?.capabilities.map((model) => (
                            <option key={model}>{model}</option>
                          ))}
                        </select>
                      </label>
                    </div>
                  ) : null}
                </article>
              );
            })}
            <button
              className="text-action"
              type="button"
              onClick={() =>
                setModelMappings((items) => [
                  ...items,
                  newModelMapping(availableServices),
                ])
              }
            >
              <Plus />
              添加模型映射
            </button>
            {duplicateModelNames ? (
              <div className="form-note is-error">
                <AlertTriangle />
                客户端模型名不能重复
              </div>
            ) : null}
          </div>
        ) : (
          <>
            <div className="form-grid">
              <label className="field-wide">
                <span>{type === "API" ? "HTTP 服务" : "MCP 服务"}</span>
                <select
                  value={effectiveServiceID}
                  onChange={(event) => {
                    setServiceID(event.target.value);
                    setSelectedTools([]);
                    setToolQuery("");
                    setSecondServiceID("");
                  }}
                  required
                >
                  {compatible.map((service) => (
                    <option key={service.id} value={service.id}>
                      {service.name} ·{" "}
                      {service.type === "HTTP"
                        ? `${service.endpoints.length} 个端点`
                        : service.provider}
                    </option>
                  ))}
                </select>
              </label>
              {type === "MCP" ? (
                <fieldset className="field-wide tool-picker">
                  <legend>开放工具</legend>
                  {effectiveTools.length ? (
                    <div className="tool-picker-selected">
                      {effectiveTools.map((tool) => (
                        <span key={tool}>
                          <code>{tool}</code>
                          <button
                            type="button"
                            aria-label={`移除${tool}`}
                            onClick={() =>
                              setSelectedTools((items) =>
                                items.filter((item) => item !== tool),
                              )
                            }
                          >
                            <X />
                          </button>
                        </span>
                      ))}
                    </div>
                  ) : (
                    <div className="tool-picker-empty">
                      未开放工具，MCP 请求将被拒绝
                    </div>
                  )}
                  <div className="tool-picker-toolbar">
                    <SearchField
                      value={toolQuery}
                      onChange={setToolQuery}
                      placeholder="搜索已发现工具"
                    />
                    <span>
                      已选 {effectiveTools.length} / {primaryService?.capabilities.length ?? 0}
                    </span>
                  </div>
                  <div className="tool-picker-list">
                    {visibleTools.map((tool) => (
                      <label key={tool}>
                        <input
                          type="checkbox"
                          checked={effectiveTools.includes(tool)}
                          onChange={() =>
                            setSelectedTools((items) =>
                              items.includes(tool)
                                ? items.filter((item) => item !== tool)
                                : [...items, tool],
                            )
                          }
                        />
                        <span>
                          <code>{tool}</code>
                        </span>
                      </label>
                    ))}
                    {!visibleTools.length ? (
                      <small>没有匹配的工具</small>
                    ) : null}
                  </div>
                </fieldset>
              ) : null}
            </div>
            {type === "API" && secondCandidates.length ? (
              <div className="optional-target">
                <button
                  type="button"
                  onClick={() => setSecondLineEnabled((value) => !value)}
                >
                  {secondLineEnabled ? "移除第二线路" : "+ 添加第二线路"}
                </button>
                {secondLineEnabled ? (
                  <div className="form-grid">
                    <label>
                      <span>第二线路服务</span>
                      <select
                        value={effectiveSecondServiceID}
                        onChange={(event) =>
                          setSecondServiceID(event.target.value)
                        }
                      >
                        {secondCandidates.map((service) => (
                          <option key={service.id} value={service.id}>
                            {service.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      <span>转发方式</span>
                      <select
                        value={strategy}
                        onChange={(event) =>
                          setStrategy(
                            event.target.value as "主备切换" | "权重分流",
                          )
                        }
                      >
                        <option>主备切换</option>
                        <option>权重分流</option>
                      </select>
                    </label>
                    {strategy === "权重分流" ? (
                      <label>
                        <span>主线路权重（%）</span>
                        <input
                          type="number"
                          min="1"
                          max="99"
                          value={primaryWeight}
                          onChange={(event) =>
                            setPrimaryWeight(Number(event.target.value))
                          }
                        />
                        <small>第二线路权重为 {100 - primaryWeight}%</small>
                      </label>
                    ) : null}
                  </div>
                ) : null}
              </div>
            ) : null}
          </>
        )}
      </section>
      <details className="advanced-config">
        <summary>高级转发设置</summary>
        <div className="form-grid">
          <label>
            <span>请求超时（秒）</span>
            <input
              type="number"
              min="1"
              value={timeout}
              onChange={(event) => setTimeout(event.target.value)}
            />
          </label>
          <label>
            <span>失败重试次数</span>
            <select
              value={retries}
              onChange={(event) => setRetries(event.target.value)}
            >
              <option value="0">不重试</option>
              <option value="1">1 次</option>
              <option value="2">2 次</option>
              <option value="3">3 次</option>
            </select>
          </label>
          {type === "API" ? (
            <>
              <label>
                <span>路径处理</span>
                <select
                  value={pathHandling}
                  onChange={(event) => setPathHandling(event.target.value)}
                >
                  <option>保持原路径</option>
                  <option>移除匹配前缀</option>
                </select>
              </label>
              <label>
                <span>转发主机名</span>
                <select
                  value={hostRewrite}
                  onChange={(event) =>
                    setHostRewrite(
                      event.target.value as RouteForwarding["hostRewrite"],
                    )
                  }
                >
                  <option>使用服务地址</option>
                  <option>保持请求主机</option>
                  <option>自定义主机名</option>
                </select>
              </label>
              {hostRewrite === "自定义主机名" ? (
                <label>
                  <span>自定义主机名</span>
                  <input
                    value={customHostname}
                    onChange={(event) => setCustomHostname(event.target.value)}
                    placeholder="例如 www.baidu.com"
                  />
                  <small>不包含 http://、路径或端口</small>
                </label>
              ) : null}
              <label>
                <span>改写路径前缀</span>
                <input
                  value={rewritePath}
                  onChange={(event) => setRewritePath(event.target.value)}
                  placeholder="例如 /internal/orders"
                />
              </label>
              <label>
                <span>删除请求头</span>
                <input
                  value={removeHeaders}
                  onChange={(event) => setRemoveHeaders(event.target.value)}
                  placeholder="多个名称用逗号分隔"
                />
              </label>
              <label>
                <span>新增请求头</span>
                <input
                  value={addHeaderName}
                  onChange={(event) => setAddHeaderName(event.target.value)}
                  placeholder="例如 x-route-source"
                />
              </label>
              <label>
                <span>请求头值</span>
                <input
                  value={addHeaderValue}
                  onChange={(event) => setAddHeaderValue(event.target.value)}
                  placeholder="例如 ingate"
                />
              </label>
            </>
          ) : null}
        </div>
        {type === "API" ? (
          <div className="form-note">
            <ShieldCheck />
            服务连续失败 5 次后摘除 30 秒，其余健康端点继续承载流量。
          </div>
        ) : null}
        {type === "AI" &&
        normalizedMappings.some((mapping) => mapping.backupEnabled) ? (
          <fieldset className="failover-picker">
            <legend>切换到备用线路的条件</legend>
            {["连接失败", "超时", "HTTP 429", "HTTP 5xx"].map(
              (reason) => (
                <label key={reason}>
                  <input
                    type="checkbox"
                    checked={failoverOn.includes(reason)}
                    onChange={() =>
                      setFailoverOn((items) =>
                        items.includes(reason)
                          ? items.filter((item) => item !== reason)
                          : [...items, reason],
                      )
                    }
                  />
                  {reason}
                </label>
              ),
            )}
          </fieldset>
        ) : null}
      </details>
    </>
  );
}
