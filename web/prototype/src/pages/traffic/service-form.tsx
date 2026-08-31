import {
  Drawer,
  FormActions,
  ServiceTypeBadge,
  submitForm,
} from "../../components/ui";
import { ServiceCapabilitiesSection } from "./service-capabilities-section";
import { ServiceConnectionSection } from "./service-connection-section";
import { ServiceResilienceSection } from "./service-resilience-section";
import {
  type ServiceFormProps,
  useServiceForm,
} from "./use-service-form";

export function CreateService(props: ServiceFormProps) {
  const { initial, onClose } = props;
  const form = useServiceForm(props);
  const {
    canSave,
    canTest,
    changeType,
    endpoints,
    save,
    tested,
    type,
  } = form;

  return (
    <Drawer
      title={initial ? "编辑服务" : "创建服务"}
      description="连接、认证、TLS、端点与健康检查"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="type-selector">
          {(["HTTP", "MODEL", "MCP"] as const).map((item) => (
            <button
              key={item}
              type="button"
              className={type === item ? "is-selected" : ""}
              onClick={() => changeType(item)}
            >
              <ServiceTypeBadge type={item} />
              <strong>
                {item === "HTTP"
                  ? "普通业务服务"
                  : item === "MODEL"
                    ? "模型厂商或推理服务"
                    : "远程工具服务"}
              </strong>
              <small>
                {item === "HTTP"
                  ? "普通业务接口"
                  : item === "MODEL"
                    ? "模型厂商或自托管推理服务"
                    : "远程 MCP 服务"}
              </small>
            </button>
          ))}
        </div>
        <ServiceConnectionSection form={form} />
        <ServiceResilienceSection form={form} />
        <ServiceCapabilitiesSection form={form} />
        <FormActions
          submitLabel={
            initial ? "保存修改" : tested ? "保存服务" : "保存为待验证"
          }
          submitDisabled={
            !canTest || !canSave ||
            (endpoints.length > 1 &&
              endpoints.reduce((sum, item) => sum + item.weight, 0) !== 100)
          }
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
