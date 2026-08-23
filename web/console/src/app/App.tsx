import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { UpstreamPage } from '@/features/upstreams/UpstreamPage';
import { CertificatePage } from '@/features/certificates/CertificatePage';
import { PolicyPage } from '@/features/policies/PolicyPage';
import { RequestRecordPage } from '@/features/requests/RequestRecordPage';
import { TrafficAnalysisPage } from '@/features/traffic/TrafficAnalysisPage';
import { CallerPage } from '@/features/callers/CallerPage';
import { AIUsagePage } from '@/features/aiusage/AIUsagePage';
import { PluginPage } from '@/features/plugins/PluginPage';
import { SessionProvider } from '@/features/auth/SessionProvider';

export default function App() {
  return (
    <SessionProvider>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<Navigate to="/gateways" replace />} />
          <Route path="gateways" element={<GatewayPage />} />
          <Route path="routes" element={<RoutePage />} />
          <Route path="services" element={<UpstreamPage />} />
          <Route path="certificates" element={<CertificatePage />} />
          <Route path="policies" element={<PolicyPage />} />
          <Route path="plugins" element={<PluginPage />} />
          <Route path="callers" element={<CallerPage />} />
          <Route path="requests" element={<RequestRecordPage />} />
          <Route path="analysis" element={<TrafficAnalysisPage />} />
          <Route path="ai-usage" element={<AIUsagePage />} />
        </Route>
        <Route path="*" element={<Navigate to="/gateways" replace />} />
      </Routes>
    </SessionProvider>
  );
}
