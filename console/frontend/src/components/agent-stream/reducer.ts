import type {
  AgentCommitChannel,
  AgentCommitReason,
  AgentEventV1,
  AgentExecutionStatus,
  AgentFinalizeReason,
  AgentReasoningTimelineItem,
  AgentSegmentChannel,
  AgentSegmentSource,
  AgentStreamState,
  AgentToolFinishEvent,
  AgentToolRecord,
  AgentToolStatus,
} from './types';

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const isNonEmptyString = (value: unknown): value is string =>
  typeof value === 'string' && value.trim().length > 0;

const isOptionalFiniteNumber = (value: unknown): boolean =>
  value === undefined || (typeof value === 'number' && Number.isFinite(value));

const isNonNegativeSafeInteger = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;

const isOptionalNonNegativeSafeInteger = (value: unknown): boolean =>
  value === undefined || isNonNegativeSafeInteger(value);

const isSegmentSource = (value: unknown): value is AgentSegmentSource =>
  value === 'text' || value === 'thinking';

const isSegmentChannel = (value: unknown): value is AgentSegmentChannel =>
  value === 'pending' || value === 'reasoning' || value === 'content';

const isCommitChannel = (value: unknown): value is AgentCommitChannel =>
  value === 'reasoning' || value === 'content';

const isCommitReason = (value: unknown): value is AgentCommitReason =>
  value === 'tool_call' ||
  value === 'message_end' ||
  value === 'cancelled' ||
  value === 'error';

const isFinishedToolStatus = (
  value: unknown
): value is Exclude<AgentToolStatus, 'running'> =>
  value === 'success' || value === 'error' || value === 'cancelled';

const isExecutionStatus = (value: unknown): value is AgentExecutionStatus =>
  value === 'running' || isFinishedToolStatus(value);

const hasOwn = (value: Record<string, unknown>, key: string): boolean =>
  Object.prototype.hasOwnProperty.call(value, key);

const isAgentSegmentRecord = (value: unknown): boolean =>
  isRecord(value) &&
  isNonEmptyString(value.runId) &&
  isNonEmptyString(value.segmentId) &&
  isNonEmptyString(value.turnId) &&
  isSegmentSource(value.source) &&
  isSegmentChannel(value.channel) &&
  value.visibility === 'user' &&
  typeof value.text === 'string' &&
  typeof value.order === 'number' &&
  Number.isSafeInteger(value.order) &&
  typeof value.ended === 'boolean' &&
  typeof value.partial === 'boolean';

const isAgentToolRecord = (value: unknown): boolean =>
  isRecord(value) &&
  isNonEmptyString(value.runId) &&
  isNonEmptyString(value.callId) &&
  isNonEmptyString(value.turnId) &&
  isNonEmptyString(value.name) &&
  (value.status === 'running' || isFinishedToolStatus(value.status)) &&
  typeof value.order === 'number' &&
  Number.isSafeInteger(value.order);

const isAgentExecutionRecord = (value: unknown): boolean => {
  if (
    !isRecord(value) ||
    !isNonEmptyString(value.runId) ||
    !isExecutionStatus(value.status) ||
    !isOptionalNonNegativeSafeInteger(value.startedAt) ||
    !isOptionalNonNegativeSafeInteger(value.finishedAt) ||
    !isOptionalNonNegativeSafeInteger(value.durationMs)
  ) {
    return false;
  }
  if (
    value.usage !== undefined &&
    (!isRecord(value.usage) ||
      !isNonNegativeSafeInteger(value.usage.inputTokens) ||
      !isNonNegativeSafeInteger(value.usage.outputTokens) ||
      !isNonNegativeSafeInteger(value.usage.totalTokens))
  ) {
    return false;
  }
  return (
    value.error === undefined ||
    (isRecord(value.error) &&
      isNonEmptyString(value.error.code) &&
      isNonEmptyString(value.error.message) &&
      isNonNegativeSafeInteger(value.error.occurredAt))
  );
};

const hasValidTurnId = (value: Record<string, unknown>): boolean =>
  isNonEmptyString(value.turnId);

const isValidSegmentStart = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) &&
  isNonEmptyString(value.segmentId) &&
  isSegmentSource(value.source) &&
  isSegmentChannel(value.channel);

const isValidSegmentDelta = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) &&
  isNonEmptyString(value.segmentId) &&
  typeof value.delta === 'string';

const isValidSegmentEnd = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) && isNonEmptyString(value.segmentId);

const isValidTurnCommit = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) &&
  isCommitChannel(value.channel) &&
  typeof value.partial === 'boolean' &&
  isCommitReason(value.reason);

