import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { UpstreamPage } from '@/features/upstreams/UpstreamPage';
import { CertificatePage } from '@/features/certificates/CertificatePage';
import { PolicyPage } from '@/features/policies/PolicyPage';
import { OverviewPage } from '@/features/overview/OverviewPage';
import { PlaygroundPage } from '@/features/playground/PlaygroundPage';
import { CallerPage } from '@/features/callers/CallerPage';
import { ObservabilityPage } from '@/features/observability/ObservabilityPage';
import { AuditLogPage } from '@/features/product/AuditLogPage';
import { TrafficAnalysisPage } from '@/features/product/TrafficAnalysisPage';
import { HealthAlertPage } from '@/features/product/HealthAlertPage';

export default function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/overview" replace />} />
        <Route path="overview" element={<OverviewPage />} />
        <Route path="gateways" element={<GatewayPage />} />
        <Route path="routes" element={<RoutePage />} />
        <Route path="services" element={<UpstreamPage />} />
        <Route path="certificates" element={<CertificatePage />} />
        <Route path="playground" element={<PlaygroundPage />} />
        <Route path="callers" element={<CallerPage />} />
        <Route path="policies" element={<PolicyPage />} />
        <Route path="audit" element={<AuditLogPage />} />
        <Route path="requests" element={<ObservabilityPage />} />
        <Route path="analysis" element={<TrafficAnalysisPage />} />
        <Route path="health" element={<HealthAlertPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/overview" replace />} />
    </Routes>
  );
}
