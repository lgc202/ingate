import { apiListAll, apiRequest, type PagedResponse } from './client';
import {
  sortConfigurationItems,
  type ConfigurationStatusItem,
  type ConfigurationStatusSummary,
  type ConfigurationStatusView,
} from '@/domain/configuration';

interface ConfigurationItemsResponse extends PagedResponse {
  items?: ConfigurationStatusItem[];
}

export async function getConfigurationSummary(): Promise<ConfigurationStatusSummary> {
  return apiRequest<ConfigurationStatusSummary>('/configuration/summary');
}

export async function getConfigurationStatus(): Promise<ConfigurationStatusView> {
  const [summary, items] = await Promise.all([
    getConfigurationSummary(),
    apiListAll<ConfigurationItemsResponse, ConfigurationStatusItem>(
      '/configuration/items',
      (page) => page.items ?? [],
    ),
  ]);
  return { summary, items: sortConfigurationItems(items) };
}
