import type { AgentStreamState } from './types';

export type ExecutionPanelStatus =
  | 'running'
  | 'success'
  | 'error'
  | 'cancelled'
  | 'transport_closed';

export interface ExecutionPresentation {
  status: ExecutionPanelStatus;
  statusLabel: string;
  toolCount: number;
  durationMs?: number;
}

const statusLabels: Record<ExecutionPanelStatus, string> = {
  running: '正在执行',
  success: '执行完成',
  error: '执行失败',
  cancelled: '已取消',
  transport_closed: '连接中断',
};

export const deriveExecutionPresentation = (
  state: AgentStreamState,
  isStreaming: boolean
): ExecutionPresentation => {
  const executions = Object.values(state.executions);
  let status: ExecutionPanelStatus = 'success';
  if (isStreaming) status = 'running';
  else if (state.interruptionReason === 'transport_closed') {
    status = 'transport_closed';
  } else if (
    state.interruptionReason === 'error' ||
    executions.some(execution => execution.status === 'error')
  ) {
    status = 'error';
  } else if (
    state.interruptionReason === 'cancelled' ||
    executions.some(execution => execution.status === 'cancelled')
  ) {
    status = 'cancelled';
  } else if (executions.some(execution => execution.status === 'running')) {
    status = 'transport_closed';
  }

  const durationMs =
    executions.length === 1 ? executions[0]?.durationMs : undefined;

  return {
    status,
    statusLabel: statusLabels[status],
    toolCount: Object.keys(state.tools).length,
    durationMs,
  };
};

export interface PanelDisclosureState {
  expanded: boolean;
  wasActive: boolean;
  autoCollapsed: boolean;
}

export type PanelDisclosureAction =
  | { type: 'toggle' }
  | { type: 'activity_changed'; active: boolean };

export const createPanelDisclosure = (
  active: boolean
): PanelDisclosureState => ({
  expanded: active,
  wasActive: active,
  autoCollapsed: !active,
});

export const reducePanelDisclosure = (
  state: PanelDisclosureState,
  action: PanelDisclosureAction
): PanelDisclosureState => {
  if (action.type === 'toggle') {
    return { ...state, expanded: !state.expanded };
  }
  if (!state.wasActive && action.active) {
    return {
      expanded: true,
      wasActive: true,
      autoCollapsed: false,
    };
  }
  if (state.wasActive && !action.active && !state.autoCollapsed) {
    return {
      expanded: false,
      wasActive: false,
      autoCollapsed: true,
    };
  }
  return { ...state, wasActive: action.active };
};
