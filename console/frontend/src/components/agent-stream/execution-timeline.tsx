import React from 'react';

import { selectReasoningTimeline } from './reducer';
import { ReasoningStep } from './reasoning-step';
import { ToolStep } from './tool-step';
import type { AgentStreamState } from './types';

interface ExecutionTimelineProps {
  state: AgentStreamState;
  isStreaming: boolean;
}

export const ExecutionTimeline = ({
  state,
  isStreaming,
}: ExecutionTimelineProps): React.ReactElement => {
  const timeline = selectReasoningTimeline(state);
  let reasoningIndex = 0;

  return (
    <div className="flex flex-col gap-3 border-t border-[#e5e7eb] px-4 py-3 text-sm text-[#5b6472]">
      {timeline.map(item => {
        if (item.kind === 'tool') {
          return (
            <ToolStep key={`${item.runId}:${item.callId}`} tool={item.tool} />
          );
        }
        const label = reasoningIndex++ === 0 ? '任务分析' : '继续分析';
        return (
          <ReasoningStep
            key={`${item.runId}:${item.segmentId}`}
            segment={item}
            label={label}
            streaming={isStreaming}
          />
        );
      })}
    </div>
  );
};
