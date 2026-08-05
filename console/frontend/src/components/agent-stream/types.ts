export type AgentSegmentSource = 'text' | 'thinking';
export type AgentSegmentChannel = 'pending' | 'reasoning' | 'content';
export type AgentCommitChannel = Exclude<AgentSegmentChannel, 'pending'>;
export type AgentCommitReason =
  | 'tool_call'
  | 'message_end'
  | 'cancelled'
  | 'error';
export type AgentFinalizeReason = AgentCommitReason | 'transport_closed';
export type AgentToolStatus = 'running' | 'success' | 'error' | 'cancelled';
export type AgentExecutionStatus = AgentToolStatus;
export type AgentVisibility = 'user' | 'debug' | 'runtime';

interface AgentEventBase {
  version: 1;
  runId: string;
  seq: number;
}

interface AgentTurnEventBase extends AgentEventBase {
  turnId: string;
}

export interface AgentExecutionStartEvent extends AgentEventBase {
  type: 'execution_start';
  startedAt: number;
}

export interface AgentSegmentStartEvent extends AgentTurnEventBase {
  type: 'segment_start';
  segmentId: string;
  source: AgentSegmentSource;
  channel: AgentSegmentChannel;
  visibility: AgentVisibility;
}

export interface AgentSegmentDeltaEvent extends AgentTurnEventBase {
  type: 'segment_delta';
  segmentId: string;
  delta: string;
}

export interface AgentSegmentEndEvent extends AgentTurnEventBase {
  type: 'segment_end';
  segmentId: string;
}

export interface AgentTurnCommitEvent extends AgentTurnEventBase {
  type: 'turn_commit';
  channel: AgentCommitChannel;
  partial: boolean;
  reason: AgentCommitReason;
}

export interface AgentToolStartEvent extends AgentTurnEventBase {
  type: 'tool_start';
  callId: string;
  name: string;
  arguments: unknown;
  status?: 'running';
  startedAt?: number;
}

export interface AgentToolProgressEvent extends AgentTurnEventBase {
  type: 'tool_progress';
  callId: string;
  summary: string;
}

export interface AgentToolFinishEvent extends AgentTurnEventBase {
  type: 'tool_finish';
  callId: string;
  name?: string;
  response?: unknown;
  status: Exclude<AgentToolStatus, 'running'>;
  finishedAt?: number;
  durationMs?: number;
}

export interface AgentUsageUpdateEvent extends AgentEventBase {
  type: 'usage_update';
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
}

export interface AgentExecutionErrorEvent extends AgentEventBase {
  type: 'execution_error';
  code: string;
  message: string;
  occurredAt: number;
}

export interface AgentExecutionEndEvent extends AgentEventBase {
  type: 'execution_end';
  status: Exclude<AgentExecutionStatus, 'running'>;
  finishedAt: number;
  durationMs: number;
}

export type AgentEventV1 =
  | AgentExecutionStartEvent
  | AgentSegmentStartEvent
  | AgentSegmentDeltaEvent
  | AgentSegmentEndEvent
  | AgentTurnCommitEvent
  | AgentToolStartEvent
  | AgentToolProgressEvent
  | AgentToolFinishEvent
  | AgentUsageUpdateEvent
  | AgentExecutionErrorEvent
  | AgentExecutionEndEvent;

export interface AgentUsage {
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
}

export interface AgentExecutionError {
  code: string;
  message: string;
  occurredAt: number;
}

export interface AgentExecutionRecord {
  runId: string;
  status: AgentExecutionStatus;
  startedAt?: number;
  finishedAt?: number;
  durationMs?: number;
  usage?: AgentUsage;
  error?: AgentExecutionError;
}

export interface AgentSegment {
  runId: string;
  segmentId: string;
  turnId: string;
  source: AgentSegmentSource;
  channel: AgentSegmentChannel;
  visibility: AgentVisibility;
  text: string;
  order: number;
  ended: boolean;
  partial: boolean;
  commitReason?: AgentFinalizeReason;
}

export interface AgentToolRecord {
  runId: string;
  callId: string;
  turnId: string;
  name: string;
  arguments: unknown;
  response?: unknown;
  progress?: string;
  status: AgentToolStatus;
  order: number;
  startedAt?: number;
  finishedAt?: number;
  durationMs?: number;
}

export interface AgentStreamState {
  schemaVersion: 3;
  hasStructuredEvents: boolean;
  executions: Record<string, AgentExecutionRecord>;
  segments: Record<string, AgentSegment>;
  tools: Record<string, AgentToolRecord>;
  lastSeqByRun: Record<string, number>;
  nextOrder: number;
  hasObservedToolByTurn: Record<string, true>;
  interrupted: boolean;
  interruptionReason: AgentFinalizeReason | null;
}

export type AgentReasoningTimelineItem =
  | ({ kind: 'reasoning' } & AgentSegment)
  | {
      kind: 'tool';
      runId: string;
      callId: string;
      turnId: string;
      order: number;
      tool: AgentToolRecord;
    };
