import assert from 'node:assert/strict';
import test from 'node:test';

import useChatStore from '../src/store/chat-store.ts';
import { shouldIgnoreChatStreamCallback } from '../src/hooks/chat-stream-guard.ts';

const event = (seq, type, extra = {}) => ({
  version: 1,
  runId: 'run-1',
  seq,
  type,
  turnId: 'turn-1',
  ...extra,
});

test('cancelled published chat rejects late text and structured events', () => {
  const store = useChatStore.getState();
  store.initChatStore();
  store.startStreamingMessage({ id: 1, message: '', reqType: 'BOT' });
  const messageId = useChatStore.getState().messageList.at(-1)?.id;
  store.applyAgentStreamEvent(
    event(1, 'segment_start', {
      segmentId: 'segment-1',
      source: 'text',
      channel: 'pending',
    })
  );
  store.applyAgentStreamEvent(
    event(2, 'segment_delta', { segmentId: 'segment-1', delta: 'partial' })
  );
  store.updateStreamingMessage(messageId, 'partial');
  store.finalizeAgentStream('cancelled');
  store.finishStreamingMessage(1, undefined, undefined, 'cancelled');

  const settled = useChatStore.getState().messageList.at(-1);
  assert.equal(settled?.streamStatus, 'cancelled');
  assert.equal(settled?.message, 'partial');

  useChatStore.getState().updateStreamingMessage(messageId, 'late text');
  useChatStore
    .getState()
    .applyAgentStreamEvent(
      event(3, 'segment_delta', { segmentId: 'segment-1', delta: ' late' })
    );

  const afterLateEvents = useChatStore.getState().messageList.at(-1);
  assert.equal(afterLateEvents, settled);
  assert.equal(afterLateEvents?.message, 'partial');
});

test('aborted queued SSE callback is rejected before ancillary side effects', () => {
  const controller = new AbortController();
  let streamId = '';
  let reasoning = '';
  controller.abort('用户停止');

  if (!shouldIgnoreChatStreamCallback(false, controller.signal)) {
    streamId = 'late-stream-id';
    reasoning = 'late reasoning';
  }

  assert.equal(streamId, '');
  assert.equal(reasoning, '');
  assert.equal(
    shouldIgnoreChatStreamCallback(true, new AbortController().signal),
    true
  );
  assert.equal(
    shouldIgnoreChatStreamCallback(false, new AbortController().signal),
    false
  );
});

test('failed chat stream preserves the server diagnostic reason', () => {
  const store = useChatStore.getState();
  store.initChatStore();
  store.startStreamingMessage({ id: 2, message: '', reqType: 'BOT' });
  store.finishStreamingMessage(
    2,
    'sid-1',
    11,
    'error',
    '未查询到对应的工作流版本'
  );

  const failed = useChatStore.getState().messageList.at(-1);
  assert.equal(failed?.streamStatus, 'error');
  assert.equal(failed?.errorMessage, '未查询到对应的工作流版本');
});

test('late events update only the bot message owned by their request', () => {
  const store = /** @type {any} */ (useChatStore.getState());
  store.initChatStore();
  store.startStreamingMessage({ id: 101, message: '', reqType: 'BOT' });
  store.startStreamingMessage({ id: 202, message: '', reqType: 'BOT' });

  store.updateStreamingMessage(101, 'late PPT output');

  const messages = useChatStore.getState().messageList;
  assert.equal(messages[0]?.message, 'late PPT output');
  assert.equal(messages[1]?.message, '');
  assert.equal(messages[1]?.streamStatus, 'streaming');
});

test('late completion settles only the bot message owned by its request', () => {
  const store = /** @type {any} */ (useChatStore.getState());
  store.initChatStore();
  store.startStreamingMessage({ id: 301, message: 'old', reqType: 'BOT' });
  store.startStreamingMessage({ id: 302, message: '', reqType: 'BOT' });

  store.finishStreamingMessage(301, 'old-sid', 17, 'completed');

  const messages = useChatStore.getState().messageList;
  assert.equal(messages[0]?.streamStatus, 'completed');
  assert.equal(messages[0]?.sid, 'old-sid');
  assert.equal(messages[1]?.streamStatus, 'streaming');
  assert.equal(messages[1]?.sid, undefined);
});