const isValidToolStart = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) &&
  isNonEmptyString(value.callId) &&
  isNonEmptyString(value.name) &&
  hasOwn(value, 'arguments') &&
  (value.status === undefined || value.status === 'running') &&
  isOptionalFiniteNumber(value.startedAt);

const isValidToolProgress = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) &&
  isNonEmptyString(value.callId) &&
  typeof value.summary === 'string';

const isValidToolFinish = (value: Record<string, unknown>): boolean =>
  hasValidTurnId(value) &&
  isNonEmptyString(value.callId) &&
  (value.name === undefined || isNonEmptyString(value.name)) &&
  isFinishedToolStatus(value.status) &&
  isOptionalFiniteNumber(value.finishedAt) &&
  isOptionalFiniteNumber(value.durationMs);

const isValidUsageUpdate = (value: Record<string, unknown>): boolean =>
  isNonNegativeSafeInteger(value.inputTokens) &&
  isNonNegativeSafeInteger(value.outputTokens) &&
  isNonNegativeSafeInteger(value.totalTokens);

const isValidExecutionError = (value: Record<string, unknown>): boolean =>
  isNonEmptyString(value.code) &&
  value.code.length <= 100 &&
  isNonEmptyString(value.message) &&
  value.message.length <= 500 &&
  isNonNegativeSafeInteger(value.occurredAt);

const isValidExecutionEnd = (value: Record<string, unknown>): boolean =>
  isFinishedToolStatus(value.status) &&
  isNonNegativeSafeInteger(value.finishedAt) &&
  isNonNegativeSafeInteger(value.durationMs);

export const parseAgentEvent = (value: unknown): AgentEventV1 | null => {
  if (
    !isRecord(value) ||
    value.version !== 1 ||
    !isNonEmptyString(value.runId) ||
    typeof value.seq !== 'number' ||
    !Number.isSafeInteger(value.seq) ||
    value.seq <= 0 ||
    !isNonEmptyString(value.type)
  ) {
    return null;
  }

  switch (value.type) {
    case 'execution_start':
      if (!isNonNegativeSafeInteger(value.startedAt)) return null;
      break;
    case 'segment_start':
      if (!isValidSegmentStart(value)) return null;
      if ((value.visibility ?? 'user') !== 'user') return null;
      return {
        ...value,
        visibility: 'user',
      } as unknown as AgentEventV1;
    case 'segment_delta':
      if (!isValidSegmentDelta(value)) return null;
      break;
    case 'segment_end':
      if (!isValidSegmentEnd(value)) return null;
      break;
    case 'turn_commit':
      if (!isValidTurnCommit(value)) return null;
      break;
    case 'tool_start':
      if (!isValidToolStart(value)) return null;
      break;
    case 'tool_progress':
      if (!isValidToolProgress(value)) return null;
      break;
    case 'tool_finish':
      if (!isValidToolFinish(value)) return null;
      break;
    case 'usage_update':
      if (!isValidUsageUpdate(value)) return null;
      break;
    case 'execution_error':
      if (!isValidExecutionError(value)) return null;
      break;
    case 'execution_end':
      if (!isValidExecutionEnd(value)) return null;
      break;
    default:
      return null;
  }

  return value as unknown as AgentEventV1;
};

export const createAgentStreamState = (): AgentStreamState => ({
  schemaVersion: 3,
  hasStructuredEvents: false,
  executions: {},
  segments: {},
  tools: {},
  lastSeqByRun: {},
  nextOrder: 0,
  hasObservedToolByTurn: {},
  interrupted: false,
  interruptionReason: null,
});

const entityKey = (runId: string, id: string): string =>
  JSON.stringify([runId, id]);

const applyToolFinish = (
  existing: AgentToolRecord | undefined,
  order: number,
  event: AgentToolFinishEvent
): AgentToolRecord => {
  const tool: AgentToolRecord = existing
    ? { ...existing }
    : {
        runId: event.runId,
        callId: event.callId,
        turnId: event.turnId,
        name: event.name ?? 'unknown',
        arguments: null,
        status: event.status,
        order,
      };

  tool.status = event.status;
  if (event.name) tool.name = event.name;
  if (hasOwn(event as unknown as Record<string, unknown>, 'response')) {
    tool.response = event.response;
  }
  if (event.finishedAt !== undefined) tool.finishedAt = event.finishedAt;
  if (event.durationMs !== undefined) tool.durationMs = event.durationMs;
  return tool;
};

