import { useState } from "react";
import {
  initialCallers,
  initialIdentitySources,
  initialPolicies,
  type CallerQuota,
} from "../data";
import type { PrototypeContextValue } from "../prototype-context-value";
import type { RecordAudit } from "./operations-state";

type GovernanceActions = Pick<
  PrototypeContextValue,
  | "addCaller"
  | "updateCaller"
  | "deleteCaller"
  | "updateCallerPermissions"
  | "setCallerQuota"
  | "removeCallerQuota"
  | "issueCallerKey"
  | "rotateCallerKey"
  | "revokeCallerKey"
  | "addIdentitySource"
  | "updateIdentitySource"
  | "deleteIdentitySource"
  | "addPolicy"
  | "updatePolicy"
  | "deletePolicy"
>;

export function useGovernanceState(recordAudit: RecordAudit) {
  const [callers, setCallers] = useState(initialCallers);
  const [identitySources, setIdentitySources] = useState(initialIdentitySources);
  const [policies, setPolicies] = useState(initialPolicies);

  const actions: GovernanceActions = {
    addCaller: (caller) => {
      setCallers((items) => [...items, caller]);
      recordAudit(
        "创建调用方",
        "调用方",
        caller.name,
        caller.permissions.length
          ? `已授予 ${caller.permissions.length} 条路由权限`
          : "尚未授予路由权限",
      );
    },
    updateCaller: (caller) => {
      setCallers((items) =>
        items.map((item) => (item.id === caller.id ? caller : item)),
      );
      recordAudit(
        `更新调用方“${caller.name}”`,
        "调用方",
        caller.name,
        "基本信息已更新",
      );
    },
    deleteCaller: (callerID) => {
      const caller = callers.find((item) => item.id === callerID);
      if (!caller) return;
      setCallers((items) => items.filter((item) => item.id !== callerID));
      recordAudit(
        `删除调用方“${caller.name}”`,
        "调用方",
        caller.name,
        "密钥、权限和额度已一并移除",
      );
    },
    updateCallerPermissions: (callerID, permissions) => {
      setCallers((items) =>
        items.map((caller) => {
          if (caller.id !== callerID) return caller;
          const routeIDs = new Set(
            permissions.map((permission) => permission.routeID),
          );
          const quotas = caller.quotas.filter((quota) =>
            routeIDs.has(quota.routeID),
          );
          return { ...caller, permissions, quotas, state: callerState(quotas) };
        }),
      );
      recordAudit(
        `更新“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”权限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "路由访问权限已提交",
      );
    },
    setCallerQuota: (callerID, quota) => {
      setCallers((items) =>
        items.map((caller) => {
          if (caller.id !== callerID) return caller;
          const current = caller.quotas.find(
            (item) => item.routeID === quota.routeID,
          );
          const nextQuota = { ...quota, used: current?.used ?? quota.used };
          const quotas = current
            ? caller.quotas.map((item) =>
                item.routeID === quota.routeID ? nextQuota : item,
              )
            : [...caller.quotas, nextQuota];
          return { ...caller, quotas, state: callerState(quotas) };
        }),
      );
      recordAudit(
        `更新“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”用量上限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "累计用量上限已提交",
      );
    },
    removeCallerQuota: (callerID, routeID) => {
      setCallers((items) =>
        items.map((caller) => {
          if (caller.id !== callerID) return caller;
          const quotas = caller.quotas.filter(
            (quota) => quota.routeID !== routeID,
          );
          return { ...caller, quotas, state: callerState(quotas) };
        }),
      );
      recordAudit(
        `移除“${callers.find((item) => item.id === callerID)?.name ?? "调用方"}”用量上限`,
        "调用方",
        callers.find((item) => item.id === callerID)?.name ?? callerID,
        "累计用量上限已移除",
      );
    },
    issueCallerKey: (callerID, key) => {
      setCallers((items) =>
        items.map((caller) =>
          caller.id === callerID
            ? { ...caller, keys: [...caller.keys, key] }
            : caller,
        ),
      );
      const caller = callers.find((item) => item.id === callerID);
      recordAudit(
        "签发密钥",
        "调用方",
        caller?.name ?? callerID,
        `签发“${key.name}”访问密钥，有效期至 ${key.expiresAt}`,
      );
    },
    rotateCallerKey: (callerID, keyID, key, graceUntil) => {
      const caller = callers.find((item) => item.id === callerID);
      const previousKey = caller?.keys.find((item) => item.id === keyID);
      setCallers((items) =>
        items.map((item) =>
          item.id === callerID
            ? {
                ...item,
                keys: [
                  ...item.keys.map((current) =>
                    current.id === keyID
                      ? {
                          ...current,
                          state: "warning" as const,
                          graceUntil,
                          replacedByID: key.id,
                        }
                      : current,
                  ),
                  { ...key, rotatedFromID: keyID },
                ],
              }
            : item,
        ),
      );
      recordAudit(
        "轮换密钥",
        "调用方",
        caller?.name ?? callerID,
        `以“${key.name}”替换“${previousKey?.name ?? keyID}”，旧密钥可用至 ${graceUntil}`,
      );
    },
    revokeCallerKey: (callerID, keyID) => {
      const caller = callers.find((item) => item.id === callerID);
      const key = caller?.keys.find((item) => item.id === keyID);
      setCallers((items) =>
        items.map((item) =>
          item.id === callerID
            ? {
                ...item,
                keys: item.keys.map((current) =>
                  current.id === keyID
                    ? { ...current, state: "disabled" }
                    : current,
                ),
              }
            : item,
        ),
      );
      recordAudit(
        "停用密钥",
        "调用方",
        caller?.name ?? callerID,
        `停用“${key?.name ?? keyID}”访问密钥`,
      );
    },
    addIdentitySource: (identitySource) => {
      setIdentitySources((items) => [...items, identitySource]);
    },
    updateIdentitySource: (identitySource) => {
      setIdentitySources((items) =>
        items.map((item) =>
          item.id === identitySource.id ? identitySource : item,
        ),
      );
    },
    deleteIdentitySource: (identitySourceID) => {
      setIdentitySources((items) =>
        items.filter((item) => item.id !== identitySourceID),
      );
    },
    addPolicy: (policy) => {
      setPolicies((items) => [
        ...items,
        {
          ...policy,
          configState: policy.targets.length ? "active" : "not-applied",
        },
      ]);
      if (!policy.targets.length) {
        recordAudit(
          "创建流量策略",
          "流量策略",
          policy.name,
          "策略已保存，尚未选择生效目标",
        );
        return;
      }
      recordAudit(
        `创建流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        `已选择 ${policy.targets.length} 个生效目标`,
      );
    },
    updatePolicy: (policy) => {
      setPolicies((items) =>
        items.map((item) =>
          item.id === policy.id
            ? {
                ...policy,
                configState: policy.targets.length ? "active" : "not-applied",
              }
            : item,
        ),
      );
      if (!policy.targets.length) {
        recordAudit(
          "更新流量策略",
          "流量策略",
          policy.name,
          "策略已保存，尚未选择生效目标",
        );
        return;
      }
      recordAudit(
        `更新流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        `已选择 ${policy.targets.length} 个生效目标`,
      );
    },
    deletePolicy: (policyID) => {
      const policy = policies.find((item) => item.id === policyID);
      if (!policy) return;
      setPolicies((items) => items.filter((item) => item.id !== policyID));
      recordAudit(
        `删除流量策略“${policy.name}”`,
        "流量策略",
        policy.name,
        "策略已从当前环境移除",
      );
    },
  };

  return {
    state: { callers, identitySources, policies },
    actions,
    setPolicies,
    reset: () => {
      setCallers(initialCallers);
      setIdentitySources(initialIdentitySources);
      setPolicies(initialPolicies);
    },
  };
}

function callerState(quotas: CallerQuota[]) {
  return quotas.some((quota) => quota.used >= quota.limit)
    ? ("warning" as const)
    : ("healthy" as const);
}
