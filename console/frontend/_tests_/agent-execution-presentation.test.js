import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createPanelDisclosure,
  deriveExecutionPresentation,
  reducePanelDisclosure,
} from '../src/components/agent-stream/presentation.ts';
import {
  createAgentStreamState,
  reduceAgentEvent,
} from '../src/components/agent-stream/reducer.ts';

const event = (seq, type, data = {}) => ({
  version: 1,
  runId: 'run-1',
  seq,
  type,
  ...data,
});

test('presentation prefers live parent streaming state', () => {
  let state = createAgentStreamState();
  state = reduceAgentEvent(
    state,
    event(1, 'execution_start', { startedAt: 100 })
  );

  assert.deepEqual(deriveExecutionPresentation(state, true), {
    status: 'running',
    statusLabel: '正在执行',
    toolCount: 0,
    durationMs: undefined,
  });
});

test('presentation derives completed, error, cancellation and transport states', () => {
  let completed = createAgentStreamState();
  completed = reduceAgentEvent(
    completed,
    event(1, 'execution_end', {
      status: 'success',
      finishedAt: 150,
      durationMs: 50,
    })
  );
  assert.equal(deriveExecutionPresentation(completed, false).status, 'success');

  let failed = createAgentStreamState();
  failed = reduceAgentEvent(
    failed,
    event(1, 'execution_error', {
      code: 'PI_RUNTIME_ERROR',
      message: 'Pi agent runtime failed',
      occurredAt: 140,
    })
  );
  assert.equal(deriveExecutionPresentation(failed, false).status, 'error');

  let cancelled = createAgentStreamState();
  cancelled = reduceAgentEvent(
    cancelled,
    event(1, 'execution_end', {
      status: 'cancelled',
      finishedAt: 150,
      durationMs: 50,
    })
  );
  assert.equal(
    deriveExecutionPresentation(cancelled, false).status,
    'cancelled'
  );

  assert.equal(
    deriveExecutionPresentation(
      {
        ...completed,
        interrupted: true,
        interruptionReason: 'transport_closed',
      },
      false
    ).status,
    'transport_closed'
  );
});

test('disclosure starts open live, starts closed historical and collapses once', () => {
  let live = createPanelDisclosure(true);
  assert.equal(live.expanded, true);
  live = reducePanelDisclosure(live, {
    type: 'activity_changed',
    active: false,
  });
  assert.equal(live.expanded, false);
  assert.equal(live.autoCollapsed, true);

  live = reducePanelDisclosure(live, { type: 'toggle' });
  assert.equal(live.expanded, true);
  assert.equal(
    reducePanelDisclosure(live, {
      type: 'activity_changed',
      active: false,
    }).expanded,
    true
  );

  assert.equal(createPanelDisclosure(false).expanded, false);

  let delayed = createPanelDisclosure(false);
  delayed = reducePanelDisclosure(delayed, {
    type: 'activity_changed',
    active: true,
  });
  assert.equal(delayed.expanded, true);
  assert.equal(delayed.autoCollapsed, false);
});