export const parseAgentStreamState = (
  value: unknown
): AgentStreamState | null => {
  if (!isRecord(value)) return null;
  const normalized =
    value.schemaVersion === 2
      ? {
          ...value,
          schemaVersion: 3,
          executions: {},
          segments: isRecord(value.segments)
            ? Object.fromEntries(
                Object.entries(value.segments).map(([key, segment]) => [
                  key,
                  isRecord(segment)
                    ? { ...segment, visibility: 'user' }
                    : segment,
                ])
              )
            : value.segments,
        }
      : value;
  if (
    normalized.schemaVersion !== 3 ||
    typeof normalized.hasStructuredEvents !== 'boolean' ||
    !isRecord(normalized.executions) ||
    !isRecord(normalized.segments) ||
    !isRecord(normalized.tools) ||
    !isRecord(normalized.lastSeqByRun) ||
    typeof normalized.nextOrder !== 'number' ||
    !Number.isSafeInteger(normalized.nextOrder) ||
    !isRecord(normalized.hasObservedToolByTurn) ||
    typeof normalized.interrupted !== 'boolean' ||
    (normalized.interruptionReason !== null &&
      !isCommitReason(normalized.interruptionReason) &&
      normalized.interruptionReason !== 'transport_closed')
  ) {
    return null;
  }
  if (
    !Object.values(normalized.executions).every(isAgentExecutionRecord) ||
    !Object.values(normalized.segments).every(isAgentSegmentRecord) ||
    !Object.values(normalized.tools).every(isAgentToolRecord) ||
    !Object.values(normalized.lastSeqByRun).every(
      seq => typeof seq === 'number' && Number.isSafeInteger(seq) && seq >= 0
    ) ||
    !Object.values(normalized.hasObservedToolByTurn).every(
      flag => flag === true
    )
  ) {
    return null;
  }
  return normalized as unknown as AgentStreamState;
};

const acceptEvent = (
  state: AgentStreamState,
  event: AgentEventV1
): AgentStreamState | null => {
  const lastSeq = state.lastSeqByRun[event.runId] ?? 0;
  if (event.seq <= lastSeq) return null;
  return {
    ...state,
    hasStructuredEvents: true,
    lastSeqByRun: { ...state.lastSeqByRun, [event.runId]: event.seq },
    nextOrder: state.nextOrder + 1,
  };
};

export const reduceAgentEvent = (
  state: AgentStreamState,
  event: AgentEventV1
): AgentStreamState => {
  const next = acceptEvent(state, event);
  if (!next) return state;
  const order = state.nextOrder;

  switch (event.type) {
    case 'execution_start':
      return {
        ...next,
        executions: {
          ...state.executions,
          [event.runId]: {
            ...state.executions[event.runId],
            runId: event.runId,
            status: 'running',
            startedAt: event.startedAt,
          },
        },
      };
    case 'segment_start': {
      const key = entityKey(event.runId, event.segmentId);
      if (state.segments[key]) return next;
      return {
        ...next,
        segments: {
          ...state.segments,
          [key]: {
            runId: event.runId,
            segmentId: event.segmentId,
            turnId: event.turnId,
            source: event.source,
            channel: event.channel,
            visibility: event.visibility,
            text: '',
            order,
            ended: false,
            partial: false,
          },
        },
      };
    }
    case 'segment_delta': {
      const key = entityKey(event.runId, event.segmentId);
      const segment = state.segments[key];
      if (!segment) return next;
      return {
        ...next,
        segments: {
          ...state.segments,
          [key]: { ...segment, text: segment.text + event.delta },
        },
      };
    }
    case 'segment_end': {
      const key = entityKey(event.runId, event.segmentId);
      const segment = state.segments[key];
      if (!segment) return next;
      return {
        ...next,
        segments: {
          ...state.segments,
          [key]: { ...segment, ended: true },
        },
      };
    }
    case 'turn_commit': {
      let segments = state.segments;
      for (const [key, segment] of Object.entries(state.segments)) {
        if (
          segment.runId === event.runId &&
          segment.turnId === event.turnId &&
          segment.channel === 'pending'
        ) {
          if (segments === state.segments) segments = { ...state.segments };
          segments[key] = {
            ...segment,
            channel: event.channel,
            partial: event.partial,
            commitReason: event.reason,
          };
        }
      }
      return {
        ...next,
        segments,
        ...(event.partial
          ? { interrupted: true, interruptionReason: event.reason }
          : {}),
      };
    }
    case 'tool_start': {
      const key = entityKey(event.runId, event.callId);
      const turnKey = entityKey(event.runId, event.turnId);
      return {
        ...next,
        hasObservedToolByTurn: {
          ...state.hasObservedToolByTurn,
          [turnKey]: true,
        },
        tools: {
          ...state.tools,
          [key]: {
            runId: event.runId,
            callId: event.callId,
            turnId: event.turnId,
            name: event.name,
            arguments: event.arguments,
            status: 'running',
            order: state.tools[key]?.order ?? order,
            ...(event.startedAt === undefined
              ? {}
              : { startedAt: event.startedAt }),
          },
        },
      };
    }
    case 'tool_progress': {
      const key = entityKey(event.runId, event.callId);
      const tool = state.tools[key];
      if (!tool) return next;
      return {
        ...next,
        tools: {
          ...state.tools,
          [key]: { ...tool, progress: event.summary },
        },
      };
    }
    case 'tool_finish': {
      const key = entityKey(event.runId, event.callId);
      return {
        ...next,
        tools: {
          ...state.tools,
          [key]: applyToolFinish(state.tools[key], order, event),
        },
      };
    }
    case 'usage_update':
      return {
        ...next,
        executions: {
          ...state.executions,
          [event.runId]: {
            ...state.executions[event.runId],
            runId: event.runId,
            status: state.executions[event.runId]?.status ?? 'running',
            usage: {
              inputTokens: event.inputTokens,
              outputTokens: event.outputTokens,
              totalTokens: event.totalTokens,
            },
          },
        },
      };
    case 'execution_error':
      return {
        ...next,
        executions: {
          ...state.executions,
          [event.runId]: {
            ...state.executions[event.runId],
            runId: event.runId,
            status: 'error',
            error: {
              code: event.code,
              message: event.message,
              occurredAt: event.occurredAt,
            },
          },
        },
      };
    case 'execution_end':
      return {
        ...next,
        executions: {
          ...state.executions,
          [event.runId]: {
            ...state.executions[event.runId],
            runId: event.runId,
            status: event.status,
            finishedAt: event.finishedAt,
            durationMs: event.durationMs,
          },
        },
      };
  }
};

