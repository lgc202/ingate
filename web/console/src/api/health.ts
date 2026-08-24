import { apiRequest } from './client';

interface Health {
  status: string;
  requestID: string;
  version: string;
}

export async function getHealth(): Promise<Health> {
  return apiRequest<Health>('/healthz');
}
