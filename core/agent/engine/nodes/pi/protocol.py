import re
from collections.abc import Sequence
from typing import Any

from agent.api.schemas.llm_message import LLMMessage
from agent.service.plugin.base import BasePlugin

_RESERVED_RUNTIME_NAMES = {"wait": "wait"}


def normalize_tool_name(name: str) -> str:
    normalized = re.sub(r"[^A-Za-z0-9_]+", "_", name.strip()).strip("_") or "tool"
    if not re.match(r"^[A-Za-z_]", normalized):
        return f"tool_{normalized}"
    return normalized


def _allocation_base(name: str) -> str:
    normalized = normalize_tool_name(name)
    return _RESERVED_RUNTIME_NAMES.get(normalized.casefold(), normalized)


def _allocate_runtime_name(base_name: str, used_names: set[str]) -> str:
    if base_name not in used_names:
        used_names.add(base_name)
        return base_name

    suffix = 2
    while f"{base_name}__{suffix}" in used_names:
        suffix += 1
    runtime_name = f"{base_name}__{suffix}"
    used_names.add(runtime_name)
    return runtime_name


def build_tool_contracts(
    plugins: Sequence[BasePlugin],
) -> tuple[list[dict[str, Any]], dict[str, BasePlugin]]:
    used_names = set(_RESERVED_RUNTIME_NAMES.values())
    contracts: list[dict[str, Any]] = []
    plugin_by_runtime_name: dict[str, BasePlugin] = {}
    for plugin in plugins:
        runtime_name = _allocate_runtime_name(_allocation_base(plugin.name), used_names)
        contracts.append(
            {
                "name": plugin.name,
                "description": plugin.description,
                "parameters": plugin.parameters,
                "toolType": plugin.typ,
            }
        )
        plugin_by_runtime_name[runtime_name] = plugin
    return contracts, plugin_by_runtime_name


def build_system_prompt(instruct: str, knowledge: str) -> str:
    parts = [part for part in [instruct.strip()] if part]
    if knowledge.strip():
        parts.append(f"Reference context:\n{knowledge.strip()}")
    return "\n\n".join(parts)


def history_payload(chat_history: Sequence[LLMMessage]) -> list[dict[str, str]]:
    return [
        {"role": message.role, "content": message.content}
        for message in chat_history
        if message.role in {"user", "assistant"}
    ]