export const finalizePendingSegments = (
  state: AgentStreamState,
  reason: AgentFinalizeReason
): AgentStreamState => {
  if (!state.hasStructuredEvents) return state;

  const pending = Object.values(state.segments).filter(
    segment => segment.channel === 'pending'
  );
  const hasRunningTools = Object.values(state.tools).some(
    tool => tool.status === 'running'
  );
  if (pending.length === 0 && !hasRunningTools && state.interrupted) {
    return state;
  }

  let segments = state.segments;
  for (const [key, segment] of Object.entries(state.segments)) {
    if (segment.channel !== 'pending') continue;
    if (segments === state.segments) segments = { ...state.segments };
    segments[key] = {
      ...segment,
      channel: state.hasObservedToolByTurn[
        entityKey(segment.runId, segment.turnId)
      ]
        ? 'reasoning'
        : 'content',
      partial: true,
      commitReason: reason,
    };
  }
  let tools = state.tools;
  for (const [key, tool] of Object.entries(state.tools)) {
    if (tool.status !== 'running') continue;
    if (tools === state.tools) tools = { ...state.tools };
    tools[key] = {
      ...tool,
      status:
        reason === 'error' || reason === 'transport_closed'
          ? 'error'
          : 'cancelled',
    };
  }
  return {
    ...state,
    segments,
    tools,
    interrupted: true,
    interruptionReason: reason,
  };
};

export const selectLiveContent = (state: AgentStreamState): string =>
  Object.values(state.segments)
    .filter(
      segment => segment.channel === 'pending' || segment.channel === 'content'
    )
    .sort((left, right) => left.order - right.order)
    .map(segment => segment.text)
    .join('');

export const selectHasPartialContent = (state: AgentStreamState): boolean =>
  Object.values(state.segments).some(
    segment => segment.channel === 'content' && segment.partial
  );

export const selectReasoningTimeline = (
  state: AgentStreamState
): AgentReasoningTimelineItem[] => {
  const reasoning: AgentReasoningTimelineItem[] = Object.values(state.segments)
    .filter(
      segment => segment.channel === 'reasoning' && segment.text.length > 0
    )
    .map(segment => ({ kind: 'reasoning', ...segment }));
  const tools: AgentReasoningTimelineItem[] = Object.values(state.tools).map(
    tool => ({
      kind: 'tool',
      runId: tool.runId,
      callId: tool.callId,
      turnId: tool.turnId,
      order: tool.order,
      tool,
    })
  );

  return [...reasoning, ...tools].sort(
    (left, right) => left.order - right.order
  );
};
