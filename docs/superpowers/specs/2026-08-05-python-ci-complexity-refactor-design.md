# Python CI Complexity Refactor Design

**Date:** 2026-08-05
**Scope:** `core/agent` and `core/workflow` quality-check failures on fork PR #1

## Context

The fork pull request is behaviorally green in the focused Agent, Workflow, frontend, and browser acceptance suites, but the repository's Python quality job fails under:

```text
flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10
```

The failures are limited to one unused import, one late module import, and three methods whose cyclomatic complexity exceeds the repository limit. The refactor must remove those quality failures without changing the Agent Event Protocol, Pi runtime wire format, event ordering, tool execution timing, error classification, or Workflow stream behavior.

## Selected Approach

Use small private helpers and explicit state objects to separate responsibilities. Do not add `noqa` complexity exemptions and do not relax the CI threshold.

### Agent runtime

- Remove the unused `AgentEventBase` import.
- Move the schema generator's repository-dependent import inside `main()`, after its path setup, so the script remains directly executable without violating module import ordering.
- Split synchronous/awaitable plugin completion from streamed plugin aggregation. Preserve progressive `_ExecutionEvent` yields and the final `PluginResponse` exactly.
- Introduce a private Pi run-state object for handled calls, pending wait calls, cumulative usage, and completion status.
- Extract WebSocket message decoding from the run loop.
- Dispatch runtime event types through a handler map. Each handler owns one event family and emits the same `AgentResponse` sequence as today.
- Keep connection setup, cancellation, exception translation, and terminal fallback in `run()`, with shared helpers for repeated failure/cancellation event construction.

### Workflow output

- Extract the terminal-frame predicate from `_process_queue_output()` into a private helper.
- Keep structured `agent_event` frames independent from text/reasoning frames and preserve the current rule that a structured-only frame does not mark the LLM stream complete.

## Invariants

- Runtime `agent_event`, reasoning, content, usage, tool call, tool progress, tool completion, error, and done messages retain their current output order.
- Tool streaming remains incremental; no handler may buffer the full tool execution before yielding progress.
- Pending `wait` calls still receive exactly one terminal event on success, cancellation, runtime failure, or disconnect.
- Existing exception causes remain chained into the same public `AgentInternalExc` categories.
- Workflow structured-only frames continue through the queue without suppressing later text or terminal frames.

## Verification

1. Re-run the exact failing flake8 commands for `core/agent` and `core/workflow` and require zero findings.
2. Run focused Pi protocol, adapter, runner, Workflow runner, and structured-event passthrough tests.
3. Run Black, isort, mypy, and compile checks for the touched Python modules.
4. Push the fix to fork PR #1, wait for the fork CI to complete, and merge only when substantive checks pass. Repository-side optional review integrations may remain non-blocking only when their failure is unrelated to code.

## Non-goals

- No protocol v2 changes.
- No frontend behavior changes.
- No upstream `iflytek/astron-agent` pull request.
- No deployment or image publication as part of this fork merge.
