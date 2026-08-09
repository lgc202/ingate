import { apiRequest } from './client';

export interface AuthenticationConfiguration {
  enabled: boolean;
  issuer: string;
  clientID: string;
  scopes: string[];
}

export type PrincipalRole = 'viewer' | 'operator' | 'admin';

export interface CurrentPrincipal {
  subject: string;
  name: string;
  email: string;
  role: PrincipalRole;
}

export function getAuthenticationConfiguration(): Promise<AuthenticationConfiguration> {
  return apiRequest<AuthenticationConfiguration>('/auth/config');
}

export function getCurrentPrincipal(): Promise<CurrentPrincipal> {
  return apiRequest<CurrentPrincipal>('/auth/me');
}
