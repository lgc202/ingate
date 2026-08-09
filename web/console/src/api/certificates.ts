import { apiListAll, apiRequest } from './client';
import type { PagedResponse } from './client';
import type {
  Certificate,
  CertificateListView,
  CertificateMutationPayload,
  CertificateMutationResult,
} from '@/domain/certificate';

interface CertificateListResponse extends PagedResponse {
  certificates?: Certificate[];
}

interface CertificateMutationResponse {
  success: boolean;
  id?: string;
}

export async function listCertificates(): Promise<CertificateListView> {
  const certificates = await apiListAll<CertificateListResponse, Certificate>(
    '/certificates',
    (page) => page.certificates ?? [],
  );
  return { certificates };
}

export async function saveCertificate(payload: CertificateMutationPayload): Promise<CertificateMutationResult> {
  const path = payload.id ? `/certificates/${encodeURIComponent(payload.id)}` : '/certificates';
  const response = await apiRequest<CertificateMutationResponse>(path, {
    method: payload.id ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });
  return { id: response.id ?? payload.id };
}

export async function deleteCertificate(id: string) {
  await apiRequest<CertificateMutationResponse>(`/certificates/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
