"""Tests for built-in table tools and precise internal MCP trust."""

import re

from plugin.link.alembic.default_tools import TABLE_TOOL_INSERT_STATEMENTS
from plugin.link.utils.security.access_interceptor import is_trusted_internal_mcp_url


def test_only_exact_tabletools_builtin_mcp_is_trusted(monkeypatch):
    monkeypatch.setenv("MCP_INTERNAL_ALLOWLIST", "core-aitools:18669")
    assert is_trusted_internal_mcp_url("http://core-aitools:18669/mcp/sse")
    assert not is_trusted_internal_mcp_url("http://core-aitools:18668/mcp/sse")
    assert not is_trusted_internal_mcp_url("http://mysql:3306/")
    assert not is_trusted_internal_mcp_url("ftp://core-aitools:18669/mcp/sse")


def test_default_seed_contains_four_table_tools():
    assert len(TABLE_TOOL_INSERT_STATEMENTS) == 4
    joined = "\n".join(TABLE_TOOL_INSERT_STATEMENTS)
    for name in ["表格数据提取", "可视化图表-柱形图", "可视化图表-饼图", "Excel表格生成"]:
        assert name in joined
    assert joined.count("http://core-aitools:18669") == 4
    assert joined.count("'V1.0'") == 4
    assert joined.count('"x-display":true') >= 4
    assert joined.count('"x-from":0') >= 4


def test_table_tool_ids_are_accepted_by_link_management_api():
    joined = "\n".join(TABLE_TOOL_INSERT_STATEMENTS)
    tool_ids = re.findall(r"'(tool@[^']+)'", joined)
    assert len(tool_ids) == 4
    assert all(re.fullmatch(r"tool@[0-9A-Za-z]+", tool_id) for tool_id in tool_ids)


def test_table_tool_openapi_describes_prompt_selectable_outputs():
    joined = "\n".join(TABLE_TOOL_INSERT_STATEMENTS)
    for field in [
        '"image_url"',
        '"image_url_md"',
        '"file_url"',
        '"file_url_md"',
        '"rows"',
        '"json"',
        '"markdown"',
    ]:
        assert field in joined
