import { apiRequest } from './client';
import type { ConfigurationStatusView } from '@/domain/configuration';

export async function getConfigurationStatus(): Promise<ConfigurationStatusView> {
  return apiRequest<ConfigurationStatusView>('/configuration/status');
}
