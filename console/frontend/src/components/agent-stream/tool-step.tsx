import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  DownOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { Tag } from 'antd';
import React, { useMemo, useState } from 'react';

import { describeToolValue } from './tool-value';
import { ToolValueSection } from './tool-value-section';
import type { AgentToolRecord, AgentToolStatus } from './types';

interface ToolStepProps {
  tool: AgentToolRecord;
}

const statusPresentation: Record<
  AgentToolStatus,
  { color: string; label: string; icon: React.ReactNode }
> = {
  running: {
    color: 'processing',
    label: '运行中',
    icon: <ClockCircleOutlined spin />,
  },
  success: {
    color: 'success',
    label: '成功',
    icon: <CheckCircleOutlined />,
  },
  error: {
    color: 'error',
    label: '失败',
    icon: <CloseCircleOutlined />,
  },
  cancelled: {
    color: 'default',
    label: '已取消',
    icon: <StopOutlined />,
  },
};

const formatDuration = (durationMs?: number): string => {
  if (durationMs === undefined) return '';
  if (durationMs < 1000) return `${durationMs} ms`;
  return `${(durationMs / 1000).toFixed(1)} s`;
};

export const ToolStep = ({ tool }: ToolStepProps): React.ReactElement => {
  const [expanded, setExpanded] = useState(false);
  const presentation = statusPresentation[tool.status];
  const hasResponse = Object.prototype.hasOwnProperty.call(tool, 'response');
  const responseSummary = useMemo(
    () => (hasResponse ? describeToolValue(tool.response).summary : undefined),
    [hasResponse, tool.response]
  );
  const compactSummary =
    tool.status === 'running'
      ? (tool.progress ?? '等待工具返回…')
      : (responseSummary ?? tool.progress ?? '工具执行结束');
  const duration = formatDuration(tool.durationMs);

  return (
    <div className="overflow-hidden rounded-xl border border-[#dfe3eb] bg-[#f8f9fb] text-[#242933]">
      <button
        type="button"
        aria-expanded={expanded}
        className="flex w-full items-center gap-3 px-3 py-2.5 text-left hover:bg-[#f1f3f7] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#5b5bf7]"
        onClick={() => setExpanded(current => !current)}
      >
        <DownOutlined
          className={`text-xs text-[#7b8494] transition-transform ${
            expanded ? 'rotate-180' : ''
          }`}
        />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium">
            调用工具 {tool.name}
          </span>
          <span className="mt-0.5 block truncate text-xs text-[#7b8494]">
            {compactSummary}
          </span>
        </span>
        {duration ? (
          <span className="text-xs text-[#8b93a1]">{duration}</span>
        ) : null}
        <Tag
          color={presentation.color}
          icon={presentation.icon}
          className="m-0"
        >
          {presentation.label}
        </Tag>
      </button>
      {expanded ? (
        <div className="flex flex-col gap-2 border-t border-[#e5e7eb] p-3">
          <ToolValueSection title="参数 Arguments" value={tool.arguments} />
          {hasResponse ? (
            <ToolValueSection title="响应 Response" value={tool.response} />
          ) : (
            <div className="rounded-lg border border-dashed border-[#dfe3eb] bg-white px-3 py-2 text-xs text-[#8b93a1]">
              等待工具返回…
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
};
