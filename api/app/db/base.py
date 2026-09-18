"""Alembic 迁移所需的元数据汇总入口。

在这里导入全部模型，保证 autogenerate 能发现所有表。
"""

from __future__ import annotations

from app.db.models import (  # noqa: F401
    Note,
    Photo,
    Place,
    Share,
    Trip,
    TripPlace,
    User,
    UserSession,
)
from app.db.session import Base

__all__ = ["Base"]

target_metadata = Base.metadata
