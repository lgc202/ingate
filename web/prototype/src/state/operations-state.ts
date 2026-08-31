import { useState } from "react";
import {
  initialAuditRecords,
  initialRequests,
  type AuditRecord,
} from "../data";

export type RecordAudit = (
  action: string,
  resourceType: AuditRecord["resourceType"],
  resource: string,
  detail: string,
) => void;

export function useOperationsState() {
  const [requests, setRequests] = useState(initialRequests);
  const [auditRecords, setAuditRecords] = useState(initialAuditRecords);

  const recordAudit: RecordAudit = (action, resourceType, resource, detail) => {
    setAuditRecords((records) => [
      {
        id: crypto.randomUUID(),
        time: currentTime(),
        actor: "林工程师",
        action,
        resourceType,
        resource,
        detail,
        result: "成功",
      },
      ...records,
    ]);
  };

  return {
    state: { requests, auditRecords },
    recordAudit,
    reset: () => {
      setRequests(initialRequests);
      setAuditRecords(initialAuditRecords);
    },
  };
}

function currentTime() {
  return new Date().toLocaleTimeString("zh-CN", { hour12: false });
}
