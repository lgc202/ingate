export type ResourceState = 'Pending' | 'Ready' | 'Error' | 'Disabled';

export interface ResourceStatus {
  state: ResourceState;
  message: string;
}

export function formatDateTime(value: string) {
  if (!value) {
    return '-';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hour = String(date.getHours()).padStart(2, '0');
  const minute = String(date.getMinutes()).padStart(2, '0');

  return `${year}-${month}-${day} ${hour}:${minute}`;
}

export function normalizeResourceState(value: unknown): ResourceState {
  const states: Record<string, ResourceState> = {
    DISABLED: 'Disabled',
    PENDING: 'Pending',
    READY: 'Ready',
    ERROR: 'Error',
  };
  return states[String(value)] ?? 'Pending';
}
