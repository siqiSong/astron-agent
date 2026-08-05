import MarkdownRender from '@/components/markdown-render';
import React, { useState } from 'react';

import type { AgentSegment } from './types';

const LONG_REASONING_CHARS = 600;

interface ReasoningStepProps {
  segment: AgentSegment;
  label: string;
  streaming: boolean;
}

export const ReasoningStep = ({
  segment,
  label,
  streaming,
}: ReasoningStepProps): React.ReactElement => {
  const [expanded, setExpanded] = useState(false);
  const long = segment.text.length > LONG_REASONING_CHARS;

  return (
    <article className="border-l-2 border-[#dfe3eb] pl-3">
      <div className="mb-1 text-xs font-medium text-[#5b6472]">{label}</div>
      <div
        className="reasoning-markdown overflow-hidden"
        style={long && !expanded ? { maxHeight: '9rem' } : undefined}
      >
        <MarkdownRender
          content={segment.text}
          isSending={streaming && !segment.ended}
        />
      </div>
      {long ? (
        <button
          type="button"
          aria-expanded={expanded}
          className="mt-1 text-xs text-[#5b5bf7] hover:underline"
          onClick={() => setExpanded(current => !current)}
        >
          {expanded ? '收起' : '展开更多'}
        </button>
      ) : null}
      {segment.partial ? (
        <span className="mt-1 block text-xs text-[#9a6b16]">
          此段内容因任务中断而提前结束
        </span>
      ) : null}
    </article>
  );
};
