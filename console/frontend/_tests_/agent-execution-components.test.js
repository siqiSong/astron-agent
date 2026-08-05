import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const readComponent = name =>
  readFileSync(
    resolve(frontendRoot, 'src/components/agent-stream', name),
    'utf8'
  );

test('execution panel is capability-gated and uses disclosure state', () => {
  const source = readComponent('agent-execution-panel.tsx');
  assert.match(source, /if \(!state\.hasStructuredEvents\) return null/);
  assert.match(source, /reducePanelDisclosure/);
  assert.match(source, /activity_changed/);
  assert.match(source, /<ExecutionPanelHeader/);
  assert.match(source, /<ExecutionTimeline/);
});

test('execution header is a labelled native disclosure button', () => {
  const source = readComponent('execution-panel-header.tsx');
  assert.match(source, /<button/);
  assert.match(source, /type="button"/);
  assert.match(source, /aria-expanded=\{expanded\}/);
  assert.match(source, /任务分析与执行过程/);
  assert.match(source, /presentation\.statusLabel/);
});

test('reasoning step renders exact segment text with a local disclosure', () => {
  const source = readComponent('reasoning-step.tsx');
  assert.match(source, /content=\{segment\.text\}/);
  assert.match(source, /aria-expanded=\{expanded\}/);
  assert.match(source, /展开更多/);
  assert.match(source, /此段内容因任务中断而提前结束/);
  assert.doesNotMatch(source, /summari[sz]e/i);
});

test('tool step is an independent accessible disclosure with lazy values', () => {
  const source = readComponent('tool-step.tsx');
  assert.match(source, /<button/);
  assert.match(source, /type="button"/);
  assert.match(source, /aria-expanded=\{expanded\}/);
  assert.match(source, /调用工具 \{tool\.name\}/);
  assert.match(source, /responseSummary/);
  assert.match(source, /参数 Arguments/);
  assert.match(source, /响应 Response/);
  assert.match(source, /等待工具返回/);
});

test('AgentTimeline is the only shared chat integration wrapper', () => {
  const timeline = readComponent('agent-timeline.tsx');
  const workflowChat = readFileSync(
    resolve(
      frontendRoot,
      'src/components/workflow/drawer/chat-debugger/components/chat-content.tsx'
    ),
    'utf8'
  );
  const publishedChat = readFileSync(
    resolve(frontendRoot, 'src/pages/chat-page/components/message-list.tsx'),
    'utf8'
  );

  assert.match(timeline, /<AgentExecutionPanel \{\.\.\.props\} \/>/);
  assert.match(workflowChat, /<AgentTimeline/);
  assert.match(publishedChat, /<AgentTimeline/);
  assert.doesNotMatch(workflowChat, /<AgentExecutionPanel/);
  assert.doesNotMatch(publishedChat, /<AgentExecutionPanel/);
});
