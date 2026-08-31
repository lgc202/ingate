export type PermissionDraft = Record<string, string[]>;
export type CallerSection = "identity" | "permissions" | "quotas" | "activity";
export type CallerStatusFilter = "ALL" | "ENABLED" | "DISABLED";
export type CallerAttentionFilter = "ALL" | "NEEDS_ATTENTION";

export interface CallerAttention {
  section: "identity" | "permissions" | "quotas";
  label: string;
  detail: string;
}
