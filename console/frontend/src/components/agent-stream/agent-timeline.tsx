import React from 'react';

import { AgentExecutionPanel } from './agent-execution-panel';
import type { AgentStreamState } from './types';

interface AgentTimelineProps {
  state: AgentStreamState;
  isStreaming: boolean;
}

export const AgentTimeline = (
  props: AgentTimelineProps
): React.ReactElement | null => <AgentExecutionPanel {...props} />;
