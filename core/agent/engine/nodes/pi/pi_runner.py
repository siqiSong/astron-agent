import asyncio
import inspect
import json
import os
import time
from collections.abc import AsyncGenerator, AsyncIterator, Sequence
from contextlib import aclosing
from dataclasses import dataclass, field
from typing import Any

import aiohttp
from common.otlp.log_trace.node_trace_log import NodeTraceLog
from common.otlp.trace.span import Span
from openai.types.completion_usage import CompletionUsage
from pydantic import BaseModel

from agent.api.schemas.agent_event import AgentEventV1
from agent.api.schemas.agent_response import AgentResponse, CotStep
from agent.api.schemas.llm_message import LLMMessage
from agent.engine.nodes.pi.event_adapter import PiEventAdapter, PiEventAdapterError
from agent.engine.nodes.pi.protocol import (
    build_system_prompt,
    build_tool_contracts,
    history_payload,
)
from agent.exceptions.agent_exc import AgentExc, AgentInternalExc
from agent.service.plugin.base import BasePlugin, PluginResponse


class PiModelConfig(BaseModel):
    id: str
    provider: str = "openai"
    base_url: str
    api_key: str


@dataclass
class _ExecutionEvent:
    progress: Any | None = None
    result: PluginResponse | None = None


@dataclass
class _PiRunState:
    handled_calls: set[str] = field(default_factory=set)
    wait_calls: dict[str, dict[str, Any]] = field(default_factory=dict)
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
    completed: bool = False


