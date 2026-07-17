import type { HealthStatus } from './common';

export interface SettingsTable {
  title: string;
  subtitle: string;
  headers: string[];
  rows: string[][];
}

export interface ToggleGroup {
  title: string;
  subtitle?: string;
  items: {
    label: string;
    enabled: boolean;
  }[];
}

export interface SettingsSectionData {
  key: string;
  title: string;
  table?: SettingsTable;
  toggleGroups?: ToggleGroup[];
  keyValues?: {
    label: string;
    value: string;
    status?: HealthStatus;
  }[];
}

export interface SettingsInspectorData {
  configurationDomain: {
    label: string;
    value: string;
    status?: HealthStatus;
  }[];
  envoyHealth: {
    label: string;
    status: HealthStatus;
  }[];
  securityBaseline: {
    label: string;
    status: HealthStatus;
  }[];
}

export interface SettingsWorkspace {
  sections: Record<string, SettingsSectionData>;
  inspector: SettingsInspectorData;
}
