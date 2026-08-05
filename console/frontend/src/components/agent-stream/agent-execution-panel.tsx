import React, { useEffect, useReducer } from 'react';

import { ExecutionPanelHeader } from './execution-panel-header';
import { ExecutionTimeline } from './execution-timeline';
import {
  createPanelDisclosure,
  deriveExecutionPresentation,
  reducePanelDisclosure,
} from './presentation';
import type { AgentStreamState } from './types';

interface AgentExecutionPanelProps {
  state: AgentStreamState;
  isStreaming: boolean;
}

export const AgentExecutionPanel = ({
  state,
  isStreaming,
}: AgentExecutionPanelProps): React.ReactElement | null => {
  const presentation = deriveExecutionPresentation(state, isStreaming);
  const active = presentation.status === 'running';
  const [disclosure, dispatch] = useReducer(
    reducePanelDisclosure,
    active,
    createPanelDisclosure
  );

  useEffect(() => {
    dispatch({ type: 'activity_changed', active });
  }, [active]);

  if (!state.hasStructuredEvents) return null;

  return (
    <section className="my-2.5 overflow-hidden rounded-xl border border-[#dfe3eb] bg-[#fafbfc]">
      <ExecutionPanelHeader
        expanded={disclosure.expanded}
        presentation={presentation}
        onToggle={() => dispatch({ type: 'toggle' })}
      />
      {disclosure.expanded ? (
        <ExecutionTimeline state={state} isStreaming={isStreaming} />
      ) : null}
    </section>
  );
};
