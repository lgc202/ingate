import type { ResourceStatus } from './common';

export interface Certificate {
  id: string;
  name: string;
  certificatePEM: string;
  dnsNames: string[];
  notBefore: string;
  notAfter: string;
  state: ResourceStatus['state'];
  message: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface CertificateListView {
  certificates: Certificate[];
}

export interface CertificateMutationPayload {
  id?: string;
  version?: number;
  name: string;
  certificatePEM?: string;
  privateKeyPEM?: string;
}
