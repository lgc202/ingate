import type { ConsoleRepository } from './contracts';
import { liveConsoleRepository } from './liveConsoleRepository';
import { mockConsoleRepository } from '@/mocks/consoleRepository';

export type ApiMode = 'mock' | 'live';

export const apiMode = (import.meta.env.VITE_INGATE_API_MODE as ApiMode | undefined) ?? 'live';

export function getConsoleRepository(): ConsoleRepository {
  if (apiMode === 'live') {
    return liveConsoleRepository;
  }

  return mockConsoleRepository;
}

export const consoleRepository = getConsoleRepository();
