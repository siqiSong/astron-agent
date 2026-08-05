# Python CI Complexity Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the fork PR pass the repository's Python quality gate without changing Pi Agent or Workflow runtime behavior.

**Architecture:** Keep the public protocol and streaming paths unchanged while extracting private helpers from the three over-complex methods. A private Pi run-state object and event handlers will isolate runtime message decoding, event projection, tool handling, and terminal cleanup; Workflow will extract only its terminal-frame predicate.

**Tech Stack:** Python 3.11/3.12, aiohttp, Pydantic, pytest, flake8 7.0.0, Black 24.4.2, isort 5.13.2, mypy 1.18.2.

## Global Constraints

- Do not add `noqa: C901` or relax `--max-complexity 10`.
- Do not change Agent Event Protocol v1, Pi WebSocket payloads, emitted event order, tool streaming timing, exception categories, or Workflow stream semantics.
- Do not create another upstream `iflytek/astron-agent` pull request.
- Push only to `siqiSong/astron-agent:feat/pi-agent-runtime` and merge only fork PR #1 into the fork's `main`.
- Keep the feature branch after merge; do not deploy or publish images.

---

### Task 1: Resolve deterministic import-quality failures

**Files:**
- Modify: `core/agent/engine/nodes/pi/pi_runner.py:16`
- Modify: `core/agent/generate_agent_event_schema.py:1-10`

**Interfaces:**
- Consumes: `agent_event_v1_json_schema() -> dict[str, Any]`
- Produces: The same directly executable schema-generation entry point with flake8-clean imports.

- [ ] **Step 1: Reproduce the two import failures**

Run:

```bash
cd core/agent
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 engine/nodes/pi/pi_runner.py generate_agent_event_schema.py
```

Expected: FAIL containing both:

```text
F401 'agent.api.schemas.agent_event.AgentEventBase' imported but unused
E402 module level import not at top of file
```

- [ ] **Step 2: Remove the unused runtime import**

Change the Pi runner import to:

```python
from agent.api.schemas.agent_event import AgentEventV1
```

- [ ] **Step 3: Localize the schema import after path setup**

Keep `sys.path.insert(...)` at module initialization and move the repository import into `main()`:

```python
def main() -> None:
    from agent.api.schemas.agent_event import agent_event_v1_json_schema

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_text(
        json.dumps(
            agent_event_v1_json_schema(),
            ensure_ascii=False,
            indent=2,
            sort_keys=True,
        )
        + "\n",
        encoding="utf-8",
    )
```

- [ ] **Step 4: Verify import failures are gone and the generator still works**

Run:

```bash
cd core/agent
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 engine/nodes/pi/pi_runner.py generate_agent_event_schema.py
.venv/bin/python generate_agent_event_schema.py
git diff --exit-code ../../docs/contracts/agent-event-protocol-v1.schema.json
```

Expected: flake8 reports only the existing three `C901` findings; schema generation produces no contract diff.

- [ ] **Step 5: Commit**

```bash
git add core/agent/engine/nodes/pi/pi_runner.py core/agent/generate_agent_event_schema.py
git commit -m "fix(agent): clean Pi quality imports"
```

---

### Task 2: Split plugin stream aggregation from plugin invocation

**Files:**
- Modify: `core/agent/engine/nodes/pi/pi_runner.py:137-188`
- Test: `core/agent/tests/test_pi_runner.py`

**Interfaces:**
- Consumes: `plugin.run(arguments, span)` returning either `PluginResponse`, an awaitable, or an async iterator of `PluginResponse`.
- Produces: `_stream_plugin_invocation(plugin, invocation) -> AsyncIterator[_ExecutionEvent]` and a lower-complexity `_execute_plugin(...)` with identical yields.

- [ ] **Step 1: Record the behavioral baseline**

Run:

```bash
cd core
agent/.venv/bin/python -m pytest \
  agent/tests/test_pi_runner.py::test_remote_tool_call_executes_python_plugin_and_returns_result \
  agent/tests/test_pi_runner.py::test_subworkflow_stream_stays_visible_and_is_accumulated_for_pi \
  agent/tests/test_pi_runner.py::test_cancelled_plugin_finishes_tool_card_before_propagating_cancel -q
```

Expected: PASS, establishing result, progress, accumulation, and cancellation order before refactoring.

- [ ] **Step 2: Verify the quality red test**

Run:

```bash
cd core/agent
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 engine/nodes/pi/pi_runner.py
```

Expected: FAIL with `_execute_plugin` complexity 11.

- [ ] **Step 3: Extract streamed aggregation**

Add a helper with this exact boundary:

