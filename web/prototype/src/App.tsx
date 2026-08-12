import { Navigate, Route, Routes } from "react-router-dom";
import { Shell } from "./components/Shell";
import { OverviewPage } from "./pages/OverviewPage";
import {
  CertificatePage,
  GatewayPage,
  RoutePage,
  ServicePage,
} from "./pages/TrafficPages";
import { CallerPage, PolicyPage } from "./pages/GovernancePages";
import {
  AnalysisPage,
  AuditPage,
  RequestPage,
  UsagePage,
} from "./pages/OperationsPages";

export default function App() {
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route index element={<Navigate to="/overview" replace />} />
        <Route path="overview" element={<OverviewPage />} />
        <Route path="gateways" element={<GatewayPage />} />
        <Route path="routes" element={<RoutePage />} />
        <Route path="services" element={<ServicePage />} />
        <Route path="certificates" element={<CertificatePage />} />
        <Route path="callers" element={<CallerPage />} />
        <Route path="policies" element={<PolicyPage />} />
        <Route path="requests" element={<RequestPage />} />
        <Route path="analysis" element={<AnalysisPage />} />
        <Route path="usage" element={<UsagePage />} />
        <Route path="audit" element={<AuditPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/overview" replace />} />
    </Routes>
  );
}
