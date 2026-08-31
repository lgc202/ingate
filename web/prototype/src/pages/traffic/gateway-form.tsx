import {
  AlertTriangle,
  Plus,
  Trash2,
} from "lucide-react";
import { useState } from "react";
import {
  Drawer,
  FormActions,
  submitForm,
} from "../../components/ui";
import type {
  Certificate,
  Gateway,
  GatewayListener,
} from "../../data";
import {
  certificateCoversDomain,
  listenerLabel,
} from "./gateway-model";

export function CreateGateway({
  initial,
  gateways,
  certificates,
  onClose,
  onSave,
}: {
  initial?: Gateway;
  gateways: Gateway[];
  certificates: Certificate[];
  onClose: () => void;
  onSave: (gateway: Gateway) => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [listeners, setListeners] = useState<GatewayListener[]>(
    initial?.listeners.map((listener) => ({
      ...listener,
      bindings: listener.bindings.map((binding) => ({
        ...binding,
      })),
    })) ?? [
      {
        id: crypto.randomUUID(),
        protocol: "HTTPS",
        port: 443,
        bindings: [
          {
            domain: "new-api.example.com",
            certificateID: "",
          },
        ],
      },
    ],
  );
  const updateListener = (
    listenerID: string,
    changes: Partial<GatewayListener>,
  ) =>
    setListeners((items) =>
      items.map((listener) =>
        listener.id === listenerID
          ? {
              ...listener,
              ...changes,
            }
          : listener,
      ),
    );
  const updateBinding = (
    listenerID: string,
    index: number,
    domain: string,
    certificateID = "",
  ) =>
    setListeners((items) =>
      items.map((listener) =>
        listener.id === listenerID
          ? {
              ...listener,
              bindings: listener.bindings.map((binding, bindingIndex) =>
                bindingIndex === index
                  ? {
                      domain,
                      certificateID:
                        listener.protocol === "HTTPS"
                          ? certificateID
                          : undefined,
                    }
                  : binding,
              ),
            }
          : listener,
      ),
    );
  const invalidTLS = listeners.some(
    (listener) =>
      listener.protocol === "HTTPS" &&
      listener.bindings.some(
        (binding) =>
          !binding.certificateID ||
          !certificates.some(
            (certificate) =>
              certificate.id === binding.certificateID &&
              certificate.usage === "服务器证书" &&
              certificate.identities.some((identity) =>
                certificateCoversDomain(identity, binding.domain),
              ),
          ),
      ),
  );
  const duplicateSocket =
    new Set(listeners.map((listener) => listener.port)).size !==
    listeners.length;
  const claimedBinding = listeners.some((listener) =>
    listener.bindings.some((binding) =>
      gateways.some(
        (gateway) =>
          gateway.id !== initial?.id &&
          gateway.listeners.some(
            (current) =>
              current.port === listener.port &&
              current.bindings.some(
                (currentBinding) =>
                  currentBinding.domain.toLowerCase() ===
                  binding.domain.trim().toLowerCase(),
              ),
          ),
      ),
    ),
  );
  const duplicateDomain =
    claimedBinding ||
    listeners.some((listener) => {
      const domains = listener.bindings.map((binding) =>
        binding.domain.trim().toLowerCase(),
      );
      return new Set(domains).size !== domains.length;
    });
  const save = () =>
    onSave({
      id: initial?.id ?? `gw-${Date.now()}`,
      name,
      listeners,
      state: initial?.state ?? "healthy",
    });
  return (
    <Drawer
      title={initial ? "编辑网关" : "创建网关"}
      description="一个网关可以包含多个监听入口，每个 HTTPS 域名绑定自己的证书"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="form-grid">
          <label className="field-wide">
            <span>网关名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如生产网关"
            />
          </label>
        </div>
        <div className="listener-editor">
          {listeners.map((listener, listenerIndex) => (
            <section className="form-section" key={listener.id}>
              <header>
                <span>{listenerIndex + 1}</span>
                <div>
                  <strong>监听入口</strong>
                  <small>{listenerLabel(listener)}</small>
                </div>
                {listeners.length > 1 ? (
                  <button
                    type="button"
                    onClick={() =>
                      setListeners((items) =>
                        items.filter((item) => item.id !== listener.id),
                      )
                    }
                  >
                    <Trash2 />
                  </button>
                ) : null}
              </header>
              <div className="form-grid">
                <label>
                  <span>协议</span>
                  <select
                    value={listener.protocol}
                    onChange={(event) => {
                      const protocol = event.target.value as "HTTP" | "HTTPS";
                      updateListener(listener.id, {
                        protocol,
                        port: protocol === "HTTPS" ? 443 : 80,
                        bindings: listener.bindings.map((binding) => ({
                          domain: binding.domain,
                          certificateID: protocol === "HTTPS" ? "" : undefined,
                        })),
                      });
                    }}
                  >
                    <option>HTTPS</option>
                    <option>HTTP</option>
                  </select>
                </label>
                <label>
                  <span>端口</span>
                  <input
                    required
                    type="number"
                    min="1"
                    max="65535"
                    value={listener.port}
                    onChange={(event) =>
                      updateListener(listener.id, {
                        port: Number(event.target.value),
                      })
                    }
                  />
                </label>
              </div>
              <div className="binding-editor">
                {listener.bindings.map((binding, index) => {
                  const matching = certificates.filter(
                    (certificate) =>
                      certificate.usage === "服务器证书" &&
                      certificate.state !== "error" &&
                      certificate.identities.some((identity) =>
                        certificateCoversDomain(identity, binding.domain),
                      ),
                  );
                  const effectiveCertificateID = matching.some(
                    (certificate) => certificate.id === binding.certificateID,
                  )
                    ? (binding.certificateID ?? "")
                    : "";
                  return (
                    <div className="binding-row" key={index}>
                      <label>
                        <span>请求域名</span>
                        <input
                          required
                          value={binding.domain}
                          onChange={(event) =>
                            updateBinding(
                              listener.id,
                              index,
                              event.target.value,
                            )
                          }
                        />
                      </label>
                      {listener.protocol === "HTTPS" ? (
                        <label>
                          <span>TLS 证书</span>
                          <select
                            required
                            value={effectiveCertificateID}
                            onChange={(event) =>
                              updateBinding(
                                listener.id,
                                index,
                                binding.domain,
                                event.target.value,
                              )
                            }
                          >
                            <option value="">选择覆盖该域名的证书</option>
                            {matching.map((certificate) => (
                              <option
                                key={certificate.id}
                                value={certificate.id}
                              >
                                {certificate.name}
                              </option>
                            ))}
                          </select>
                          {binding.domain && !matching.length ? (
                            <small>没有可覆盖该域名的服务器证书</small>
                          ) : null}
                        </label>
                      ) : null}
                      <button
                        type="button"
                        aria-label="删除域名绑定"
                        disabled={listener.bindings.length === 1}
                        onClick={() =>
                          updateListener(listener.id, {
                            bindings: listener.bindings.filter(
                              (_, bindingIndex) => bindingIndex !== index,
                            ),
                          })
                        }
                      >
                        <Trash2 />
                      </button>
                    </div>
                  );
                })}
              </div>
              <button
                className="text-action"
                type="button"
                onClick={() =>
                  updateListener(listener.id, {
                    bindings: [
                      ...listener.bindings,
                      {
                        domain: "",
                        certificateID:
                          listener.protocol === "HTTPS" ? "" : undefined,
                      },
                    ],
                  })
                }
              >
                <Plus />
                添加域名
              </button>
            </section>
          ))}
        </div>
        <button
          className="text-action"
          type="button"
          onClick={() =>
            setListeners((items) => [
              ...items,
              {
                id: crypto.randomUUID(),
                protocol: "HTTP",
                port: 80,
                bindings: [
                  {
                    domain: "",
                    certificateID: undefined,
                  },
                ],
              },
            ])
          }
        >
          <Plus />
          添加监听入口
        </button>
        {duplicateSocket ? (
          <div className="form-note is-error">
            <AlertTriangle />
            同一网关内不能创建两个占用相同端口的监听入口；同一端口的多个域名应放在一个入口下。
          </div>
        ) : null}
        {duplicateDomain ? (
          <div className="form-note is-error">
            <AlertTriangle />
            同一环境中，相同端口不能重复声明同一域名；请使用已有网关，或调整端口和域名。
          </div>
        ) : null}
        <FormActions
          submitLabel={initial ? "保存修改" : "创建网关"}
          submitDisabled={
            invalidTLS ||
            duplicateSocket ||
            duplicateDomain ||
            listeners.some((listener) =>
              listener.bindings.some((binding) => !binding.domain.trim()),
            )
          }
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
