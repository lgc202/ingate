import {
  Plus,
  Trash2,
} from "lucide-react";
import type { Service } from "../../data";
import type { ServiceFormState } from "./use-service-form";

export function ServiceResilienceSection({ form }: { form: ServiceFormState }) {
  const {
    endpoints,
    setEndpoints,
    loadBalancing,
    setLoadBalancing,
    setTested,
    balanceWeights,
    updateEndpoint,
  } = form;

  return (
    <>
      <section className="form-section">
        <header>
          <span>2</span>
          <div>
            <strong>服务端点</strong>
            <small>同一服务的多个实例共享协议、认证和 TLS 配置</small>
          </div>
        </header>
        <div className="endpoint-editor">
          {endpoints.map((endpoint, index) => (
            <div className="endpoint-row" key={index}>
              <label>
                <span>端点地址</span>
                <input
                  required
                  value={endpoint.address}
                  onChange={(event) =>
                    updateEndpoint(index, {
                      address: event.target.value,
                    })
                  }
                />
              </label>
              {endpoints.length > 1 ? (
                <label>
                  <span>权重（%）</span>
                  <input
                    type="number"
                    min="1"
                    max="100"
                    value={endpoint.weight}
                    onChange={(event) =>
                      updateEndpoint(index, {
                        weight: Number(event.target.value),
                      })
                    }
                  />
                </label>
              ) : null}
              <button
                type="button"
                aria-label="删除端点"
                disabled={endpoints.length === 1}
                onClick={() => {
                  setEndpoints((items) =>
                    balanceWeights(
                      items.filter((_, itemIndex) => itemIndex !== index),
                    ),
                  );
                  setTested(false);
                }}
              >
                <Trash2 />
              </button>
            </div>
          ))}
        </div>
        <button
          className="text-action"
          type="button"
          onClick={() => {
            setEndpoints((items) =>
              balanceWeights([
                ...items,
                {
                  address: "",
                  weight: 0,
                },
              ]),
            );
            setTested(false);
          }}
        >
          <Plus />
          添加端点
        </button>
        {endpoints.length > 1 ? (
          <div className="form-grid load-balance-field">
            <label>
              <span>负载方式</span>
              <select
                value={loadBalancing}
                onChange={(event) =>
                  setLoadBalancing(
                    event.target.value as Service["loadBalancing"],
                  )
                }
              >
                <option>轮询</option>
                <option>最少请求</option>
                <option>随机</option>
              </select>
            </label>
            <div className="weight-summary">
              <span>当前权重合计</span>
              <strong
                className={
                  endpoints.reduce((sum, item) => sum + item.weight, 0) ===
                  100
                    ? ""
                    : "is-warning"
                }
              >
                {endpoints.reduce((sum, item) => sum + item.weight, 0)}%
              </strong>
            </div>
          </div>
        ) : null}
      </section>
    </>
  );
}
