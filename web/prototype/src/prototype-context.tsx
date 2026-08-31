import { createContext, useContext, type ReactNode } from "react";
import type { PrototypeContextValue } from "./prototype-context-value";
import { useGovernanceState } from "./state/governance-state";
import { useOperationsState } from "./state/operations-state";
import { useTrafficState } from "./state/traffic-state";

const PrototypeContext = createContext<PrototypeContextValue | null>(null);

export function PrototypeProvider({ children }: { children: ReactNode }) {
  const operations = useOperationsState();
  const governance = useGovernanceState(operations.recordAudit);
  const traffic = useTrafficState(
    operations.recordAudit,
    governance.setPolicies,
  );

  const value: PrototypeContextValue = {
    ...operations.state,
    ...governance.state,
    ...governance.actions,
    ...traffic.state,
    ...traffic.actions,
    resetDemo: () => {
      traffic.reset();
      governance.reset();
      operations.reset();
    },
  };

  return (
    <PrototypeContext.Provider value={value}>
      {children}
    </PrototypeContext.Provider>
  );
}

export function usePrototype() {
  const context = useContext(PrototypeContext);
  if (!context) throw new Error("PrototypeProvider is required");
  return context;
}
