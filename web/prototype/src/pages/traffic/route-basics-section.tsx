import { AlertTriangle } from "lucide-react";
import { gatewayDomains } from "./gateway-model";
import type { RouteFormState } from "./use-route-form";

export function RouteBasicsSection({ form }: { form: RouteFormState }) {
  const {
    gateways,
    type,
    name,
    setName,
    gatewayID,
    setGatewayID,
    setHost,
    path,
    setPath,
    method,
    setMethod,
    matchMode,
    setMatchMode,
    conditionKind,
    setConditionKind,
    conditionName,
    setConditionName,
    conditionValue,
    setConditionValue,
    conditionMode,
    setConditionMode,
    gateway,
    effectiveHost,
    routeConflict,
  } = form;

  return (
    <section className="form-section">
      <header>
        <span>1</span>
        <div>
          <strong>请求入口</strong>
          <small>请求按域名和路径匹配唯一一条路由</small>
        </div>
      </header>
      <div className="form-grid">
        <label>
          <span>路由名称</span>
          <input
            required
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label>
          <span>生效网关</span>
          <select
            value={gatewayID}
            onChange={(event) => {
              setGatewayID(event.target.value);
              setHost("");
            }}
            required
          >
            {gateways.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>请求域名</span>
          <select
            value={effectiveHost}
            onChange={(event) => setHost(event.target.value)}
            required
          >
            {gateway
              ? gatewayDomains(gateway).map((domain) => (
                  <option key={domain}>{domain}</option>
                ))
              : null}
          </select>
        </label>
        <label>
          <span>请求路径</span>
          <input
            required
            value={path}
            onChange={(event) => setPath(event.target.value)}
          />
        </label>
        {type === "API" ? (
          <>
            <label>
              <span>请求方法</span>
              <select
                value={method}
                onChange={(event) => setMethod(event.target.value)}
              >
                <option value="ANY">全部方法</option>
                <option>GET</option>
                <option>POST</option>
                <option>PUT</option>
                <option>PATCH</option>
                <option>DELETE</option>
              </select>
            </label>
            <label>
              <span>路径方式</span>
              <select
                value={matchMode}
                onChange={(event) =>
                  setMatchMode(
                    event.target.value as "精确匹配" | "前缀匹配",
                  )
                }
              >
                <option>精确匹配</option>
                <option>前缀匹配</option>
              </select>
            </label>
          </>
        ) : null}
      </div>
      {routeConflict ? (
        <div className="form-note is-error">
          <AlertTriangle />
          与“{routeConflict.name}
          ”冲突：同一网关、域名下的方法和路径匹配范围重叠。
        </div>
      ) : null}
      {type === "API" ? (
        <details className="advanced-config">
          <summary>Header / Query 匹配（可选）</summary>
          <div className="form-grid">
            <label>
              <span>条件类型</span>
              <select
                value={conditionKind}
                onChange={(event) =>
                  setConditionKind(event.target.value as "Header" | "Query")
                }
              >
                <option>Header</option>
                <option>Query</option>
              </select>
            </label>
            <label>
              <span>匹配方式</span>
              <select
                value={conditionMode}
                onChange={(event) =>
                  setConditionMode(
                    event.target.value as "精确匹配" | "存在",
                  )
                }
              >
                <option>精确匹配</option>
                <option>存在</option>
              </select>
            </label>
            <label>
              <span>{conditionKind} 名称</span>
              <input
                value={conditionName}
                onChange={(event) => setConditionName(event.target.value)}
                placeholder={
                  conditionKind === "Header"
                    ? "例如 x-api-version"
                    : "例如 tenant"
                }
              />
            </label>
            {conditionMode === "精确匹配" ? (
              <label>
                <span>匹配值</span>
                <input
                  value={conditionValue}
                  onChange={(event) =>
                    setConditionValue(event.target.value)
                  }
                  placeholder="例如 v2"
                />
              </label>
            ) : null}
          </div>
        </details>
      ) : null}
    </section>
  );
}
