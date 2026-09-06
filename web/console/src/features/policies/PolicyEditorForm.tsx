import {
  saveHeaderTransformationPolicy,
  saveIPRestrictionPolicy,
  saveMockResponsePolicy,
  saveRateLimitPolicy,
  saveTokenQuotaPolicy,
} from '@/api/policies';
import type { GovernancePolicy, GovernancePolicyKind, PolicyTargetOption } from '@/domain/policy';
import {
  createHeaderTransformationPolicyDraft,
  HeaderTransformationPolicyEditor,
  headerTransformationPolicyPayload,
  validateHeaderTransformationPolicyDraft,
  type HeaderTransformationPolicyDraft,
} from './HeaderTransformationPolicyEditor';
import {
  createIPRestrictionPolicyDraft,
  IPRestrictionPolicyEditor,
  ipRestrictionPolicyPayload,
  validateIPRestrictionPolicyDraft,
  type IPRestrictionPolicyDraft,
} from './IPRestrictionPolicyEditor';
import {
  createMockResponsePolicyDraft,
  MockResponsePolicyEditor,
  mockResponsePolicyPayload,
  validateMockResponsePolicyDraft,
  type MockResponsePolicyDraft,
} from './MockResponsePolicyEditor';
import {
  createRateLimitPolicyDraft,
  RateLimitPolicyEditor,
  rateLimitPolicyPayload,
  validateRateLimitPolicyDraft,
  type RateLimitPolicyDraft,
} from './RateLimitPolicyEditor';
import {
  createTokenQuotaPolicyDraft,
  TokenQuotaPolicyEditor,
  tokenQuotaPolicyPayload,
  validateTokenQuotaPolicyDraft,
  type TokenQuotaPolicyDraft,
} from './TokenQuotaPolicyEditor';

export type PolicyEditor =
  | { kind: 'IPRestrictionPolicy'; draft: IPRestrictionPolicyDraft }
  | { kind: 'RateLimitPolicy'; draft: RateLimitPolicyDraft }
  | { kind: 'TokenQuotaPolicy'; draft: TokenQuotaPolicyDraft }
  | { kind: 'HeaderTransformationPolicy'; draft: HeaderTransformationPolicyDraft }
  | { kind: 'MockResponsePolicy'; draft: MockResponsePolicyDraft };

interface PolicyEditorFormProps {
  editor: PolicyEditor;
  targets: PolicyTargetOption[];
  showValidation: boolean;
  onChange: (editor: PolicyEditor) => void;
}

export function PolicyEditorForm({ editor, targets, showValidation, onChange }: PolicyEditorFormProps) {
  switch (editor.kind) {
  case 'IPRestrictionPolicy': {
    const validation = validateIPRestrictionPolicyDraft(editor.draft);
    return (
      <IPRestrictionPolicyEditor
        draft={editor.draft}
        targets={targets}
        validation={{ ...validation, errors: showValidation ? validation.errors : {} }}
        onChange={(draft) => onChange({ kind: editor.kind, draft })}
      />
    );
  }
  case 'RateLimitPolicy': {
    const validation = validateRateLimitPolicyDraft(editor.draft);
    return (
      <RateLimitPolicyEditor
        draft={editor.draft}
        targets={targets}
        validation={{ ...validation, errors: showValidation ? validation.errors : {} }}
        onChange={(draft) => onChange({ kind: editor.kind, draft })}
      />
    );
  }
  case 'TokenQuotaPolicy': {
    const validation = validateTokenQuotaPolicyDraft(editor.draft);
    return (
      <TokenQuotaPolicyEditor
        draft={editor.draft}
        targets={targets}
        validation={{ ...validation, errors: showValidation ? validation.errors : {} }}
        onChange={(draft) => onChange({ kind: editor.kind, draft })}
      />
    );
  }
  case 'HeaderTransformationPolicy': {
    const validation = validateHeaderTransformationPolicyDraft(editor.draft);
    return (
      <HeaderTransformationPolicyEditor
        draft={editor.draft}
        targets={targets}
        validation={{ ...validation, errors: showValidation ? validation.errors : {} }}
        onChange={(draft) => onChange({ kind: editor.kind, draft })}
      />
    );
  }
  case 'MockResponsePolicy': {
    const validation = validateMockResponsePolicyDraft(editor.draft);
    return (
      <MockResponsePolicyEditor
        draft={editor.draft}
        targets={targets}
        validation={{ ...validation, errors: showValidation ? validation.errors : {} }}
        onChange={(draft) => onChange({ kind: editor.kind, draft })}
      />
    );
  }
  }
}

export function createPolicyEditor(kind: GovernancePolicyKind): PolicyEditor {
  switch (kind) {
  case 'IPRestrictionPolicy':
    return { kind, draft: createIPRestrictionPolicyDraft() };
  case 'RateLimitPolicy':
    return { kind, draft: createRateLimitPolicyDraft() };
  case 'TokenQuotaPolicy':
    return { kind, draft: createTokenQuotaPolicyDraft() };
  case 'HeaderTransformationPolicy':
    return { kind, draft: createHeaderTransformationPolicyDraft() };
  case 'MockResponsePolicy':
    return { kind, draft: createMockResponsePolicyDraft() };
  }
}

export function editPolicyEditor(policy: GovernancePolicy): PolicyEditor {
  switch (policy.kind) {
  case 'IPRestrictionPolicy':
    return { kind: policy.kind, draft: createIPRestrictionPolicyDraft(policy.raw) };
  case 'RateLimitPolicy':
    return { kind: policy.kind, draft: createRateLimitPolicyDraft(policy.raw) };
  case 'TokenQuotaPolicy':
    return { kind: policy.kind, draft: createTokenQuotaPolicyDraft(policy.raw) };
  case 'HeaderTransformationPolicy':
    return { kind: policy.kind, draft: createHeaderTransformationPolicyDraft(policy.raw) };
  case 'MockResponsePolicy':
    return { kind: policy.kind, draft: createMockResponsePolicyDraft(policy.raw) };
  }
}

export function validatePolicyEditor(editor: PolicyEditor) {
  switch (editor.kind) {
  case 'IPRestrictionPolicy':
    return validateIPRestrictionPolicyDraft(editor.draft);
  case 'RateLimitPolicy':
    return validateRateLimitPolicyDraft(editor.draft);
  case 'TokenQuotaPolicy':
    return validateTokenQuotaPolicyDraft(editor.draft);
  case 'HeaderTransformationPolicy':
    return validateHeaderTransformationPolicyDraft(editor.draft);
  case 'MockResponsePolicy':
    return validateMockResponsePolicyDraft(editor.draft);
  }
}

export function savePolicyEditor(editor: PolicyEditor) {
  switch (editor.kind) {
  case 'IPRestrictionPolicy':
    return saveIPRestrictionPolicy(ipRestrictionPolicyPayload(editor.draft));
  case 'RateLimitPolicy':
    return saveRateLimitPolicy(rateLimitPolicyPayload(editor.draft));
  case 'TokenQuotaPolicy':
    return saveTokenQuotaPolicy(tokenQuotaPolicyPayload(editor.draft));
  case 'HeaderTransformationPolicy':
    return saveHeaderTransformationPolicy(headerTransformationPolicyPayload(editor.draft));
  case 'MockResponsePolicy':
    return saveMockResponsePolicy(mockResponsePolicyPayload(editor.draft));
  }
}
