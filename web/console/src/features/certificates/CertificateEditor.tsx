import { useRef, type Dispatch, type RefObject, type SetStateAction } from 'react';
import { FileText } from 'lucide-react';

export type CertificateInputMode = 'upload' | 'paste';

export interface CertificateDraft {
  id?: string;
  version?: number;
  name: string;
  certificatePEM: string;
  privateKeyPEM: string;
}

interface CertificateEditorProps {
  draft: CertificateDraft;
  inputMode: CertificateInputMode;
  isEditing: boolean;
  submitting: boolean;
  submitError: string | null;
  onChange: Dispatch<SetStateAction<CertificateDraft>>;
  onInputModeChange: (mode: CertificateInputMode) => void;
  onError: (message: string | null) => void;
  onCancel: () => void;
  onSave: () => void;
}

const maxPEMFileSize = 1024 * 1024;

export function CertificateEditor({
  draft,
  inputMode,
  isEditing,
  submitting,
  submitError,
  onChange,
  onInputModeChange,
  onError,
  onCancel,
  onSave,
}: CertificateEditorProps) {
  const certFileInputRef = useRef<HTMLInputElement>(null);
  const keyFileInputRef = useRef<HTMLInputElement>(null);

  const handleFileUpload = (type: 'cert' | 'key', file?: File) => {
    if (!file) return;
    if (file.size > maxPEMFileSize) {
      onError('PEM 文件大小不能超过 1 MB');
      return;
    }
    onError(null);
    const reader = new FileReader();
    reader.onload = () => {
      const text = String(reader.result ?? '');
      onChange((current) => type === 'cert'
        ? { ...current, certificatePEM: text }
        : { ...current, privateKeyPEM: text });
    };
    reader.onerror = () => onError('读取 PEM 文件失败');
    reader.readAsText(file);
  };

  return (
    <div className="space-y-5">
      {submitError ? (
        <div className="p-3 bg-rose-50 border border-rose-200 text-rose-700 text-xs rounded-lg">
          {submitError}
        </div>
      ) : null}

      <div className="space-y-4">
        <div>
          <label className="block text-xs font-semibold text-slate-700 mb-1">
            证书展示名称 <span className="text-rose-500">*</span>
          </label>
          <input
            type="text"
            value={draft.name}
            onChange={(event) => onChange((current) => ({ ...current, name: event.target.value }))}
            placeholder="例如: *.example.com 通配符证书"
            className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-blue-500/20"
          />
        </div>

        {!isEditing ? (
          <div className="space-y-4">
            <div className="flex items-center gap-4 text-xs">
              <label className="flex items-center gap-2 cursor-pointer font-medium text-slate-700">
                <input
                  type="radio"
                  name="inputMode"
                  checked={inputMode === 'upload'}
                  onChange={() => onInputModeChange('upload')}
                  className="text-blue-600"
                />
                上传 PEM 文件
              </label>
              <label className="flex items-center gap-2 cursor-pointer font-medium text-slate-700">
                <input
                  type="radio"
                  name="inputMode"
                  checked={inputMode === 'paste'}
                  onChange={() => onInputModeChange('paste')}
                  className="text-blue-600"
                />
                直接粘贴 PEM 文本
              </label>
            </div>

            {inputMode === 'upload' ? (
              <div className="grid grid-cols-2 gap-4">
                <PEMFileInput
                  kind="certificate"
                  loaded={Boolean(draft.certificatePEM)}
                  inputRef={certFileInputRef}
                  onFile={(file) => handleFileUpload('cert', file)}
                />
                <PEMFileInput
                  kind="privateKey"
                  loaded={Boolean(draft.privateKeyPEM)}
                  inputRef={keyFileInputRef}
                  onFile={(file) => handleFileUpload('key', file)}
                />
              </div>
            ) : (
              <div className="space-y-3">
                <PEMTextarea
                  label="证书内容 (BEGIN CERTIFICATE)"
                  value={draft.certificatePEM}
                  placeholder="-----BEGIN CERTIFICATE-----"
                  onChange={(value) => onChange((current) => ({ ...current, certificatePEM: value }))}
                />
                <PEMTextarea
                  label="私钥内容 (BEGIN PRIVATE KEY)"
                  value={draft.privateKeyPEM}
                  placeholder="-----BEGIN PRIVATE KEY-----"
                  onChange={(value) => onChange((current) => ({ ...current, privateKeyPEM: value }))}
                />
              </div>
            )}
          </div>
        ) : null}
      </div>

      <div className="pt-4 border-t border-slate-200 flex items-center justify-end gap-3">
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors cursor-pointer"
        >
          取消
        </button>
        <button
          type="button"
          disabled={submitting}
          onClick={onSave}
          className="px-4 py-2 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-xs transition-colors disabled:opacity-50 cursor-pointer"
        >
          {submitting ? '保存中...' : '保存证书'}
        </button>
      </div>
    </div>
  );
}

export function emptyCertificateDraft(): CertificateDraft {
  return {
    name: '',
    certificatePEM: '',
    privateKeyPEM: '',
  };
}

export function validateCertificateDraft(draft: CertificateDraft, mode: 'create' | 'edit'): string | null {
  if (!draft.name.trim()) return '请输入证书展示名称';
  if (mode === 'create') {
    if (!draft.certificatePEM.trim()) return '请提供证书内容 (PEM)';
    if (!draft.privateKeyPEM.trim()) return '请提供证书私钥内容 (PEM)';
  }
  return null;
}

function PEMFileInput({
  kind,
  loaded,
  inputRef,
  onFile,
}: {
  kind: 'certificate' | 'privateKey';
  loaded: boolean;
  inputRef: RefObject<HTMLInputElement | null>;
  onFile: (file?: File) => void;
}) {
  const isCertificate = kind === 'certificate';
  const label = isCertificate ? '证书' : '私钥';

  return (
    <div className="p-4 bg-slate-50 border border-slate-200 rounded-xl space-y-2 text-center">
      <FileText className="w-5 h-5 text-slate-400 mx-auto" />
      <div className="text-xs font-medium text-slate-700">{isCertificate ? '证书文件 (.crt / .pem)' : '私钥 Key (.key / .pem)'}</div>
      <input
        ref={inputRef}
        type="file"
        accept={isCertificate ? '.pem,.crt,.cer' : '.pem,.key'}
        className="hidden"
        onChange={(event) => onFile(event.target.files?.[0])}
      />
      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        className="px-3 py-1.5 bg-white border border-slate-300 text-xs font-medium rounded-lg hover:bg-slate-50 cursor-pointer"
      >
        选择{label}文件
      </button>
      {loaded ? <p className={`text-[10px] text-emerald-600${isCertificate ? '' : ' font-mono'}`}>已加载{label}文件</p> : null}
    </div>
  );
}

function PEMTextarea({ label, value, placeholder, onChange }: { label: string; value: string; placeholder: string; onChange: (value: string) => void }) {
  return (
    <div>
      <label className="block text-xs font-semibold text-slate-700 mb-1">{label}</label>
      <textarea
        rows={5}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="w-full p-2.5 font-mono text-[11px] border border-slate-300 rounded-lg focus:outline-hidden"
      />
    </div>
  );
}
