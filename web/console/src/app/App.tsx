import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { UpstreamPage } from '@/features/upstreams/UpstreamPage';
import { CertificatePage } from '@/features/certificates/CertificatePage';
import { PolicyPage } from '@/features/policies/PolicyPage';

export default function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/gateways" replace />} />
        <Route path="gateways" element={<GatewayPage />} />
        <Route path="routes" element={<RoutePage />} />
        <Route path="services" element={<UpstreamPage />} />
        <Route path="certificates" element={<CertificatePage />} />
        <Route path="policies" element={<PolicyPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/gateways" replace />} />
    </Routes>
  );
}
