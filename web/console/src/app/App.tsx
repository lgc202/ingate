import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { UpstreamPage } from '@/features/upstreams/UpstreamPage';
import { CertificatePage } from '@/features/certificates/CertificatePage';
import { AccessKeyPage } from '@/features/accessKeys/AccessKeyPage';
import { PolicyPage } from '@/features/policies/PolicyPage';
import { ConfigurationStatusPage } from '@/features/configuration/ConfigurationStatusPage';
import { useAuth } from '@/auth/AuthContext';

export default function App() {
  const { isAdmin } = useAuth();
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/gateways" replace />} />
        <Route path="gateways" element={<GatewayPage />} />
        <Route path="routes" element={<RoutePage />} />
        <Route path="services" element={<UpstreamPage />} />
        <Route path="certificates" element={<CertificatePage />} />
        {isAdmin && <Route path="access-keys" element={<AccessKeyPage />} />}
        <Route path="policies" element={<PolicyPage />} />
        <Route path="status" element={<ConfigurationStatusPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/gateways" replace />} />
    </Routes>
  );
}
