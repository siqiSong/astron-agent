import { DownOutlined } from '@ant-design/icons';
import React from 'react';

import type { ExecutionPresentation } from './presentation';

interface ExecutionPanelHeaderProps {
  expanded: boolean;
  presentation: ExecutionPresentation;
  onToggle: () => void;
}

export const ExecutionPanelHeader = ({
  expanded,
  presentation,
  onToggle,
}: ExecutionPanelHeaderProps): React.ReactElement => (
  <button
    type="button"
    aria-expanded={expanded}
    className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-[#f4f6fa] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#5b5bf7]"
    onClick={onToggle}
  >
    <DownOutlined
      className={`text-xs text-[#7b8494] transition-transform ${
        expanded ? 'rotate-180' : ''
      }`}
    />
    <span className="min-w-0 flex-1">
      <span className="block text-sm font-medium text-[#242933]">
        任务分析与执行过程
      </span>
      <span className="mt-0.5 block text-xs text-[#7b8494]">
        {presentation.statusLabel} · 已调用 {presentation.toolCount} 个工具
      </span>
    </span>
  </button>
);
