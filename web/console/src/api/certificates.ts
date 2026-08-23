import { apiListAllByCursor, apiListPageByCursor, apiRequest, type CursorPage, type CursorPagedResponse } from './client';
import { normalizeResourceState } from '@/domain/common';
import type { Certificate, CertificateListView, CertificateMutationPayload } from '@/domain/certificate';

interface CertificateListResponse extends CursorPagedResponse {
  certificates?: Certificate[];
}

export async function listCertificates(): Promise<CertificateListView> {
  const certificates = await apiListAllByCursor<CertificateListResponse, Certificate>('/certificates', (page) => page.certificates ?? []);
  return {
    certificates: certificates.map((certificate) => ({
      ...certificate,
      version: Number(certificate.version),
      dnsNames: certificate.dnsNames ?? [],
      state: normalizeResourceState(certificate.state),
    })),
  };
}

export async function listCertificatePage(input: {
  limit: number; cursor: string; query?: string; state?: string;
}): Promise<CursorPage<Certificate>> {
  const page = await apiListPageByCursor<CertificateListResponse, Certificate>('/certificates', input, (value) => value.certificates ?? []);
  return { ...page, items: page.items.map(certificateFromAPI) };
}

function certificateFromAPI(certificate: Certificate): Certificate {
  return {
    ...certificate,
    version: Number(certificate.version),
    dnsNames: certificate.dnsNames ?? [],
    state: normalizeResourceState(certificate.state),
  };
}

export async function saveCertificate(payload: CertificateMutationPayload): Promise<Certificate> {
  const path = payload.id ? `/certificates/${encodeURIComponent(payload.id)}` : '/certificates';
  const certificate = await apiRequest<Certificate>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
  return { ...certificate, version: Number(certificate.version), state: normalizeResourceState(certificate.state) };
}

export async function deleteCertificate(id: string, version: number) {
  await apiRequest<Record<string, never>>(`/certificates/${encodeURIComponent(id)}?version=${version}`, { method: 'DELETE' });
}
