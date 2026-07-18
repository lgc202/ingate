import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { UpstreamPage } from '@/features/upstreams/UpstreamPage';
import { ConfigurationStatusPage } from '@/features/configuration/ConfigurationStatusPage';
import { PolicyPage } from '@/features/policies/PolicyPage';
import { CertificatePage } from '@/features/certificates/CertificatePage';
import { CredentialPage } from '@/features/credentials/CredentialPage';

export default function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/gateways" replace />} />
        <Route path="gateways" element={<GatewayPage />} />
        <Route path="certificates" element={<CertificatePage />} />
        <Route path="routes" element={<RoutePage />} />
        <Route path="services" element={<UpstreamPage />} />
        <Route path="credentials" element={<CredentialPage />} />
        <Route path="policies" element={<PolicyPage />} />
        <Route path="status" element={<ConfigurationStatusPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/gateways" replace />} />
    </Routes>
  );
}
