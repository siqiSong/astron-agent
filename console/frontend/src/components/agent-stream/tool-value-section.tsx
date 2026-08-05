import { CopyOutlined } from '@ant-design/icons';
import copy from 'copy-to-clipboard';
import React, { useMemo, useState } from 'react';

import { describeToolValue } from './tool-value';

export interface ToolValueSectionProps {
  title: string;
  value: unknown;
}

export const ToolValueSection = ({
  title,
  value,
}: ToolValueSectionProps): React.ReactElement => {
  const description = useMemo(() => describeToolValue(value), [value]);
  const [showFull, setShowFull] = useState(!description.large);
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    copy(description.serialized);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  };

  return (
    <section className="rounded-lg border border-[#e5e7eb] bg-white p-3">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-medium text-[#4b5563]">{title}</div>
          <div className="mt-0.5 truncate text-xs text-[#8b93a1]">
            {description.summary}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {description.large ? (
            <button
              type="button"
              aria-expanded={showFull}
              className="rounded px-2 py-1 text-xs text-[#5b5bf7] hover:bg-[#f1f1ff]"
              onClick={() => setShowFull(current => !current)}
            >
              {showFull ? '收起' : '查看全部'}
            </button>
          ) : null}
          <button
            type="button"
            className="flex items-center gap-1 rounded px-2 py-1 text-xs text-[#5b6472] hover:bg-[#f3f4f6]"
            onClick={handleCopy}
          >
            <CopyOutlined />
            {copied ? '已复制' : '复制完整内容'}
          </button>
        </div>
      </div>
      {showFull ? (
        <pre className="mt-3 max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md bg-[#f7f8fa] p-3 text-xs leading-5 text-[#303846]">
          {description.serialized}
        </pre>
      ) : null}
    </section>
  );
};
