import {
  ChevronLeft,
  ChevronRight,
  ShieldCheck,
  X,
} from "lucide-react";
import { useState } from "react";
import {
  Drawer,
  FilterSelect,
  FormActions,
  RouteTypeBadge,
  SearchField,
  submitForm,
} from "../../components/ui";
import type {
  GatewayRoute,
  Policy,
} from "../../data";
import {
  defaultPolicyNames,
  policyGroup,
  targetPageSize,
  type PolicyTargetCandidate,
  type TargetTrafficFilter,
} from "./policy-model";

export function CreatePolicy({
  initial,
  gateways,
  routes,
  onClose,
  onSave,
}: {
  initial?: Policy;
  gateways: Array<{ id: string; name: string }>;
  routes: GatewayRoute[];
  onClose: () => void;
  onSave: (policy: Policy) => void;
}) {
  const [type, setType] = useState<Policy["type"]>(initial?.type ?? "请求限流");
  const [name, setName] = useState(
    initial?.name ?? defaultPolicyNames["请求限流"],
  );
  const [selectedTargets, setSelectedTargets] = useState<string[]>(
    initial?.targets.map(
      (target) =>
        `${target.kind === "网关" ? "gateway" : "route"}:${target.id}`,
    ) ?? [],
  );
  const [targetQuery, setTargetQuery] = useState("");
  const [targetKind, setTargetKind] = useState<"ALL" | "网关" | "路由">(
    "ALL",
  );
  const [targetGatewayID, setTargetGatewayID] = useState("ALL");
  const [targetTraffic, setTargetTraffic] =
    useState<TargetTrafficFilter>("ALL");
  const [targetPage, setTargetPage] = useState(1);
  const [rateLimit, setRateLimit] = useState(
    initial?.settings?.rateLimit ??
      initial?.rule.match(/[\d,]+(?= 次)/)?.[0].replaceAll(",", "") ??
      "1000",
  );
  const [ratePeriod, setRatePeriod] = useState(
    initial?.settings?.ratePeriod ??
      (initial?.rule.includes("每小时")
        ? "小时"
        : initial?.rule.includes("每秒")
          ? "秒"
          : "分钟"),
  );
  const [rateDimension, setRateDimension] = useState(
    initial?.settings?.rateDimension ??
      (initial?.rule.startsWith("全部请求")
        ? "全部请求共享"
        : initial?.rule.startsWith("每个客户端 IP")
          ? "每个客户端 IP"
          : "每个调用方"),
  );
  const [ipMode, setIPMode] = useState(
    initial?.settings?.ipMode ??
      (initial?.rule.startsWith("拒绝") ? "拒绝" : "仅允许"),
  );
  const [ipRanges, setIPRanges] = useState(
    initial?.settings?.ipRanges ?? "10.0.0.0/8\n192.168.0.0/16",
  );
  const candidates: PolicyTargetCandidate[] =
    type === "请求限流" && rateDimension === "每个调用方"
        ? routes
            .filter((route) => route.accessMode === "API Key")
            .map((route) => ({
              key: `route:${route.id}`,
              kind: "路由" as const,
              id: route.id,
              name: route.name,
              gatewayID: route.gatewayID,
              gatewayName: route.gatewayName,
              trafficType: route.type,
              detail: `${route.host}${route.path}`,
            }))
        : [
            ...gateways.map((gateway) => ({
              key: `gateway:${gateway.id}`,
              kind: "网关" as const,
              id: gateway.id,
              name: gateway.name,
              gatewayID: gateway.id,
              gatewayName: gateway.name,
              detail: "该网关下的全部流量",
            })),
            ...routes.map((route) => ({
              key: `route:${route.id}`,
              kind: "路由" as const,
              id: route.id,
              name: route.name,
              gatewayID: route.gatewayID,
              gatewayName: route.gatewayName,
              trafficType: route.type,
              detail: `${route.host}${route.path}`,
            })),
          ];
  const effectiveTargets = selectedTargets.filter((key) =>
    candidates.some((candidate) => candidate.key === key),
  );
  const selectedCandidates = candidates.filter((candidate) =>
    effectiveTargets.includes(candidate.key),
  );
  const filteredCandidates = candidates.filter(
    (candidate) =>
      (targetKind === "ALL" || candidate.kind === targetKind) &&
      (targetGatewayID === "ALL" ||
        candidate.gatewayID === targetGatewayID) &&
      (targetTraffic === "ALL" ||
        candidate.trafficType === targetTraffic) &&
      `${candidate.name}${candidate.gatewayName}${candidate.detail}${candidate.trafficType ?? ""}`
        .toLowerCase()
        .includes(targetQuery.toLowerCase()),
  );
  const targetPageCount = Math.max(
    1,
    Math.ceil(filteredCandidates.length / targetPageSize),
  );
  const effectiveTargetPage = Math.min(targetPage, targetPageCount);
  const visibleCandidates = filteredCandidates.slice(
    (effectiveTargetPage - 1) * targetPageSize,
    effectiveTargetPage * targetPageSize,
  );
  const candidateKinds = new Set(candidates.map((candidate) => candidate.kind));
  const routeCandidates = candidates.filter(
    (candidate) => candidate.kind === "路由",
  );
  const availableGateways = gateways.filter((gateway) =>
    routeCandidates.some((candidate) => candidate.gatewayID === gateway.id),
  );
  const allVisibleSelected =
    visibleCandidates.length > 0 &&
    visibleCandidates.every((candidate) =>
      effectiveTargets.includes(candidate.key),
    );
  const ipRangeCount = ipRanges
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean).length;
  const rule =
    type === "请求限流"
      ? `${rateDimension}每${ratePeriod} ${Number(rateLimit).toLocaleString("zh-CN")} 次${effectiveTargets.length > 1 ? "，各目标独立计数" : ""}`
      : `${ipMode} ${ipRangeCount} 个网段`;
  const save = () =>
    onSave({
      id: initial?.id ?? `policy-${Date.now()}`,
      name,
      type,
      targets: candidates
        .filter((candidate) => effectiveTargets.includes(candidate.key))
        .map(({ kind, id, name: targetName }) => ({
          kind,
          id,
          name: targetName,
        })),
      rule,
      effect: initial?.effect ?? "暂无执行记录",
      state: effectiveTargets.length ? "healthy" : "disabled",
      settings: {
        rateLimit,
        ratePeriod,
        rateDimension,
        ipMode,
        ipRanges,
      },
    });
  const changeType = (next: Policy["type"]) => {
    setType(next);
    setName(defaultPolicyNames[next]);
    setSelectedTargets([]);
    setTargetQuery("");
    setTargetKind("ALL");
    setTargetGatewayID("ALL");
    setTargetTraffic("ALL");
    setTargetPage(1);
  };
  const toggleTarget = (key: string) =>
    setSelectedTargets((items) =>
      items.includes(key)
        ? items.filter((item) => item !== key)
        : [...items, key],
    );
  const toggleVisibleTargets = () =>
    setSelectedTargets((items) => {
      const visibleKeys = new Set(visibleCandidates.map((candidate) => candidate.key));
      if (allVisibleSelected) return items.filter((item) => !visibleKeys.has(item));
      return [...new Set([...items, ...visibleKeys])];
    });
  return (
    <Drawer
      title={initial ? "编辑流量策略" : "创建流量策略"}
      description={policyGroup(type)}
      onClose={onClose}
      width="wide"
    >
      <form onSubmit={(event) => submitForm(event, save)}>
        <div className="form-grid">
          <label>
            <span>策略名称</span>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <label>
            <span>策略类型</span>
            <select
              value={type}
              onChange={(event) =>
                changeType(event.target.value as Policy["type"])
              }
            >
              <option>请求限流</option>
              <option>IP 访问限制</option>
            </select>
          </label>
          {type === "请求限流" ? (
            <>
              <label>
                <span>计数维度</span>
                <select
                  value={rateDimension}
                  onChange={(event) => {
                    setRateDimension(event.target.value);
                    setSelectedTargets([]);
                    setTargetQuery("");
                    setTargetKind("ALL");
                    setTargetGatewayID("ALL");
                    setTargetTraffic("ALL");
                    setTargetPage(1);
                  }}
                >
                  <option>每个调用方</option>
                  <option>全部请求共享</option>
                  <option>每个客户端 IP</option>
                </select>
                <small>
                  {rateDimension === "每个调用方"
                    ? "只可应用到需要密钥的路由"
                    : "可应用到网关或路由"}
                </small>
              </label>
              <label>
                <span>时间窗口</span>
                <select
                  value={ratePeriod}
                  onChange={(event) => setRatePeriod(event.target.value)}
                >
                  <option>秒</option>
                  <option>分钟</option>
                  <option>小时</option>
                </select>
              </label>
              <label className="field-wide">
                <span>最大请求数</span>
                <input
                  required
                  type="number"
                  min="1"
                  step="1"
                  value={rateLimit}
                  onChange={(event) => setRateLimit(event.target.value)}
                />
              </label>
            </>
          ) : null}
          {type === "IP 访问限制" ? (
            <>
              <label>
                <span>处理方式</span>
                <select
                  value={ipMode}
                  onChange={(event) => setIPMode(event.target.value)}
                >
                  <option>仅允许</option>
                  <option>拒绝</option>
                </select>
              </label>
              <label className="field-wide">
                <span>IP 或 CIDR（每行一个）</span>
                <textarea
                  required
                  value={ipRanges}
                  onChange={(event) => setIPRanges(event.target.value)}
                />
              </label>
            </>
          ) : null}
        </div>
        <fieldset className="target-picker">
          <legend>生效范围</legend>
          <header className="target-picker-header">
            <div>
              <strong>
                {effectiveTargets.length
                  ? `已选择 ${effectiveTargets.length} 个目标`
                  : "尚未选择目标"}
              </strong>
              <span>未选择目标时保存为“未应用”</span>
            </div>
            {effectiveTargets.length ? (
              <button
                type="button"
                onClick={() => setSelectedTargets([])}
              >
                清空选择
              </button>
            ) : null}
          </header>
          {selectedCandidates.length ? (
            <div className="selected-targets" aria-label="已选生效目标">
              {selectedCandidates.map((candidate) => (
                <span key={candidate.key}>
                  <small>{candidate.kind}</small>
                  <strong>{candidate.name}</strong>
                  <button
                    type="button"
                    aria-label={`移除${candidate.name}`}
                    onClick={() => toggleTarget(candidate.key)}
                  >
                    <X />
                  </button>
                </span>
              ))}
            </div>
          ) : (
            <div className="selected-targets-empty">
              策略尚未关联流量目标
            </div>
          )}
          <div className="target-picker-toolbar">
            <SearchField
              value={targetQuery}
              onChange={(value) => {
                setTargetQuery(value);
                setTargetPage(1);
              }}
              placeholder="搜索目标名称、域名或路径"
            />
            {candidateKinds.size > 1 ? (
              <FilterSelect
                label="资源类型"
                value={targetKind}
                onChange={(value) => {
                  setTargetKind(value);
                  if (value === "网关") setTargetTraffic("ALL");
                  setTargetPage(1);
                }}
                options={[
                  { value: "ALL", label: "全部类型", count: candidates.length },
                  {
                    value: "网关",
                    label: "网关",
                    count: candidates.filter((item) => item.kind === "网关").length,
                  },
                  {
                    value: "路由",
                    label: "路由",
                    count: candidates.filter((item) => item.kind === "路由").length,
                  },
                ]}
              />
            ) : null}
            {routeCandidates.length ? (
              <FilterSelect
                label="所属网关"
                value={targetGatewayID}
                onChange={(value) => {
                  setTargetGatewayID(value);
                  setTargetPage(1);
                }}
                options={[
                  {
                    value: "ALL",
                    label: "全部网关",
                    count: routeCandidates.length,
                  },
                  ...availableGateways.map((gateway) => ({
                    value: gateway.id,
                    label: gateway.name,
                    count: routeCandidates.filter(
                      (candidate) => candidate.gatewayID === gateway.id,
                    ).length,
                  })),
                ]}
              />
            ) : null}
            {routeCandidates.length ? (
              <FilterSelect
                label="流量类型"
                value={targetTraffic}
                onChange={(value) => {
                  setTargetTraffic(value);
                  setTargetPage(1);
                }}
                options={[
                  {
                    value: "ALL",
                    label: "全部流量",
                    count: routeCandidates.length,
                  },
                  ...(["API", "AI", "MCP"] as const).map((trafficType) => ({
                    value: trafficType,
                    label: `${trafficType} 路由`,
                    count: routeCandidates.filter(
                      (candidate) => candidate.trafficType === trafficType,
                    ).length,
                  })),
                ]}
              />
            ) : null}
          </div>
          <div className="target-candidate-heading">
            <span>
              匹配 {filteredCandidates.length} 项 · 第 {effectiveTargetPage} /{" "}
              {targetPageCount} 页
            </span>
            {visibleCandidates.length ? (
              <button type="button" onClick={toggleVisibleTargets}>
                {allVisibleSelected
                  ? "取消选择本页"
                  : `选择本页 ${visibleCandidates.length} 项`}
              </button>
            ) : null}
          </div>
          <div className="target-candidate-list">
            {visibleCandidates.length ? (
              visibleCandidates.map((candidate) => (
                <label
                  className={
                    effectiveTargets.includes(candidate.key)
                      ? "is-selected"
                      : ""
                  }
                  key={candidate.key}
                >
                  <input
                    type="checkbox"
                    checked={effectiveTargets.includes(candidate.key)}
                    onChange={() => toggleTarget(candidate.key)}
                  />
                  {candidate.trafficType ? (
                    <RouteTypeBadge type={candidate.trafficType} />
                  ) : (
                    <span className="target-gateway-badge">网关</span>
                  )}
                  <span>
                    <strong>{candidate.name}</strong>
                    <small>
                      {candidate.kind === "路由"
                        ? `${candidate.gatewayName} · ${candidate.detail}`
                        : candidate.detail}
                    </small>
                  </span>
                </label>
              ))
            ) : (
              <div className="target-candidate-empty">
                没有匹配的生效目标，请调整搜索条件
              </div>
            )}
          </div>
          {targetPageCount > 1 ? (
            <div className="target-pagination">
              <button
                type="button"
                disabled={effectiveTargetPage === 1}
                onClick={() => setTargetPage((page) => Math.max(1, page - 1))}
              >
                <ChevronLeft />
                上一页
              </button>
              <span>
                本页 {visibleCandidates.length} 项，已选 {effectiveTargets.length} 项
              </span>
              <button
                type="button"
                disabled={effectiveTargetPage === targetPageCount}
                onClick={() =>
                  setTargetPage((page) => Math.min(targetPageCount, page + 1))
                }
              >
                下一页
                <ChevronRight />
              </button>
            </div>
          ) : null}
        </fieldset>
        <div className="form-note">
          <ShieldCheck />
          将生成规则：{rule}
        </div>
        <FormActions
          submitLabel={initial ? "保存修改" : "保存流量策略"}
          submitDisabled={type === "IP 访问限制" && ipRangeCount === 0}
          onCancel={onClose}
        />
      </form>
    </Drawer>
  );
}
