import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { HomePage } from '@/features/home/HomePage';
import { GatewayPage } from '@/features/gateways/GatewayPage';
import { RoutePage } from '@/features/routes/RoutePage';
import { ServicePage } from '@/features/services/ServicePage';
import { RuntimePage } from '@/features/runtime/RuntimePage';
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
          <Route path="runtime" element={<RuntimePage />} />
          <Route path="policies" element={<PolicyPage />} />
          <Route path="plugins" element={<PluginPage />} />
        </Route>
        <Route path="assets" element={<InsightPage kind="assets" />} />
        <Route path="risks" element={<InsightPage kind="risks" />} />
        <Route path="governance" element={<InsightPage kind="governance" />} />
        <Route path="system">
          <Route index element={<Navigate to="users" replace />} />
          <Route path=":section" element={<SettingsPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
