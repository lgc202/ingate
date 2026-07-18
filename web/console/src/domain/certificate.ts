import type { ResourceStatus } from './common';

export interface Certificate {
  id: string;
  version?: string;
  name: string;
  description: string;
  certificatePEM?: string;
  dnsNames: string[];
  notBefore: string;
  notAfter: string;
  status: ResourceStatus;
  createdAt: string;
}

export interface CertificateListView {
  certificates: Certificate[];
}

export interface CertificateMutationPayload {
  id?: string;
  version?: string;
  name: string;
  description: string;
  certificatePEM: string;
  privateKeyPEM: string;
}

export interface CertificateMutationResult {
  id?: string;
}