```python
async def _stream_plugin_invocation(
    self,
    plugin: BasePlugin,
    invocation: Any,
) -> AsyncIterator[_ExecutionEvent]:
    content_parts: list[str] = []
    reasoning_parts: list[str] = []
    last_response: PluginResponse | None = None
    async for response in invocation:
        if not isinstance(response, PluginResponse):
            raise TypeError(f"Plugin {plugin.name} streamed an invalid response")
        last_response = response
        result = self._dict_result(response.result)
        reasoning_content = result.get("reasoning_content") or ""
        content = result.get("content") or ""
        if reasoning_content:
            reasoning_parts.append(str(reasoning_content))
        if content:
            content_parts.append(str(content))
        yield _ExecutionEvent(progress=result)
        if response.code != 0:
            break

    final_response = self._final_plugin_response(
        plugin,
        last_response,
        reasoning_parts,
        content_parts,
    )
    plugin.run_result = final_response
    yield _ExecutionEvent(result=final_response)
```

Add `_final_plugin_response(...) -> PluginResponse` to own only the empty-stream fallback and accumulated content assembly. Keep `_execute_plugin()` responsible for invoking, awaiting, validating async iteration, and delegating streamed responses.

- [ ] **Step 4: Verify quality and behavior**

Run the Task 2 flake8 command and the three baseline tests again.

Expected: `_execute_plugin` no longer appears in flake8 output; all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/agent/engine/nodes/pi/pi_runner.py core/agent/tests/test_pi_runner.py
git commit -m "refactor(agent): split Pi plugin streaming"
```

---

### Task 3: Extract Pi runtime decoding, state, and event dispatch

**Files:**
- Modify: `core/agent/engine/nodes/pi/pi_runner.py:34-650`
- Test: `core/agent/tests/test_pi_runner.py`

**Interfaces:**
- Produces: `_PiRunState`, `_decode_runtime_payload(...)`, `_runtime_event_handlers()`, and one handler per supported runtime event type.
- Preserves: `PiRunner.run(span, node_trace_log) -> AsyncIterator[AgentResponse]`.

- [ ] **Step 1: Verify the run-loop quality red test**

Run:

```bash
cd core/agent
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 engine/nodes/pi/pi_runner.py
```

Expected: FAIL with `PiRunner.run` complexity 36.

- [ ] **Step 2: Add explicit private run state**

```python
@dataclass
class _PiRunState:
    handled_calls: set[str] = field(default_factory=set)
    wait_calls: dict[str, dict[str, Any]] = field(default_factory=dict)
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
    completed: bool = False
```

- [ ] **Step 3: Extract WebSocket message validation**

```python
@staticmethod
def _decode_runtime_payload(message: aiohttp.WSMessage) -> dict[str, Any] | None:
    if message.type == aiohttp.WSMsgType.ERROR:
        raise AgentInternalExc("Pi runtime WebSocket failed")
    if message.type != aiohttp.WSMsgType.TEXT:
        return None
    try:
        payload = json.loads(message.data)
    except json.JSONDecodeError as error:
        raise AgentInternalExc("Pi runtime returned invalid JSON") from error
    if not isinstance(payload, dict):
        raise AgentInternalExc("Pi runtime returned an invalid event")
    return payload
```

- [ ] **Step 4: Extract event handlers behind a dispatch map**

Create private async-generator handlers with the common signature:

```python
async def _handle_runtime_<type>(
    self,
    payload: dict[str, Any],
    state: _PiRunState,
    plugin_by_runtime_name: dict[str, BasePlugin],
    websocket: aiohttp.ClientWebSocketResponse,
    span: Span,
) -> AsyncIterator[AgentResponse]:
```

Implement handlers for `agent_event`, `reasoning_delta`, `content_delta`, `usage`, `tool_call`, `tool_completed`, `tool_progress`, `error`, and `done`. Move existing statements without changing their order. The `usage` handler updates cumulative state before yielding `usage_update`; `tool_call` delegates `_handle_tool_call()` without buffering; `done` yields the success `execution_end` and sets `state.completed = True`.

Build and use the dispatch map:

```python
handler = self._runtime_event_handlers().get(str(payload.get("type")))
if handler is None:
    runtime_error = ValueError(
        f"Pi runtime returned unknown event: {payload.get('type')}"
    )
    raise AgentInternalExc("Pi agent runtime failed") from runtime_error
async for response in handler(
    payload, state, plugin_by_runtime_name, websocket, span
):
    yield response
