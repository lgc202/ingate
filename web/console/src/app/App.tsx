import { lazy } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from './layout/AppShell';
import { SessionProvider } from '@/features/auth/SessionProvider';

const GatewayPage = lazy(() => import('@/features/gateways/GatewayPage')
  .then((module) => ({ default: module.GatewayPage })));
const RoutePage = lazy(() => import('@/features/routes/RoutePage')
  .then((module) => ({ default: module.RoutePage })));
const ServicePage = lazy(() => import('@/features/services/ServicePage')
  .then((module) => ({ default: module.ServicePage })));
const CertificatePage = lazy(() => import('@/features/certificates/CertificatePage')
  .then((module) => ({ default: module.CertificatePage })));
const PolicyPage = lazy(() => import('@/features/policies/PolicyPage')
  .then((module) => ({ default: module.PolicyPage })));
const PluginPage = lazy(() => import('@/features/plugins/PluginPage')
  .then((module) => ({ default: module.PluginPage })));
const CallerPage = lazy(() => import('@/features/callers/CallerPage')
  .then((module) => ({ default: module.CallerPage })));
const RequestRecordPage = lazy(() => import('@/features/requests/RequestRecordPage')
  .then((module) => ({ default: module.RequestRecordPage })));
const TrafficAnalysisPage = lazy(() => import('@/features/traffic/TrafficAnalysisPage')
  .then((module) => ({ default: module.TrafficAnalysisPage })));
const AIUsagePage = lazy(() => import('@/features/aiusage/AIUsagePage')
  .then((module) => ({ default: module.AIUsagePage })));
const AssistantPage = lazy(() => import('@/features/assistant/AssistantPage')
  .then((module) => ({ default: module.AssistantPage })));

export default function App() {
  return (
    <SessionProvider>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<Navigate to="/gateways" replace />} />
          <Route path="gateways" element={<GatewayPage />} />
          <Route path="routes" element={<RoutePage />} />
          <Route path="services" element={<ServicePage />} />
          <Route path="certificates" element={<CertificatePage />} />
          <Route path="policies" element={<PolicyPage />} />
          <Route path="plugins" element={<PluginPage />} />
          <Route path="callers" element={<CallerPage />} />
          <Route path="requests" element={<RequestRecordPage />} />
          <Route path="analysis" element={<TrafficAnalysisPage />} />
          <Route path="ai-usage" element={<AIUsagePage />} />
          <Route path="assistant" element={<AssistantPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/gateways" replace />} />
      </Routes>
    </SessionProvider>
  );
}
