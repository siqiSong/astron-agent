"""Regression test for raw JSON SQL execution in the table-tools migration."""

import importlib.util
from pathlib import Path


class _RecordingConnection:
    def __init__(self) -> None:
        self.statements: list[str] = []

    def exec_driver_sql(self, statement: str) -> None:
        self.statements.append(statement)


class _MigrationOp:
    def __init__(self, connection: _RecordingConnection) -> None:
        self.connection = connection

    def get_bind(self) -> _RecordingConnection:
        return self.connection

    def execute(self, _statement: str) -> None:
        raise AssertionError("JSON SQL must bypass SQLAlchemy text bind parsing")


def test_tabletools_migration_executes_json_as_raw_driver_sql() -> None:
    migration_path = (
        Path(__file__).parents[2]
        / "alembic/versions/2026_08_05_1200-a17c8e5d91f4_add_table_tools.py"
    )
    spec = importlib.util.spec_from_file_location("tabletools_migration", migration_path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    connection = _RecordingConnection()
    module.op = _MigrationOp(connection)
    module.upgrade()

    assert len(connection.statements) == 5
    assert all(":true" in statement or statement.startswith("UPDATE") for statement in connection.statements)