@dataclass
class PiRunner:
    app_id: str
    uid: str
    run_id: str
    model_config: PiModelConfig
    chat_history: list[LLMMessage]
    instruct: str
    knowledge: str
    question: str
    plugins: Sequence[BasePlugin]
    runtime_url: str = field(
        default_factory=lambda: os.getenv(
            "PI_AGENT_RUNTIME_URL",
            "ws://core-pi-agent:8090/internal/v1/runs",
        )
    )
    internal_secret: str = field(
        default_factory=lambda: os.getenv("PI_AGENT_INTERNAL_SECRET", "")
    )
    _event_adapter: PiEventAdapter = field(init=False)

    def __post_init__(self) -> None:
        self._event_adapter = PiEventAdapter(
            run_id=self.run_id,
            started_at=self._now_ms(),
        )

    def _start_message(self) -> tuple[dict[str, Any], dict[str, BasePlugin]]:
        tools, plugin_by_runtime_name = build_tool_contracts(self.plugins)
        return (
            {
                "type": "start",
                "runId": self.run_id,
                "model": {
                    "id": self.model_config.id,
                    "provider": self.model_config.provider or "openai",
                    "baseUrl": self.model_config.base_url,
                    "apiKey": self.model_config.api_key,
                },
                "systemPrompt": build_system_prompt(self.instruct, self.knowledge),
                "messages": history_payload(self.chat_history),
                "question": self.question,
                "tools": tools,
            },
            plugin_by_runtime_name,
        )

    @staticmethod
    def _dict_result(result: Any) -> dict[str, Any]:
        return result if isinstance(result, dict) else {"result": result}

    @staticmethod
    def _required_text(payload: dict[str, Any], field_name: str) -> str:
        value = payload.get(field_name)
        if not isinstance(value, str) or not value:
            raise AgentInternalExc(f"Pi runtime returned invalid {field_name}")
        return value

    @staticmethod
    def _usage_tokens(payload: dict[str, Any]) -> tuple[int, int, int]:
        values: list[int] = []
        for field_name in ("inputTokens", "outputTokens", "totalTokens"):
            raw_value = payload.get(field_name, 0)
            if isinstance(raw_value, bool):
                raise AgentInternalExc("Pi runtime returned invalid usage")
            try:
                value = int(raw_value or 0)
            except (TypeError, ValueError) as error:
                raise AgentInternalExc("Pi runtime returned invalid usage") from error
            if value < 0:
                raise AgentInternalExc("Pi runtime returned invalid usage")
            values.append(value)
        return values[0], values[1], values[2]

    @staticmethod
    def _decode_runtime_payload(
        message: aiohttp.WSMessage,
    ) -> dict[str, Any] | None:
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

    def _tool_call_fields(
        self, payload: dict[str, Any]
    ) -> tuple[str, str, str, dict[str, Any]]:
        call_id = self._required_text(payload, "callId")
        turn_id = self._required_text(payload, "turnId")
        runtime_name = self._required_text(payload, "name")
        arguments = payload.get("arguments")
        if not isinstance(arguments, dict):
            raise AgentInternalExc("Pi runtime returned invalid tool arguments")
        return call_id, turn_id, runtime_name, arguments

    @staticmethod
    def _now_ms() -> int:
        return int(time.time() * 1000)

    def _event_response(self, event: AgentEventV1) -> AgentResponse:
        return AgentResponse(
            typ="agent_event", content=event, model=self.model_config.id
        )

    async def _execute_plugin(
        self,
        plugin: BasePlugin,
        arguments: dict[str, Any],
        span: Span,
    ) -> AsyncIterator[_ExecutionEvent]:
        invocation = plugin.run(arguments, span)
        if inspect.isawaitable(invocation):
            response = await invocation
            if not isinstance(response, PluginResponse):
                raise TypeError(f"Plugin {plugin.name} returned an invalid response")
            plugin.run_result = response
            yield _ExecutionEvent(result=response)
            return

        if not hasattr(invocation, "__aiter__"):
            raise TypeError(f"Plugin {plugin.name} is not async")

        async for event in self._stream_plugin_invocation(plugin, invocation):
            yield event

    def _final_plugin_response(
        self,
        plugin: BasePlugin,
        last_response: PluginResponse | None,
        reasoning_parts: list[str],
        content_parts: list[str],
    ) -> PluginResponse:
        if last_response is None:
            return PluginResponse(
                code=500,
                result={"message": f"Plugin {plugin.name} returned no result"},
            )
        if content_parts or reasoning_parts:
            return last_response.model_copy(
                update={
                    "result": {
                        "reasoning_content": "".join(reasoning_parts),
                        "content": "".join(content_parts),
                    }
                }
            )
        return last_response

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

    async def _handle_tool_call(
        self,
        payload: dict[str, Any],
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        call_id, turn_id, runtime_name, arguments = self._tool_call_fields(payload)

        plugin = plugin_by_runtime_name.get(runtime_name)
        started_at = self._now_ms()
        yield self._event_response(
            self._event_adapter.tool_started(
                turn_id=turn_id,
                call_id=call_id,
                name=plugin.name if plugin is not None else runtime_name,
                arguments=arguments,
                started_at=started_at,
            )
        )
        if plugin is None:
            error_result = {"message": f"Unknown tool: {runtime_name}"}
            finished_at = self._now_ms()
            yield self._event_response(
                self._event_adapter.tool_finished(
                    turn_id=turn_id,
                    call_id=call_id,
                    name=runtime_name,
                    response=error_result,
                    status="error",
                    finished_at=finished_at,
                    duration_ms=finished_at - started_at,
                )
            )
            await websocket.send_json(
                {
                    "type": "tool_result",
                    "callId": call_id,
                    "result": error_result,
                    "isError": True,
                }
            )
            return

        result: PluginResponse | None = None
        try:
            with span.start(f"RunPiTool-{runtime_name}") as tool_span:
                async for event in self._execute_plugin(plugin, arguments, tool_span):
                    if event.progress is not None:
                        yield self._event_response(
                            self._event_adapter.tool_progressed(
                                turn_id=turn_id,
                                call_id=call_id,
                                value=event.progress,
                            )
                        )
                    if event.result is not None:
                        result = event.result
        except asyncio.CancelledError:
            finished_at = self._now_ms()
            yield self._event_response(
                self._event_adapter.tool_finished(
                    turn_id=turn_id,
                    call_id=call_id,
                    name=plugin.name,
                    response={"message": "Tool execution cancelled"},
                    status="cancelled",
                    finished_at=finished_at,
                    duration_ms=finished_at - started_at,
                )
            )
            raise
        except Exception as error:  # Plugin failures are model-visible tool errors.
            span.record_exception(
                error,
                attributes={"pi.tool_name": runtime_name},
            )
            result = PluginResponse(
                code=500,
                result={"message": "Tool execution failed"},
            )
            plugin.run_result = result

        if result is None:
            result = PluginResponse(
                code=500,
                result={"message": f"Plugin {plugin.name} returned no result"},
            )
            plugin.run_result = result

        action_output = self._dict_result(result.result)
        finished_at = self._now_ms()
        yield self._event_response(
            self._event_adapter.tool_finished(
                turn_id=turn_id,
                call_id=call_id,
                name=plugin.name,
                response=action_output,
                status="error" if result.code != 0 else "success",
                finished_at=finished_at,
                duration_ms=finished_at - started_at,
            )
        )
        yield AgentResponse(
            typ="cot_step",
            content=CotStep(
                action=plugin.name,
                action_input=arguments,
                action_output=action_output,
                tool_type="workflow" if plugin.typ == "workflow" else "tool",
                plugin=plugin,
            ),
            model=self.model_config.id,
        )
        await websocket.send_json(
            {
                "type": "tool_result",
                "callId": call_id,
                "result": result.result,
                "isError": result.code != 0,
            }
        )

    def _wait_completion(self, payload: dict[str, Any]) -> AgentResponse:
        arguments = payload.get("arguments")
        result = payload.get("result")
        return AgentResponse(
            typ="cot_step",
            content=CotStep(
                action=str(payload.get("name") or "wait"),
                action_input=arguments if isinstance(arguments, dict) else {},
                action_output=self._dict_result(result),
                tool_type="tool",
            ),
            model=self.model_config.id,
        )

    def _finish_pending_wait_calls(
        self,
        wait_calls: dict[str, Any],
        *,
        status: str,
        message: str,
        span: Span,
    ) -> list[AgentResponse]:
        responses: list[AgentResponse] = []
        try:
            for call_id, wait_call in wait_calls.items():
                try:
                    if not isinstance(call_id, str) or not call_id:
                        raise ValueError("invalid pending wait call id")
                    if not isinstance(wait_call, dict):
                        raise ValueError("invalid pending wait record")
                    turn_id = wait_call.get("turnId")
                    name = wait_call.get("name") or "wait"
                    started_at = wait_call.get("startedAt")
                    if not isinstance(turn_id, str) or not turn_id:
                        raise ValueError("invalid pending wait turn id")
                    if not isinstance(name, str) or not name:
                        raise ValueError("invalid pending wait name")
                    if not isinstance(started_at, int):
                        raise ValueError("invalid pending wait start time")
                    finished_at = self._now_ms()
                    responses.append(
                        self._event_response(
                            self._event_adapter.tool_finished(
                                turn_id=turn_id,
                                call_id=call_id,
                                name=name,
                                response={"message": message},
                                status=status,
                                finished_at=finished_at,
                                duration_ms=finished_at - started_at,
                            )
                        )
                    )
                except Exception as cleanup_error:  # noqa: PERF203
                    span.record_exception(
                        cleanup_error,
                        attributes={"pi.cleanup": "pending_wait"},
                    )
        finally:
            wait_calls.clear()
        return responses

    async def _handle_runtime_agent_event(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        try:
            events = self._event_adapter.adapt_runtime_event(payload)
        except PiEventAdapterError as error:
            raise AgentInternalExc(str(error)) from error
        for event in events:
            yield self._event_response(event)

    async def _handle_runtime_reasoning_delta(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        yield AgentResponse(
            typ="reasoning_content",
            content=str(payload.get("delta") or ""),
            model=self.model_config.id,
        )

    async def _handle_runtime_content_delta(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        yield AgentResponse(
            typ="content",
            content=str(payload.get("delta") or ""),
            model=self.model_config.id,
        )

    async def _handle_runtime_usage(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        input_tokens, output_tokens, total_tokens = self._usage_tokens(payload)
        next_input_tokens = state.input_tokens + input_tokens
        next_output_tokens = state.output_tokens + output_tokens
        next_total_tokens = state.total_tokens + total_tokens
        usage_event = self._event_adapter.usage_updated(
            input_tokens=next_input_tokens,
            output_tokens=next_output_tokens,
            total_tokens=next_total_tokens,
        )
        state.input_tokens = next_input_tokens
        state.output_tokens = next_output_tokens
        state.total_tokens = next_total_tokens
        yield self._event_response(usage_event)
        yield AgentResponse(
            typ="content",
            content="",
            model=self.model_config.id,
            usage=CompletionUsage(
                prompt_tokens=input_tokens,
                completion_tokens=output_tokens,
                total_tokens=total_tokens,
            ),
        )

    async def _handle_runtime_tool_call(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        call_id, turn_id, runtime_name, arguments = self._tool_call_fields(payload)
        if runtime_name == "wait":
            started_at = self._now_ms()
            state.wait_calls[call_id] = {
                "turnId": turn_id,
                "name": "wait",
                "arguments": arguments,
                "startedAt": started_at,
            }
            yield self._event_response(
                self._event_adapter.tool_started(
                    turn_id=state.wait_calls[call_id]["turnId"],
                    call_id=call_id,
                    name="wait",
                    arguments=state.wait_calls[call_id]["arguments"],
                    started_at=started_at,
                )
            )
            return
        async for response in self._handle_tool_call(
            payload,
            plugin_by_runtime_name,
            websocket,
            span,
        ):
            yield response
        state.handled_calls.add(call_id)

    async def _handle_runtime_tool_completed(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        call_id = self._required_text(payload, "callId")
        if call_id in state.handled_calls:
            return
        wait_call = state.wait_calls.get(call_id)
        if not isinstance(wait_call, dict):
            raise AgentInternalExc("Pi runtime returned an invalid tool completion")
        finished_at = self._now_ms()
        finish_event = self._event_adapter.tool_finished(
            turn_id=self._required_text(wait_call, "turnId"),
            call_id=call_id,
            name=str(payload.get("name") or "wait"),
            response=self._dict_result(payload.get("result")),
            status="error" if payload.get("isError") else "success",
            finished_at=finished_at,
            duration_ms=finished_at - int(wait_call["startedAt"]),
        )
        state.wait_calls.pop(call_id)
        yield self._event_response(finish_event)
        yield self._wait_completion(payload)

    async def _handle_runtime_tool_progress(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        call_id = self._required_text(payload, "callId")
        wait_call = state.wait_calls.get(call_id)
        if not isinstance(wait_call, dict):
            raise AgentInternalExc("Pi runtime returned invalid tool progress")
        yield self._event_response(
            self._event_adapter.tool_progressed(
                turn_id=self._required_text(wait_call, "turnId"),
                call_id=call_id,
                value=payload.get("result"),
            )
        )

    async def _handle_runtime_error(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        runtime_error = RuntimeError(
            f"Pi runtime error: {payload.get('message') or 'unknown'}"
        )
        raise AgentInternalExc("Pi agent runtime failed") from runtime_error
        yield

    async def _handle_runtime_done(
        self,
        payload: dict[str, Any],
        state: _PiRunState,
        plugin_by_runtime_name: dict[str, BasePlugin],
        websocket: aiohttp.ClientWebSocketResponse,
        span: Span,
    ) -> AsyncIterator[AgentResponse]:
        finished_at = self._now_ms()
        yield self._event_response(
            self._event_adapter.execution_finished(
                status="success", finished_at=finished_at
            )
        )
        state.completed = True

    def _runtime_event_handlers(self) -> dict[str, Any]:
        return {
            "agent_event": self._handle_runtime_agent_event,
            "reasoning_delta": self._handle_runtime_reasoning_delta,
            "content_delta": self._handle_runtime_content_delta,
            "usage": self._handle_runtime_usage,
            "tool_call": self._handle_runtime_tool_call,
            "tool_completed": self._handle_runtime_tool_completed,
            "tool_progress": self._handle_runtime_tool_progress,
            "error": self._handle_runtime_error,
            "done": self._handle_runtime_done,
        }

    def _failed_run_responses(
        self,
        wait_calls: dict[str, Any],
        *,
        wait_message: str,
        code: str,
        message: str,
        span: Span,
    ) -> list[AgentResponse]:
        responses = self._finish_pending_wait_calls(
            wait_calls,
            status="error",
            message=wait_message,
            span=span,
        )
        failed_at = self._now_ms()
        responses.extend(
            [
                self._event_response(
                    self._event_adapter.execution_failed(
                        code=code,
                        message=message,
                        occurred_at=failed_at,
                    )
                ),
                self._event_response(
                    self._event_adapter.execution_finished(
                        status="error", finished_at=failed_at
                    )
                ),
            ]
        )
        return responses

    def _cancelled_run_responses(
        self,
        wait_calls: dict[str, Any],
        *,
        span: Span,
    ) -> list[AgentResponse]:
        responses = self._finish_pending_wait_calls(
            wait_calls,
            status="cancelled",
            message="Tool execution cancelled",
            span=span,
        )
        cancelled_at = self._now_ms()
        responses.append(
            self._event_response(
                self._event_adapter.execution_finished(
                    status="cancelled", finished_at=cancelled_at
                )
            )
        )
        return responses

    @staticmethod
    def _failed_run_details(error: Exception) -> tuple[str, str, str]:
        if isinstance(error, AgentExc):
            return (
                "Pi runtime stopped before the wait completed",
                "PI_RUNTIME_ERROR",
                "Pi agent runtime failed",
            )
        if isinstance(error, (aiohttp.ClientError, OSError)):
            return (
                "Pi runtime disconnected before the wait completed",
                "PI_RUNTIME_UNAVAILABLE",
                "Pi agent runtime unavailable",
            )
        return (
            "Pi runtime stopped before the wait completed",
            "PI_RUNTIME_ERROR",
            "Pi agent runtime failed",
        )

    async def _stream_runtime_responses(
        self,
        start_message: dict[str, Any],
        plugin_by_runtime_name: dict[str, BasePlugin],
        state: _PiRunState,
        timeout: aiohttp.ClientTimeout,
        span: Span,
    ) -> AsyncGenerator[AgentResponse, None]:
        async with aiohttp.ClientSession(timeout=timeout) as session:
            async with session.ws_connect(
                self.runtime_url,
                headers={"Authorization": f"Bearer {self.internal_secret}"},
                heartbeat=30,
            ) as websocket:
                await websocket.send_json(start_message)
                async for message in websocket:
                    payload = self._decode_runtime_payload(message)
                    if payload is None:
                        continue
                    handler = self._runtime_event_handlers().get(
                        str(payload.get("type"))
                    )
                    if handler is None:
                        runtime_error = ValueError(
                            "Pi runtime returned unknown event: "
                            f"{payload.get('type')}"
                        )
                        raise AgentInternalExc("Pi agent runtime failed") from (
                            runtime_error
                        )
                    async for response in handler(
                        payload,
                        state,
                        plugin_by_runtime_name,
                        websocket,
                        span,
                    ):
                        yield response
                    if state.completed:
                        return

    async def run(
        self, span: Span, node_trace_log: NodeTraceLog
    ) -> AsyncIterator[AgentResponse]:
        del node_trace_log  # Public trace conversion consumes the emitted CotStep.
        if not self.internal_secret:
            raise AgentInternalExc("PI_AGENT_INTERNAL_SECRET is required")

        start_message, plugin_by_runtime_name = self._start_message()
        self._event_adapter = PiEventAdapter(
            run_id=self.run_id,
            started_at=self._now_ms(),
        )
        yield self._event_response(self._event_adapter.execution_started())
        timeout = aiohttp.ClientTimeout(total=None, connect=10, sock_read=None)
        state = _PiRunState()
        try:
            async with aclosing(
                self._stream_runtime_responses(
                    start_message,
                    plugin_by_runtime_name,
                    state,
                    timeout,
                    span,
                )
            ) as runtime_responses:
                async for response in runtime_responses:
                    yield response
        except (asyncio.CancelledError, Exception) as error:
            if isinstance(error, asyncio.CancelledError):
                responses = self._cancelled_run_responses(state.wait_calls, span=span)
                for response in responses:
                    yield response
                raise
            wait_message, code, message = self._failed_run_details(error)
            responses = self._failed_run_responses(
                state.wait_calls,
                wait_message=wait_message,
                code=code,
                message=message,
                span=span,
            )
            for response in responses:
                yield response
            raise AgentInternalExc(message) from error

        if not state.completed:
            for response in self._failed_run_responses(
                state.wait_calls,
                wait_message="Pi runtime disconnected before the wait completed",
                code="PI_RUNTIME_DISCONNECTED",
                message="Pi agent runtime disconnected",
                span=span,
            ):
                yield response
            raise AgentInternalExc("Pi agent runtime disconnected")
