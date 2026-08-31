import {
  Drawer,
  FormActions,
  RouteTypeBadge,
  submitForm,
} from "../../components/ui";
import { RouteAccessSection } from "./route-access-section";
import { RouteBasicsSection } from "./route-basics-section";
import { RouteDestinationSection } from "./route-destination-section";
import {
  type RouteFormProps,
  useRouteForm,
} from "./use-route-form";

export function CreateRoute(props: RouteFormProps) {
  const { initial, onClose } = props;
  const form = useRouteForm(props);
  const { changeType, routeReady, save, type } = form;

  return (
    <Drawer
      title={initial ? "编辑路由" : "创建路由"}
      description="定义外部请求如何匹配并转发到服务"
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="type-selector">
          {(["API", "AI", "MCP"] as const).map((item) => (
            <button
              key={item}
              type="button"
              className={type === item ? "is-selected" : ""}
              onClick={() => changeType(item)}
            >
              <RouteTypeBadge type={item} />
              <strong>
                {item === "API"
                  ? "普通 HTTP API"
                  : item === "AI"
                    ? "统一大模型接口"
                    : "远程工具调用"}
              </strong>
              <small>
                {item === "API"
                  ? "一个方法与路径匹配"
                  : item === "AI"
                    ? "发布一个或多个客户端模型名"
                    : "发布一个服务中的多个工具"}
              </small>
            </button>
          ))}
        </div>
        <RouteBasicsSection form={form} />
        <RouteAccessSection form={form} />
        <RouteDestinationSection form={form} />
        <FormActions
          submitLabel={initial ? "保存修改" : "创建路由"}
          submitDisabled={!routeReady}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
