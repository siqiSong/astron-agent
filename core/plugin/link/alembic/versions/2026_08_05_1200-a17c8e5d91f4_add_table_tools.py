"""add built-in table tools

Revision ID: a17c8e5d91f4
Revises: 5c4f1b5ab83d
"""

from typing import Sequence, Union

from alembic import op  # type: ignore[attr-defined]
from plugin.link.alembic.default_tools import TABLE_TOOL_INSERT_STATEMENTS

revision: str = "a17c8e5d91f4"
down_revision: Union[str, None] = "5c4f1b5ab83d"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    connection = op.get_bind()
    for statement in TABLE_TOOL_INSERT_STATEMENTS:
        connection.exec_driver_sql(statement)
    connection.exec_driver_sql(
        "UPDATE tools_schema SET version = 'V1.0' "
        "WHERE tool_id IN ('tool@tableextractv1','tool@tablebarv1','tool@tablepiev1','tool@excelgeneratev1') "
        "AND version = ''"
    )


def downgrade() -> None:
    op.get_bind().exec_driver_sql(
        "DELETE FROM tools_schema WHERE tool_id IN ('tool@tableextractv1','tool@tablebarv1','tool@tablepiev1','tool@excelgeneratev1')"
    )
