import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { HomePage } from '@/features/home/HomePage';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { ServicePage } from '@/features/services/ServicePage';
import { PublishPage } from '@/features/publish/PublishPage';
import { PolicyPage } from '@/features/policies/PolicyPage';
import { PluginPage } from '@/features/plugins/PluginPage';
import { SettingsPage } from '@/features/settings/SettingsPage';
import { InsightPage } from '@/features/insights/InsightPage';

export default function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<HomePage />} />
        <Route path="traffic">
          <Route index element={<Navigate to="gateways" replace />} />
          <Route path="gateways" element={<GatewayPage />} />
          <Route path="routes" element={<RoutePage />} />
          <Route path="services" element={<ServicePage />} />
          <Route path="runtime" element={<PublishPage />} />
          <Route path="publish" element={<Navigate to="runtime" replace />} />
          <Route path="policies" element={<PolicyPage />} />
          <Route path="plugins" element={<PluginPage />} />
        </Route>
        <Route path="assets" element={<InsightPage kind="assets" />} />
        <Route path="risks" element={<InsightPage kind="risks" />} />
        <Route path="governance" element={<InsightPage kind="governance" />} />
        <Route path="strategy" element={<Navigate to="/governance" replace />} />
        <Route path="system">
          <Route index element={<Navigate to="users" replace />} />
          <Route path=":section" element={<SettingsPage />} />
        </Route>
        <Route path="gateways" element={<Navigate to="/traffic/gateways" replace />} />
        <Route path="routes" element={<Navigate to="/traffic/routes" replace />} />
        <Route path="services" element={<Navigate to="/traffic/services" replace />} />
        <Route path="policies" element={<Navigate to="/traffic/policies" replace />} />
        <Route path="plugins" element={<Navigate to="/traffic/plugins" replace />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