```

- [ ] **Step 5: Reduce repeated terminal event construction**

Add `_failed_run_responses(...) -> list[AgentResponse]` and `_cancelled_run_responses(...) -> list[AgentResponse]`. They must call `_finish_pending_wait_calls()` first and then append the same execution error/end events currently emitted by each exception branch. Keep exception translation and `raise ... from error` inside `run()` so public causes remain unchanged.

- [ ] **Step 6: Verify the exact Agent quality gate**

Run:

```bash
cd core/agent
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 .
.venv/bin/python -m isort --check-only --profile black .
.venv/bin/python -m black --check .
```

Expected: all commands exit 0.

- [ ] **Step 7: Verify Pi runtime behavior**

Run:

```bash
cd core
agent/.venv/bin/python -m pytest \
  agent/tests/test_agent_event_protocol.py \
  agent/tests/test_pi_event_adapter.py \
  agent/tests/test_pi_runner.py \
  agent/tests/test_workflow_agent_runner.py -q
```

Expected: all tests PASS, including usage accumulation, structured event sequencing, progressive tools, wait completion, cancellation, runtime error, malformed input, unavailable runtime, and disconnect cases.

- [ ] **Step 8: Commit**

```bash
git add core/agent/engine/nodes/pi/pi_runner.py core/agent/tests/test_pi_runner.py
git commit -m "refactor(agent): simplify Pi runtime dispatch"
```

---

### Task 4: Extract Workflow stream terminal predicate

**Files:**
- Modify: `core/workflow/engine/nodes/base_node.py:840-940`
- Test: `core/workflow/tests/engine/nodes/test_agent_event_stream.py`

**Interfaces:**
- Produces: `_stream_frame_is_complete(status, template_type, reasoning_content, is_reasoning, content) -> bool`.
- Preserves: structured-only Agent events do not set `llm_output_status` or terminate the queue.

- [ ] **Step 1: Verify the Workflow quality red test**

Run:

```bash
cd core/workflow
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 engine/nodes/base_node.py
```

Expected: FAIL with `_process_queue_output` complexity 11.

- [ ] **Step 2: Add the terminal predicate**

```python
@staticmethod
def _stream_frame_is_complete(
    status: int,
    template_type: TemplateType,
    reasoning_content: str,
    is_reasoning: bool,
    content: str,
) -> bool:
    return status == SparkLLMStatus.END.value or (
        template_type == TemplateType.REASONING
        and reasoning_content == ""
        and is_reasoning
        and bool(content)
    )
```

Replace only the final inline condition in `_process_queue_output()` with a call to this helper.

- [ ] **Step 3: Verify quality and structured-event behavior**

Run:

```bash
cd core/workflow
.venv/bin/python -m flake8 --max-line-length 88 --ignore=E203,W503,E501 --max-complexity 10 .
.venv/bin/python -m isort --check-only --profile black .
.venv/bin/python -m black --check .

cd ../
workflow/.venv/bin/python -m pytest \
  workflow/tests/engine/nodes/test_agent_event_stream.py \
  workflow/tests/engine/nodes/test_agent_node.py \
  workflow/tests/service/test_chat_service_response_filter.py -q
```

Expected: all quality commands and tests PASS.

- [ ] **Step 4: Commit**

```bash
git add core/workflow/engine/nodes/base_node.py core/workflow/tests/engine/nodes/test_agent_event_stream.py
git commit -m "refactor(workflow): simplify stream completion check"
```

---

### Task 5: Final verification, fork push, and merge

**Files:**
- Verify only; no new source files expected.

**Interfaces:**
- Consumes: fork PR #1 at `siqiSong/astron-agent`.
- Produces: fork `main` containing the verified Agent Execution Experience changes.

- [ ] **Step 1: Run the exact Python quality gates**

Run the full flake8, isort, Black, and mypy commands from `.github/workflows/ci.yml` in both `core/agent` and `core/workflow`. Expected: exit 0 with no findings.

- [ ] **Step 2: Run focused regression suites**

Run the complete test selections from Tasks 3 and 4. Expected: all PASS.

- [ ] **Step 3: Run syntax and diff checks**

```bash
core/agent/.venv/bin/python -m compileall -q core/agent
core/workflow/.venv/bin/python -m compileall -q core/workflow
git diff --check
git status --short --branch
```

Expected: compile commands and diff check exit 0; only intentional committed changes exist.

- [ ] **Step 4: Push the feature branch normally**

```bash
git push ssh://git@ssh.github.com:443/siqiSong/astron-agent.git \
  feat/pi-agent-runtime:feat/pi-agent-runtime
```

Expected: normal fast-forward push; never force push.

- [ ] **Step 5: Wait for fork PR checks**

Poll fork PR #1 until all substantive CI and CodeQL jobs complete. Diagnose and fix any code-related failure before merging. A repository integration may be treated as optional only with direct evidence that its failure is caused by missing external configuration rather than source code.

- [ ] **Step 6: Merge fork PR #1**

Use the signed-in GitHub session to merge `feat/pi-agent-runtime` into `siqiSong/astron-agent:main`. Verify the PR shows `Merged` and the fork's `main` points to the resulting merge commit. Do not delete the feature branch.
