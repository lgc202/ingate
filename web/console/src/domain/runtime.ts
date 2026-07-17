export type DeliveryState = 'NoConfig' | 'WaitingForEnvoy' | 'WaitingForACK' | 'Active' | 'Degraded';

export interface RuntimeACKSummary {
  required: number;
  received: number;
}

export interface RuntimeNACK {
  nodeID: string;
  typeURL: string;
  version: string;
  time: string;
  message: string;
}

export interface RuntimeStatusView {
  available: boolean;
  message: string;
  configReady: boolean;
  deliveryState: DeliveryState;
  candidateVersion?: string;
  activeVersion?: string;
  connectedEnvoys: number;
  ack: RuntimeACKSummary;
  lastNack?: RuntimeNACK;
}

export function deliveryStateLabel(state: DeliveryState) {
  const labels: Record<DeliveryState, string> = {
    NoConfig: '暂无配置',
    WaitingForEnvoy: '等待 Envoy 连接',
    WaitingForACK: '等待 Envoy 确认',
    Active: '配置已生效',
    Degraded: '降级运行',
  };

  return labels[state];
}

export function deliveryStateTone(state: DeliveryState): 'green' | 'amber' | 'red' | 'neutral' {
  if (state === 'Active') {
    return 'green';
  }
  if (state === 'WaitingForEnvoy' || state === 'WaitingForACK') {
    return 'amber';
  }
  if (state === 'Degraded') {
    return 'red';
  }
  return 'neutral';
}
