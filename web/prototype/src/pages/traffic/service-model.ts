export function normalizedAuthentication(authentication?: string) {
  if (authentication?.startsWith("mTLS")) return "mTLS";
  return authentication ?? "无认证";
}

export function summarizeItems(items: string[], limit = 3) {
  const visible = items.slice(0, limit).join("、");
  return items.length > limit ? `${visible} 等 ${items.length} 项` : visible;
}
